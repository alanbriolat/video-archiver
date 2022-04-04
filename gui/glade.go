package gui

import (
	"embed"
	"log"
	"reflect"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"

	"github.com/alanbriolat/video-archiver/generic"
)

//go:embed *.glade
var gladeFiles embed.FS

func MustNewBuilderFromEmbed(filename string) *gtk.Builder {
	data := generic.Unwrap(gladeFiles.ReadFile(filename))
	return generic.Unwrap(gtk.BuilderNewFromString(string(data)))
}

func MustGetObject[T glib.IObject](builder *gtk.Builder, name string) T {
	return generic.Unwrap(builder.GetObject(name)).(T)
}

func MustReadObject[T glib.IObject](target *T, builder *gtk.Builder, name string) {
	*target = MustGetObject[T](builder, name)
}

func MustBuildFromEmbed(s interface{}, filename string) {
	builder := MustNewBuilderFromEmbed(filename)
	MustBuild(s, builder)
}

func MustBuild(s interface{}, builder *gtk.Builder) {
	v := reflect.Indirect(reflect.ValueOf(s))
	gladeFields := reflectGladeFields(v.Type())
	for _, gladeField := range gladeFields {
		f := v.FieldByIndex(gladeField.index)
		o := generic.Unwrap(builder.GetObject(gladeField.name))
		f.Set(reflect.ValueOf(o))
	}
}

type gladeField struct {
	index []int
	name  string
}

func getGladeFields[T any]() []gladeField {
	var v *T
	return reflectGladeFields(reflect.TypeOf(v).Elem())
}

// TODO: recursively get glade fields on nested/embedded structures?
func reflectGladeFields(t reflect.Type) (out []gladeField) {
	out = make([]gladeField, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if name, ok := f.Tag.Lookup("glade"); !ok {
			continue
		} else if name == "" {
			log.Panicf("field '%v' of struct '%v' has empty glade tag (`glade:\"\"`)", f.Name, t)
		} else if !f.IsExported() {
			log.Panicf("field '%v' of struct '%v' has `glade:\"%v\"` but is not exported", f.Name, t, name)
		} else {
			out = append(out, gladeField{index: []int{i}, name: name})
		}
	}
	return out
}
