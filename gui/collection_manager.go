package gui

import (
	"fmt"
	"strings"
	"sync"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"

	"github.com/alanbriolat/video-archiver/database"
	"github.com/alanbriolat/video-archiver/generic"
)

const (
	COLLECTION_COLUMN_ID = iota
	COLLECTION_COLUMN_NAME
	COLLECTION_COLUMN_PATH
	COLLECTION_COLUMN_ID_STR
)

type collectionManager struct {
	app Application

	items       map[database.RowID]*collection
	itemUpdated chan *collection
	current     *collection

	actionNew    *glib.SimpleAction
	actionEdit   *glib.SimpleAction
	actionDelete *glib.SimpleAction

	Store     *gtk.ListStore `glade:"store"`
	View      *gtk.TreeView  `glade:"tree"`
	selection *gtk.TreeSelection

	dlgEdit *collectionEditDialog

	OnCurrentChanged func(*collection)
}

func (m *collectionManager) onAppActivate(app Application) {
	m.app = app
	m.items = make(map[database.RowID]*collection)
	m.itemUpdated = make(chan *collection)

	go func() {
		logger := m.app.Logger().Sugar()
		for item := range m.itemUpdated {
			logger.Debugf("item updated: %v", item)
			generic.Unwrap_(m.app.DB().UpdateCollection(&item.Collection))
			treeRef := item.getTreeRef()
			columns, values := item.getDisplay()
			// Modifying the store must be done in the GTK main thread
			glib.IdleAdd(func() { m.mustUpdateItem(treeRef, columns, values) })
		}
		logger.Debug("stopping itemUpdated goroutine")
	}()

	m.actionNew = m.app.RegisterSimpleWindowAction("new_collection", nil, m.onNewButtonClicked)
	m.actionEdit = m.app.RegisterSimpleWindowAction("edit_collection", nil, m.onEditButtonClicked)
	m.actionEdit.SetEnabled(false)
	m.actionDelete = m.app.RegisterSimpleWindowAction("delete_collection", nil, m.onDeleteButtonClicked)
	m.actionDelete.SetEnabled(false)

	// Get additional GTK references
	m.selection = generic.Unwrap(m.View.GetSelection())
	m.dlgEdit = newCollectionEditDialog()

	m.selection.SetMode(gtk.SELECTION_SINGLE)
	m.selection.Connect("changed", m.onViewSelectionChanged)
}

func (m *collectionManager) onAppShutdown() {
	close(m.itemUpdated)
}

func (m *collectionManager) selectionDisabled(f func()) {
	mode := m.selection.GetMode()
	m.selection.SetMode(gtk.SELECTION_NONE)
	defer m.selection.SetMode(mode)
	f()
}

func (m *collectionManager) mustCreateTreeRef() *gtk.TreeRowReference {
	treePath := generic.Unwrap(m.Store.GetPath(m.Store.Append()))
	treeRef := generic.Unwrap(gtk.TreeRowReferenceNew(m.Store.ToTreeModel(), treePath))
	return treeRef
}

func (m *collectionManager) mustRemoveItem(treeRef *gtk.TreeRowReference) {
	iter := generic.Unwrap(m.Store.GetIter(treeRef.GetPath()))
	m.selection.UnselectIter(iter)
	m.selectionDisabled(func() {
		m.Store.Remove(iter)
	})
}

func (m *collectionManager) mustUpdateItem(treeRef *gtk.TreeRowReference, columns []int, values []interface{}) {
	iter := generic.Unwrap(m.Store.GetIter(treeRef.GetPath()))
	generic.Unwrap_(m.Store.Set(iter, columns, values))
}

func (m *collectionManager) mustRefresh() {
	m.items = make(map[database.RowID]*collection)
	m.selection.UnselectAll()
	// Disable selection while we refresh the store, otherwise we get a load of spurious "changed" signals even though
	// nothing should be selected...
	m.selectionDisabled(func() {
		m.Store.Clear()
		for _, dbCollection := range generic.Unwrap(m.app.DB().GetAllCollections()) {
			c := newCollectionFromDB(dbCollection, m.itemUpdated, m.mustCreateTreeRef())
			m.items[c.ID] = c
			c.onUpdate()
		}
	})
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
		if generic.Unwrap(m.app.DB().GetCollectionByName(c.Name)) != nil {
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
		if other := generic.Unwrap(m.app.DB().GetCollectionByName(c.Name)); other != nil && other.ID != c.ID {
			v.AddError("name", "Collection name in use by another collection")
		}
		if c.Path == "" {
			v.AddError("path", "Collection path must be set")
		}
		if v.IsOk() {
			m.current.Collection = c
			m.current.onUpdate()
			break
		} else {
			m.dlgEdit.showError(strings.Join(v.GetAllErrors(), "\n"))
		}
	}
}

func (m *collectionManager) onDeleteButtonClicked() {
	if !m.app.RunWarningDialog("Are you sure you want to delete the collection \"%s\"?", m.current.Name) {
		return
	}
	generic.Unwrap_(m.app.DB().DeleteCollection(m.current.ID))
	m.mustRemoveItem(m.current.getTreeRef())
	m.selection.UnselectAll()
}

func (m *collectionManager) create(dbCollection *database.Collection) {
	generic.Unwrap_(m.app.DB().InsertCollection(dbCollection))
	c := newCollectionFromDB(*dbCollection, m.itemUpdated, m.mustCreateTreeRef())
	m.items[c.ID] = c
	c.onUpdate()
}

func (m *collectionManager) setCurrent(id database.RowID) {
	if m.current != nil && m.current.ID == id {
		return
	}
	m.current = m.items[id]
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
	updated chan<- *collection
	treeRef *gtk.TreeRowReference
	mu      sync.Mutex
}

func newCollectionFromDB(dbCollection database.Collection, updated chan<- *collection, treeRef *gtk.TreeRowReference) *collection {
	return &collection{
		Collection: dbCollection,
		updated:    updated,
		treeRef:    treeRef,
	}
}

func (c *collection) onUpdate() {
	c.updated <- c
}

func (c *collection) getTreeRef() *gtk.TreeRowReference {
	return c.treeRef
}

func (c *collection) getDisplay() (columns []int, values []interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	columns = []int{
		COLLECTION_COLUMN_ID,
		COLLECTION_COLUMN_NAME,
		COLLECTION_COLUMN_PATH,
		COLLECTION_COLUMN_ID_STR,
	}
	values = []interface{}{
		c.ID,
		c.Name,
		c.Path,
		fmt.Sprintf("%d", c.ID),
	}
	return columns, values
}

func (c *collection) String() string {
	return fmt.Sprintf("collection{ID: %v, Name:%#v, Path:%#v}", c.ID, c.Name, c.Path)
}
