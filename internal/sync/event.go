package sync

import "sync"

// Event is inspired by Python's `threading.Event`, providing an asynchronous boolean flag that goroutines can wait on.
type Event interface {
	// IsSet returns the current state of the event.
	IsSet() bool
	// Set ensures the event is true (idempotent), notifying any waiters.
	Set()
	// Clear ensures the event is false (idempotent).
	Clear()
	// Wait returns a channel that will close when the event is true (which may be immediately).
	Wait() <-chan struct{}
}

type event struct {
	mu    sync.RWMutex
	c     chan struct{}
	value bool
}

func NewEvent() Event {
	return &event{
		c: make(chan struct{}),
	}
}

func (e *event) IsSet() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.value
}

func (e *event) Set() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.value {
		e.value = true
		close(e.c)
	}
}

func (e *event) Clear() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.value {
		e.value = false
		e.c = make(chan struct{})
	}
}

func (e *event) Wait() <-chan struct{} {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.c
}
