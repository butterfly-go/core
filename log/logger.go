package log

import (
	"context"
	"log/slog"
)

type loggerContextKey struct{}

func FromContext(ctx context.Context) *slog.Logger {
	return slog.Default()
}
