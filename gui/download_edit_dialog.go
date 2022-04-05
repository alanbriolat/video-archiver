package gui

import (
	"fmt"

	"github.com/gotk3/gotk3/gtk"

	"github.com/alanbriolat/video-archiver/database"
	"github.com/alanbriolat/video-archiver/generic"
)

type downloadEditDialog struct {
	collectionStore *gtk.ListStore
	Dialog          *gtk.Dialog           `glade:"dialog"`
	Collection      *gtk.ComboBox         `glade:"collection"`
	CollectionName  *gtk.CellRendererText `glade:"collection_name"`
	URL             *gtk.Entry            `glade:"url"`
}

func newDownloadEditDialog(collectionStore *gtk.ListStore) *downloadEditDialog {
	d := &downloadEditDialog{
		collectionStore: collectionStore,
	}

	GladeRepository.MustBuild(d, "download_edit_dialog.glade")
	d.Collection.SetModel(d.collectionStore.ToTreeModel())
	d.Collection.SetIDColumn(COLLECTION_COLUMN_ID_STR)
	d.Collection.SetEntryTextColumn(COLLECTION_COLUMN_NAME)
	d.Collection.AddAttribute(d.CollectionName, "text", COLLECTION_COLUMN_NAME)
	// Whenever either input is edited, update the "sensitive" state of the OK button
	d.Collection.Connect("changed", d.updateOkButton)
	d.URL.Connect("changed", d.updateOkButton)

	return d
}

func (d *downloadEditDialog) run(dl *database.Download) bool {
	if dl.ID == database.NullRowID {
		d.Collection.SetSensitive(true)
		d.Dialog.SetTitle("New download")
	} else {
		d.Collection.SetSensitive(false)
		d.Dialog.SetTitle("Edit download")
	}
	d.Collection.SetActiveID(fmt.Sprintf("%d", dl.CollectionID))
	d.URL.SetText(dl.URL)
	d.updateOkButton()
	d.Dialog.ShowAll()

	if response := d.Dialog.Run(); response == gtk.RESPONSE_OK {
		dl.CollectionID = d.getCollectionID()
		dl.URL = d.getURL()
		return true
	} else {
		return false
	}
}

func (d *downloadEditDialog) hide() {
	d.Dialog.Hide()
}

func (d *downloadEditDialog) showError(format string, args ...interface{}) {
	dlg := gtk.MessageDialogNew(d.Dialog, gtk.DIALOG_MODAL, gtk.MESSAGE_ERROR, gtk.BUTTONS_OK, format, args...)
	defer dlg.Destroy()
	dlg.Run()
}

func (d *downloadEditDialog) updateOkButton() {
	enabled := d.getCollectionID() != database.NullRowID && d.getURL() != ""
	generic.Unwrap(d.Dialog.GetWidgetForResponse(gtk.RESPONSE_OK)).ToWidget().SetSensitive(enabled)
}

func (d *downloadEditDialog) getCollectionID() database.RowID {
	if iter, err := d.Collection.GetActiveIter(); err != nil {
		return database.NullRowID
	} else {
		return generic.Unwrap(generic.Unwrap(d.collectionStore.ToTreeModel().GetValue(iter, COLLECTION_COLUMN_ID)).GoValue()).(int64)
	}
}

func (d *downloadEditDialog) getURL() string {
	return generic.Unwrap(d.URL.GetText())
}
