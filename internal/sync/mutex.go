package sync_

import "sync"

type RMutexer[T any] interface {
	// Locked runs a functions with the lock acquired.
	Locked(f func(T) error) error
	// Get returns a copy of the inner value.
	Get() T
}

type Mutexer[T any] interface {
	RMutexer[T]
	// Set overwrites the inner value.
	Set(value T)
	// Swap overwrites the inner value, returning the previous inner value.
	Swap(value T) T
}

type Mutexed[T any] struct {
	mu    sync.Mutex
	value T
}

func NewMutexed[T any](value T) *Mutexed[T] {
	m := &Mutexed[T]{value: value}
	return m
}

func (m *Mutexed[T]) Locked(f func(T) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return f(m.value)
}

func (m *Mutexed[T]) Get() T {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.value
}

func (m *Mutexed[T]) Set(value T) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.value = value
}

func (m *Mutexed[T]) Swap(value T) T {
	m.mu.Lock()
	defer m.mu.Unlock()
	old := m.value
	m.value = value
	return old
}

type RWMutexed[T any] struct {
	mu    sync.RWMutex
	value T
}

func NewRWMutexed[T any](value T) *RWMutexed[T] {
	m := &RWMutexed[T]{value: value}
	return m
}

func (m *RWMutexed[T]) Locked(f func(T) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return f(m.value)
}

// RLocked is a shortcut for Locked() on RWMutexed.RMutexer().
func (m *RWMutexed[T]) RLocked(f func(T) error) error {
	return m.RMutexer().Locked(f)
}

func (m *RWMutexed[T]) Get() T {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.value
}

func (m *RWMutexed[T]) Set(value T) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.value = value
}

func (m *RWMutexed[T]) Swap(value T) T {
	m.mu.Lock()
	defer m.mu.Unlock()
	old := m.value
	m.value = value
	return old
}

func (m *RWMutexed[T]) RMutexer() RMutexer[T] {
	return &rwMutexedReader[T]{m}
}

type rwMutexedReader[T any] struct {
	*RWMutexed[T]
}

func (m *rwMutexedReader[T]) Locked(f func(T) error) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return f(m.value)
}
