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
	downloadColumnID = iota
	downloadColumnURL
	downloadColumnAdded
	downloadColumnState
	downloadColumnProgress
	downloadColumnName
	downloadColumnTooltip
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
	ListView[*download]

	app Application

	collection *collection
	active     *activeDownloadManager

	actionNew         *glib.SimpleAction
	actionPaste       *glib.SimpleAction
	actionEdit        *glib.SimpleAction
	actionDelete      *glib.SimpleAction
	actionStart       *glib.SimpleAction
	actionStop        *glib.SimpleAction
	actionRefresh     *glib.SimpleAction
	collectionActions actionGroup
	downloadActions   actionGroup

	PaneDownloads *gtk.Paned `glade:"pane"`
	PaneDetails   *gtk.Box   `glade:"details_pane"`

	dlgEdit *downloadEditDialog

	OnCurrentChanged func(*download)
}

func (m *downloadManager) onAppActivate(app Application, c *collectionManager) {
	m.app = app
	m.active = newActiveDownloadManager(app)
	m.ListView.idColumn = downloadColumnID
	m.ListView.onCurrentChanged = func(d *download) {
		enabled := d != nil
		m.downloadActions.setEnabled(enabled)
		if m.OnCurrentChanged != nil {
			m.OnCurrentChanged(d)
		}
	}
	m.ListView.onItemAdding = func(d *download) {
		if d.ID == database.NullRowID {
			generic.Unwrap_(m.app.DB().InsertDownload(&d.Download))
		}
	}
	m.ListView.onItemAdded = func(d *download) {
		if d.State == database.DownloadStateNew {
			m.active.StartDownload(d, downloadStageResolved)
		}
	}
	m.ListView.onItemUpdating = func(d *download) {
		generic.Unwrap_(m.app.DB().UpdateDownload(&d.Download))
	}
	m.ListView.onItemRemoving = func(d *download) {
		generic.Unwrap_(m.app.DB().DeleteDownload(m.current.ID))
	}
	m.ListView.onRefresh = func() []*download {
		var items []*download
		if m.collection != nil {
			for _, dbDownload := range generic.Unwrap(m.app.DB().GetDownloadsByCollectionID(m.collection.ID)) {
				d := m.active.GetDownload(newDownloadFromDB(dbDownload, m.collection))
				items = append(items, d)
			}
		}
		return items
	}
	m.InitListView()

	m.actionNew = m.app.RegisterSimpleWindowAction("new_download", nil, m.onActionNew)
	m.actionPaste = m.app.RegisterSimpleWindowAction("paste_download", nil, m.onActionPaste)
	m.app.SetWindowActionAccels("paste_download", []string{"<Primary>V"})
	m.actionEdit = m.app.RegisterSimpleWindowAction("edit_download", nil, m.onActionEdit)
	m.actionDelete = m.app.RegisterSimpleWindowAction("delete_download", nil, m.onActionDelete)
	// TODO: awareness of current download state
	m.actionStart = m.app.RegisterSimpleWindowAction("start_download", nil, m.onActionStart)
	m.actionStop = m.app.RegisterSimpleWindowAction("stop_download", nil, m.onActionStop)
	m.actionRefresh = m.app.RegisterSimpleWindowAction("refresh_download", nil, m.onActionRefresh)

	// Actions that require a collection to be selected
	m.collectionActions = newActionGroup(m.actionPaste)
	m.collectionActions.setEnabled(false)
	// Actions that require a download to be selected
	m.downloadActions = newActionGroup(m.actionEdit, m.actionDelete, m.actionStart, m.actionStop, m.actionRefresh)
	m.downloadActions.setEnabled(false)

	// Get additional GTK references
	m.selection = generic.Unwrap(m.View.GetSelection())
	m.dlgEdit = newDownloadEditDialog(c.Store)

	m.PaneDownloads.SetVisible(false)
	m.PaneDetails.SetVisible(false)
}

func (m *downloadManager) onAppShutdown() {
	m.ListView.StopItemUpdates()
	m.active.StopAll()
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
		} else if err := ValidateURL(d.URL); err != nil {
			v.AddError("url", "Invalid URL: %v", err)
		}
		if d.CollectionID == database.NullRowID {
			v.AddError("collection", "Must select a collection")
		}
		if v.IsOk() {
			m.MustAddItem(newDownloadFromDB(d, m.collection))
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
		} else if err := ValidateURL(d.URL); err != nil {
			v.AddError("url", "Invalid URL: %v", err)
		}
		if d.CollectionID != m.current.CollectionID {
			v.AddError("collection", "Cannot change collection when editing a download")
			d.CollectionID = m.current.CollectionID
		}
		if v.IsOk() {
			shouldReset := false
			m.current.updating(func() {
				if d.URL != m.current.URL {
					shouldReset = true
				}
				m.current.Download = d
			})
			if shouldReset {
				m.current.reset()
				m.active.StartDownload(m.current, downloadStageResolved)
			}
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
		m.app.RunErrorDialog("Could not add download: %v", err)
	}
}

func (m *downloadManager) onActionDelete() {
	if !m.app.RunWarningDialog("Are you sure you want to delete the download \"%v\"?", m.current.URL) {
		return
	}
	m.MustRemoveItem(m.current)
}

func (m *downloadManager) onActionStart() {
	m.app.Logger().Info("onActionStart")
	m.active.StartDownload(m.current, downloadStageDownloaded)
}

func (m *downloadManager) onActionStop() {
	m.app.Logger().Info("onActionStop")
	m.active.StopDownload(m.current, false)
}

func (m *downloadManager) onActionRefresh() {
	m.app.Logger().Info("onActionRefresh")
	if m.active.IsRunning(m.current) {
		// TODO: handle this in a better way, e.g. not enabling refresh during running, or stopping before refresh
		m.app.Logger().Warn("download running, not doing refresh")
		return
	}
	m.current.reset()
	m.active.StartDownload(m.current, downloadStageResolved)
}

func (m *downloadManager) addDownloadURL(text string) error {
	if text == "" {
		return fmt.Errorf("no URL provided")
	} else if m.collection == nil {
		return fmt.Errorf("no collection selected")
	} else if err := ValidateURL(text); err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	} else {
		m.MustAddItem(newDownloadFromDB(database.Download{CollectionID: m.collection.ID, URL: text}, m.collection))
		return nil
	}
}

func (m *downloadManager) setCollection(c *collection) {
	if c == m.collection {
		return
	}
	m.collection = c
	enabled := m.collection != nil
	m.PaneDownloads.SetVisible(enabled)
	m.collectionActions.setEnabled(enabled)
	m.MustRefresh()
}
