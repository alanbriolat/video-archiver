package sync

import "sync"

// Event is inspired by Python's `threading.Event`, providing an asynchronous boolean flag that goroutines can wait on.
type Event struct {
	mu    sync.RWMutex
	ch    chan struct{}
	value bool
}

// IsSet returns the current state of the Event.
func (e *Event) IsSet() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.value
}

// Set ensures the Event is true (idempotent), notifying any waiters. Returns true if the state was changed.
func (e *Event) Set() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.value {
		return false
	}
	e.value = true
	close(e.getChannel(true))
	return true
}

// Clear ensures the Event is false (idempotent). Returns true if the state was changed.
func (e *Event) Clear() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.value {
		return false
	}
	e.value = false
	e.ch = nil // Next getChannel() will create a new channel
	return true
}

// Wait returns a channel that will close when the Event is true (which may be immediately).
func (e *Event) Wait() <-chan struct{} {
	return e.getChannel(false)
}

func (e *Event) getChannel(alreadyLocked bool) chan struct{} {
	// Only fiddle with locks if not already locked as part of another operation (i.e. Clear())
	if !alreadyLocked {
		// Try to get the channel with only a read lock
		e.mu.RLock()
		if e.ch != nil {
			// Channel already exists, can return it
			defer e.mu.RUnlock()
			return e.ch
		} else {
			// Channel doesn't exist, exchange read lock for write lock and then fall through to "already locked" code
			e.mu.RUnlock()
			e.mu.Lock()
			defer e.mu.Unlock()
		}
	}
	// Only create a new channel if one doesn't exist
	if e.ch == nil {
		e.ch = make(chan struct{})
	}
	return e.ch
}
