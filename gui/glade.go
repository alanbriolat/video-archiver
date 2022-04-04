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
	gladeFields := reflectGladeFields(v.Type(), nil, "")
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
	return reflectGladeFields(reflect.TypeOf(v).Elem(), nil, "")
}

var gobjectType = reflect.TypeOf((*glib.IObject)(nil)).Elem()

func reflectGladeFields(t reflect.Type, indexPrefix []int, namePrefix string) (out []gladeField) {
	out = make([]gladeField, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		log.Printf(
			"%v.%v `%v`: exported=%v anonymous=%v kind=%v",
			t.Name(),
			f.Name,
			f.Tag,
			f.IsExported(),
			f.Anonymous,
			f.Type.Kind(),
		)
		name, hasTag := f.Tag.Lookup("glade")
		name = namePrefix + name
		if !hasTag {
			// Ignore anything without a `glade:"..."` tag
			continue
		} else if !f.IsExported() {
			// Panic if a field is tagged but not exported - we won't be able to set its value, so best to fail
			// loudly here instead of confusing the developer
			log.Panicf("field '%v' of struct '%v' has `glade:\"%v\"` but is not exported", f.Name, t, name)
		} else if f.Type.Kind() == reflect.Pointer && f.Type.Implements(gobjectType) {
			// e.g. foo *gtk.Window `glade:"foo"`
			if name == "" {
				// Don't allow empty names, because presumably this would break gtk.Builder.GetObject()
				log.Panicf("field '%v' of struct '%v' has empty glade tag (`glade:\"\"`)", f.Name, t)
			}
			out = append(out, gladeField{index: append(indexPrefix, i), name: name})
		} else if f.Type.Kind() == reflect.Struct {
			// e.g. InnerType `glade:"foo_"`, Inner someType `glade:"foo_"`
			if reflect.PointerTo(f.Type).Implements(gobjectType) {
				// Catch a possible common mistake: gtk.Widget instead of *gtk.Widget
				log.Panicf(
					"field '%v' of struct '%v' has unsupported type %v, expected *%v",
					f.Name, t, f.Type, f.Type,
				)
			}
			out = append(out, reflectGladeFields(f.Type, append(indexPrefix, i), name)...)
		} else {
			log.Panicf(
				"field '%v' of struct '%v' has unsupported type for `glade:\"...\"` tag (%v)",
				f.Name, t, f.Type,
			)
		}
	}
	return out
}
