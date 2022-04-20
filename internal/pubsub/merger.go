package pubsub

import (
	"fmt"
	"sync"
)

const (
	DefaultMergerBufSize = 1
)

type Merger[T any] struct {
	mu      sync.RWMutex
	ch      Channel[T]
	done    chan struct{}
	running sync.WaitGroup
	closed  bool
}

func NewMerger[T any](receivers ...interface{}) Merger[T] {
	return NewMergerBufSize[T](DefaultMergerBufSize, receivers...)
}

func NewMergerBufSize[T any](bufSize int, receivers ...interface{}) Merger[T] {
	m := Merger[T]{
		ch:   NewChannel[T](bufSize),
		done: make(chan struct{}),
	}
	for _, r := range receivers {
		m.Add(r)
	}
	return m
}

func (m *Merger[T]) Add(receiver interface{}) bool {
	switch r := receiver.(type) {
	case ReceiverCloser[T]:
		return m.AddPrimitive(r.Receive(), r.Close)
	//case Receiver[T]:
	//	return m.AddPrimitive(r.Receive(), nil)
	case chan T:
		return m.AddPrimitive(r, nil)
	default:
		panic(fmt.Sprintf("unhandled receiver type: %T", receiver))
	}
}

func (m *Merger[T]) AddPrimitive(ch <-chan T, close func()) bool {
	// Atomically ensure that either the goroutine is never started or that Close() can wait until it ends
	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return false
	}
	m.running.Add(1)
	m.mu.RUnlock()

	// Launch goroutine to funnel messages from the channel
	go func() {
		// Track when the goroutine exits
		defer m.running.Done()
		// Before that, call the closer associated with the channel
		if close != nil {
			defer close()
		}
		// Run until either the channel or Merger closes
		for {
			select {
			// Channel message or closed
			case msg, recvOk := <-ch:
				if !recvOk {
					// Subscriber closed, goroutine should exit
					return
				}
				sendOk := m.ch.Send(msg)
				if !sendOk {
					// Merger closed between receiving from channel and sending to merger
					return
				}
			// Merger closed while waiting on channel
			case <-m.done:
				return
			}
		}
	}()

	return true
}

func (m *Merger[T]) Receive() <-chan T {
	return m.ch.Receive()
}

func (m *Merger[T]) Close() {
	// Acquire write lock, preventing any new Add() goroutines being spawned; after lock is released, all future Add()
	// calls should exit immediately after checking `closed`
	m.mu.Lock()
	defer m.mu.Unlock()
	// Did we already do this?
	if m.closed {
		return
	}
	// Stop any waiting senders and receivers
	close(m.done)
	m.ch.Close()
	// Wait for the senders to all exit
	m.running.Wait()
	// Mark the Merger as closed forever
	m.closed = true
}

func (m *Merger[T]) Closed() <-chan struct{} {
	return m.done
}
