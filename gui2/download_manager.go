package gui2

import (
	"html"
	"strings"
	"text/template"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"

	"github.com/alanbriolat/video-archiver/generic"
	"github.com/alanbriolat/video-archiver/gui"
	"github.com/alanbriolat/video-archiver/internal/pubsub"
	"github.com/alanbriolat/video-archiver/internal/session"
)

const (
	downloadColumnID = iota
	downloadColumnURL
	downloadColumnSavePath
	downloadColumnAdded
	downloadColumnStatus
	downloadColumnProgress
	downloadColumnName
	downloadColumnTooltip
)

type downloadManager struct {
	app    Application
	events pubsub.ReceiverCloser[session.Event]

	Store     *gtk.ListStore `glade:"store"`
	View      *gtk.TreeView  `glade:"tree"`
	items     map[session.DownloadID]*session.Download
	treeRefs  map[session.DownloadID]*gtk.TreeRowReference
	selection *gtk.TreeSelection

	actionNew    *glib.SimpleAction
	actionRemove *glib.SimpleAction
	actionStart  *glib.SimpleAction
	actionStop   *glib.SimpleAction

	dlgNew *downloadNewDialog
}

func (m *downloadManager) onAppActivate(app Application) {
	m.app = app

	m.items = make(map[session.DownloadID]*session.Download)
	m.treeRefs = make(map[session.DownloadID]*gtk.TreeRowReference)
	m.selection = generic.Unwrap(m.View.GetSelection())

	m.actionNew = m.app.RegisterSimpleWindowAction("new_download", nil, m.onActionNew)
	m.app.SetWindowActionAccels("new_download", []string{"<Primary>N"})
	m.actionRemove = m.app.RegisterSimpleWindowAction("remove_download", nil, m.onActionRemove)
	m.app.SetWindowActionAccels("remove_download", []string{"Delete"})
	m.actionStart = m.app.RegisterSimpleWindowAction("start_download", nil, m.onActionStart)
	m.actionStop = m.app.RegisterSimpleWindowAction("stop_download", nil, m.onActionStop)

	m.dlgNew = newDownloadNewDialog()

	m.events = generic.Unwrap(m.app.Session().Subscribe())
	go func() {
		for event := range m.events.Receive() {
			// Important: must make sure the loop variables aren't captured in the closure passed to IdleAdd()
			e := event
			// TODO: do more work outside of the GTK main loop?
			glib.IdleAdd(func() { m.onSessionEvent(e) })
		}
	}()

	m.mustRefresh()
}

func (m *downloadManager) onAppShutdown() {
	m.events.Close()
}

func (m *downloadManager) onSessionEvent(event session.Event) {
	logger := m.app.Logger().Sugar()
	logger.Debugf("event %T: %v", event, event.Download())
	switch e := event.(type) {
	case session.DownloadAdded:
		m.mustUpdateItem(e.Download(), nil)
	case session.DownloadRemoved:
		m.mustRemoveItem(e.Download())
	case session.DownloadUpdated:
		m.mustUpdateItem(e.Download(), &e.NewState)
	default:
	}
}

func (m *downloadManager) onActionNew() {
	defer m.dlgNew.hide()
	for {
		if !m.dlgNew.run() {
			break
		}
		v := gui.ValidationResult{}
		if m.dlgNew.URL == "" {
			v.AddError("url", "URL must not be empty")
		} else if err := gui.ValidateURL(m.dlgNew.URL); err != nil {
			v.AddError("url", "Invalid URL: %v", err)
		}
		if v.IsOk() {
			options := session.AddDownloadOptions{SavePath: m.dlgNew.SavePath}
			_, err := m.app.Session().AddDownload(m.dlgNew.URL, &options)
			if err != nil {
				m.dlgNew.showError(err.Error())
			} else {
				break
			}
		} else {
			m.dlgNew.showError(strings.Join(v.GetAllErrors(), "\n"))
		}
	}
}

func (m *downloadManager) onActionRemove() {
	m.forEachSelectedAsync(
		func(downloads []*session.Download) bool {
			return m.app.RunWarningDialog("Are you sure you want to delete %d download(s)?", len(downloads))
		},
		func(d *session.Download) {
			generic.Unwrap_(m.app.Session().RemoveDownload(d.ID()))
		},
	)
}

