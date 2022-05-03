package gui2

import (
	"github.com/gotk3/gotk3/gtk"

	"github.com/alanbriolat/video-archiver/generic"
)

type downloadNewDialog struct {
	Dialog         *gtk.Dialog            `glade:"dialog"`
	UrlWidget      *gtk.Entry             `glade:"url_entry"`
	SavePathWidget *gtk.FileChooserButton `glade:"path_chooser"`
	URL            string
	SavePath       string
}

func newDownloadNewDialog() *downloadNewDialog {
	d := &downloadNewDialog{}

	GladeRepository.MustBuild(d, "download_new_dialog.glade")
	d.UrlWidget.Connect("changed", func() {
		d.URL = generic.Unwrap(d.UrlWidget.GetText())
		d.updateOkButton()
	})
	d.SavePathWidget.Connect("file-set", func() {
		d.SavePath = d.SavePathWidget.GetFilename()
		d.updateOkButton()
	})

	return d
}

func (d *downloadNewDialog) run() bool {
	d.UrlWidget.SetText("")
	d.URL = ""
	d.SavePathWidget.SelectFilename(d.SavePath)
	d.SavePath = d.SavePathWidget.GetFilename()
	d.updateOkButton()

	d.UrlWidget.GrabFocus()
	response := d.Dialog.Run()
	return response == gtk.RESPONSE_OK
}

func (d *downloadNewDialog) hide() {
	d.Dialog.Hide()
}

func (d *downloadNewDialog) showError(format string, args ...interface{}) {
	dlg := gtk.MessageDialogNew(d.Dialog, gtk.DIALOG_MODAL, gtk.MESSAGE_ERROR, gtk.BUTTONS_OK, format, args...)
	defer dlg.Destroy()
	dlg.Run()
}

func (d *downloadNewDialog) updateOkButton() {
	enabled := d.URL != "" && d.SavePath != ""
	generic.Unwrap(d.Dialog.GetWidgetForResponse(gtk.RESPONSE_OK)).ToWidget().SetSensitive(enabled)
}
