package gui

import (
	"fmt"
	"strings"

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

	actionNew    *glib.SimpleAction
	actionEdit   *glib.SimpleAction
	actionDelete *glib.SimpleAction

	Store     *gtk.ListStore `glade:"list_store_collections"`
	View      *gtk.TreeView  `glade:"tree_collections"`
	selection *gtk.TreeSelection

	dlgEdit *collectionEditDialog

	OnCurrentChanged func(*collection)
}

func newCollectionManager(app *application, builder *gtk.Builder) *collectionManager {
	m := &collectionManager{
		app:         app,
		collections: make(map[database.RowID]*collection),
	}

	m.actionNew = m.app.registerSimpleWindowAction("new_collection", nil, m.onNewButtonClicked)
	m.actionEdit = m.app.registerSimpleWindowAction("edit_collection", nil, m.onEditButtonClicked)
	m.actionEdit.SetEnabled(false)
	m.actionDelete = m.app.registerSimpleWindowAction("delete_collection", nil, m.onDeleteButtonClicked)
	m.actionDelete.SetEnabled(false)

	// Get widget references from the builder
	MustBuild(m, builder)
	m.selection = generic.Unwrap(m.View.GetSelection())
	m.dlgEdit = newCollectionEditDialog()

	m.selection.SetMode(gtk.SELECTION_SINGLE)
	m.selection.Connect("changed", m.onViewSelectionChanged)

	return m
}

func (m *collectionManager) mustRefresh() {
	m.collections = make(map[database.RowID]*collection)
	m.unsetCurrent()
	m.Store.Clear()
	for _, dbCollection := range generic.Unwrap(m.app.database.GetAllCollections()) {
		c := &collection{Collection: dbCollection}
		m.collections[c.ID] = c
		generic.Unwrap_(c.addToStore(m.Store))
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
	c := database.Collection{}
	defer m.dlgEdit.hide()
	for {
		if !m.dlgEdit.run(&c) {
			break
		}
		v := ValidationResult{}
		if c.Name == "" {
			v.AddError("name", "Collection name must not be empty")
		}
		if generic.Unwrap(m.app.database.GetCollectionByName(c.Name)) != nil {
			v.AddError("name", "Collection name in use by another collection")
		}
		if c.Path == "" {
			v.AddError("path", "Collection path must be set")
		}
		if v.IsOk() {
			m.create(&c)
			break
		} else {
			m.dlgEdit.showError(strings.Join(v.GetAllErrors(), "\n"))
		}
	}
}

func (m *collectionManager) onEditButtonClicked() {
	c := m.current.Collection
	defer m.dlgEdit.hide()
	for {
		if !m.dlgEdit.run(&c) {
			break
		}
		v := ValidationResult{}
		if c.Name == "" {
			v.AddError("name", "Collection name must not be empty")
		}
		if other := generic.Unwrap(m.app.database.GetCollectionByName(c.Name)); other != nil && other.ID != c.ID {
			v.AddError("name", "Collection name in use by another collection")
		}
		if c.Path == "" {
			v.AddError("path", "Collection path must be set")
		}
		if v.IsOk() {
			m.current.Collection = c
			m.update(m.current)
			break
		} else {
			m.dlgEdit.showError(strings.Join(v.GetAllErrors(), "\n"))
		}
	}
}

func (m *collectionManager) onDeleteButtonClicked() {
	if !m.app.runWarningDialog("Are you sure you want to delete the collection \"%s\"?", m.current.Name) {
		return
	}
	generic.Unwrap_(m.app.database.DeleteCollection(m.current.ID))
	generic.Unwrap_(m.current.removeFromStore())
	m.selection.UnselectAll()
}

func (m *collectionManager) create(dbCollection *database.Collection) {
	generic.Unwrap_(m.app.database.InsertCollection(dbCollection))
	c := &collection{Collection: *dbCollection}
	m.collections[c.ID] = c
	generic.Unwrap_(c.addToStore(m.Store))
	// TODO: just always select the new row instead?
	if len(m.collections) == 1 {
		m.selection.SelectPath(generic.Unwrap(gtk.TreePathNewFirst()))
	}
}

func (m *collectionManager) update(c *collection) {
	generic.Unwrap_(m.app.database.UpdateCollection(&c.Collection))
	generic.Unwrap_(c.updateView())
}

func (m *collectionManager) setCurrent(id database.RowID) {
	if m.current != nil && m.current.ID == id {
		return
	}
	m.current = m.collections[id]
	m.actionEdit.SetEnabled(true)
	m.actionDelete.SetEnabled(true)
	if m.OnCurrentChanged != nil {
		m.OnCurrentChanged(m.current)
	}
}

func (m *collectionManager) unsetCurrent() {
	if m.current == nil {
		return
	}
	m.current = nil
	m.actionEdit.SetEnabled(false)
	m.actionDelete.SetEnabled(false)
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

func (c *collection) removeFromStore() error {
	if c.treeRef == nil {
		return fmt.Errorf("collection not in store")
	} else if model, err := c.treeRef.GetModel(); err != nil {
		return fmt.Errorf("failed to get view model: %w", err)
	} else if iter, err := model.ToTreeModel().GetIter(c.treeRef.GetPath()); err != nil {
		return fmt.Errorf("failed to get store iter: %w", err)
	} else {
		model.(*gtk.ListStore).Remove(iter)
		return nil
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
