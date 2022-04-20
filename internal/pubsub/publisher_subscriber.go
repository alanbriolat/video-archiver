package pubsub

import (
	"errors"
	"sync"

	"github.com/alanbriolat/video-archiver/generic"
)

const (
	DefaultPublisherBufSize  = 1
	DefaultSubscriberBufSize = 1
)

var (
	ErrPublisherClosed = errors.New("publisher closed")
)

type Publisher[T any] interface {
	SenderCloser[T]
	Subscribe() (Subscriber[T], error)
}

type Subscriber[T any] interface {
	ReceiverCloser[T]
}

type publisher[T any] struct {
	mu          sync.Mutex
	ch          Channel[T]
	running     sync.WaitGroup // Goroutines in progress
	pending     sync.WaitGroup // Messages not yet sent to all subscribers
	subscribers generic.Set[*subscriber[T]]
	closed      bool
}

func NewPublisher[T any]() Publisher[T] {
	return NewPublisherBufSize[T](DefaultPublisherBufSize)
}

func NewPublisherBufSize[T any](bufSize int) Publisher[T] {
	p := &publisher[T]{
		ch:          NewChannel[T](bufSize),
		subscribers: generic.NewSet[*subscriber[T]](),
	}
	p.running.Add(1)
	go func() {
		defer p.running.Done()
		for v := range p.ch.Receive() {
			// Hold lock for minimum amount of time to get the latest set of subscribers
			p.mu.Lock()
			subscribers := p.subscribers.ToSlice()
			p.mu.Unlock()
			// Attempt to send to each subscriber
			for _, s := range subscribers {
				if ok := s.Send(v); !ok {
					p.unsubscribe(s)
				}
			}
			p.pending.Done()
		}
	}()
	return p
}

// Send will publish the value to all subscribers (non-blocking).
func (p *publisher[T]) Send(msg T) bool {
	p.pending.Add(1)
	if ok := p.ch.Send(msg); !ok {
		// Message was not sent, so don't wait for it
		p.pending.Done()
		return false
	} else {
		return true
	}
}

func (p *publisher[T]) Subscribe() (Subscriber[T], error) {
	return p.SubscribeBufSize(DefaultSubscriberBufSize)
}

func (p *publisher[T]) SubscribeBufSize(bufSize int) (Subscriber[T], error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil, ErrPublisherClosed
	}
	s := newSubscriber[T](bufSize)
	p.subscribers.Add(s)
	return s, nil
}

func (p *publisher[T]) unsubscribe(s *subscriber[T]) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.subscribers.Remove(s)
}

// Close idempotently shuts down the publisher, closing all subscribers too.
func (p *publisher[T]) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	// Did we already do this?
	if p.closed {
		return
	}
	// Close the send channel, and wait for the channel to be flushed
	p.ch.Close()
	p.pending.Wait()
	p.running.Wait()
	// Close all subscribers, waiting for each one to end
	subscribers := p.subscribers.ToSlice()
	p.subscribers.Clear()
	for _, s := range subscribers {
		s.Close()
	}
	// Finally, record the publisher as closed
	p.closed = true
}

func (p *publisher[T]) Closed() <-chan struct{} {
	return p.ch.Closed()
}

type subscriber[T any] struct {
	channel[T]
}

func newSubscriber[T any](bufSize int) *subscriber[T] {
	return &subscriber[T]{newChannel[T](bufSize)}
}