func (m *downloadManager) onActionStart() {
	m.forEachSelectedAsync(nil, func(d *session.Download) {
		if !d.IsComplete() {
			d.Start()
		}
	})
}

func (m *downloadManager) onActionStop() {
	m.forEachSelectedAsync(nil, func(d *session.Download) {
		d.Stop()
	})
}

func (m *downloadManager) mustRefresh() {
	for _, d := range m.app.Session().ListDownloads() {
		m.mustUpdateItem(d, nil)
	}
}

func (m *downloadManager) mustUpdateItem(d *session.Download, ds *session.DownloadState) {
	id := d.ID()
	if ds == nil {
		state := generic.Unwrap(d.State())
		ds = &state
	}
	var iter *gtk.TreeIter
	if treeRef, ok := m.treeRefs[id]; !ok {
		// If this is a new download, add it
		iter = m.Store.Append()
		treePath := generic.Unwrap(m.Store.GetPath(iter))
		treeRef = generic.Unwrap(gtk.TreeRowReferenceNew(m.Store.ToTreeModel(), treePath))
		m.items[id] = d
		m.treeRefs[id] = treeRef
	} else {
		// Otherwise just get the iter for the row in the TreeView
		iter = generic.Unwrap(m.Store.GetIter(treeRef.GetPath()))
	}
	columns := []int{
		downloadColumnID,
		downloadColumnURL,
		downloadColumnSavePath,
		downloadColumnAdded,
		downloadColumnStatus,
		downloadColumnProgress,
		downloadColumnName,
		downloadColumnTooltip,
	}
	values := []interface{}{
		string(ds.ID),
		ds.URL,
		ds.SavePath,
		ds.AddedAt.Local().Format("2006-01-02 15:04:05"),
		string(ds.Status),
		getDownloadStateDisplayProgress(ds),
		getDownloadStateDisplayName(ds),
		html.EscapeString(getDownloadStateDisplayTooltip(ds)),
	}
	generic.Unwrap_(m.Store.Set(iter, columns, values))
}

func (m *downloadManager) mustRemoveItem(d *session.Download) {
	id := d.ID()
	if treeRef, ok := m.treeRefs[id]; ok {
		iter := generic.Unwrap(m.Store.GetIter(treeRef.GetPath()))
		m.selection.UnselectIter(iter)
		m.selectionDisabled(func() {
			m.Store.Remove(iter)
		})
		delete(m.items, id)
		delete(m.treeRefs, id)
	} else {
		m.app.Logger().Sugar().Warnf("attempted to remove unknown download %v", id)
	}
}

func (m *downloadManager) getSelectedDownloads() (downloads []*session.Download) {
	// TODO: support multiple selection instead
	if model, iter, ok := m.selection.GetSelected(); ok {
		id := generic.Unwrap(generic.Unwrap(model.ToTreeModel().GetValue(iter, downloadColumnID)).GoValue()).(string)
		downloads = append(downloads, m.items[session.DownloadID(id)])
	}
	return downloads
}

func (m *downloadManager) forEachSelectedAsync(confirm func(downloads []*session.Download) bool, act func(*session.Download)) {
	downloads := m.getSelectedDownloads()
	if confirm == nil || confirm(downloads) {
		go func() {
			for _, d := range downloads {
				act(d)
			}
		}()
	}
}

func (m *downloadManager) selectionDisabled(f func()) {
	mode := m.selection.GetMode()
	m.selection.SetMode(gtk.SELECTION_NONE)
	defer m.selection.SetMode(mode)
	f()
}

func getDownloadStateDisplayProgress(ds *session.DownloadState) int {
	if ds.Status == session.DownloadStatusComplete {
		return 100
	} else {
		return ds.Progress
	}
}

func getDownloadStateDisplayName(ds *session.DownloadState) string {
	if ds.Name != "" {
		return ds.Name
	} else {
		return ds.URL
	}
}

func getDownloadStateDisplayTooltip(ds *session.DownloadState) string {
	sb := &strings.Builder{}
	generic.Unwrap_(downloadTooltipTemplate.Execute(sb, ds))
	return sb.String()
}

var downloadTooltipTemplate = template.Must(
	template.New("tooltip").Funcs(template.FuncMap{"trim": strings.TrimSpace}).Parse(strings.TrimSpace(`
{{if .Provider}}[{{ .Provider }}] {{end}}{{ .URL }}{{if .Error}}

{{ trim .Error }}{{end}}
`)))
