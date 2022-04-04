package gui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetGladeFields(t *testing.T) {
	assert := assert.New(t)

	type empty struct{}
	assert.Equal([]gladeField{}, getGladeFields[empty]())

	type hasFields struct {
		Untagged interface{}
		Correct  interface{} `glade:"correct"`
	}
	assert.Equal(
		[]gladeField{
			{[]int{1}, "correct"},
		},
		getGladeFields[hasFields](),
	)

	type unexported struct {
		foo interface{} `glade:"foo"`
	}
	assert.Panics(func() { getGladeFields[unexported]() }, "unexported but tagged field should panic")

	type emptyTag struct {
		Foo interface{} `glade:""`
	}
	assert.Panics(func() { getGladeFields[emptyTag]() }, "empty glade tag should panic")
}
