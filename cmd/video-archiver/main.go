package main

import (
	_ "embed"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"

	"github.com/alanbriolat/video-archiver/database"
)

const appName = "video-archiver"
const appId = "co.hexi.video-archiver"

//go:embed main.glade
var glade string

var databasePath = flag.String("database", filepath.Join(glib.GetUserConfigDir(), appName, "database.sqlite3"), "override database path")

func expect(err error) {
	if err != nil {
		log.Panic(err)
	}
}

func expectResult[T any](val T, err error) T {
	expect(err)
	return val
}

const (
	COLLECTION_COLUMN_ID = iota
	COLLECTION_COLUMN_NAME
	COLLECTION_COLUMN_PATH
)

const (
	DOWNLOAD_COLUMN_URL = iota
	DOWNLOAD_COLUMN_PROGRESS
)

type application struct {
	database              *database.Database
	collections           map[database.RowID]*collection
	currentCollection     *collection
	gtkApplication        *gtk.Application
	gtkBuilder            *gtk.Builder
	window                *gtk.Window
	collectionListView    *gtk.TreeView
	collectionListStore   *gtk.ListStore
	collectionContextMenu *gtk.Menu
	downloadListView      *gtk.TreeView
	downloadListStore     *gtk.ListStore
}

func newApplication() (*application, error) {
	var err error
	a := &application{
		collections: make(map[database.RowID]*collection),
	}

	configPath := filepath.Join(glib.GetUserConfigDir(), appName)
	expect(os.MkdirAll(configPath, 0750))
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

	a.gtkBuilder = expectResult(gtk.BuilderNewFromString(glade))
	a.window = expectResult(a.gtkBuilder.GetObject("window_main")).(*gtk.Window)
	a.window.Show()
	a.gtkApplication.AddWindow(a.window)

	a.collectionListView = expectResult(a.gtkBuilder.GetObject("tree_collections")).(*gtk.TreeView)
	collectionSelection := expectResult(a.collectionListView.GetSelection())
	collectionSelection.SetMode(gtk.SELECTION_SINGLE)
	collectionSelection.Connect("changed", func(selection *gtk.TreeSelection) {
		model, iter, ok := selection.GetSelected()
		if ok {
			id := expectResult(expectResult(model.ToTreeModel().GetValue(iter, COLLECTION_COLUMN_ID)).GoValue()).(int64)
			a.setCurrentCollection(id)
		} else {
			a.unsetCurrentCollection()
		}
	})
	a.collectionListStore = expectResult(a.gtkBuilder.GetObject("list_store_collections")).(*gtk.ListStore)

	a.downloadListView = expectResult(a.gtkBuilder.GetObject("tree_downloads")).(*gtk.TreeView)
	a.downloadListStore = expectResult(a.gtkBuilder.GetObject("list_store_downloads")).(*gtk.ListStore)

	btnNewCollection := expectResult(a.gtkBuilder.GetObject("btn_new_collection")).(*gtk.Button)
	btnNewCollection.Connect("clicked", func() {
		dialog := expectResult(a.gtkBuilder.GetObject("dialog_new_collection")).(*gtk.Dialog)
		nameEntry := expectResult(a.gtkBuilder.GetObject("entry_new_collection_name")).(*gtk.Entry)
		pathChooser := expectResult(a.gtkBuilder.GetObject("choose_new_collection_path")).(*gtk.FileChooserButton)
		nameEntry.GrabFocus()
		dialog.ShowAll()

		if response := dialog.Run(); response == gtk.RESPONSE_OK {
			name := expectResult(nameEntry.GetText())
			path := pathChooser.GetFilename()
			a.addNewCollection(name, path)
		} else {
			log.Printf("non-accept response: %v", response)
		}
		// Prepare for next use
		dialog.Hide()
		nameEntry.SetText("")
		pathChooser.UnselectAll()
	})

	btnNewDownload := expectResult(a.gtkBuilder.GetObject("btn_new_download")).(*gtk.Button)
	btnNewDownload.Connect("clicked", func() {
		urlEntry := expectResult(a.gtkBuilder.GetObject("entry_new_download_url")).(*gtk.Entry)
		url := expectResult(urlEntry.GetText())
		a.addNewDownload(url)
		urlEntry.SetText("")
	})

	a.mustRefreshCollections()
}

