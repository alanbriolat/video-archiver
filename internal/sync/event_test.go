package sync

import (
	"fmt"
	"sync"
	"testing"
	"time"

	assert_ "github.com/stretchr/testify/assert"
)

func TestEventSync(t *testing.T) {
	assert := assert_.New(t)
	var e Event
	// Initial value should be unset
	assert.False(e.IsSet())
	// Waiting on the event should block
	select {
	case <-e.Wait():
		assert.Fail("<-e.Wait() should be blocking")
	default:
	}
	// Can we set the event?
	assert.True(e.Set())
	assert.True(e.IsSet())
	// Waiting on the event should succeed immediately
	select {
	case <-e.Wait():
	default:
		assert.Fail("<-e.Wait() should not block")
	}
	// Setting the event should be idempotent, but also aware of the current state
	assert.False(e.Set())
	assert.True(e.IsSet())
	// Can we clear the event?
	assert.True(e.Clear())
	assert.False(e.IsSet())
	// Waiting on the event should block again
	select {
	case <-e.Wait():
		assert.Fail("<-e.Wait() should be blocking")
	default:
	}
	// Clearing the event should be idempotent, but also aware of the current state
	assert.False(e.Clear())
	assert.False(e.IsSet())
}

func TestEventAsync(t *testing.T) {
	assert := assert_.New(t)
	var e Event
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

func BenchmarkEvent_Wait(b *testing.B) {
	for nGoroutines := 1; nGoroutines <= 512; nGoroutines *= 2 {
		b.Run(fmt.Sprintf("%d-goro", nGoroutines), func(b *testing.B) {
			var event Event
			var wg sync.WaitGroup
			wg.Add(nGoroutines)
			for i := 0; i < nGoroutines; i++ {
				go func() {
					defer wg.Done()
					for j := 0; j < b.N; j++ {
						_ = event.Wait()
					}
				}()
			}
			wg.Wait()
		})
	}
}
