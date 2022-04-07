package video_archiver

import (
	"context"
	"io"
)

// A context-aware io.Reader wrapper.
type readerContext struct {
	ctx context.Context
	r   io.Reader
}

func (r *readerContext) Read(p []byte) (n int, err error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}
