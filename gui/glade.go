package gui

import (
	"embed"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"

	"github.com/alanbriolat/video-archiver/generic"
)

//go:embed *.glade
var gladeFiles embed.FS

func MustNewBuilder(filename string) *gtk.Builder {
	data := generic.Unwrap(gladeFiles.ReadFile(filename))
	return generic.Unwrap(gtk.BuilderNewFromString(string(data)))
}

func MustGetObject[T glib.IObject](builder *gtk.Builder, name string) T {
	return generic.Unwrap(builder.GetObject(name)).(T)
}

func MustReadObject[T glib.IObject](target *T, builder *gtk.Builder, name string) {
	*target = MustGetObject[T](builder, name)
}
