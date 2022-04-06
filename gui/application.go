package gui

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"go.uber.org/zap"

	"github.com/alanbriolat/video-archiver"
	"github.com/alanbriolat/video-archiver/database"
	"github.com/alanbriolat/video-archiver/generic"
)

const DefaultAppName = "video-archiver"
const DefaultAppID = "co.hexi.video-archiver"

type ApplicationBuilder interface {
	Build() (Application, error)
	WithContext(ctx context.Context) ApplicationBuilder
	WithDatabasePath(path string) ApplicationBuilder
	WithProviderRegistry(*video_archiver.ProviderRegistry) ApplicationBuilder
}

type applicationBuilder struct {
	appName          string
	appID            string
	ctx              context.Context
	configPath       string
	dbPath           string
	providerRegistry *video_archiver.ProviderRegistry
}

func NewApplicationBuilder(name, id string) ApplicationBuilder {
	return &applicationBuilder{
		appName: name,
		appID:   id,
	}
}

func (b *applicationBuilder) Build() (Application, error) {
	if b.appName == "" {
		return nil, fmt.Errorf("application name must be a non-empty string")
	}
	if b.appID == "" {
		return nil, fmt.Errorf("application ID must be a non-empty string")
	}
	app := &application{applicationBuilder: *b}
	if app.ctx == nil {
		app.ctx = context.Background()
	}
	if app.configPath == "" {
		app.configPath = filepath.Join(glib.GetUserConfigDir(), app.appName)
	}
	if app.dbPath == "" {
		app.dbPath = filepath.Join(app.configPath, "database.sqlite3")
	}
	if app.providerRegistry == nil {
		app.providerRegistry = &video_archiver.DefaultProviderRegistry
	}
	return app, nil
}

func (b *applicationBuilder) WithContext(ctx context.Context) ApplicationBuilder {
	b.ctx = ctx
	return b
}

func (b *applicationBuilder) WithDatabasePath(path string) ApplicationBuilder {
	b.dbPath = path
	return b
}

func (b *applicationBuilder) WithProviderRegistry(r *video_archiver.ProviderRegistry) ApplicationBuilder {
	b.providerRegistry = r
	return b
}

type Application interface {
	Init() error
	Run() int
	RunWithArgs([]string) int
	Quit()
	Close() error
}

// TODO: recursive glade field handling for collectionManager and downloadManager
type application struct {
	applicationBuilder
	cancel         context.CancelFunc
	log            *zap.Logger
	database       *database.Database
	Collections    collectionManager `glade:"collections_"`
	Downloads      downloadManager   `glade:"downloads_"`
	gtkApplication *gtk.Application
	Window         *gtk.ApplicationWindow `glade:"window_main"`
}

func (a *application) Init() error {
	var err error
	if a.cancel != nil {
		return fmt.Errorf("init already run")
	}

	a.ctx, a.cancel = context.WithCancel(a.ctx)
	a.log = video_archiver.Logger(a.ctx)

	if err = os.MkdirAll(a.configPath, 0750); err != nil {
		return fmt.Errorf("failed to create config path %v: %w", a.configPath, err)
	}
	if err = os.MkdirAll(filepath.Dir(a.dbPath), 0750); err != nil {
		return fmt.Errorf("failed to create database %v: %w", a.dbPath, err)
	}
	if a.database, err = database.NewDatabase(a.dbPath, a.log); err != nil {
		return fmt.Errorf("failed to create database %v: %w", a.dbPath, err)
	}
	if err = a.database.Migrate(); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	if a.gtkApplication, err = gtk.ApplicationNew(a.appID, glib.APPLICATION_FLAGS_NONE); err != nil {
		return fmt.Errorf("failed to create GTK application: %w", err)
	}
	a.gtkApplication.Connect("startup", a.onStartup)
	a.gtkApplication.Connect("activate", a.onActivate)
	a.gtkApplication.Connect("shutdown", a.onShutdown)

	return nil
}

func (a *application) Run() int {
	return a.RunWithArgs([]string{})
}

func (a *application) RunWithArgs(args []string) int {
	// Ensure the GTK application quits if the context is cancelled
	go func() {
		<-a.ctx.Done()
		glib.IdleAddPriority(glib.PRIORITY_HIGH, func() { a.gtkApplication.Quit() })
	}()
	return a.gtkApplication.Run(args)
}

func (a *application) Quit() {
	a.cancel()
}

func (a *application) Close() error {
	if a.cancel != nil {
		a.cancel()
		a.cancel = nil
	}
	if a.database != nil {
		a.database.Close()
		a.database = nil
	}
	return nil
}

func (a *application) onStartup() {
	a.log.Info("application startup")
}

func (a *application) onActivate() {
	a.log.Info("application activate")

	builder := generic.Unwrap(GladeRepository.GetBuilder("application.glade"))
	builder.MustBuild(a)
	a.Window.SetApplication(a.gtkApplication)

	a.Downloads.onAppActivate(a)
	a.Downloads.OnCurrentChanged = func(d *download) {
		a.log.Sugar().Infof("selected download changed: %v", d)
	}
	a.Collections.onAppActivate(a)
	a.Collections.OnCurrentChanged = func(c *collection) {
		a.log.Sugar().Infof("selected collection changed: %v", c)
		a.Downloads.setCollection(c)
	}

	a.Window.Show()
	a.Collections.mustRefresh()
}

func (a *application) onShutdown() {
	a.log.Info("application shutdown")
	// Ensure context is cancelled no matter how the application shutdown was triggered
	a.cancel()
}

func (a *application) registerSimpleWindowAction(name string, parameterType *glib.VariantType, f func()) *glib.SimpleAction {
	action := glib.SimpleActionNew(name, parameterType)
	action.Connect("activate", f)
	a.Window.AddAction(action)
	return action
}

// runWarningDialog will show a modal warning dialog with "OK" and "Cancel" buttons, returning true if "OK" was clicked.
func (a *application) runWarningDialog(format string, args ...interface{}) bool {
	dlg := gtk.MessageDialogNew(a.Window, gtk.DIALOG_MODAL, gtk.MESSAGE_WARNING, gtk.BUTTONS_OK_CANCEL, format, args...)
	defer dlg.Destroy()
	response := dlg.Run()
	return response == gtk.RESPONSE_OK
}

// runErrorDialog will show a modal error dialog with an "OK" button.
func (a *application) runErrorDialog(format string, args ...interface{}) {
	dlg := gtk.MessageDialogNew(a.Window, gtk.DIALOG_MODAL, gtk.MESSAGE_ERROR, gtk.BUTTONS_OK, format, args...)
	defer dlg.Destroy()
	_ = dlg.Run()
}
