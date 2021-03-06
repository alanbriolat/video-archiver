package sync_

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

func TestRWMutexed_Simple(t *testing.T) {
	assert := assert_.New(t)
	rw := NewRWMutexed(123)
	r := rw.RMutexer()
	assert.Equal(123, rw.Get())
	assert.Equal(123, r.Get())
	rw.Set(234)
	assert.Equal(234, rw.Get())
	assert.Equal(234, r.Get())
	assert.Equal(234, rw.Swap(345))
	assert.Equal(345, rw.Get())
	assert.Equal(345, r.Get())
}

func TestRWMutexed_Race(t *testing.T) {
	assert := assert_.New(t)
	value := 0
	rw := NewRWMutexed(&value)
	var start Event
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

	assert.Equal(2500, *rw.Get())
}

func TestLocked(t *testing.T) {
	assert := assert_.New(t)

	m := NewMutexed[int](123)
	result := Locked(m, func(x int) int {
		return x * 2
	})
	assert.Equal(246, result)

}
