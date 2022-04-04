package gui

import (
	"path"

	"github.com/gotk3/gotk3/gtk"

	"github.com/alanbriolat/video-archiver/database"
	"github.com/alanbriolat/video-archiver/generic"
)

type collectionEditDialog struct {
	Dialog *gtk.Dialog            `glade:"dialog"`
	Path   *gtk.FileChooserWidget `glade:"path"`
	Name   *gtk.Entry             `glade:"name"`
}

func newCollectionEditDialog() *collectionEditDialog {
	d := &collectionEditDialog{}
	GladeRepository.MustBuild(d, "collection_edit_dialog.glade")
	// If user hasn't customised the collection name, they'll see placeholder, which will follow selected directory name
	d.Path.Connect("selection-changed", func(fileChooser *gtk.FileChooserWidget) {
		dirPath := fileChooser.GetFilename()
		var dirName string
		if dirPath == "" {
			dirName = ""
		} else {
			dirName = path.Base(dirPath)
		}
		d.Name.SetPlaceholderText(dirName)
	})
	// If user gives focus to collection name entry that is showing placeholder, copy the placeholder before editing
	d.Name.Connect("focus-in-event", func() {
		if generic.Unwrap(d.Name.GetText()) == "" {
			d.Name.SetText(generic.Unwrap(d.Name.GetPlaceholderText()))
		}
	})
	// Whenever either input is edited, update the "sensitive" state of the OK button
	d.Path.Connect("selection-changed", d.updateOkButton)
	d.Name.Connect("changed", d.updateOkButton)
	return d
}

func (d *collectionEditDialog) reset() {
	d.Path.UnselectAll()
	d.Name.SetText("")
	d.Name.SetPlaceholderText("")
}

func (d *collectionEditDialog) run(c *database.Collection) bool {
	if c.Path == "" {
		d.Path.UnselectAll()
	} else {
		d.Path.SelectFilename(c.Path)
	}
	d.Name.SetText(c.Name)
	d.Dialog.ShowAll()

	if response := d.Dialog.Run(); response == gtk.RESPONSE_OK {
		c.Name = d.getName()
		c.Path = d.getPath()
		return true
	} else {
		return false
	}
}

func (d *collectionEditDialog) hide() {
	d.Dialog.Hide()
}

func (d *collectionEditDialog) showError(format string, args ...interface{}) {
	dlg := gtk.MessageDialogNew(d.Dialog, gtk.DIALOG_MODAL, gtk.MESSAGE_ERROR, gtk.BUTTONS_OK, format, args...)
	defer dlg.Destroy()
	dlg.Run()
}

func (d *collectionEditDialog) updateOkButton() {
	enabled := d.getName() != "" && d.getPath() != ""
	generic.Unwrap(d.Dialog.GetWidgetForResponse(gtk.RESPONSE_OK)).ToWidget().SetSensitive(enabled)
}

func (d *collectionEditDialog) getPath() string {
	return d.Path.GetFilename()
}

// getName gets the intended collection name, which is either a value entered by the user or the placeholder populated
// from the selected directory name.
func (d *collectionEditDialog) getName() string {
	if name := generic.Unwrap(d.Name.GetText()); name != "" {
		return name
	} else {
		return generic.Unwrap(d.Name.GetPlaceholderText())
	}
}
