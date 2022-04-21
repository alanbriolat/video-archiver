package lpc

import (
	"errors"
	"testing"

	assert_ "github.com/stretchr/testify/assert"
)

type ExampleCommand = *Command[int, int]

func TestCommand_New(t *testing.T) {
	assert := assert_.New(t)

	type CommandType = Command[int, int]
	type CommandPointer = *Command[int, int]

	// Can create an instance from a type alias
	a := (*CommandType).New(nil, 1)
	a.Close()

	// Can create an instance from a pointer alias
	b := CommandPointer.New(nil, 1)
	b.Close()

	// Invalid to call method on uninitialized value
	assert.Panics(func() {
		var c CommandType
		c.Close()
	})
	assert.Panics(func() {
		var c CommandType
		_ = c.Respond(3)
	})
	assert.Panics(func() {
		var c CommandType
		_ = c.RespondError(errors.New("hello, world"))
	})
	assert.Panics(func() {
		var c CommandType
		_, _ = c.Wait()
	})

	// Invalid to call method on uninitialized pointer
	assert.Panics(func() {
		var d CommandPointer
		d.Close()
	})
	assert.Panics(func() {
		var d CommandType
		_ = d.Respond(3)
	})
	assert.Panics(func() {
		var d CommandType
		_ = d.RespondError(errors.New("hello, world"))
	})
	assert.Panics(func() {
		var d CommandType
		_, _ = d.Wait()
	})
}

func TestCommand_Close(t *testing.T) {
	assert := assert_.New(t)

	// If command is prematurely closed, then the response is an error
	c := ExampleCommand(nil).New(1)
	c.Close()
	_, err := c.Wait()
	assert.ErrorIs(err, ErrNoResponse)
}

func TestCommand_Respond(t *testing.T) {
	assert := assert_.New(t)
	exampleError := errors.New("example error")

	a := ExampleCommand(nil).New(1)
	// First response gets sent
	assert.Nil(a.Respond(3))
	v, err := a.Wait()
	assert.Nil(err)
	assert.Equal(3, v)
	// Any further attempts to respond will fail
	assert.ErrorIs(a.Respond(4), ErrClosed)
	assert.ErrorIs(a.RespondError(exampleError), ErrClosed)

	b := ExampleCommand(nil).New(1)
	// First error gets sent
	assert.Nil(b.RespondError(exampleError))
	_, err = b.Wait()
	assert.ErrorIs(err, exampleError)
	// Any further attempts to respond will fail
	assert.ErrorIs(a.Respond(4), ErrClosed)
	assert.ErrorIs(a.RespondError(exampleError), ErrClosed)
}

func BenchmarkCommand_New_Respond_Wait(b *testing.B) {
	type MyCommand = *Command[int, int]

	commands := make(chan MyCommand, 1)
	go func() {
		for c := range commands {
			_ = c.Respond(c.Arg())
		}
	}()
	for i := 0; i < b.N; i++ {
		c := MyCommand(nil).New(i)
		commands <- c
		_, _ = c.Wait()
	}
	close(commands)
}
