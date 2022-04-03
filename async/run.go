package async

// Run will run a function in a goroutine, returning its result via a channel.
func Run[T any](f func() T) <-chan T {
	c := make(chan T)
	go func() {
		c <- f()
	}()
	return c
}
