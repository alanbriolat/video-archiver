package video_archiver

import (
	"context"
)

import (
	"go.uber.org/zap"
)

type contextKey int

const (
	loggerKey contextKey = iota
)

// WithLogger derives a new context that uses the specified logger.
func WithLogger(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// Logger gets the context's logger, or the global logger if none is set.
func Logger(ctx context.Context) *zap.Logger {
	if logger, ok := ctx.Value(loggerKey).(*zap.Logger); ok {
		if logger != nil {
			return logger
		}
	}
	return zap.L()
}
