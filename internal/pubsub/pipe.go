package pubsub

import "sync"

const (
	DefaultPipeBufSize = 1
)

type Pipe[T any] interface {
	Closer
}

type pipe[T any] struct {
	mu      sync.Mutex
	in      ReceiverCloser[T]
	out     SenderCloser[T]
	done    chan struct{}
	closed  bool
	running sync.WaitGroup
}

// NewPipe creates a new Pipe from new input and output channels, using DefaultPipeBufSize for the output.
func NewPipe[T any]() (in SenderCloser[T], out ReceiverCloser[T], p Pipe[T]) {
	return NewPipeBufSize[T](DefaultPipeBufSize)
}

// NewPipeBufSize creates a new Pipe with new input and output channels, using the specified buffer size for the output.
func NewPipeBufSize[T any](bufSize int) (in SenderCloser[T], out ReceiverCloser[T], p Pipe[T]) {
	cIn := NewChannel[T](0)
	cOut := NewChannel[T](bufSize)
	p = NewPipeChannels[T](cIn, cOut)
	return cIn, cOut, p
}

// NewPipeChannels creates a new Pipe from existing channels.
//
// Note that buffered input channels can cause unexpected behaviour, such as the `in` channel accepting Send() calls
// when the `out` channel is already closed.
func NewPipeChannels[T any](in ReceiverCloser[T], out SenderCloser[T]) Pipe[T] {
	p := &pipe[T]{
		in:   in,
		out:  out,
		done: make(chan struct{}),
	}
	p.running.Add(1)
	go func() {
		// Ensure both sides of the pipe are closed once the goroutine exits
		defer p.Close()
		// Allow Close() to wait for the goroutine to exit (i.e. if called from elsewhere)
		defer p.running.Done()
		for {
			select {
			case msg, ok := <-p.in.Receive():
				if !ok {
					// Input channel closed before there was anything to read
					return
				}
				ok = p.out.Send(msg)
				if !ok {
					// Output channel closed before we could send
					return
				}
			case <-p.out.Closed():
				// Output channel closed before we could read from input channel
				return
			case <-p.Closed():
				// Pipe is being closed directly
				return
			}
		}
	}()
	return p
}

func (p *pipe[T]) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return
	}
	close(p.done)
	p.in.Close()
	p.out.Close()
	p.running.Wait()
	p.closed = true
}

func (p *pipe[T]) Closed() <-chan struct{} {
	return p.done
}
