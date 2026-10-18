package log

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"butterfly.orx.me/core/internal/redact"
	"butterfly.orx.me/core/mod"
	"go.opentelemetry.io/otel/trace"
)

func Init(cfg mod.LogConfig) {
	level := parseLevel(cfg.Level)
	addSource := true
	if cfg.AddSource != nil {
		addSource = *cfg.AddSource
	}
	opts := &slog.HandlerOptions{
		Level:       level,
		AddSource:   addSource,
		ReplaceAttr: redactAttr,
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

// redactAttr is a safety net for every log line the global logger writes:
// values under credential-like keys ("password", "token", "dsn", ...) are
// masked, and the password of any URI or DSN inside a string or error value is
// scrubbed. Call sites still avoid logging secrets in the first place.
func redactAttr(groups []string, a slog.Attr) slog.Attr {
	if len(groups) == 0 && a.Key == slog.SourceKey {
		return a
	}
	v := a.Value.Resolve()
	if v.Kind() == slog.KindGroup {
		return a
	}
	if redact.IsSensitiveKey(a.Key) {
		if v.Kind() == slog.KindString && v.String() == "" {
			return a
		}
		return slog.String(a.Key, redact.Mask)
	}
	switch v.Kind() {
	case slog.KindString:
		if s := v.String(); s != "" {
			a.Value = slog.StringValue(redact.String(s))
		}
	case slog.KindAny:
		if err, ok := v.Any().(error); ok && err != nil {
			a.Value = slog.StringValue(redact.String(err.Error()))
		}
	}
	return a
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

type loggerContextKey struct{}

// Attribute keys added by FromContext when the context carries a valid span.
const (
	TraceIDKey = "trace_id"
	SpanIDKey  = "span_id"
)

// FromContext returns the logger attached with WithLogger, or slog.Default().
//
// If ctx carries a valid OpenTelemetry span, the returned logger also has
// trace_id and span_id attributes, so log lines can be joined with traces. The
// ids are read when FromContext is called, so call it again after starting a
// child span to log with the child's span_id. Without a span, the logger is
// returned unchanged.
func FromContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return slog.Default()
	}
	logger := slog.Default()
	if v := ctx.Value(loggerContextKey{}); v != nil {
		if l, ok := v.(*slog.Logger); ok && l != nil {
			logger = l
		}
	}
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		return logger.With(
			TraceIDKey, sc.TraceID().String(),
			SpanIDKey, sc.SpanID().String(),
		)
	}
	return logger
}

func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey{}, logger)
}
