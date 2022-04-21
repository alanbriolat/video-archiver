// Package lpc stands for "Local Procedure Call". It's a typed RPC-like mechanism implemented over Go channels, intended
// for communication with long-running goroutines.
package lpc

import (
	"errors"

	"github.com/alanbriolat/video-archiver/generic"
	sync_ "github.com/alanbriolat/video-archiver/internal/sync"
)

var (
	ErrClosed     = errors.New("command response already sent")
	ErrNoResponse = errors.New("no response")
)

// TODO: figure out how to make Command something we can type-match on

type Command[Arg any, Response any] struct {
	initialized bool
	arg         Arg
	response    generic.Result[Response]
	done        sync_.Event
}

func (*Command[Arg, Response]) New(arg Arg) *Command[Arg, Response] {
	return &Command[Arg, Response]{
		initialized: true,
		arg:         arg,
		response:    generic.Err[Response](ErrNoResponse), // Default error if closed with no response
	}
}

func (c *Command[Arg, Response]) Arg() Arg {
	return c.arg
}

func (c *Command[Arg, Response]) Respond(response Response) error {
	if c == nil || !c.initialized {
		panic("attempted to call .Respond() on uninitialized Command, must use .New() first")
	} else if c.done.IsSet() {
		return ErrClosed
	} else {
		c.response = generic.Ok[Response](response)
		c.Close()
		return nil
	}
}

func (c *Command[Arg, Response]) RespondError(err error) error {
	if c == nil || !c.initialized {
		panic("attempted to call .RespondError() on uninitialized Command, must use .New() first")
	} else if c.done.IsSet() {
		return ErrClosed
	} else {
		c.response = generic.Err[Response](err)
		c.Close()
		return nil
	}
}

func (c *Command[Arg, Response]) Wait() (Response, error) {
	if c == nil || !c.initialized {
		panic("attempted to call .Wait() on uninitialized Command, must use .New() first")
	} else {
		<-c.done.Wait()
		return c.response.Parts()
	}
}

func (c *Command[Arg, Response]) Close() {
	if c == nil || !c.initialized {
		panic("attempted to call .Close() on uninitialized Command, must use .New() first")
	} else {
		c.done.Set()
	}
}
