package pubsub

import (
	"errors"
	"sync"

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
	AddSubscriber(s SenderCloser[T], close bool) error
	Subscribe() (ReceiverCloser[T], error)
	SubscribeBufSize(int) (ReceiverCloser[T], error)
}

type subscriberMap[T any] map[SenderCloser[T]]bool

type publisher[T any] struct {
	mu          sync.Mutex
	ch          Channel[T]
	running     sync.WaitGroup // Goroutines in progress
	pending     sync.WaitGroup // Messages not yet sent to all subscribers
	subscribers *sync_.Mutexed[subscriberMap[T]]
	closed      bool
}

func NewPublisher[T any]() Publisher[T] {
	return NewPublisherBufSize[T](DefaultPublisherBufSize)
}

func NewPublisherBufSize[T any](bufSize int) Publisher[T] {
	p := &publisher[T]{
		ch:          NewChannel[T](bufSize),
		subscribers: sync_.NewMutexed[subscriberMap[T]](make(subscriberMap[T])),
	}
	p.running.Add(1)
	go func() {
		defer p.running.Done()
		for v := range p.ch.Receive() {
			// Get the latest set of subscribers, to avoid holding a lock that prevents adding new subscribers
			var subscriberSlice []SenderCloser[T]
			_ = p.subscribers.Locked(func(subscribers subscriberMap[T]) error {
				subscriberSlice = make([]SenderCloser[T], 0, len(subscribers))
				for s, _ := range subscribers {
					subscriberSlice = append(subscriberSlice, s)
				}
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
	if err := p.AddSubscriber(s, true); err != nil {
		return nil, err
	} else {
		return s, nil
	}
}

// AddSubscriber registers a SenderCloser to receive messages from the Publisher. If `subscriber` is closed while the
// Publisher is not, it will automatically become unsubscribed. If `close` is true, then when the Publisher is closed,
// `subscriber` will also be closed. If `subscriber` is already subscribed, the `close` flag will be updated.
func (p *publisher[T]) AddSubscriber(subscriber SenderCloser[T], close bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return ErrPublisherClosed
	}
	return p.subscribers.Locked(func(subscribers subscriberMap[T]) error {
		subscribers[subscriber] = close
		return nil
	})
}

func (p *publisher[T]) unsubscribe(s SenderCloser[T]) {
	_ = p.subscribers.Locked(func(subscribers subscriberMap[T]) error {
		if _, ok := subscribers[s]; ok {
			delete(subscribers, s)
		}
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
	subscribers := p.subscribers.Swap(nil)
	for subscriber, closeSubscriber := range subscribers {
		if closeSubscriber {
			subscriber.Close()
		}
	}
	// Finally, record the publisher as closed
	p.closed = true
}

func (p *publisher[T]) Closed() <-chan struct{} {
	return p.ch.Closed()
}
