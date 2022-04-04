package gui

import (
	"testing"

	"github.com/gotk3/gotk3/gtk"

	"github.com/stretchr/testify/assert"
)

func TestGetGladeFields(t *testing.T) {
	assert := assert.New(t)

	type empty struct{}
	assert.Equal([]gladeField{}, getGladeFields[empty]())

	type hasFields struct {
		Untagged *gtk.Widget
		Correct  *gtk.Widget `glade:"correct"`
	}
	assert.Equal(
		[]gladeField{
			{[]int{1}, "correct"},
		},
		getGladeFields[hasFields](),
	)

	type unexported struct {
		foo *gtk.Widget `glade:"foo"`
	}
	assert.Panics(func() { getGladeFields[unexported]() }, "unexported but tagged field should panic")

	type emptyTag struct {
		Foo *gtk.Widget `glade:""`
	}
	assert.Panics(func() { getGladeFields[emptyTag]() }, "empty glade tag should panic")

	type wrongType struct {
		Foo int `glade:"foo"`
	}
	assert.Panics(func() { getGladeFields[wrongType]() }, "glade tag on wrong type should panic")

	type wrongGTKType struct {
		// A realistic example: forgetting to use a pointer to a GTK type
		Foo gtk.Widget `glade:"foo"`
	}
	assert.Panics(func() { getGladeFields[wrongGTKType]() }, "glade tag on wrong type should panic")
}

func TestGetGladeFieldsEmbedded(t *testing.T) {
	assert := assert.New(t)

	// Inner type must be exported for an anonymous struct field using it to be exported
	type Inner struct {
		A *gtk.Widget `glade:"a"`
		B *gtk.Widget `glade:"b"`
	}

	type ignored struct {
		Inner
		C *gtk.Widget `glade:"c"`
		D *gtk.Widget `glade:"d"`
	}
	assert.Equal(
		[]gladeField{
			{[]int{1}, "c"},
			{[]int{2}, "d"},
		},
		getGladeFields[ignored](),
	)

	type noPrefix struct {
		Inner `glade:""`
		C     *gtk.Widget `glade:"c"`
		D     *gtk.Widget `glade:"d"`
	}
	assert.Equal(
		[]gladeField{
			{[]int{0, 0}, "a"},
			{[]int{0, 1}, "b"},
			{[]int{1}, "c"},
			{[]int{2}, "d"},
		},
		getGladeFields[noPrefix](),
	)

	type withPrefix struct {
		Inner `glade:"foo_"`
		C     *gtk.Widget `glade:"c"`
		D     *gtk.Widget `glade:"d"`
	}
	assert.Equal(
		[]gladeField{
			{[]int{0, 0}, "foo_a"},
			{[]int{0, 1}, "foo_b"},
			{[]int{1}, "c"},
			{[]int{2}, "d"},
		},
		getGladeFields[withPrefix](),
	)
}

func TestGetGladeFieldsNested(t *testing.T) {
	assert := assert.New(t)

	// No need to export inner type, because we export the struct field that uses it instead
	type inner struct {
		A *gtk.Widget `glade:"a"`
		B *gtk.Widget `glade:"b"`
	}

	type ignored struct {
		Inner inner
		C     *gtk.Widget `glade:"c"`
		D     *gtk.Widget `glade:"d"`
	}
	assert.Equal(
		[]gladeField{
			{[]int{1}, "c"},
			{[]int{2}, "d"},
		},
		getGladeFields[ignored](),
	)

	type noPrefix struct {
		Inner inner       `glade:""`
		C     *gtk.Widget `glade:"c"`
		D     *gtk.Widget `glade:"d"`
	}
	assert.Equal(
		[]gladeField{
			{[]int{0, 0}, "a"},
			{[]int{0, 1}, "b"},
			{[]int{1}, "c"},
			{[]int{2}, "d"},
		},
		getGladeFields[noPrefix](),
	)

	type withPrefix struct {
		Inner inner       `glade:"foo_"`
		C     *gtk.Widget `glade:"c"`
		D     *gtk.Widget `glade:"d"`
	}
	assert.Equal(
		[]gladeField{
			{[]int{0, 0}, "foo_a"},
			{[]int{0, 1}, "foo_b"},
			{[]int{1}, "c"},
			{[]int{2}, "d"},
		},
		getGladeFields[withPrefix](),
	)
}
