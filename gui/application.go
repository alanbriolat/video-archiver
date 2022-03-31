package gui

import (
	_ "embed"
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"

	"github.com/alanbriolat/video-archiver/database"
	"github.com/alanbriolat/video-archiver/generic"
)

const appName = "video-archiver"
const appId = "co.hexi.video-archiver"

//go:embed main.glade
var glade string

var databasePath = flag.String("database", filepath.Join(glib.GetUserConfigDir(), appName, "database.sqlite3"), "override database path")

type application struct {
	database       *database.Database
	collections    *collectionManager
	downloads      *downloadManager
	gtkApplication *gtk.Application
	window         *gtk.ApplicationWindow
}

func newApplication() (*application, error) {
	var err error
	a := &application{}

	configPath := filepath.Join(glib.GetUserConfigDir(), appName)
	generic.Unwrap_(os.MkdirAll(configPath, 0750))
	if a.database, err = database.NewDatabase(*databasePath); err != nil {
		return nil, err
	}
	if err = a.database.Migrate(); err != nil {
		return nil, err
	}

	if a.gtkApplication, err = gtk.ApplicationNew(appId, glib.APPLICATION_FLAGS_NONE); err != nil {
		return nil, err
	}

	a.gtkApplication.Connect("startup", a.onStartup)
	a.gtkApplication.Connect("activate", a.onActivate)
	a.gtkApplication.Connect("shutdown", a.onShutdown)

	return a, nil
}

func (a *application) run() int {
	return a.runWithArgs(os.Args)
}

func (a *application) runWithArgs(args []string) int {
	return a.gtkApplication.Run(args)
}

func (a *application) runAndExit() {
	a.runWithArgsAndExit(os.Args)
}

func (a *application) runWithArgsAndExit(args []string) {
	os.Exit(a.runWithArgs(args))
}

func (a *application) onStartup() {
	log.Println("application startup")
}

func (a *application) onActivate() {
	log.Println("application activate")

	builder := generic.Unwrap(gtk.BuilderNewFromString(glade))
	a.window = generic.Unwrap(builder.GetObject("window_main")).(*gtk.ApplicationWindow)
	a.window.SetApplication(a.gtkApplication)

	a.downloads = newDownloadManager(a, builder)
	a.downloads.OnCurrentChanged = func(d *download) {
		log.Printf("selected download changed: %v", d)
	}
	a.collections = newCollectionManager(a, builder)
	a.collections.OnCurrentChanged = func(c *collection) {
		log.Printf("selected collection changed: %v", c)
		a.downloads.setCollection(c)
	}

	a.window.Show()
	a.collections.mustRefresh()
}

func (a *application) onShutdown() {
	log.Println("application shutdown")
	a.database.Close()
}

func (a *application) registerSimpleWindowAction(name string, parameterType *glib.VariantType, f func()) *glib.SimpleAction {
	action := glib.SimpleActionNew(name, parameterType)
	action.Connect("activate", f)
	a.window.AddAction(action)
	return action
}

// runWarningDialog will show a modal warning dialog with "OK" and "Cancel" buttons, returning true if "OK" was clicked.
func (a *application) runWarningDialog(format string, args ...interface{}) bool {
	dlg := gtk.MessageDialogNew(a.window, gtk.DIALOG_MODAL, gtk.MESSAGE_WARNING, gtk.BUTTONS_OK_CANCEL, format, args...)
	defer dlg.Destroy()
	response := dlg.Run()
	return response == gtk.RESPONSE_OK
}

func Main() {
	flag.Parse()
	a := generic.Unwrap(newApplication())
	a.runWithArgsAndExit([]string{})
}
