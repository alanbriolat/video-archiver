package sync

import (
	"sync"
	"testing"
	"time"

	assert_ "github.com/stretchr/testify/assert"
)

func TestEventSync(t *testing.T) {
	assert := assert_.New(t)
	e := NewEvent()
	// Initial value should be unset
	assert.False(e.IsSet())
	// Waiting on the event should block
	select {
	case <-e.Wait():
		assert.Fail("<-e.Wait() should be blocking")
	default:
	}
	// Can we set the event?
	e.Set()
	assert.True(e.IsSet())
	// Waiting on the event should succeed immediately
	select {
	case <-e.Wait():
	default:
		assert.Fail("<-e.Wait() should not block")
	}
	// Setting the event should be idempotent
	e.Set()
	assert.True(e.IsSet())
	// Can we clear the event?
	e.Clear()
	assert.False(e.IsSet())
	// Waiting on the event should block again
	select {
	case <-e.Wait():
		assert.Fail("<-e.Wait() should be blocking")
	default:
	}
	// Clearing the event should be idempotent
	e.Clear()
	assert.False(e.IsSet())
}

func TestEventAsync(t *testing.T) {
	assert := assert_.New(t)
	e := NewEvent()
	wg := sync.WaitGroup{}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-e.Wait()
		}()
	}

	blockedDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(blockedDone)
	}()

	select {
	case <-blockedDone:
		assert.Fail("event should be blocking all goroutines")
	case <-time.After(time.Second):
		// Give goroutines enough time to exit immediately if they weren't blocked
	}

	e.Set()
	select {
	case <-blockedDone:
	case <-time.After(5 * time.Second):
		assert.Fail("event should not longer be blocking all goroutines")
	}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-e.Wait()
		}()
	}

	unblockedDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(unblockedDone)
	}()

	select {
	case <-unblockedDone:
	case <-time.After(5 * time.Second):
		assert.Fail("event should not have blocked goroutines")
	}
}
