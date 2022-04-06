package gui

import (
	"fmt"
	"strings"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
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
	DOWNLOAD_COLUMN_NAME
	DOWNLOAD_COLUMN_TOOLTIP
)

type actionGroup []*glib.SimpleAction

func newActionGroup(actions ...*glib.SimpleAction) actionGroup {
	return actions
}

func (g actionGroup) setEnabled(enabled bool) {
	for _, a := range g {
		a.SetEnabled(enabled)
	}
}

type downloadManager struct {
	app *application

	collection *collection
	downloads  map[database.RowID]*download
	current    *download

	actionNew         *glib.SimpleAction
	actionPaste       *glib.SimpleAction
	actionEdit        *glib.SimpleAction
	actionDelete      *glib.SimpleAction
	actionStart       *glib.SimpleAction
	actionStop        *glib.SimpleAction
	collectionActions actionGroup
	downloadActions   actionGroup

	Store         *gtk.ListStore `glade:"store"`
	View          *gtk.TreeView  `glade:"tree"`
	selection     *gtk.TreeSelection
	PaneDownloads *gtk.Paned `glade:"pane"`
	PaneDetails   *gtk.Box   `glade:"details_pane"`

	dlgEdit *downloadEditDialog

	OnCurrentChanged func(*download)
}

func (m *downloadManager) onAppActivate(app *application) {
	m.app = app
	m.downloads = make(map[database.RowID]*download)

	m.actionNew = m.app.registerSimpleWindowAction("new_download", nil, m.onActionNew)
	m.actionPaste = m.app.registerSimpleWindowAction("paste_download", nil, m.onActionPaste)
	m.app.gtkApplication.SetAccelsForAction("win.paste_download", []string{"<Primary>V"})
	m.actionEdit = m.app.registerSimpleWindowAction("edit_download", nil, m.onActionEdit)
	m.actionDelete = m.app.registerSimpleWindowAction("delete_download", nil, m.onActionDelete)
	m.actionStart = m.app.registerSimpleWindowAction("start_download", nil, m.onActionStart)
	m.actionStop = m.app.registerSimpleWindowAction("stop_download", nil, m.onActionStop)

	// Actions that require a collection to be selected
	m.collectionActions = newActionGroup(m.actionPaste)
	m.collectionActions.setEnabled(false)
	// Actions that require a download to be selected
	m.downloadActions = newActionGroup(m.actionEdit, m.actionDelete, m.actionStart, m.actionStop)
	m.downloadActions.setEnabled(false)

	// Get additional GTK references
	m.selection = generic.Unwrap(m.View.GetSelection())
	m.dlgEdit = newDownloadEditDialog(m.app.Collections.Store)

	m.selection.SetMode(gtk.SELECTION_SINGLE)
	m.selection.Connect("changed", m.onViewSelectionChanged)
	m.PaneDownloads.SetVisible(false)
	m.PaneDetails.SetVisible(false)
}

