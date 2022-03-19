package main

import (
	_ "embed"
	"fmt"
	"log"
	"os"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

const appId = "co.hexi.video-archiver"

//go:embed main.glade
var glade string

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
	DOWNLOAD_COLUMN_URL = iota
	DOWNLOAD_COLUMN_PROGRESS
)

type application struct {
	collections           []collection
	currentCollection     *collection
	gtkApplication        *gtk.Application
	gtkBuilder            *gtk.Builder
	window                *gtk.Window
	collectionList        *gtk.ListBox
	collectionContextMenu *gtk.Menu
	downloadListView      *gtk.TreeView
	downloadListModel     *gtk.ListStore
}

func newApplication() (*application, error) {
	var err error
	a := &application{}

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

	a.collectionList = expectResult(a.gtkBuilder.GetObject("list_collections")).(*gtk.ListBox)
	a.collectionList.Connect("row-selected", func(listBox *gtk.ListBox, listRow *gtk.ListBoxRow) {
		collection := &a.collections[listRow.GetIndex()]
		log.Println("selected collection:", collection.name)
		a.setCurrentCollection(listRow.GetIndex())
	})

	a.downloadListView = expectResult(a.gtkBuilder.GetObject("tree_downloads")).(*gtk.TreeView)
	urlCellRenderer := expectResult(gtk.CellRendererTextNew())
	urlColumn := expectResult(gtk.TreeViewColumnNewWithAttribute("URL", urlCellRenderer, "text", DOWNLOAD_COLUMN_URL))
	a.downloadListView.AppendColumn(urlColumn)
	progressCellRenderer := expectResult(gtk.CellRendererProgressNew())
	progressColumn := expectResult(gtk.TreeViewColumnNewWithAttribute("Progress", progressCellRenderer, "value", DOWNLOAD_COLUMN_PROGRESS))
	a.downloadListView.AppendColumn(progressColumn)
	a.downloadListModel = expectResult(gtk.ListStoreNew(glib.TYPE_STRING, glib.TYPE_INT))
	a.downloadListView.SetModel(a.downloadListModel)

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
	})
}

func (a *application) onShutdown() {
	log.Println("application shutdown")
}

func (a *application) addNewCollection(name string, path string) {
	c := collection{name: name, path: path}
	c.label = expectResult(gtk.LabelNew(c.name))
	c.label.SetHAlign(gtk.ALIGN_START)
	a.collections = append(a.collections, c)
	a.collectionList.Add(c.label)
	// If this is the first collection, automatically select it
	if len(a.collections) == 1 {
		a.setCurrentCollection(0)
	}
	c.label.Show()
}

func (a *application) addNewDownload(url string) {
	log.Printf("Adding %#v to %v", url, a.currentCollection)
	a.currentCollection.downloads = append(a.currentCollection.downloads, download{url})
	download := &a.currentCollection.downloads[len(a.currentCollection.downloads)-1]
	download.appendToListStore(a.downloadListModel)
}

func (a *application) setCurrentCollection(index int) {
	a.downloadListModel.Clear()
	if index < 0 {
		a.currentCollection = nil
		a.collectionList.UnselectAll()
		expectResult(a.gtkBuilder.GetObject("pane_downloads")).(*gtk.Box).SetSensitive(false)
	} else {
		a.currentCollection = &a.collections[index]
		if row := a.collectionList.GetSelectedRow(); row == nil || row.GetIndex() != index {
			a.collectionList.SelectRow(a.collectionList.GetRowAtIndex(index))
		}
		for _, download := range a.currentCollection.downloads {
			download.appendToListStore(a.downloadListModel)
		}
		expectResult(a.gtkBuilder.GetObject("pane_downloads")).(*gtk.Box).SetSensitive(true)
	}
}

type collection struct {
	name      string
	path      string
	downloads []download
	label     *gtk.Label
}

func (c *collection) String() string {
	return fmt.Sprintf("collection{name:%#v, path:%#v}", c.name, c.path)
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
	a := expectResult(newApplication())
	a.runAndExit()
}
