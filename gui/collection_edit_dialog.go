package gui

import (
	"path"

	"github.com/gotk3/gotk3/gtk"

	"github.com/alanbriolat/video-archiver/database"
	"github.com/alanbriolat/video-archiver/generic"
)

type collectionEditDialog struct {
	dialog *gtk.Dialog
	path   *gtk.FileChooserWidget
	name   *gtk.Entry
}

func newCollectionEditDialog() *collectionEditDialog {
	d := &collectionEditDialog{}
	builder := MustNewBuilder("collection_edit_dialog.glade")
	MustReadObject(&d.dialog, builder, "dialog")
	MustReadObject(&d.path, builder, "path")
	MustReadObject(&d.name, builder, "name")
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
	} else {
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
