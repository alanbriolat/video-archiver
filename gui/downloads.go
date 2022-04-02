package gui

import (
	"fmt"

	"github.com/gotk3/gotk3/gtk"

	"github.com/alanbriolat/video-archiver/database"
	"github.com/alanbriolat/video-archiver/generic"
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

	store         *gtk.ListStore
	view          *gtk.TreeView
	selection     *gtk.TreeSelection
	paneDownloads *gtk.Paned
	paneDetails   *gtk.Box
	btnNew        *gtk.Button
	entryNewURL   *gtk.Entry

	OnCurrentChanged func(*download)
}

func newDownloadManager(app *application, builder *gtk.Builder) *downloadManager {
	m := &downloadManager{
		app:       app,
		downloads: make(map[database.RowID]*download),
	}

	// Get widget references from the builder
	MustReadObject(&m.store, builder, "list_store_downloads")
	MustReadObject(&m.view, builder, "tree_downloads")
	m.selection = generic.Unwrap(m.view.GetSelection())
	MustReadObject(&m.paneDownloads, builder, "pane_downloads")
	MustReadObject(&m.paneDetails, builder, "pane_download_details")
	MustReadObject(&m.btnNew, builder, "btn_new_download")
	MustReadObject(&m.entryNewURL, builder, "entry_new_download_url")

	m.selection.SetMode(gtk.SELECTION_SINGLE)
	m.selection.Connect("changed", m.onViewSelectionChanged)
	m.paneDownloads.SetVisible(false)
	m.paneDetails.SetVisible(false)
	m.btnNew.Connect("clicked", m.onNewButtonClicked)

	return m
}

func (m *downloadManager) mustRefresh() {
	m.downloads = make(map[database.RowID]*download)
	m.unsetCurrent()
	m.store.Clear()
	if m.collection == nil {
		return
	}
	for _, dbDownload := range generic.Unwrap(m.app.database.GetDownloadsByCollectionID(m.collection.ID)) {
		d := &download{Download: dbDownload}
		m.downloads[d.ID] = d
		generic.Unwrap_(d.addToStore(m.store))
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
	url := generic.Unwrap(m.entryNewURL.GetText())
	d := &database.Download{CollectionID: m.collection.ID, URL: url}
	m.create(d)
	m.entryNewURL.SetText("")
}

func (m *downloadManager) create(dbDownload *database.Download) {
	generic.Unwrap_(m.app.database.InsertDownload(dbDownload))
	d := &download{Download: *dbDownload}
	m.downloads[d.ID] = d
	generic.Unwrap_(d.addToStore(m.store))
}

func (m *downloadManager) setCollection(c *collection) {
	if c == m.collection {
		return
	}
	m.collection = c
	m.paneDownloads.SetVisible(m.collection != nil)
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
