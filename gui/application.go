package gui

import (
	_ "embed"
	"fmt"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"

	"github.com/alanbriolat/video-archiver/generic"
)

const DefaultAppName = "video-archiver"
const DefaultAppID = "co.hexi.video-archiver"

type Application interface {
	Env

	Run() int
	RunWithArgs([]string) int

	RegisterSimpleWindowAction(name string, parameterType *glib.VariantType, callback func()) *glib.SimpleAction
	SetWindowActionAccels(name string, accels []string)
	RunWarningDialog(format string, args ...interface{}) bool
	RunErrorDialog(format string, args ...interface{})
}

type application struct {
	Env
	gtkApplication *gtk.Application
	Window         *gtk.ApplicationWindow `glade:"window_main"`
	Collections    collectionManager      `glade:"collections_"`
	Downloads      downloadManager        `glade:"downloads_"`
}

func NewApplication(env Env, appID string) (_ Application, err error) {
	a := &application{
		Env: env,
	}

	if a.gtkApplication, err = gtk.ApplicationNew(appID, glib.APPLICATION_FLAGS_NONE); err != nil {
		return nil, fmt.Errorf("failed to create GTK application: %w", err)
	}
	a.gtkApplication.Connect("startup", a.onStartup)
	a.gtkApplication.Connect("activate", a.onActivate)
	a.gtkApplication.Connect("shutdown", a.onShutdown)

	return a, nil
}

func (a *application) Run() int {
	return a.RunWithArgs([]string{})
}

func (a *application) RunWithArgs(args []string) int {
	// Ensure the GTK application quits if the context is cancelled
	go func() {
		<-a.Context().Done()
		glib.IdleAddPriority(glib.PRIORITY_HIGH, func() { a.gtkApplication.Quit() })
	}()
	return a.gtkApplication.Run(args)
}

func (a *application) RegisterSimpleWindowAction(name string, parameterType *glib.VariantType, callback func()) *glib.SimpleAction {
	action := glib.SimpleActionNew(name, parameterType)
	action.Connect("activate", callback)
	a.Window.AddAction(action)
	return action
}

func (a *application) SetWindowActionAccels(name string, accels []string) {
	a.gtkApplication.SetAccelsForAction("win."+name, accels)
}

// RunWarningDialog will show a modal warning dialog with "OK" and "Cancel" buttons, returning true if "OK" was clicked.
func (a *application) RunWarningDialog(format string, args ...interface{}) bool {
	dlg := gtk.MessageDialogNew(a.Window, gtk.DIALOG_MODAL, gtk.MESSAGE_WARNING, gtk.BUTTONS_OK_CANCEL, format, args...)
	defer dlg.Destroy()
	response := dlg.Run()
	return response == gtk.RESPONSE_OK
}

// RunErrorDialog will show a modal error dialog with an "OK" button.
func (a *application) RunErrorDialog(format string, args ...interface{}) {
	dlg := gtk.MessageDialogNew(a.Window, gtk.DIALOG_MODAL, gtk.MESSAGE_ERROR, gtk.BUTTONS_OK, format, args...)
	defer dlg.Destroy()
	_ = dlg.Run()
}

func (a *application) onStartup() {
	a.Logger().Info("application startup")
}

func (a *application) onActivate() {
	a.Logger().Info("application activate")

	builder := generic.Unwrap(GladeRepository.GetBuilder("application.glade"))
	builder.MustBuild(a)
	a.Window.SetApplication(a.gtkApplication)

	a.Collections.onAppActivate(a)
	a.Collections.OnCurrentChanged = func(c *collection) {
		a.Logger().Sugar().Infof("selected collection changed: %v", c)
		a.Downloads.setCollection(c)
	}
	a.Downloads.onAppActivate(a, &a.Collections)
	a.Downloads.OnCurrentChanged = func(d *download) {
		a.Logger().Sugar().Infof("selected download changed: %v", d)
	}

	a.Window.Show()
	a.Collections.mustRefresh()
}

func (a *application) onShutdown() {
	a.Logger().Info("application shutdown")
}
