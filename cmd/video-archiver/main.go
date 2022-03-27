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
	"github.com/alanbriolat/video-archiver/generic"
)

const appName = "video-archiver"
const appId = "co.hexi.video-archiver"

//go:embed main.glade
var glade string

var databasePath = flag.String("database", filepath.Join(glib.GetUserConfigDir(), appName, "database.sqlite3"), "override database path")

const (
	COLLECTION_COLUMN_ID = iota
	COLLECTION_COLUMN_NAME
	COLLECTION_COLUMN_PATH
)

const (
	DOWNLOAD_COLUMN_ID = iota
	DOWNLOAD_COLUMN_URL
	DOWNLOAD_COLUMN_ADDED
	DOWNLOAD_COLUMN_STATE
	DOWNLOAD_COLUMN_PROGRESS
)

type application struct {
	database              *database.Database
	collections           map[database.RowID]*collection
	currentCollection     *collection
	downloads             map[database.RowID]*download
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

	a.gtkBuilder = generic.Unwrap(gtk.BuilderNewFromString(glade))
	a.window = generic.Unwrap(a.gtkBuilder.GetObject("window_main")).(*gtk.Window)
	a.window.Show()
	a.gtkApplication.AddWindow(a.window)

	a.collectionListView = generic.Unwrap(a.gtkBuilder.GetObject("tree_collections")).(*gtk.TreeView)
	collectionSelection := generic.Unwrap(a.collectionListView.GetSelection())
	collectionSelection.SetMode(gtk.SELECTION_SINGLE)
	collectionSelection.Connect("changed", func(selection *gtk.TreeSelection) {
		model, iter, ok := selection.GetSelected()
		if ok {
			id := generic.Unwrap(generic.Unwrap(model.ToTreeModel().GetValue(iter, COLLECTION_COLUMN_ID)).GoValue()).(int64)
			a.setCurrentCollection(id)
		} else {
			a.unsetCurrentCollection()
		}
	})
	a.collectionListStore = generic.Unwrap(a.gtkBuilder.GetObject("list_store_collections")).(*gtk.ListStore)

	a.downloadListView = generic.Unwrap(a.gtkBuilder.GetObject("tree_downloads")).(*gtk.TreeView)
	a.downloadListStore = generic.Unwrap(a.gtkBuilder.GetObject("list_store_downloads")).(*gtk.ListStore)

	btnNewCollection := generic.Unwrap(a.gtkBuilder.GetObject("btn_new_collection")).(*gtk.Button)
	btnNewCollection.Connect("clicked", func() {
		dialog := generic.Unwrap(a.gtkBuilder.GetObject("dialog_new_collection")).(*gtk.Dialog)
		nameEntry := generic.Unwrap(a.gtkBuilder.GetObject("entry_new_collection_name")).(*gtk.Entry)
		pathChooser := generic.Unwrap(a.gtkBuilder.GetObject("choose_new_collection_path")).(*gtk.FileChooserButton)
		nameEntry.GrabFocus()
		dialog.ShowAll()

		if response := dialog.Run(); response == gtk.RESPONSE_OK {
			name := generic.Unwrap(nameEntry.GetText())
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

	btnNewDownload := generic.Unwrap(a.gtkBuilder.GetObject("btn_new_download")).(*gtk.Button)
	btnNewDownload.Connect("clicked", func() {
		urlEntry := generic.Unwrap(a.gtkBuilder.GetObject("entry_new_download_url")).(*gtk.Entry)
		url := generic.Unwrap(urlEntry.GetText())
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

	for _, dbCollection := range generic.Unwrap(a.database.GetAllCollections()) {
		c := newCollectionFromDB(dbCollection)
		a.collections[c.ID] = c
		c.appendToListStore(a.collectionListStore)
	}
}

func (a *application) mustRefreshDownloads() {
	a.downloads = make(map[database.RowID]*download)
	a.downloadListStore.Clear()

	if a.currentCollection == nil {
		return
	} else {
		for _, dbDownload := range generic.Unwrap(a.database.GetCollectionDownloads(a.currentCollection.ID)) {
			d := newDownloadFromDB(dbDownload)
			a.downloads[d.ID] = d
			d.appendToListStore(a.downloadListStore)
		}
	}
}

func (a *application) addNewCollection(name string, path string) {
	c := newCollection(name, path)
	// TODO: handle duplicate collection name
	generic.Unwrap_(a.database.InsertCollection(&c.Collection))
	a.collections[c.ID] = c
	c.appendToListStore(a.collectionListStore)
	// TODO: just always select the new row instead
	if len(a.collections) == 1 {
		generic.Unwrap(a.collectionListView.GetSelection()).SelectPath(generic.Unwrap(gtk.TreePathNewFirst()))
	}
}

func (a *application) addNewDownload(url string) {
	d := newDownload(a.currentCollection.ID, url)
	generic.Unwrap_(a.database.InsertDownload(&d.Download))
	a.downloads[d.ID] = d
	d.appendToListStore(a.downloadListStore)
}

func (a *application) unsetCurrentCollection() {
	a.currentCollection = nil
	a.mustRefreshDownloads()
	generic.Unwrap(a.gtkBuilder.GetObject("pane_downloads")).(*gtk.Box).SetSensitive(false)
}

func (a *application) setCurrentCollection(id database.RowID) {
	a.unsetCurrentCollection()
	a.currentCollection = a.collections[id]
	// TODO: ensure this row is selected
	//if row := a.collectionList.GetSelectedRow(); row == nil || row.GetIndex() != index {
	//	a.collectionList.SelectRow(a.collectionList.GetRowAtIndex(index))
	//}
	a.mustRefreshDownloads()
	generic.Unwrap(a.gtkBuilder.GetObject("pane_downloads")).(*gtk.Box).SetSensitive(true)
}

type collection struct {
	database.Collection
	treeRef *gtk.TreeRowReference
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
	if c.treeRef != nil {
		panic("already added to list store")
	}
	c.treeRef = generic.Unwrap(gtk.TreeRowReferenceNew(store.ToTreeModel(), generic.Unwrap(store.GetPath(iter))))
	c.updateView()
}

func (c *collection) updateView() {
	// TODO: figure out how best to avoid code duplication here
	if c.treeRef == nil {
		log.Println("no tree row reference")
	} else if !c.treeRef.Valid() {
		log.Println("tree row reference invalid")
		c.treeRef = nil
	} else {
		model := generic.Unwrap(c.treeRef.GetModel())
		iter := generic.Unwrap(model.ToTreeModel().GetIter(c.treeRef.GetPath()))
		generic.Unwrap_(model.(*gtk.ListStore).Set(
			iter,
			[]int{COLLECTION_COLUMN_ID, COLLECTION_COLUMN_NAME, COLLECTION_COLUMN_PATH},
			[]interface{}{c.ID, c.Name, c.Path},
		))
	}
}

func (c *collection) String() string {
	return fmt.Sprintf("collection{Name:%#v, Path:%#v}", c.Name, c.Path)
}

type download struct {
	database.Download
	progress int
	treeRef  *gtk.TreeRowReference
}

func newDownload(collectionID database.RowID, url string) *download {
	d := &download{}
	d.CollectionID = collectionID
	d.URL = url
	return d
}

func newDownloadFromDB(d database.Download) *download {
	return &download{Download: d}
}

func (d *download) appendToListStore(store *gtk.ListStore) {
	d.addToListStore(store, store.Append())
}

func (d *download) addToListStore(store *gtk.ListStore, iter *gtk.TreeIter) {
	if d.treeRef != nil {
		panic("already added to list store")
	}
	d.treeRef = generic.Unwrap(gtk.TreeRowReferenceNew(store.ToTreeModel(), generic.Unwrap(store.GetPath(iter))))
	d.updateView()
}

func (d *download) updateView() {
	// TODO: figure out how best to avoid code duplication here
	if d.treeRef == nil {
		log.Println("no tree row reference")
	} else if !d.treeRef.Valid() {
		log.Println("tree row reference invalid")
		d.treeRef = nil
	} else {
		model := generic.Unwrap(d.treeRef.GetModel())
		iter := generic.Unwrap(model.ToTreeModel().GetIter(d.treeRef.GetPath()))
		generic.Unwrap_(model.(*gtk.ListStore).Set(
			iter,
			[]int{DOWNLOAD_COLUMN_ID, DOWNLOAD_COLUMN_URL, DOWNLOAD_COLUMN_ADDED, DOWNLOAD_COLUMN_STATE, DOWNLOAD_COLUMN_PROGRESS},
			[]interface{}{d.ID, d.URL, d.Added.Format("2006-01-02 15:04:05"), d.State.String(), d.progress},
		))
	}
}

func main() {
	flag.Parse()
	a := generic.Unwrap(newApplication())
	a.runWithArgsAndExit([]string{})
}
