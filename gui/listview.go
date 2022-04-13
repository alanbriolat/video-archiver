package gui

import (
	"fmt"
	"github.com/gotk3/gotk3/glib"
	"go.uber.org/zap"

	"github.com/gotk3/gotk3/gtk"

	"github.com/alanbriolat/video-archiver/database"
	"github.com/alanbriolat/video-archiver/generic"
)

type ListItem[T any] interface {
	fmt.Stringer
	Bind(itemUpdated chan<- T)
	GetID() database.RowID
	GetDisplay() (columns []int, values []interface{})
}

type ListView[T ListItem[T]] struct {
	Store       *gtk.ListStore `glade:"store"`
	View        *gtk.TreeView  `glade:"tree"`
	items       map[database.RowID]T
	treeRefs    map[database.RowID]*gtk.TreeRowReference
	itemUpdated chan T
	selection   *gtk.TreeSelection
	current     T

	idColumn         int
	onItemAdding     func(item T) // Before item added
	onItemAdded      func(item T) // After item added
	onItemUpdating   func(item T) // Before item updated
	onItemUpdated    func(item T) // After item updated
	onItemRemoving   func(item T) // Before item removed
	onItemRemoved    func(item T) // After item removed
	onCurrentChanged func(item T)
	onRefresh        func() []T
}

func (v *ListView[T]) InitListView() {
	v.items = make(map[database.RowID]T)
	v.itemUpdated = make(chan T)
	v.treeRefs = make(map[database.RowID]*gtk.TreeRowReference)
	v.selection = generic.Unwrap(v.View.GetSelection())
	v.selection.Connect("changed", func(selection *gtk.TreeSelection) {
		if v.onCurrentChanged == nil {
			return
		} else if model, iter, ok := selection.GetSelected(); ok {
			id := generic.Unwrap(generic.Unwrap(model.ToTreeModel().GetValue(iter, v.idColumn)).GoValue()).(int64)
			item := v.items[id]
			v.current = item
			v.onCurrentChanged(v.current)
		} else {
			// Send a zero value, which for a pointer type will be nil
			var empty T
			v.current = empty
			v.onCurrentChanged(v.current)
		}
	})

	go func() {
		logger := zap.S()
		logger.Debug("starting itemUpdated goroutine")
		for item := range v.itemUpdated {
			logger.Debugf("item updated: %v", item)
			glib.IdleAdd(func() { v.MustUpdateItem(item) })
		}
		logger.Debug("stopping itemUpdated goroutine")
	}()
}

func (v *ListView[T]) StopItemUpdates() {
	close(v.itemUpdated)
}

func (v *ListView[T]) SelectionDisabled(f func()) {
	mode := v.selection.GetMode()
	v.selection.SetMode(gtk.SELECTION_NONE)
	defer v.selection.SetMode(mode)
	f()
}

func (v *ListView[T]) MustAddItem(item T) {
	if v.onItemAdding != nil {
		v.onItemAdding(item)
	}
	iter := v.Store.Append()
	treePath := generic.Unwrap(v.Store.GetPath(iter))
	treeRef := generic.Unwrap(gtk.TreeRowReferenceNew(v.Store.ToTreeModel(), treePath))
	id := item.GetID()
	v.items[id] = item
	v.treeRefs[id] = treeRef
	item.Bind(v.itemUpdated)
	if v.onItemAdded != nil {
		v.onItemAdded(item)
	}
	v.MustUpdateItem(item)
}

func (v *ListView[T]) MustRemoveItem(item T) {
	if v.onItemRemoving != nil {
		v.onItemRemoving(item)
	}
	id := item.GetID()
	iter := generic.Unwrap(v.Store.GetIter(v.treeRefs[id].GetPath()))
	v.selection.UnselectIter(iter)
	item.Bind(nil)
	v.SelectionDisabled(func() {
		v.Store.Remove(iter)
	})
	delete(v.items, item.GetID())
	delete(v.treeRefs, item.GetID())
	if v.onItemRemoved != nil {
		v.onItemRemoved(item)
	}
}

func (v *ListView[T]) MustUpdateItem(item T) {
	if v.onItemUpdating != nil {
		v.onItemUpdating(item)
	}
	id := item.GetID()
	// Only attempt to update the view if this is an item we know about
	if treeRef, ok := v.treeRefs[id]; ok {
		iter := generic.Unwrap(v.Store.GetIter(treeRef.GetPath()))
		columns, values := item.GetDisplay()
		generic.Unwrap_(v.Store.Set(iter, columns, values))
	}
	if v.onItemUpdated != nil {
		v.onItemUpdated(item)
	}
}

func (v *ListView[T]) MustRefresh() {
	var items []T
	// Get new items
	if v.onRefresh != nil {
		items = v.onRefresh()
	}
	// Make sure nothing is selected
	v.selection.UnselectAll()
	// Remove all old items, insert new items
	v.SelectionDisabled(func() {
		v.Store.Clear()
		v.items = make(map[database.RowID]T)
		v.treeRefs = make(map[database.RowID]*gtk.TreeRowReference)
		for _, item := range items {
			v.MustAddItem(item)
		}
	})
}
