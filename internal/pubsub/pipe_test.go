package pubsub

import (
	"sync"
	"testing"

	assert_ "github.com/stretchr/testify/assert"
)

func TestPipe_Send_Receive(t *testing.T) {
	assert := assert_.New(t)

	in, out, p := NewPipe[int]()

	var pending sync.WaitGroup
	pending.Add(100)
	var waiting sync.WaitGroup
	waiting.Add(2)
	go func() {
		defer waiting.Done()
		for i := 0; i < 100; i++ {
			assert.True(in.Send(i))
		}
	}()
	go func() {
		defer waiting.Done()
		j := 0
		for i := range out.Receive() {
			assert.Equal(j, i)
			j++
			pending.Done()
		}
	}()
	pending.Wait()
	p.Close()
	waiting.Wait()
}

func TestPipe_Close_Pipe(t *testing.T) {
	assert := assert_.New(t)

	in, out, p := NewPipe[int]()
	p.Close()
	assert.False(in.Send(1), "expected pipe input to be closed")
	_, ok := <-out.Receive()
	if ok {
		assert.Fail("expected pipe output to be closed")
	}
}

func TestPipe_Close_In(t *testing.T) {
	assert := assert_.New(t)

	in, out, _ := NewPipe[int]()
	in.Close()
	_, ok := <-out.Receive()
	if ok {
		assert.Fail("expected pipe output to be closed")
	}
}

func TestPipe_Close_Out(t *testing.T) {
	assert := assert_.New(t)

	in, out, _ := NewPipe[int]()
	out.Close()
	assert.False(in.Send(1), "expected pipe input to be closed")
}

// TODO: test behaviour with various buffer sizes?
