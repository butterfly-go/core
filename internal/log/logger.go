package log

import (
	"log/slog"
)

// CoreLogger New Logger for core
func CoreLogger(component string) *slog.Logger {
	logger := slog.Default()
	return logger.With("component", component)
}
