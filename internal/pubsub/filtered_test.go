package pubsub

import (
	"sync"
	"testing"

	assert_ "github.com/stretchr/testify/assert"
)

func TestFilteredSender_Send(t *testing.T) {
	assert := assert_.New(t)

	ch := NewChannel[int](10)
	filtered := NewFilteredSender[int](ch, func(v int) bool { return v%2 == 0 })

	// Every message is accepted, no indication of filtering
	assert.True(filtered.Send(0))
	assert.True(filtered.Send(1))
	assert.True(filtered.Send(2))
	assert.True(filtered.Send(3))
	assert.True(filtered.Send(4))
	// However, only the filtered messages are received
	assert.Equal(0, <-ch.Receive())
	assert.Equal(2, <-ch.Receive())
	assert.Equal(4, <-ch.Receive())
}

func TestFilteredSender_Close(t *testing.T) {
	assert := assert_.New(t)

	ch := NewChannel[int](10)
	filtered := NewFilteredSender[int](ch, func(v int) bool { return v%2 == 0 })

	// Closing the filtered sender should close the underlying sender
	filtered.Close()
	<-ch.Closed()
	// And sends should now fail
	assert.False(filtered.Send(0))
}

func TestFilteredSender_Close_Inner(t *testing.T) {
	assert := assert_.New(t)

	ch := NewChannel[int](10)
	filtered := NewFilteredSender[int](ch, func(v int) bool { return v%2 == 0 })

	// Closing the underlying sender should close the filtered sender
	ch.Close()
	<-filtered.Closed()
	// And sends should now fail
	assert.False(filtered.Send(0))
}

func TestFilteredSender_Publisher_AddSubscriber(t *testing.T) {
	assert := assert_.New(t)

	pub := NewPublisher[int]()
	ch := NewChannel[int](1)
	filtered := NewFilteredSender[int](ch, func(v int) bool { return v%2 == 0 })
	assert.Nil(pub.AddSubscriber(filtered))
	senderDone := make(chan struct{})
	var running sync.WaitGroup
	running.Add(2)
	go func() {
		defer close(senderDone)
		for i := 0; i < 10; i++ {
			pub.Send(i)
		}
	}()
	expected := []int{0, 2, 4, 6, 8}
	var received []int
	receiverDone := make(chan struct{})
	go func() {
		defer close(receiverDone)
		for v := range ch.Receive() {
			received = append(received, v)
		}
	}()
	<-senderDone
	pub.Close()
	<-receiverDone
	assert.Equal(expected, received)
}
