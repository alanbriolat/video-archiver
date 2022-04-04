package glade

import (
	"github.com/gotk3/gotk3/gtk"
	"testing"

	assert_ "github.com/stretchr/testify/assert"

	"github.com/alanbriolat/video-archiver/generic"
)

type summary struct {
	FullIndex []int
	GladeName string
}

func summarize(fields []field, err error) []summary {
	fields = generic.Unwrap(fields, err)
	out := make([]summary, 0, len(fields))
	for _, f := range fields {
		out = append(out, summary{FullIndex: f.FullIndex, GladeName: f.GladeName})
	}
	return out
}

func TestReflectFields(t *testing.T) {
	assert := assert_.New(t)
	var result generic.Result[[]field]

	type empty struct{}
	assert.Equal([]summary{}, summarize(reflectFieldsOf[empty]()))

	type hasFields struct {
		Untagged *gtk.Widget
		Correct  *gtk.Widget `glade:"correct"`
	}
	assert.Equal(
		[]summary{{[]int{1}, "correct"}},
		summarize(reflectFieldsOf[hasFields]()),
	)

	type unexported struct {
		foo *gtk.Widget `glade:"foo"`
	}
	result = generic.NewResult(reflectFieldsOf[unexported]())
	assert.True(result.IsErr())

	type emptyTag struct {
		Foo *gtk.Widget `glade:""`
	}
	result = generic.NewResult(reflectFieldsOf[emptyTag]())
	assert.True(result.IsErr())

	type wrongType struct {
		Foo int `glade:"foo"`
	}
	result = generic.NewResult(reflectFieldsOf[wrongType]())
	assert.True(result.IsErr())

	type wrongGTKType struct {
		// A realistic example: forgetting to use a pointer to a GTK type
		Foo gtk.Widget `glade:"foo"`
	}
	result = generic.NewResult(reflectFieldsOf[wrongGTKType]())
	assert.True(result.IsErr())
}

func TestReflectFieldsNested(t *testing.T) {
	assert := assert_.New(t)
	//var result generic.Result[[]field]

	type unexported struct {
		A *gtk.Widget `glade:"a"`
		B *gtk.Widget `glade:"b"`
	}

	type Exported struct {
		C *gtk.Widget `glade:"c"`
		D *gtk.Widget `glade:"d"`
	}

	type anonymousUnexported struct {
		unexported
		E *gtk.Widget `glade:"e"`
		F *gtk.Widget `glade:"f"`
	}
	assert.Equal(
		[]summary{
			{[]int{0, 0}, "a"},
			{[]int{0, 1}, "b"},
			{[]int{1}, "e"},
			{[]int{2}, "f"},
		},
		summarize(reflectFieldsOf[anonymousUnexported]()),
	)

	type anonymousExported struct {
		Exported
		E *gtk.Widget `glade:"e"`
		F *gtk.Widget `glade:"f"`
	}
	assert.Equal(
		[]summary{
			{[]int{0, 0}, "c"},
			{[]int{0, 1}, "d"},
			{[]int{1}, "e"},
			{[]int{2}, "f"},
		},
		summarize(reflectFieldsOf[anonymousExported]()),
	)

	type namedUnexported struct {
		Inner unexported  `glade:"inner_"`
		E     *gtk.Widget `glade:"e"`
		F     *gtk.Widget `glade:"f"`
	}
	assert.Equal(
		[]summary{
			{[]int{0, 0}, "inner_a"},
			{[]int{0, 1}, "inner_b"},
			{[]int{1}, "e"},
			{[]int{2}, "f"},
		},
		summarize(reflectFieldsOf[namedUnexported]()),
	)

	type namedExported struct {
		Inner Exported    `glade:"inner_"`
		E     *gtk.Widget `glade:"e"`
		F     *gtk.Widget `glade:"f"`
	}
	assert.Equal(
		[]summary{
			{[]int{0, 0}, "inner_c"},
			{[]int{0, 1}, "inner_d"},
			{[]int{1}, "e"},
			{[]int{2}, "f"},
		},
		summarize(reflectFieldsOf[namedExported]()),
	)

	type namedNoPrefix struct {
		Inner unexported  `glade:""`
		E     *gtk.Widget `glade:"e"`
		F     *gtk.Widget `glade:"f"`
	}
	assert.Equal(
		[]summary{
			{[]int{0, 0}, "a"},
			{[]int{0, 1}, "b"},
			{[]int{1}, "e"},
			{[]int{2}, "f"},
		},
		summarize(reflectFieldsOf[namedNoPrefix]()),
	)
}
