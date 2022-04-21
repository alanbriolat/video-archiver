package pubsub

import (
	"sync"
	"testing"

	assert_ "github.com/stretchr/testify/assert"

	"github.com/alanbriolat/video-archiver/generic"
)

var _ ReceiverCloser[int] = &Merger[int]{}

func TestMerger_Add_Close(t *testing.T) {
	assert := assert_.New(t)

	m := NewMerger[int]()
	rawChannel := make(chan int, 1)
	ch := NewChannel[int](1)
	pub := NewPublisher[int]()
	sub, err := pub.Subscribe()
	assert.Nil(err)
	invalid := 3

	// Can add things it's reasonable to receive from
	assert.True(m.Add(rawChannel))
	assert.True(m.Add(ch))
	assert.True(m.Add(sub))
	// Panic if adding something we can't receive from
	assert.Panics(func() {
		m.Add(invalid)
	})

	// Closing should close ReceiverCloser things, but nothing else
	m.Close()
	// Primitive channel is not closed
	rawChannel <- -1
	assert.Equal(-1, <-rawChannel)
	// `Channel` is closed
	assert.False(ch.Send(-2))
	// Subscriber is closed, but the publisher isn't; can verify this by showing no blocking behaviour when sending
	for i := 0; i < 100; i++ {
		assert.True(pub.Send(i))
	}

	// Adding valid receivers to a closed merger fails
	assert.False(m.Add(rawChannel))
	assert.False(m.Add(ch))
	assert.False(m.Add(sub))
	// But still panic if adding something we can't receive from
	assert.Panics(func() {
		m.Add(invalid)
	})

	// Oh, also, should be able to specify receivers during construction too...
	_ = NewMerger[int](rawChannel, ch, sub)
	// (And doing so with something invalid panics)
	assert.Panics(func() {
		_ = NewMerger[int](invalid)
	})
}

func TestMerger_MergePrimitiveChannels(t *testing.T) {
	assert := assert_.New(t)

	m := NewMerger[int]()

	var senders sync.WaitGroup
	var messages sync.WaitGroup
	for i := 0; i < 50; i++ {
		c := make(chan int)
		m.Add(c)
		start := i * 100
		end := start + 100
		senders.Add(1)
		go func() {
			defer senders.Done()
			defer close(c)
			for j := start; j < end; j++ {
				messages.Add(1)
				c <- j
			}
		}()
	}

	received := generic.NewSet[int]()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for v := range m.Receive() {
			received.Add(v)
			messages.Done()
		}
	}()

	// All senders should exit after all messages are consumed from their channels
	senders.Wait()
	// The right number of messages should have been received
	messages.Wait()
	// The right messages should have been received
	for i := 0; i < 5000; i++ {
		assert.True(received.Contains(i))
	}
	// Receiver should exit when closing the merger
	m.Close()
	<-done
	// Closing the merger should be idempotent
	m.Close()
}

func TestMerger_MergeChannels(t *testing.T) {
	assert := assert_.New(t)

	m := NewMerger[int]()
	c1 := NewChannel[int](0)
	c2 := NewChannel[int](0)
	m.Add(c1)
	m.Add(c2)

	// Should be able to send on either channel and receive via the merger
	assert.True(c1.Send(1))
	assert.Equal(1, <-m.Receive())
	assert.True(c2.Send(2))
	assert.Equal(2, <-m.Receive())
	// After closing one channel, should still be able to send via the other
	c1.Close()
	assert.False(c1.Send(3))
	assert.True(c2.Send(4))
	assert.Equal(4, <-m.Receive())
	// Closing the merger should close all the channels
	m.Close()
	assert.False(c2.Send(5))
	_, ok := <-m.Receive()
	assert.False(ok)
}

func TestMerger_MergeSubscribers(t *testing.T) {
	assert := assert_.New(t)
	var err error

	m := NewMerger[int]()
	p1 := NewPublisher[int]()
	p2 := NewPublisher[int]()
	s1, err := p1.Subscribe()
	assert.Nil(err)
	m.Add(s1)
	s2, err := p1.Subscribe()
	assert.Nil(err)
	m.Add(s2)
	s3, err := p2.Subscribe()
	assert.Nil(err)
	m.Add(s3)
	s4, err := p2.Subscribe()
	assert.Nil(err)
	m.Add(s4)
	s5, err := p2.Subscribe()
	assert.Nil(err)
	m.Add(s5)

	assert.True(p1.Send(1))
	assert.Equal(1, <-m.Receive())
	assert.Equal(1, <-m.Receive())
	assert.True(p2.Send(2))
	assert.Equal(2, <-m.Receive())
	assert.Equal(2, <-m.Receive())
	assert.Equal(2, <-m.Receive())
	// TODO: wrap up this pattern as a test helper...
	select {
	case <-m.Receive():
		assert.Fail("expected to wait on merger receive")
	default:
	}
}
