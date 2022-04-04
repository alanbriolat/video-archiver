package gui

import (
	"fmt"
	"log"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"

	"github.com/alanbriolat/video-archiver/database"
	"github.com/alanbriolat/video-archiver/generic"
	"github.com/alanbriolat/video-archiver/glade"
)

const (
	DOWNLOAD_COLUMN_ID = iota
	DOWNLOAD_COLUMN_URL
	DOWNLOAD_COLUMN_ADDED
	DOWNLOAD_COLUMN_STATE
	DOWNLOAD_COLUMN_PROGRESS
)

type downloadManager struct {
	app *application

	collection *collection
	downloads  map[database.RowID]*download
	current    *download

	actionPaste *glib.SimpleAction

	Store         *gtk.ListStore `glade:"list_store_downloads"`
	View          *gtk.TreeView  `glade:"tree_downloads"`
	selection     *gtk.TreeSelection
	PaneDownloads *gtk.Paned  `glade:"pane_downloads"`
	PaneDetails   *gtk.Box    `glade:"pane_download_details"`
	BtnNew        *gtk.Button `glade:"btn_new_download"`
	EntryNewURL   *gtk.Entry  `glade:"entry_new_download_url"`

	OnCurrentChanged func(*download)
}

func newDownloadManager(app *application, builder glade.Builder) *downloadManager {
	m := &downloadManager{
		app:       app,
		downloads: make(map[database.RowID]*download),
	}

	m.actionPaste = m.app.registerSimpleWindowAction("paste_download", nil, m.onPasteButtonClicked)
	m.app.gtkApplication.SetAccelsForAction("win.paste_download", []string{"<Primary>V"})
	m.actionPaste.SetEnabled(false)

	// Get widget references from the builder
	builder.MustBuild(m)
	m.selection = generic.Unwrap(m.View.GetSelection())

	m.selection.SetMode(gtk.SELECTION_SINGLE)
	m.selection.Connect("changed", m.onViewSelectionChanged)
	m.PaneDownloads.SetVisible(false)
	m.PaneDetails.SetVisible(false)
	m.BtnNew.Connect("clicked", m.onNewButtonClicked)

	return m
}

func (m *downloadManager) mustRefresh() {
	m.downloads = make(map[database.RowID]*download)
	m.unsetCurrent()
	m.Store.Clear()
	if m.collection == nil {
		return
	}
	for _, dbDownload := range generic.Unwrap(m.app.database.GetDownloadsByCollectionID(m.collection.ID)) {
		d := &download{Download: dbDownload}
		m.downloads[d.ID] = d
		generic.Unwrap_(d.addToStore(m.Store))
	}
}

func (m *downloadManager) onViewSelectionChanged(selection *gtk.TreeSelection) {
	model, iter, ok := selection.GetSelected()
	if ok {
		id := generic.Unwrap(generic.Unwrap(model.ToTreeModel().GetValue(iter, DOWNLOAD_COLUMN_ID)).GoValue()).(int64)
		m.setCurrent(id)
	} else {
		m.unsetCurrent()
	}
}

func (m *downloadManager) onNewButtonClicked() {
	text := generic.Unwrap(m.EntryNewURL.GetText())
	err := m.addDownloadURL(text)
	if err != nil {
		m.app.runErrorDialog("Could not add download: %v", err)
	} else {
		m.EntryNewURL.SetText("")
	}
}

func (m *downloadManager) onPasteButtonClicked() {
	log.Println("onPasteButtonClicked")
	// Get the clipboard text
	clipboard := generic.Unwrap(gtk.ClipboardGet(gdk.SELECTION_CLIPBOARD))
	text := generic.Unwrap(clipboard.WaitForText())
	// Attempt to add the URL as a download
	err := m.addDownloadURL(text)
	if err != nil {
		m.app.runErrorDialog("Could not add download: %v", err)
	}
}

func (m *downloadManager) addDownloadURL(text string) error {
	if text == "" {
		return fmt.Errorf("no URL provided")
	} else if m.collection == nil {
		return fmt.Errorf("no collection selected")
	} else if err := validateURL(text); err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	} else {
		d := &database.Download{CollectionID: m.collection.ID, URL: text}
		m.create(d)
		return nil
	}
}

func (m *downloadManager) create(dbDownload *database.Download) {
	generic.Unwrap_(m.app.database.InsertDownload(dbDownload))
	d := &download{Download: *dbDownload}
	m.downloads[d.ID] = d
	generic.Unwrap_(d.addToStore(m.Store))
}

func (m *downloadManager) setCollection(c *collection) {
	if c == m.collection {
		return
	}
	m.collection = c
	enabled := m.collection != nil
	m.PaneDownloads.SetVisible(enabled)
	m.actionPaste.SetEnabled(enabled)
	m.mustRefresh()
}

func (m *downloadManager) setCurrent(id database.RowID) {
	if m.current != nil && m.current.ID == id {
		return
	}
	m.current = m.downloads[id]
	if m.OnCurrentChanged != nil {
		m.OnCurrentChanged(m.current)
	}
}

func (m *downloadManager) unsetCurrent() {
	if m.current == nil {
		return
	}
	m.current = nil
	if m.OnCurrentChanged != nil {
		m.OnCurrentChanged(m.current)
	}
}

type download struct {
	database.Download
	progress int
	treeRef  *gtk.TreeRowReference
}

// TODO: remove duplication
func (d *download) addToStore(store *gtk.ListStore) error {
	if d.treeRef != nil {
		return fmt.Errorf("download already in store")
	} else if treePath, err := store.GetPath(store.Append()); err != nil {
		return err
	} else if treeRef, err := gtk.TreeRowReferenceNew(store.ToTreeModel(), treePath); err != nil {
		return err
	} else {
		d.treeRef = treeRef
		return d.updateView()
	}
}

// TODO: remove duplication
func (d *download) updateView() error {
	if d.treeRef == nil {
		return fmt.Errorf("download not in view")
	} else if model, err := d.treeRef.GetModel(); err != nil {
		return fmt.Errorf("failed to get view model: %w", err)
	} else if iter, err := model.ToTreeModel().GetIter(d.treeRef.GetPath()); err != nil {
		return fmt.Errorf("failed to get store iter: %w", err)
	} else {
		err := model.(*gtk.ListStore).Set(
			iter,
			[]int{DOWNLOAD_COLUMN_ID, DOWNLOAD_COLUMN_URL, DOWNLOAD_COLUMN_ADDED, DOWNLOAD_COLUMN_STATE, DOWNLOAD_COLUMN_PROGRESS},
			[]interface{}{d.ID, d.URL, d.Added.Format("2006-01-02 15:04:05"), d.State.String(), d.progress},
		)
		if err != nil {
			return fmt.Errorf("failed to update store: %w", err)
		} else {
			return nil
		}
	}
}

func (d *download) String() string {
	return fmt.Sprintf("download{ID: %v, URL: %v}", d.ID, d.URL)
}
