package pubsub

import (
	"errors"
	"sync"

	"github.com/alanbriolat/video-archiver/generic"
	"github.com/alanbriolat/video-archiver/internal/sync_"
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
	AddSubscriber(SenderCloser[T]) error
	Subscribe() (ReceiverCloser[T], error)
	SubscribeBufSize(int) (ReceiverCloser[T], error)
}

type publisher[T any] struct {
	mu          sync.Mutex
	ch          Channel[T]
	running     sync.WaitGroup // Goroutines in progress
	pending     sync.WaitGroup // Messages not yet sent to all subscribers
	subscribers *sync_.Mutexed[generic.Set[SenderCloser[T]]]
	closed      bool
}

func NewPublisher[T any]() Publisher[T] {
	return NewPublisherBufSize[T](DefaultPublisherBufSize)
}

func NewPublisherBufSize[T any](bufSize int) Publisher[T] {
	p := &publisher[T]{
		ch:          NewChannel[T](bufSize),
		subscribers: sync_.NewMutexed[generic.Set[SenderCloser[T]]](generic.NewPolymorphicSet[SenderCloser[T]]()),
	}
	p.running.Add(1)
	go func() {
		defer p.running.Done()
		for v := range p.ch.Receive() {
			// Get the latest set of subscribers, to avoid holding a lock that prevents adding new subscribers
			var subscriberSlice []SenderCloser[T]
			_ = p.subscribers.Locked(func(subscribers generic.Set[SenderCloser[T]]) error {
				subscriberSlice = subscribers.ToSlice()
				return nil
			})
			// Attempt to send to each subscriber
			for _, s := range subscriberSlice {
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

func (p *publisher[T]) Subscribe() (ReceiverCloser[T], error) {
	return p.SubscribeBufSize(DefaultSubscriberBufSize)
}

func (p *publisher[T]) SubscribeBufSize(bufSize int) (ReceiverCloser[T], error) {
	s := NewChannel[T](bufSize)
	if err := p.AddSubscriber(s); err != nil {
		return nil, err
	} else {
		return s, nil
	}
}

func (p *publisher[T]) AddSubscriber(s SenderCloser[T]) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return ErrPublisherClosed
	}
	return p.subscribers.Locked(func(subscribers generic.Set[SenderCloser[T]]) error {
		subscribers.Add(s)
		return nil
	})
}

func (p *publisher[T]) unsubscribe(s SenderCloser[T]) {
	_ = p.subscribers.Locked(func(subscribers generic.Set[SenderCloser[T]]) error {
		subscribers.Remove(s)
		return nil
	})
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
	var subscriberSlice []SenderCloser[T]
	_ = p.subscribers.Locked(func(subscribers generic.Set[SenderCloser[T]]) error {
		subscriberSlice = subscribers.ToSlice()
		subscribers.Clear()
		return nil
	})
	for _, s := range subscriberSlice {
		s.Close()
	}
	// Finally, record the publisher as closed
	p.closed = true
}

func (p *publisher[T]) Closed() <-chan struct{} {
	return p.ch.Closed()
}
