package async

import (
	"sync"

	"github.com/alanbriolat/video-archiver/generic"
)

type simpleCallback[T any] func() T
type resultCallback[T any] func() (T, error)

// Run will run a function in a goroutine, returning its result via a channel.
func Run[T any](f simpleCallback[T]) <-chan T {
	return RunInGroup(&DefaultRunGroup, f)
}

func RunResult[T any](f resultCallback[T]) <-chan generic.Result[T] {
	return RunResultInGroup(&DefaultRunGroup, f)
}

type RunGroup struct {
	wg sync.WaitGroup
}

func (g *RunGroup) Wait() {
	g.wg.Wait()
}

func RunInGroup[T any](g *RunGroup, f simpleCallback[T]) <-chan T {
	c := make(chan T)
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		c <- f()
	}()
	return c
}

func RunResultInGroup[T any](g *RunGroup, f resultCallback[T]) <-chan generic.Result[T] {
	return RunInGroup(g, wrapResult(f))
}

var DefaultRunGroup RunGroup

func wrapResult[T any](f resultCallback[T]) simpleCallback[generic.Result[T]] {
	return func() generic.Result[T] {
		return generic.NewResult(f())
	}
}
