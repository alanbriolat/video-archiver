package gui

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"text/template"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"

	"github.com/alanbriolat/video-archiver"
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
	ListView[*download]

	app Application

	collection *collection

	actionNew         *glib.SimpleAction
	actionPaste       *glib.SimpleAction
	actionEdit        *glib.SimpleAction
	actionDelete      *glib.SimpleAction
	actionStart       *glib.SimpleAction
	actionStop        *glib.SimpleAction
	collectionActions actionGroup
	downloadActions   actionGroup

	PaneDownloads *gtk.Paned `glade:"pane"`
	PaneDetails   *gtk.Box   `glade:"details_pane"`

	dlgEdit *downloadEditDialog

	OnCurrentChanged func(*download)
}

func (m *downloadManager) onAppActivate(app Application, c *collectionManager) {
	m.app = app
	m.ListView.idColumn = DOWNLOAD_COLUMN_ID
	m.ListView.onCurrentChanged = func(d *download) {
		enabled := d != nil
		m.downloadActions.setEnabled(enabled)
		if m.OnCurrentChanged != nil {
			m.OnCurrentChanged(d)
		}
	}
	m.ListView.onItemUpdated = func(d *download) {
		generic.Unwrap_(m.app.DB().UpdateDownload(&d.Download))
	}
	m.ListView.onRefresh = func() []*download {
		var items []*download
		if m.collection != nil {
			for _, dbDownload := range generic.Unwrap(m.app.DB().GetDownloadsByCollectionID(m.collection.ID)) {
				items = append(items, newDownloadFromDB(dbDownload))
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
			m.current.onUpdate()
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
	generic.Unwrap_(m.app.DB().DeleteDownload(m.current.ID))
	m.MustRemoveItem(m.current)
}

func (m *downloadManager) onActionStart() {
	m.app.Logger().Info("onActionStart")
	if m.current.cancel != nil {
		m.app.Logger().Info("doing nothing, task in progress")
	}
	if m.current.Match == nil {
		m.current.doMatch(m.app)
	} else if m.current.Resolved == nil {
		m.current.doRecon(m.app)
	} else if m.current.State == database.DownloadStateNew {
		m.current.doDownload(m.app, generic.Unwrap(m.createDownloadBuilder()))
	}
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
	generic.Unwrap_(m.app.DB().InsertDownload(dbDownload))
	d := newDownloadFromDB(*dbDownload)
	m.MustAddItem(d)
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

func (m *downloadManager) createDownloadBuilder() (video_archiver.DownloadBuilder, error) {
	if m.collection == nil {
		return nil, fmt.Errorf("no collection selected")
	} else if m.current == nil {
		return nil, fmt.Errorf("no download selected")
	} else if m.collection.ID != m.current.CollectionID {
		return nil, fmt.Errorf("selected download does not belong to selected collection")
	}
	prefix := strings.TrimRight(m.collection.Path, string(os.PathSeparator)) + string(os.PathSeparator)
	builder := video_archiver.NewDownloadBuilder().WithTargetPrefix(prefix)
	return builder, nil
}

type download struct {
	database.Download
	updated  chan<- *download
	mu       sync.Mutex
	progress int
	Match    *video_archiver.Match
	Resolved video_archiver.ResolvedSource
	cancel   context.CancelFunc
	Err      error
}

func newDownloadFromDB(dbDownload database.Download) *download {
	return &download{Download: dbDownload}
}

func (d *download) onUpdate() {
	if d.updated != nil {
		d.updated <- d
	}
}

func (d *download) Bind(itemUpdated chan<- *download) {
	d.updated = itemUpdated
}

func (d *download) GetID() database.RowID {
	return d.ID
}

func (d *download) GetDisplay() (columns []int, values []interface{}) {
	d.mu.Lock()
	defer d.mu.Unlock()
	columns = []int{
		DOWNLOAD_COLUMN_ID,
		DOWNLOAD_COLUMN_URL,
		DOWNLOAD_COLUMN_ADDED,
		DOWNLOAD_COLUMN_STATE,
		DOWNLOAD_COLUMN_PROGRESS,
		DOWNLOAD_COLUMN_NAME,
		DOWNLOAD_COLUMN_TOOLTIP,
	}
	values = []interface{}{
		d.ID,
		d.URL,
		// TODO: get in current timezone
		d.Added.Format("2006-01-02 15:04:05"),
		d.State.String(),
		d.getDisplayProgress(),
		d.getDisplayName(),
		d.getDisplayTooltip(),
	}
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
	if d.Resolved != nil {
		return d.Resolved.String()
	} else if d.Match != nil {
		return d.Match.Source.String()
	} else {
		return d.URL
	}
}

func (d *download) getDisplayTooltip() string {
	sb := &strings.Builder{}
	generic.Unwrap_(downloadTooltipTemplate.Execute(sb, d))
	return sb.String()
}

func (d *download) doMatch(app Application) {
	if d.Match, d.Err = app.ProviderRegistry().Match(d.URL); d.Err != nil {
		d.Err = fmt.Errorf("no match: %w", d.Err)
		d.State = database.DownloadStateError
	}
	log.Printf("match=%v err=%v", d.Match, d.Err)
	d.onUpdate()
}

func (d *download) doRecon(app Application) {
	logger := app.Logger().Sugar().With("id", d.ID, "url", d.URL)
	if d.cancel != nil {
		logger.Warn("skipping doRecon(), task already in progress")
		return
	} else if d.Match == nil {
		logger.Warn("skipping doRecon(), no provider match")
		return
	}
	logger.Debug("starting recon")
	var ctx context.Context
	ctx, d.cancel = context.WithCancel(app.Context())
	go func() {
		defer func() { d.cancel = nil }()
		if resolved, err := d.Match.Source.Recon(ctx); err != nil {
			logger.Errorf("failed to recon download: %v", err)
			d.Err = err
			d.State = database.DownloadStateError
		} else {
			logger.Debug("recon complete")
			d.Resolved = resolved
		}
		d.onUpdate()
	}()
}

func (d *download) doDownload(app Application, builder video_archiver.DownloadBuilder) {
	logger := app.Logger().Sugar().With("id", d.ID, "url", d.URL)
	if d.cancel != nil {
		logger.Warn("skipping download, task already in progress")
		return
	} else if d.Resolved == nil {
		logger.Warn("skipping download, recon not complete")
		return
	}
	logger.Debug("starting download")
	var ctx context.Context
	ctx, d.cancel = context.WithCancel(app.Context())
	builder = builder.WithContext(ctx).WithProgressCallback(func(downloaded int, expected int) {
		if expected == 0 {
			d.progress = 0
		} else {
			d.progress = (downloaded * 100) / expected
		}
		// TODO: rate-limit update frequency
		d.onUpdate()
	})
	d.State = database.DownloadStateDownloading
	go func() {
		defer func() { d.cancel = nil }()
		if download, err := builder.Build(); err != nil {
			logger.Errorf("failed to create download: %v", err)
			d.Err = err
			d.State = database.DownloadStateError
		} else if err := d.Resolved.Download(download); err != nil {
			logger.Errorf("failed to download: %v", err)
			d.Err = err
			d.State = database.DownloadStateError
		} else {
			logger.Debug("download complete")
			d.State = database.DownloadStateComplete
		}
		d.onUpdate()
	}()
}

func (d *download) String() string {
	return fmt.Sprintf("download{ID: %v, URL: %v}", d.ID, d.URL)
}

var downloadTooltipTemplate = template.Must(
	template.New("tooltip").Funcs(template.FuncMap{"trim": strings.TrimSpace}).Parse(strings.TrimSpace(`
{{if .Match}}[{{ .Match.ProviderName }}] {{end}}{{ .URL }}{{if .Err}}

{{ trim .Err.Error }}{{end}}
`)))
