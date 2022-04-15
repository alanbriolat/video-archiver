package gui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"text/template"

	"go.uber.org/zap"

	"github.com/alanbriolat/video-archiver"
	"github.com/alanbriolat/video-archiver/database"
	"github.com/alanbriolat/video-archiver/generic"
)

type download struct {
	database.Download
	mu         sync.Mutex
	Collection *collection
	updated    chan<- *download
	progress   int
	Match      *video_archiver.Match
	Resolved   video_archiver.ResolvedSource
}

func newDownloadFromDB(dbDownload database.Download, collection *collection) *download {
	d := &download{Download: dbDownload, Collection: collection}
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
		d.Match = nil
		d.Resolved = nil
		d.State = database.DownloadStateNew
		d.Error = ""
		d.Provider = ""
		d.Name = ""
	})
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

func (d *download) doMatch(app Application, _ context.Context) error {
	log := d.getLogger(app)
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
		log.Debugf("matching using provider '%v'", provider)
		match, err = app.ProviderRegistry().MatchWith(provider, url)
	} else {
		// ... otherwise match against any provider
		log.Debug("matching with any provider")
		match, err = app.ProviderRegistry().Match(url)
	}
	if err == nil {
		log.Debugf("successfully matched with provider '%v'", match.ProviderName)
		d.updating(func() {
			d.Provider = match.ProviderName
			d.Match = match
			d.State = database.DownloadStateNew
			d.Error = ""
			d.Name = match.Source.String()
		})
	} else {
		log.Errorf("failed to match: %v", err)
		d.updating(func() {
			d.Provider = ""
			d.Match = nil
			d.State = database.DownloadStateError
			d.Error = err.Error()
			d.Name = ""
		})
	}
	return err
}

func (d *download) doRecon(app Application, ctx context.Context) error {
	log := d.getLogger(app)
	log.Debug("starting recon")
	resolved, err := d.Match.Source.Recon(ctx)
	if err == nil {
		log.Debug("recon complete")
		d.updating(func() {
			d.Resolved = resolved
			d.State = database.DownloadStateReady
			d.Error = ""
			d.Name = d.Resolved.String()
		})
	} else {
		log.Errorf("failed to recon: %v", err)
		d.updating(func() {
			d.Resolved = nil
			d.State = database.DownloadStateError
			d.Error = err.Error()
			d.Name = ""
		})
	}
	return err
}

func (d *download) doDownload(app Application, ctx context.Context) error {
	log := d.getLogger(app)
	log.Debug("starting download")

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
			log.Errorf("failed to create download: %v", err)
			return err
		} else if err = d.Resolved.Download(download); err != nil {
			log.Errorf("failed to download: %v", err)
			return err
		} else {
			log.Debug("download complete")
			return nil
		}
	}()
	if err == nil {
		d.updating(func() {
			d.State = database.DownloadStateComplete
			d.Error = ""
		})
	} else {
		d.updating(func() {
			d.State = database.DownloadStateError
			d.Error = err.Error()
		})
	}
	return err
}
