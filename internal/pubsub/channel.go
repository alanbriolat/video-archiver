package pubsub

import (
	"sync"
)

type Sender[T any] interface {
	Send(T) bool
}

type Receiver[T any] interface {
	Receive() <-chan T
}

type Closer interface {
	Close()
}

type SenderCloser[T any] interface {
	Sender[T]
	Closer
}

type ReceiverCloser[T any] interface {
	Receiver[T]
	Closer
}

type Channel[T any] interface {
	Sender[T]
	Receiver[T]
	Closer
}

// channel wraps a primitive `chan` in some concurrency-safe state management.
type channel[T any] struct {
	mu      sync.RWMutex
	ch      chan T
	done    chan struct{}
	closed  bool
	waiting sync.WaitGroup
}

// NewChannel creates a new channel of the specified type and buffer size.
func NewChannel[T any](bufSize int) Channel[T] {
	c := newChannel[T](bufSize)
	return &c
}

func newChannel[T any](bufSize int) channel[T] {
	return channel[T]{
		ch:   make(chan T, bufSize),
		done: make(chan struct{}),
	}
}

// Receive returns a channel receiver for awaiting the next message.
func (c *channel[T]) Receive() <-chan T {
	return c.ch
}

// Send will attempt to send a message on the channel, returning true if successful, or false if the channel is closed.
func (c *channel[T]) Send(msg T) bool {
	// Atomically ensure that either the channel send is never attempted or that Close() can wait until no more channel
	// sends will occur
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return false
	} else {
		c.waiting.Add(1)
		defer c.waiting.Done()
		c.mu.RUnlock()
	}

	// Now attempt to send the message, bailing out if Close() is called while waiting
	select {
	case c.ch <- msg:
		return true
	case <-c.done:
		return false
	}
}

// Close idempotently ends the channel so that all current and future Send calls will fail.
func (c *channel[T]) Close() {
	// Acquire write lock, preventing any new Send() calls from being added; after the lock is released, all future
	// Send() calls should exit immediately after checking `closed`
	c.mu.Lock()
	defer c.mu.Unlock()
	// Did we already do this?
	if c.closed {
		return
	}
	// Stop any waiting senders
	close(c.done)
	// Wait for those senders to all exit
	c.waiting.Wait()
	// Close the channel to notify receiver(s)
	close(c.ch)
	c.closed = true
}
