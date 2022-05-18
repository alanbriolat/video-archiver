package glade

import (
	"fmt"
	"reflect"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
)

type FieldError struct {
	Type  reflect.Type
	Field field
	Msg   string
}

func NewFieldError(t reflect.Type, f field, format string, args ...interface{}) *FieldError {
	return &FieldError{
		Type:  t,
		Field: f,
		Msg:   fmt.Sprintf(format, args...),
	}
}

func (e *FieldError) Error() string {
	return fmt.Sprintf("glade: field '%v' of struct '%v': %v", e.Field.Name, e.Type, e.Msg)
}

type field struct {
	reflect.StructField
	FullIndex []int
	GladeTag  string
	GladeName string
}

func reflectFields(t reflect.Type, indexPrefix []int, namePrefix string) ([]field, error) {
	out := make([]field, 0, t.NumField())
	for _, rf := range reflect.VisibleFields(t) {
		if rf.Anonymous {
			// Skip anonymous fields, because their promoted fields will be handled instead
			continue
		} else if _, ok := rf.Tag.Lookup("glade"); !ok {
			// Skip any field missing a `glade:"..."` tag
			continue
		}
		f := field{
			StructField: rf,
			FullIndex:   append(indexPrefix, rf.Index...),
			GladeTag:    rf.Tag.Get("glade"),
			GladeName:   namePrefix + rf.Tag.Get("glade"),
		}
		if !f.IsExported() {
			// If a field is tagged but not exported, it's almost certainly a developer mistake, and we shouldn't
			// silently ignore it
			return nil, NewFieldError(t, f, "tagged but not exported")
		} else if f.Type.Kind() == reflect.Pointer && f.Type.Implements(gobjectType) {
			// e.g. Foo *gtk.Window `glade:"foo"`
			if f.GladeTag == "" {
				// Don't allow empty names, because presumably this would break gtk.Builder.GetObject()
				return nil, NewFieldError(t, f, "empty glade tag")
			} else {
				out = append(out, f)
			}
		} else if f.Type.Kind() == reflect.Struct {
			// e.g. Inner someType `glade:"foo_"`
			if reflect.PointerTo(f.Type).Implements(gobjectType) {
				// Catch a possible common mistake: gtk.Widget instead of *gtk.Widget
				return nil, NewFieldError(t, f, "expected gtk pointer type, not struct type")
			} else if fields, err := reflectFields(f.Type, f.FullIndex, f.GladeName); err != nil {
				return nil, err
			} else {
				// Recurse into nested struct
				out = append(out, fields...)
			}
		} else {
			return nil, NewFieldError(t, f, "unsupported field type: %v", f.Type)
		}
	}
	return out, nil
}

func reflectFieldsOf[T any]() ([]field, error) {
	return reflectFields(reflect.TypeOf((*T)(nil)).Elem(), nil, "")
}

var gobjectType = reflect.TypeOf((*glib.Objector)(nil)).Elem()
