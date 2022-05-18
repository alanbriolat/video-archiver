package gui

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v3"
	"go.uber.org/zap"

	"github.com/alanbriolat/video-archiver/generic"
	"github.com/alanbriolat/video-archiver/internal/session"
)

const DefaultAppName = "video-archiver"
const DefaultAppID = "co.hexi.video-archiver"

type Application interface {
	Env

	Run(args ...string) int

	RegisterSimpleWindowAction(name string, parameterType *glib.VariantType, callback func()) *gio.SimpleAction
	SetWindowActionAccels(name string, accels []string)
	RunWarningDialog(format string, args ...interface{}) bool
}

type application struct {
	Env
	gtkApplication *gtk.Application
	Window         *gtk.ApplicationWindow `glade:"main_window"`
	Downloads      downloadManager        `glade:"download_"`

	items    map[string]*session.Download
	treeRefs map[string]*gtk.TreeRowReference
}

func NewApplication(env Env, appID string) (_ Application, err error) {
	a := &application{
		Env: env,
	}

	if a.gtkApplication = gtk.NewApplication(appID, gio.ApplicationFlagsNone); a.gtkApplication == nil {
		return nil, fmt.Errorf("failed to create GTK application")
	}
	a.gtkApplication.Connect("startup", a.onStartup)
	a.gtkApplication.Connect("activate", a.onActivate)
	a.gtkApplication.Connect("shutdown", a.onShutdown)

	return a, nil
}

func (a *application) Run(args ...string) int {
	go func() {
		<-a.Context().Done()
		glib.IdleAddPriority(glib.PRIORITY_HIGH, func() { a.gtkApplication.Quit() })
	}()
	return a.gtkApplication.Run(args)
}

func (a *application) onStartup() {
	a.Logger().Info("application startup")
}

func (a *application) onActivate() {
	a.Logger().Info("application activate")

	builder := generic.Unwrap(GladeRepository.GetBuilder("application.glade"))
	builder.MustBuild(a)
	a.Window.SetApplication(a.gtkApplication)

	a.Downloads.onAppActivate(a)

	a.Window.Show()
}

func (a *application) onShutdown() {
	a.Logger().Info("application shutdown")

	a.Downloads.onAppShutdown()
}

func (a *application) RegisterSimpleWindowAction(name string, parameterType *glib.VariantType, callback func()) *gio.SimpleAction {
	action := gio.NewSimpleAction(name, parameterType)
	action.Connect("activate", callback)
	a.Window.AddAction(action)
	return action
}

func (a *application) SetWindowActionAccels(name string, accels []string) {
	a.gtkApplication.SetAccelsForAction("win."+name, accels)
}

// RunWarningDialog will show a modal warning dialog with "OK" and "Cancel" buttons, returning true if "OK" was clicked.
func (a *application) RunWarningDialog(format string, args ...interface{}) bool {
	dlg := gtk.NewMessageDialog(&a.Window.Window, gtk.DialogModal, gtk.MessageWarning, gtk.ButtonsOKCancel)
	dlg.SetMarkup(fmt.Sprintf(format, args...))
	defer dlg.Destroy()
	response := dlg.Run()
	return response == int(gtk.ResponseOK)
}

func Main() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("cannot initialize logger: %v", err)
	}
	defer logger.Sync()
	zap.RedirectStdLog(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	go func() {
		<-ctx.Done()
		stop()
	}()

	env := generic.Unwrap(NewEnvBuilder().Context(ctx).Logger(logger).UserConfigDir(DefaultAppName).Build())
	defer env.Close()
	app := generic.Unwrap(NewApplication(env, DefaultAppID))
	exitCode := app.Run()
	os.Exit(exitCode)
}