func (m *downloadManager) mustRefresh() {
	m.downloads = make(map[database.RowID]*download)
	m.selection.UnselectAll()
	// Disable selection while we refresh the store, otherwise we get a load of spurious "changed" signals even though
	// nothing should be selected...
	m.selection.SetMode(gtk.SELECTION_NONE)
	m.Store.Clear()
	if m.collection == nil {
		return
	}
	for _, dbDownload := range generic.Unwrap(m.app.database.GetDownloadsByCollectionID(m.collection.ID)) {
		d := &download{Download: dbDownload}
		m.downloads[d.ID] = d
		generic.Unwrap_(d.addToStore(m.Store))
	}
	m.selection.SetMode(gtk.SELECTION_SINGLE)
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

func (m *downloadManager) onActionNew() {
	d := database.Download{}
	if m.collection != nil {
		d.CollectionID = m.collection.ID
	}
	defer m.dlgEdit.hide()
	for {
		if !m.dlgEdit.run(&d) {
			break
		}
		v := ValidationResult{}
		if d.URL == "" {
			v.AddError("url", "URL must not be empty")
		} else if err := validateURL(d.URL); err != nil {
			v.AddError("url", "Invalid URL: %v", err)
		}
		if d.CollectionID == database.NullRowID {
			v.AddError("collection", "Must select a collection")
		}
		if v.IsOk() {
			m.create(&d)
			break
		} else {
			m.dlgEdit.showError(strings.Join(v.GetAllErrors(), "\n"))
		}
	}
}

func (m *downloadManager) onActionEdit() {
	d := m.current.Download
	defer m.dlgEdit.hide()
	for {
		if !m.dlgEdit.run(&d) {
			break
		}
		v := ValidationResult{}
		if d.URL == "" {
			v.AddError("url", "URL must not be empty")
		} else if err := validateURL(d.URL); err != nil {
			v.AddError("url", "Invalid URL: %v", err)
		}
		if d.CollectionID != m.current.CollectionID {
			v.AddError("collection", "Cannot change collection when editing a download")
			d.CollectionID = m.current.CollectionID
		}
		if v.IsOk() {
			m.current.Download = d
			m.update(m.current)
			break
		} else {
			m.dlgEdit.showError(strings.Join(v.GetAllErrors(), "\n"))
		}
	}
}

func (m *downloadManager) onActionPaste() {
	// Get the clipboard text
	clipboard := generic.Unwrap(gtk.ClipboardGet(gdk.SELECTION_CLIPBOARD))
	text := generic.Unwrap(clipboard.WaitForText())
	// Attempt to add the URL as a download
	err := m.addDownloadURL(text)
	if err != nil {
		m.app.runErrorDialog("Could not add download: %v", err)
	}
}

func (m *downloadManager) onActionDelete() {
	if !m.app.runWarningDialog("Are you sure you want to delete the download \"%v\"?", m.current.URL) {
		return
	}
	generic.Unwrap_(m.app.database.DeleteDownload(m.current.ID))
	generic.Unwrap_(m.current.removeFromStore())
	m.selection.UnselectAll()
}

func (m *downloadManager) onActionStart() {

}

func (m *downloadManager) onActionStop() {

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

func (m *downloadManager) update(d *download) {
	generic.Unwrap_(m.app.database.UpdateDownload(&d.Download))
	generic.Unwrap_(d.updateView())
}

func (m *downloadManager) setCollection(c *collection) {
	if c == m.collection {
		return
	}
	m.collection = c
	enabled := m.collection != nil
	m.PaneDownloads.SetVisible(enabled)
	m.collectionActions.setEnabled(enabled)
	m.mustRefresh()
}

func (m *downloadManager) setCurrent(id database.RowID) {
	if m.current != nil && m.current.ID == id {
		return
	}
	m.current = m.downloads[id]
	m.downloadActions.setEnabled(true)
	if m.OnCurrentChanged != nil {
		m.OnCurrentChanged(m.current)
	}
}

func (m *downloadManager) unsetCurrent() {
	if m.current == nil {
		return
	}
	m.current = nil
	m.downloadActions.setEnabled(false)
	if m.OnCurrentChanged != nil {
		m.OnCurrentChanged(m.current)
	}
}

type download struct {
	database.Download
	progress int
	err      error
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
func (d *download) removeFromStore() error {
	if d.treeRef == nil {
		return fmt.Errorf("collection not in store")
	} else if model, err := d.treeRef.GetModel(); err != nil {
		return fmt.Errorf("failed to get view model: %w", err)
	} else if iter, err := model.ToTreeModel().GetIter(d.treeRef.GetPath()); err != nil {
		return fmt.Errorf("failed to get store iter: %w", err)
	} else {
		model.(*gtk.ListStore).Remove(iter)
		return nil
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
			[]int{
				DOWNLOAD_COLUMN_ID,
				DOWNLOAD_COLUMN_URL,
				DOWNLOAD_COLUMN_ADDED,
				DOWNLOAD_COLUMN_STATE,
				DOWNLOAD_COLUMN_PROGRESS,
				DOWNLOAD_COLUMN_NAME,
				DOWNLOAD_COLUMN_TOOLTIP,
			},
			[]interface{}{
				d.ID,
				d.URL,
				// TODO: get in current timezone
				d.Added.Format("2006-01-02 15:04:05"),
				d.State.String(),
				d.getDisplayProgress(),
				d.getDisplayName(),
				d.getDisplayTooltip(),
			},
		)
		if err != nil {
			return fmt.Errorf("failed to update store: %w", err)
		} else {
			return nil
		}
	}
}

func (d *download) getDisplayProgress() int {
	if d.State == database.DownloadStateComplete {
		return 100
	} else {
		return d.progress
	}
}

func (d *download) getDisplayName() string {
	return d.URL
}

func (d *download) getDisplayTooltip() string {
	if d.err != nil {
		return fmt.Sprintf("%v\n\n%v", d.URL, d.err)
	} else {
		return d.URL
	}
}

func (d *download) String() string {
	return fmt.Sprintf("download{ID: %v, URL: %v}", d.ID, d.URL)
}
