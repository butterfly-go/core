package log

import (
	"log/slog"
	"os"
	"strings"

	"butterfly.orx.me/core/mod"
)

func Init(cfg mod.LogConfig) {
	level := parseLevel(cfg.Level)
	addSource := true
	if cfg.AddSource != nil {
		addSource = *cfg.AddSource
	}
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: addSource,
	}

	var handler slog.Handler
	switch strings.ToLower(cfg.Format) {
	case "json":
		handler = slog.NewJSONHandler(os.Stderr, opts)
	default:
		handler = slog.NewTextHandler(os.Stderr, opts)
	}

	slog.SetDefault(slog.New(handler))
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// CoreLogger New Logger for core
func CoreLogger(component string) *slog.Logger {
	logger := slog.Default()
	return logger.With("component", component)
}
