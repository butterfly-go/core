package log

import (
	"context"
	"log/slog"
)

type loggerContextKey struct{}

func FromContext(context.Context) *slog.Logger {
	return slog.Default()
}
 

func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey{}, logger)
}

