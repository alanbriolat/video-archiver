package gui

import (
	"fmt"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"

	"github.com/alanbriolat/video-archiver/database"
	"github.com/alanbriolat/video-archiver/generic"
)

const (
	COLLECTION_COLUMN_ID = iota
	COLLECTION_COLUMN_NAME
	COLLECTION_COLUMN_PATH
)

type collectionManager struct {
	app *application

	collections map[database.RowID]*collection
	current     *collection

	actionNew *glib.SimpleAction

	store     *gtk.ListStore
	view      *gtk.TreeView
	selection *gtk.TreeSelection

	dlgCreate *collectionCreationDialog

	OnCurrentChanged func(*collection)
}

func newCollectionManager(app *application, builder *gtk.Builder) *collectionManager {
	m := &collectionManager{
		app:         app,
		collections: make(map[database.RowID]*collection),
	}

	m.actionNew = m.app.registerSimpleWindowAction("new_collection", nil, m.onNewButtonClicked)

	// Get widget references from the builder
	m.store = generic.Unwrap(builder.GetObject("list_store_collections")).(*gtk.ListStore)
	m.view = generic.Unwrap(builder.GetObject("tree_collections")).(*gtk.TreeView)
	m.selection = generic.Unwrap(m.view.GetSelection())
	m.dlgCreate = newCollectionCreationDialog(builder)

	m.selection.SetMode(gtk.SELECTION_SINGLE)
	m.selection.Connect("changed", m.onViewSelectionChanged)

	return m
}

func (m *collectionManager) mustRefresh() {
	m.collections = make(map[database.RowID]*collection)
	m.unsetCurrent()
	m.store.Clear()
	for _, dbCollection := range generic.Unwrap(m.app.database.GetAllCollections()) {
		c := &collection{Collection: dbCollection}
		m.collections[c.ID] = c
		generic.Unwrap_(c.addToStore(m.store))
	}
}

func (m *collectionManager) onViewSelectionChanged(selection *gtk.TreeSelection) {
	model, iter, ok := selection.GetSelected()
	if ok {
		id := generic.Unwrap(generic.Unwrap(model.ToTreeModel().GetValue(iter, COLLECTION_COLUMN_ID)).GoValue()).(int64)
		m.setCurrent(id)
	} else {
		m.unsetCurrent()
	}
}

func (m *collectionManager) onNewButtonClicked() {
	if c := m.dlgCreate.run(); c != nil {
		m.create(c)
	}
}

func (m *collectionManager) create(dbCollection *database.Collection) {
	generic.Unwrap_(m.app.database.InsertCollection(dbCollection))
	c := &collection{Collection: *dbCollection}
	m.collections[c.ID] = c
	generic.Unwrap_(c.addToStore(m.store))
	// TODO: just always select the new row instead?
	if len(m.collections) == 1 {
		m.selection.SelectPath(generic.Unwrap(gtk.TreePathNewFirst()))
	}
}

func (m *collectionManager) setCurrent(id database.RowID) {
	if m.current != nil && m.current.ID == id {
		return
	}
	m.current = m.collections[id]
	if m.OnCurrentChanged != nil {
		m.OnCurrentChanged(m.current)
	}
}

func (m *collectionManager) unsetCurrent() {
	if m.current == nil {
		return
	}
	m.current = nil
	if m.OnCurrentChanged != nil {
		m.OnCurrentChanged(m.current)
	}
}

type collection struct {
	database.Collection
	treeRef *gtk.TreeRowReference
}

func (c *collection) addToStore(store *gtk.ListStore) error {
	if c.treeRef != nil {
		return fmt.Errorf("collection already in store")
	} else if treePath, err := store.GetPath(store.Append()); err != nil {
		return err
	} else if treeRef, err := gtk.TreeRowReferenceNew(store.ToTreeModel(), treePath); err != nil {
		return err
	} else {
		c.treeRef = treeRef
		return c.updateView()
	}
}

func (c *collection) updateView() error {
	if c.treeRef == nil {
		return fmt.Errorf("collection not in view")
	} else if model, err := c.treeRef.GetModel(); err != nil {
		return fmt.Errorf("failed to get view model: %w", err)
	} else if iter, err := model.ToTreeModel().GetIter(c.treeRef.GetPath()); err != nil {
		return fmt.Errorf("failed to get store iter: %w", err)
	} else {
		err := model.(*gtk.ListStore).Set(
			iter,
			[]int{COLLECTION_COLUMN_ID, COLLECTION_COLUMN_NAME, COLLECTION_COLUMN_PATH},
			[]interface{}{c.ID, c.Name, c.Path},
		)
		if err != nil {
			return fmt.Errorf("failed to update store: %w", err)
		} else {
			return nil
		}
	}
}

func (c *collection) String() string {
	return fmt.Sprintf("collection{ID: %v, Name:%#v, Path:%#v}", c.ID, c.Name, c.Path)
}

type collectionCreationDialog struct {
	dialog *gtk.Dialog
	name   *gtk.Entry
	path   *gtk.FileChooserButton
}

func newCollectionCreationDialog(builder *gtk.Builder) *collectionCreationDialog {
	d := &collectionCreationDialog{}
	d.dialog = generic.Unwrap(builder.GetObject("dialog_new_collection")).(*gtk.Dialog)
	d.name = generic.Unwrap(builder.GetObject("entry_new_collection_name")).(*gtk.Entry)
	d.path = generic.Unwrap(builder.GetObject("choose_new_collection_path")).(*gtk.FileChooserButton)
	return d
}

func (d *collectionCreationDialog) run() (new *database.Collection) {
	// Clear from any previous use
	d.name.SetText("")
	d.path.UnselectAll()
	// Show it to the user
	d.name.GrabFocus()
	d.dialog.ShowAll()

	// TODO: do this in a loop, with some validation
	if response := d.dialog.Run(); response == gtk.RESPONSE_OK {
		name := generic.Unwrap(d.name.GetText())
		path := d.path.GetFilename()
		new = &database.Collection{
			Name: name,
			Path: path,
		}
	} else {
		// Doesn't set `new`, so returns nil
	}

	// All done, hide it
	d.dialog.Hide()
	return new
}
