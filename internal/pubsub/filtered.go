package pubsub

func NewFilteredSender[T any](s SenderCloser[T], f func(T) bool) SenderCloser[T] {
	return &filteredSender[T]{
		SenderCloser: s,
		filter:       f,
	}
}

type filteredSender[T any] struct {
	SenderCloser[T]
	filter func(T) bool
}

func (s *filteredSender[T]) Send(msg T) bool {
	select {
	case <-s.Closed():
		return false
	default:
		if s.filter == nil || s.filter(msg) {
			return s.SenderCloser.Send(msg)
		}
		// "true" because channel is not closed, it "accepted" the message, it just dropped it
		return true
	}
}
