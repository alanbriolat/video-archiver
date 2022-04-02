package gui

import (
	"fmt"
	"path"
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
	actionDelete *glib.SimpleAction

	store     *gtk.ListStore
	view      *gtk.TreeView
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
	m.actionDelete = m.app.registerSimpleWindowAction("delete_collection", nil, m.onDeleteButtonClicked)
	m.actionDelete.SetEnabled(false)

	// Get widget references from the builder
	MustReadObject(&m.store, builder, "list_store_collections")
	MustReadObject(&m.view, builder, "tree_collections")
	m.selection = generic.Unwrap(m.view.GetSelection())
	m.dlgEdit = newCollectionEditDialog(builder)

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
		if generic.Unwrap(m.app.database.CollectionNameExists(c.Name)) {
			v.AddError("name", "Collection name already in use")
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

type collectionEditDialog struct {
	dialog *gtk.Dialog
	path   *gtk.FileChooserWidget
	name   *gtk.Entry
}

func newCollectionEditDialog(builder *gtk.Builder) *collectionEditDialog {
	d := &collectionEditDialog{}
	MustReadObject(&d.dialog, builder, "dialog_new_collection")
	MustReadObject(&d.path, builder, "choose_new_collection_path")
	MustReadObject(&d.name, builder, "entry_new_collection_name")
	// If user hasn't customised the collection name, they'll see placeholder, which will follow selected directory name
	d.path.Connect("selection-changed", func(fileChooser *gtk.FileChooserWidget) {
		dirPath := fileChooser.GetFilename()
		var dirName string
		if dirPath == "" {
			dirName = ""
		} else {
			dirName = path.Base(dirPath)
		}
		d.name.SetPlaceholderText(dirName)
	})
	// If user gives focus to collection name entry that is showing placeholder, copy the placeholder before editing
	d.name.Connect("focus-in-event", func() {
		if generic.Unwrap(d.name.GetText()) == "" {
			d.name.SetText(generic.Unwrap(d.name.GetPlaceholderText()))
		}
	})
	// Whenever either input is edited, update the "sensitive" state of the OK button
	d.path.Connect("selection-changed", d.updateOkButton)
	d.name.Connect("changed", d.updateOkButton)
	return d
}

func (d *collectionEditDialog) reset() {
	d.path.UnselectAll()
	d.name.SetText("")
	d.name.SetPlaceholderText("")
}

func (d *collectionEditDialog) run(c *database.Collection) bool {
	if c.Path == "" {
		d.path.UnselectAll()
	} else if d.getPath() != c.Path {
		d.path.SelectFilename(c.Path)
	}
	d.name.SetText(c.Name)
	d.dialog.ShowAll()

	if response := d.dialog.Run(); response == gtk.RESPONSE_OK {
		c.Name = d.getName()
		c.Path = d.getPath()
		return true
	} else {
		return false
	}
}

func (d *collectionEditDialog) hide() {
	d.dialog.Hide()
}

func (d *collectionEditDialog) showError(format string, args ...interface{}) {
	dlg := gtk.MessageDialogNew(d.dialog, gtk.DIALOG_MODAL, gtk.MESSAGE_ERROR, gtk.BUTTONS_OK, format, args...)
	defer dlg.Destroy()
	dlg.Run()
}

func (d *collectionEditDialog) updateOkButton() {
	enabled := d.getName() != "" && d.getPath() != ""
	generic.Unwrap(d.dialog.GetWidgetForResponse(gtk.RESPONSE_OK)).ToWidget().SetSensitive(enabled)
}

func (d *collectionEditDialog) getPath() string {
	return d.path.GetFilename()
}

// getName gets the intended collection name, which is either a value entered by the user or the placeholder populated
// from the selected directory name.
func (d *collectionEditDialog) getName() string {
	if name := generic.Unwrap(d.name.GetText()); name != "" {
		return name
	} else {
		return generic.Unwrap(d.name.GetPlaceholderText())
	}
}
