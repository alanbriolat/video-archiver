package pubsub

import (
	"sync"
	"testing"

	assert_ "github.com/stretchr/testify/assert"
)

var _ Publisher[int] = &publisher[int]{}

func TestPublisher(t *testing.T) {
	assert := assert_.New(t)
	pub := NewPublisher[int]().(*publisher[int])

	// Sending to a publisher with no subscribers should just succeed
	assert.True(pub.Send(1))
	assert.True(pub.Send(2))
	pub.pending.Wait() // Make sure all messages are handled before proceeding

	// Sending to a publisher with 1 subscriber, that subscriber should get the values
	s1, err := pub.Subscribe()
	assert.Nil(err)
	select {
	case <-s1.Receive():
		assert.Fail("subscriber should be waiting")
	default:
	}
	assert.True(pub.Send(3))
	assert.Equal(3, <-s1.Receive())
	select {
	case <-s1.Receive():
		assert.Fail("subscriber should be waiting")
	default:
	}
	pub.pending.Wait() // Make sure all messages are handled before proceeding

	// Sending to a publisher with 2 subscribers, both subscribers should get the same value
	var wg sync.WaitGroup
	s2, err := pub.Subscribe()
	assert.Nil(err)
	select {
	case <-s2.Receive():
		assert.Fail("subscriber should be waiting")
	default:
	}
	var v1, v2 int
	wg.Add(2)
	go func() { v1 = <-s1.Receive(); wg.Done() }()
	go func() { v2 = <-s2.Receive(); wg.Done() }()
	assert.True(pub.Send(4))
	wg.Wait()
	assert.Equal(v1, 4)
	assert.Equal(v2, 4)
	pub.pending.Wait() // Make sure all messages are handled before proceeding

	// Once one subscriber is closed, the other subscriber should still receive sent values
	s1.Close()
	assert.True(pub.Send(5))
	select {
	case _, ok := <-s1.Receive():
		assert.False(ok, "expected closed subscriber to return closed channel")
	default:
		assert.Fail("expected closed subscriber to return closed channel")
	}
	assert.Equal(5, <-s2.Receive())
	// Closing should be idempotent
	s1.Close()
	pub.pending.Wait() // Make sure all messages are handled before proceeding

	// Once the publisher is closed, subscribing or sending should fail
	pub.Close()
	_, err = pub.Subscribe()
	assert.Equal(ErrPublisherClosed, err)
	assert.False(pub.Send(6))
	// Also the subscribers should be closed
	_, ok := <-s2.Receive()
	assert.False(ok, "expected subscriber to be closed by publisher")
	// Closing should be idempotent
	pub.Close()
}

func TestPublisher_AddSubscriber_Close(t *testing.T) {
	assert := assert_.New(t)

	pub := NewPublisher[int]()
	c1 := NewChannel[int](1)
	c2 := NewChannel[int](1)
	assert.Nil(pub.AddSubscriber(c1, true))
	assert.Nil(pub.AddSubscriber(c2, false))
	// When the publisher is closed, it should obey the "close" flag for each of its subscribers
	pub.Close()
	assert.False(c1.Send(1), "expected close=true subscriber to be closed")
	assert.True(c2.Send(1), "expected close=false subscriber to not be closed")
}
