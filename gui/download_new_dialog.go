package gui

import (
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gtk/v3"
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
		d.URL = d.UrlWidget.Text()
		d.updateOkButton()
	})
	d.SavePathWidget.Connect("file-set", func() {
		d.SavePath = d.SavePathWidget.Filename()
		d.updateOkButton()
	})

	return d
}

func (d *downloadNewDialog) run() bool {
	d.UrlWidget.SetText("")
	d.URL = ""
	d.SavePathWidget.SelectFilename(d.SavePath)
	d.SavePath = d.SavePathWidget.Filename()
	d.updateOkButton()

	d.UrlWidget.GrabFocus()
	response := d.Dialog.Run()
	return response == int(gtk.ResponseOK)
}

func (d *downloadNewDialog) hide() {
	d.Dialog.Hide()
}

func (d *downloadNewDialog) showError(format string, args ...interface{}) {
	dlg := gtk.NewMessageDialog(&d.Dialog.Window, gtk.DialogModal, gtk.MessageError, gtk.ButtonsOK)
	dlg.SetMarkup(fmt.Sprintf(format, args...))
	defer dlg.Destroy()
	dlg.Run()
}

func (d *downloadNewDialog) updateOkButton() {
	enabled := d.URL != "" && d.SavePath != ""
	d.Dialog.WidgetForResponse(int(gtk.ResponseOK)).Cast().(*gtk.Widget).SetSensitive(enabled)
}
