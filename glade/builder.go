package glade

import (
	"fmt"
	"reflect"

	"github.com/gotk3/gotk3/gtk"

	"github.com/alanbriolat/video-archiver/generic"
)

type ReadFunc = func(filename string) ([]byte, error)

type Repository interface {
	GetBuilder(filename string) (Builder, error)
	Build(v interface{}, filename string) error
	MustBuild(v interface{}, filename string)
}

type repository struct {
	read ReadFunc
}

func NewRepository(f ReadFunc) Repository {
	return &repository{read: f}
}

func (r *repository) GetBuilder(filename string) (Builder, error) {
	if data, err := r.read(filename); err != nil {
		return nil, fmt.Errorf("glade: failed to read %v: %w", filename, err)
	} else if builder, err := NewBuilder(data); err != nil {
		return nil, fmt.Errorf("glade: failed to load %v: %w", filename, err)
	} else {
		return builder, nil
	}
}

func (r *repository) Build(v interface{}, filename string) error {
	if builder, err := r.GetBuilder(filename); err != nil {
		return err
	} else {
		return builder.Build(v)
	}
}

func (r *repository) MustBuild(v interface{}, filename string) {
	generic.Unwrap_(r.Build(v, filename))
}

type Builder interface {
	ToGTK() *gtk.Builder
	Build(s interface{}) error
	MustBuild(v interface{})
}

type builder struct {
	gtkBuilder *gtk.Builder
}

func NewBuilder(data []byte) (Builder, error) {
	if gtkBuilder, err := gtk.BuilderNewFromString(string(data)); err != nil {
		return nil, err
	} else {
		return &builder{gtkBuilder}, nil
	}
}

func (b *builder) ToGTK() *gtk.Builder {
	return b.gtkBuilder
}

func (b *builder) Build(v interface{}) error {
	reflected := reflect.Indirect(reflect.ValueOf(v))
	if fields, err := reflectFields(reflected.Type(), nil, ""); err != nil {
		return err
	} else {
		for _, f := range fields {
			vf := reflected.FieldByIndex(f.FullIndex)
			if o, err := b.gtkBuilder.GetObject(f.GladeName); err != nil {
				return err
			} else {
				vf.Set(reflect.ValueOf(o))
			}
		}
	}
	return nil
}

func (b *builder) MustBuild(v interface{}) {
	generic.Unwrap_(b.Build(v))
}
