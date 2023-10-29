package log

import (
	"context"
	"log/slog"
)

type loggerContextKey struct{}

func FromContext(context.Context) *slog.Logger {
	return slog.Default()
}
