package gui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"text/template"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"go.uber.org/zap"

	"github.com/alanbriolat/video-archiver"
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

type downloadStage uint8

const (
	downloadStageNew        downloadStage = 0
	downloadStageMatched    downloadStage = 1
	downloadStageResolved   downloadStage = 2
	downloadStageDownloaded downloadStage = 3
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
			d.run(m.app, downloadStageResolved)
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
				items = append(items, newDownloadFromDB(dbDownload, m.collection))
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
	m.downloadActions = newActionGroup(m.actionEdit, m.actionDelete, m.actionStart, m.actionStop)
	m.downloadActions.setEnabled(false)

	// Get additional GTK references
	m.selection = generic.Unwrap(m.View.GetSelection())
	m.dlgEdit = newDownloadEditDialog(c.Store)

	m.PaneDownloads.SetVisible(false)
	m.PaneDetails.SetVisible(false)
}

func (m *downloadManager) onAppShutdown() {
	m.ListView.StopItemUpdates()
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
		} else if err := validateURL(d.URL); err != nil {
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
				m.current.run(m.app, downloadStageResolved)
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
	m.current.run(m.app, downloadStageDownloaded)
}

func (m *downloadManager) onActionStop() {
	m.app.Logger().Info("onActionStop")
	m.current.stop()
}

func (m *downloadManager) onActionRefresh() {
	m.app.Logger().Info("onActionRefresh")
	if m.current.isRunning() {
		// TODO: handle this in a better way, e.g. not enabling refresh during running, or stopping before refresh
		m.app.Logger().Warn("download running, not doing refresh")
		return
	}
	m.current.reset()
	m.current.run(m.app, downloadStageResolved)
}

