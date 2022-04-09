package gui

import (
	"fmt"
	"strings"
	"sync"

	"github.com/gotk3/gotk3/glib"

	"github.com/alanbriolat/video-archiver/database"
	"github.com/alanbriolat/video-archiver/generic"
)

const (
	collectionColumnID = iota
	collectionColumnName
	collectionColumnPath
	collectionColumnIDStr
)

type collectionManager struct {
	ListView[*collection]

	app Application

	actionNew    *glib.SimpleAction
	actionEdit   *glib.SimpleAction
	actionDelete *glib.SimpleAction

	dlgEdit *collectionEditDialog

	OnCurrentChanged func(*collection)
}

func (m *collectionManager) onAppActivate(app Application) {
	m.app = app
	m.ListView.idColumn = collectionColumnID
	m.ListView.onCurrentChanged = func(c *collection) {
		enabled := c != nil
		m.actionEdit.SetEnabled(enabled)
		m.actionDelete.SetEnabled(enabled)
		if m.onCurrentChanged != nil {
			m.OnCurrentChanged(c)
		}
	}
	m.ListView.onItemUpdated = func(c *collection) {
		generic.Unwrap_(m.app.DB().UpdateCollection(&c.Collection))
	}
	m.ListView.onRefresh = func() []*collection {
		var items []*collection
		for _, dbCollection := range generic.Unwrap(m.app.DB().GetAllCollections()) {
			items = append(items, newCollectionFromDB(dbCollection))
		}
		return items
	}
	m.InitListView()

	m.actionNew = m.app.RegisterSimpleWindowAction("new_collection", nil, m.onNewButtonClicked)
	m.actionEdit = m.app.RegisterSimpleWindowAction("edit_collection", nil, m.onEditButtonClicked)
	m.actionEdit.SetEnabled(false)
	m.actionDelete = m.app.RegisterSimpleWindowAction("delete_collection", nil, m.onDeleteButtonClicked)
	m.actionDelete.SetEnabled(false)

	// Get additional GTK references
	m.selection = generic.Unwrap(m.View.GetSelection())
	m.dlgEdit = newCollectionEditDialog()
}

func (m *collectionManager) onAppShutdown() {
	close(m.itemUpdated)
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
	m.MustRemoveItem(m.current)
}

func (m *collectionManager) create(dbCollection *database.Collection) {
	generic.Unwrap_(m.app.DB().InsertCollection(dbCollection))
	c := newCollectionFromDB(*dbCollection)
	m.MustAddItem(c)
}

type collection struct {
	database.Collection
	updated chan<- *collection
	mu      sync.Mutex
}

func newCollectionFromDB(dbCollection database.Collection) *collection {
	return &collection{Collection: dbCollection}
}

func (c *collection) onUpdate() {
	if c.updated != nil {
		c.updated <- c
	}
}

func (c *collection) Bind(itemUpdated chan<- *collection) {
	c.updated = itemUpdated
}

func (c *collection) GetID() database.RowID {
	return c.ID
}

func (c *collection) GetDisplay() (columns []int, values []interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	columns = []int{
		collectionColumnID,
		collectionColumnName,
		collectionColumnPath,
		collectionColumnIDStr,
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
