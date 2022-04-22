package pubsub

import "sync"

const (
	DefaultPipeBufSize = 1
)

type PipeOptions struct {
	// Size of the pipe input buffer (if not using NewPipeChannels); note that an input buffer size greater than 0 may
	// cause unexpected behaviour, such as Send() succeeding for messages that never get received.
	InputBufSize int
	// Size of the pipe output buffer (if not using NewPipeChannels).
	OutputBufSize int
	// Close the input channel when the pipe closes.
	CloseInput bool
	// Close the output channel when the pipe closes.
	CloseOutput bool
}

var DefaultPipeOptions = PipeOptions{
	InputBufSize:  0,
	OutputBufSize: 1,
	CloseInput:    true,
	CloseOutput:   true,
}

type Pipe[T any] interface {
	Closer
}

type pipe[T any] struct {
	opts    PipeOptions
	mu      sync.Mutex
	in      ReceiverCloser[T]
	out     SenderCloser[T]
	done    chan struct{}
	closed  bool
	running sync.WaitGroup
}

// NewPipe creatse a new Pipe with new input and output channels using DefaultPipeOptions.
func NewPipe[T any]() (in SenderCloser[T], out ReceiverCloser[T], p Pipe[T]) {
	return NewPipeOptions[T](DefaultPipeOptions)
}

// NewPipeOptions creates a new Pipe with new input and output channels according to the supplied options.
func NewPipeOptions[T any](opts PipeOptions) (in SenderCloser[T], out ReceiverCloser[T], p Pipe[T]) {
	cIn := NewChannel[T](opts.InputBufSize)
	cOut := NewChannel[T](opts.OutputBufSize)
	p = NewPipeChannelsOptions[T](cIn, cOut, opts)
	return cIn, cOut, p
}

// NewPipeChannels creates a new Pipe from existing channels using DefaultPipeOptions. See PipeOptions documentation for
// the consequences of the input channel having a non-zero buffer size.
func NewPipeChannels[T any](in ReceiverCloser[T], out SenderCloser[T]) Pipe[T] {
	return NewPipeChannelsOptions[T](in, out, DefaultPipeOptions)
}

// NewPipeChannelsOptions creates a new Pipe from existing channels according to the supplied PipeOptions. See
// PipeOptions documentation for the consequences of the input channel having a non-zero buffer size.
func NewPipeChannelsOptions[T any](in ReceiverCloser[T], out SenderCloser[T], opts PipeOptions) Pipe[T] {
	p := &pipe[T]{
		opts: opts,
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
	if p.opts.CloseInput {
		p.in.Close()
	}
	if p.opts.CloseOutput {
		p.out.Close()
	}
	p.running.Wait()
	p.closed = true
}

func (p *pipe[T]) Closed() <-chan struct{} {
	return p.done
}