func (m *downloadManager) addDownloadURL(text string) error {
	if text == "" {
		return fmt.Errorf("no URL provided")
	} else if m.collection == nil {
		return fmt.Errorf("no collection selected")
	} else if err := validateURL(text); err != nil {
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

type download struct {
	database.Download
	Collection   *collection
	updated      chan<- *download
	mu           sync.Mutex
	progress     int
	Match        *video_archiver.Match
	Resolved     video_archiver.ResolvedSource
	currentStage downloadStage
	targetStage  downloadStage
	cancel       context.CancelFunc
}

func newDownloadFromDB(dbDownload database.Download, collection *collection) *download {
	d := &download{Download: dbDownload, Collection: collection}
	if d.State == database.DownloadStateComplete {
		d.currentStage = downloadStageDownloaded
	}
	return d
}

func (d *download) locked(f func()) {
	d.mu.Lock()
	defer d.mu.Unlock()
	f()
}

func (d *download) updating(f func()) {
	d.locked(f)
	if d.updated != nil {
		d.updated <- d
	}
}

func (d *download) reset() {
	d.updating(func() {
		d.currentStage = downloadStageNew
		d.targetStage = d.currentStage
		d.Match = nil
		d.Resolved = nil
		d.State = database.DownloadStateNew
		d.Error = ""
		d.Provider = ""
		d.Name = ""
	})
}

func (d *download) isRunning() bool {
	return d.cancel != nil
}

func (d *download) run(app Application, targetStage downloadStage) {
	logger := d.getLogger(app)
	d.locked(func() {
		logger.Debugf("run(): current=%v target=%v newTarget=%v", d.currentStage, d.targetStage, targetStage)
		if targetStage <= d.currentStage {
			// Already reached this stage, so ignore the request
			logger.Debug("ignoring new target stage, already reached")
			return
		} else if targetStage <= d.targetStage {
			// Already going to reach this stage, so ignore the request
			logger.Debug("ignoring new target stage, already targeted")
			return
		}
		logger.Debugf("updating target stage to %v", targetStage)
		d.targetStage = targetStage
		if d.isRunning() {
			logger.Debug("download already running, not spawning goroutine")
			return
		} else {
			logger.Debug("download not running, will spawn goroutine")
			var ctx context.Context
			ctx, d.cancel = context.WithCancel(app.Context())
			go func() {
				defer func() { d.cancel = nil }()
				logger.Debug("download goroutine started")
				d.asyncRun(app, ctx)
				// Once we stop running, set target stage to the latest stage reached, so that a repeat request for the old
				// target stage (e.g. retrying a download) will trigger the goroutine again.
				d.locked(func() {
					if d.currentStage != d.targetStage {
						logger.Warnf("didn't reach target stage %v, changing to match current stage %v", d.targetStage, d.currentStage)
						d.targetStage = d.currentStage
					}
				})
				logger.Debug("download goroutine ended")
			}()
		}
	})
}

func (d *download) shouldRunStage(stage downloadStage) (run bool) {
	d.locked(func() {
		run = d.targetStage >= stage && d.currentStage == stage-1
	})
	return run
}

func (d *download) asyncRun(app Application, ctx context.Context) {
	logger := d.getLogger(app)
	if d.shouldRunStage(downloadStageMatched) {
		var provider string
		var url string
		d.locked(func() {
			provider = d.Provider
			url = d.URL
		})
		var match *video_archiver.Match
		var err error
		if provider != "" {
			// If we matched with a specific provider previously, use specifically that provider this time
			logger.Debugf("matching using provider '%v'", provider)
			match, err = app.ProviderRegistry().MatchWith(provider, url)
		} else {
			// ... otherwise match against any provider
			logger.Debug("matching with any provider")
			match, err = app.ProviderRegistry().Match(url)
		}
		if err == nil {
			logger.Debugf("successfully matched with provider '%v'", match.ProviderName)
			d.updating(func() {
				d.Provider = match.ProviderName
				d.Match = match
				d.State = database.DownloadStateNew
				d.Error = ""
				d.Name = match.Source.String()
				d.currentStage = downloadStageMatched
			})
		} else {
			logger.Errorf("failed to match: %v", err)
			d.updating(func() {
				d.Provider = ""
				d.Match = nil
				d.State = database.DownloadStateError
				d.Error = err.Error()
				d.Name = ""
			})
			// Hit an error, so can't continue any further
			return
		}
	}

	if d.shouldRunStage(downloadStageResolved) {
		logger.Debug("starting recon")
		resolved, err := d.Match.Source.Recon(ctx)
		if err == nil {
			logger.Debug("recon complete")
			d.updating(func() {
				d.Resolved = resolved
				d.State = database.DownloadStateReady
				d.Error = ""
				d.Name = d.Resolved.String()
				d.currentStage = downloadStageResolved
			})
		} else {
			logger.Errorf("failed to recon: %v", err)
			d.updating(func() {
				d.Resolved = nil
				d.State = database.DownloadStateError
				d.Error = err.Error()
				d.Name = ""
			})
			// Hit an error, so can't continue any further
			return
		}
	}

	if d.shouldRunStage(downloadStageDownloaded) {
		logger.Debug("starting download")
		prefix := strings.TrimRight(d.Collection.Path, string(os.PathSeparator)) + string(os.PathSeparator)
		builder := video_archiver.NewDownloadBuilder().WithTargetPrefix(prefix).WithContext(ctx).WithProgressCallback(func(downloaded int, expected int) {
			var progress int
			if expected == 0 {
				progress = 0
			} else {
				progress = (downloaded * 100) / expected
			}
			if progress != d.progress {
				// TODO: rate-limit update frequency
				d.updating(func() {
					d.progress = progress
				})
			}
		})
		d.updating(func() {
			d.State = database.DownloadStateDownloading
		})
		err := func() error {
			if download, err := builder.Build(); err != nil {
				logger.Errorf("failed to create download: %v", err)
				return err
			} else if err = d.Resolved.Download(download); err != nil {
				logger.Errorf("failed to download: %v", err)
				return err
			} else {
				logger.Debug("download complete")
				return nil
			}
		}()
		if err == nil {
			d.updating(func() {
				d.State = database.DownloadStateComplete
				d.Error = ""
				d.currentStage = downloadStageDownloaded
			})
		} else {
			d.updating(func() {
				d.State = database.DownloadStateError
				d.Error = err.Error()
			})
			// Hit an error, so can't continue any further
			return
		}
	}
}

func (d *download) stop() {
	if d.cancel != nil {
		d.cancel()
	}
}

func (d *download) Bind(itemUpdated chan<- *download) {
	d.updated = itemUpdated
}

func (d *download) GetID() database.RowID {
	return d.ID
}

func (d *download) GetDisplay() (columns []int, values []interface{}) {
	d.locked(func() {
		columns = []int{
			downloadColumnID,
			downloadColumnURL,
			downloadColumnAdded,
			downloadColumnState,
			downloadColumnProgress,
			downloadColumnName,
			downloadColumnTooltip,
		}
		values = []interface{}{
			d.ID,
			d.URL,
			d.Added.Local().Format("2006-01-02 15:04:05"),
			d.State.String(),
			d.getDisplayProgress(),
			d.getDisplayName(),
			d.getDisplayTooltip(),
		}
	})
	return columns, values
}

func (d *download) getDisplayProgress() int {
	if d.State == database.DownloadStateComplete {
		return 100
	} else {
		return d.progress
	}
}

func (d *download) getDisplayName() string {
	if d.Name != "" {
		return d.Name
	} else {
		return d.URL
	}
}

func (d *download) getDisplayTooltip() string {
	sb := &strings.Builder{}
	generic.Unwrap_(downloadTooltipTemplate.Execute(sb, d))
	return sb.String()
}

func (d *download) String() string {
	return fmt.Sprintf("download{ID: %v, URL: %v}", d.ID, d.URL)
}

func (d *download) getLogger(app Application) *zap.SugaredLogger {
	return app.Logger().Named("download").With(zap.Int("id", int(d.ID)), zap.String("url", d.URL)).Sugar()
}

var downloadTooltipTemplate = template.Must(
	template.New("tooltip").Funcs(template.FuncMap{"trim": strings.TrimSpace}).Parse(strings.TrimSpace(`
{{if .Provider}}[{{ .Provider }}] {{end}}{{ .URL }}{{if .Error}}

{{ html (trim .Error) }}{{end}}
`)))
