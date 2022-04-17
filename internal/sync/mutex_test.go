package sync

import (
	"sync"
	"testing"

	assert_ "github.com/stretchr/testify/assert"
)

// Verify that intended interfaces are implemented
var _ RMutexer[int] = NewMutexed(123)
var _ Mutexer[int] = NewMutexed(123)
var _ RMutexer[int] = NewRWMutexed(123)
var _ Mutexer[int] = NewRWMutexed(123)
var _ RMutexer[int] = NewRWMutexed(123).RMutexer()

func TestSimple(t *testing.T) {
	assert := assert_.New(t)
	rw := NewRWMutexed(123)
	r := rw.RMutexer()
	assert.Equal(123, rw.Get())
	assert.Equal(123, r.Get())
	assert.Equal(123, rw.Swap(456))
	assert.Equal(456, r.Get())
}

func TestRace(t *testing.T) {
	assert := assert_.New(t)
	rw := NewRWMutexed(0)
	start := NewEvent()
	wg := sync.WaitGroup{}

	// Increment by 2500 with 50 goroutines in parallel
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start.Wait()
			for j := 0; j < 50; j++ {
				_ = rw.Locked(func(v *int) error {
					*v = *v + 1
					return nil
				})
			}
		}()
	}

	// Read 2500 times with another 50 goroutines in parallel
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := rw.RMutexer()
			<-start.Wait()
			for j := 0; j < 50; j++ {
				_ = r.Get()
			}
		}()
	}

	start.Set()
	wg.Wait()

	assert.Equal(2500, rw.Get())
}