func (a *application) onShutdown() {
	log.Println("application shutdown")
	a.database.Close()
}

func (a *application) mustRefreshCollections() {
	a.collections = make(map[database.RowID]*collection)
	a.collectionListStore.Clear()

	for _, dbCollection := range expectResult(a.database.GetAllCollections()) {
		c := newCollectionFromDB(dbCollection)
		a.collections[c.ID] = c
		c.appendToListStore(a.collectionListStore)
	}
}

func (a *application) addNewCollection(name string, path string) {
	c := newCollection(name, path)
	// TODO: handle duplicate collection name
	expect(a.database.InsertCollection(&c.Collection))
	a.collections[c.ID] = c
	c.appendToListStore(a.collectionListStore)
	// If this is the first collection, automatically select it
	//if len(a.collections) == 1 {
	//	a.setCurrentCollection(c.ID)
	//}
}

func (a *application) addNewDownload(url string) {
	log.Printf("Adding %#v to %v", url, a.currentCollection)
	a.currentCollection.downloads = append(a.currentCollection.downloads, download{url})
	download := &a.currentCollection.downloads[len(a.currentCollection.downloads)-1]
	download.appendToListStore(a.downloadListStore)
}

func (a *application) unsetCurrentCollection() {
	a.currentCollection = nil
	a.downloadListStore.Clear()
	expectResult(a.gtkBuilder.GetObject("pane_downloads")).(*gtk.Box).SetSensitive(false)
}

func (a *application) setCurrentCollection(id database.RowID) {
	a.unsetCurrentCollection()
	a.currentCollection = a.collections[id]
	// TODO: ensure this row is selected
	//if row := a.collectionList.GetSelectedRow(); row == nil || row.GetIndex() != index {
	//	a.collectionList.SelectRow(a.collectionList.GetRowAtIndex(index))
	//}
	for _, download := range a.currentCollection.downloads {
		download.appendToListStore(a.downloadListStore)
	}
	expectResult(a.gtkBuilder.GetObject("pane_downloads")).(*gtk.Box).SetSensitive(true)
}

type collection struct {
	database.Collection
	downloads []download
}

func newCollection(name string, path string) *collection {
	c := &collection{}
	c.Name = name
	c.Path = path
	return c
}

func newCollectionFromDB(c database.Collection) *collection {
	return &collection{Collection: c}
}

func (c *collection) appendToListStore(store *gtk.ListStore) {
	c.addToListStore(store, store.Append())
}

func (c *collection) addToListStore(store *gtk.ListStore, iter *gtk.TreeIter) {
	log.Println("adding to list store", c.String())
	expect(store.Set(iter, []int{COLLECTION_COLUMN_ID, COLLECTION_COLUMN_NAME, COLLECTION_COLUMN_PATH}, []interface{}{c.ID, c.Name, c.Path}))
}

func (c *collection) String() string {
	return fmt.Sprintf("collection{Name:%#v, Path:%#v}", c.Name, c.Path)
}

type download struct {
	url string
}

func (d *download) appendToListStore(store *gtk.ListStore) {
	d.addToListStore(store, store.Append())
}

func (d *download) addToListStore(store *gtk.ListStore, iter *gtk.TreeIter) {
	expect(store.Set(iter, []int{DOWNLOAD_COLUMN_URL, DOWNLOAD_COLUMN_PROGRESS}, []interface{}{d.url, 0}))
}

func main() {
	flag.Parse()
	a := expectResult(newApplication())
	a.runWithArgsAndExit([]string{})
}
