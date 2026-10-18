package log

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"butterfly.orx.me/core/mod"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func boolPtr(b bool) *bool { return &b }

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"INFO", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"ERROR", slog.LevelError},
		{"", slog.LevelInfo},
		{"invalid", slog.LevelInfo},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseLevel(tt.input)
			if got != tt.want {
				t.Errorf("parseLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestInit(t *testing.T) {
	tests := []struct {
		name string
		cfg  mod.LogConfig
	}{
		{
			name: "json format with debug level and source",
			cfg:  mod.LogConfig{Level: "debug", Format: "json", AddSource: boolPtr(true)},
		},
		{
			name: "text format with error level",
			cfg:  mod.LogConfig{Level: "error", Format: "text", AddSource: boolPtr(false)},
		},
		{
			name: "default format and level (AddSource defaults to true)",
			cfg:  mod.LogConfig{},
		},
		{
			name: "unknown format falls back to text",
			cfg:  mod.LogConfig{Format: "xml"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Init(tt.cfg)
			// Verify slog.Default() was updated and is usable
			logger := slog.Default()
			if logger == nil {
				t.Fatal("slog.Default() returned nil after Init")
			}
			logger.Info("test message", "test_key", "test_value")
		})
	}
}

func TestFromContext(t *testing.T) {
	t.Run("nil context uses default", func(t *testing.T) {
		def := slog.Default()
		//nolint:staticcheck // SA1012: nil context is intentionally passed to verify the fallback
		got := FromContext(nil)
		if got != def {
			t.Fatalf("FromContext(nil) = %p, want default %p", got, def)
		}
	})

	t.Run("empty context uses default", func(t *testing.T) {
		def := slog.Default()
		got := FromContext(context.Background())
		if got != def {
			t.Fatalf("FromContext(bg) = %p, want default %p", got, def)
		}
	})

	t.Run("returns logger attached with WithLogger", func(t *testing.T) {
		var buf bytes.Buffer
		h := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
		custom := slog.New(h)
		ctx := WithLogger(context.Background(), custom)
		FromContext(ctx).Info("hello", "k", "v")
		var m map[string]any
		if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
			t.Fatalf("log line: %q", buf.String())
		}
		if m["msg"] != "hello" || m["k"] != "v" {
			t.Fatalf("unexpected payload: %v", m)
		}
	})

	t.Run("returns the exact logger attached", func(t *testing.T) {
		custom := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))
		if got := FromContext(WithLogger(context.Background(), custom)); got != custom {
			t.Fatalf("FromContext = %p, want %p", got, custom)
		}
	})

	t.Run("nil logger attached uses default", func(t *testing.T) {
		def := slog.Default()
		ctx := WithLogger(context.Background(), nil)
		if got := FromContext(ctx); got != def {
			t.Fatalf("FromContext = %p, want default %p", got, def)
		}
	})

	t.Run("value of unexpected type uses default", func(t *testing.T) {
		def := slog.Default()
		ctx := context.WithValue(context.Background(), loggerContextKey{}, "not a logger")
		if got := FromContext(ctx); got != def {
			t.Fatalf("FromContext = %p, want default %p", got, def)
		}
	})
}

func TestFromContextAddsTraceIDs(t *testing.T) {
	traceID, _ := trace.TraceIDFromHex("0102030405060708090a0b0c0d0e0f10")
	spanID, _ := trace.SpanIDFromHex("1112131415161718")
	spanCtx := func(ctx context.Context, sc trace.SpanContext) context.Context {
		return trace.ContextWithSpanContext(ctx, sc)
	}
	valid := trace.NewSpanContext(trace.SpanContextConfig{TraceID: traceID, SpanID: spanID, TraceFlags: trace.FlagsSampled})

	logLine := func(ctx context.Context) map[string]any {
		t.Helper()
		var buf bytes.Buffer
		ctx = WithLogger(ctx, slog.New(slog.NewJSONHandler(&buf, nil)))
		FromContext(ctx).Info("hello", "k", "v")
		var m map[string]any
		if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
			t.Fatalf("log line %q: %v", buf.String(), err)
		}
		return m
	}

	t.Run("valid span adds ids and keeps other attrs", func(t *testing.T) {
		m := logLine(spanCtx(context.Background(), valid))
		if m[TraceIDKey] != traceID.String() || m[SpanIDKey] != spanID.String() || m["k"] != "v" {
			t.Fatalf("payload = %v", m)
		}
	})

	t.Run("works with the default logger too", func(t *testing.T) {
		var buf bytes.Buffer
		old := slog.Default()
		slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
		t.Cleanup(func() { slog.SetDefault(old) })
		FromContext(spanCtx(context.Background(), valid)).Info("hi")
		if !bytes.Contains(buf.Bytes(), []byte(traceID.String())) {
			t.Fatalf("default logger line lacks trace id: %s", buf.String())
		}
	})

	t.Run("no span leaves the logger untouched", func(t *testing.T) {
		m := logLine(context.Background())
		if _, ok := m[TraceIDKey]; ok {
			t.Fatalf("unexpected trace_id: %v", m)
		}
		custom := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))
		if got := FromContext(WithLogger(context.Background(), custom)); got != custom {
			t.Fatal("logger must be returned as-is when there is no span")
		}
	})

	t.Run("invalid span context adds nothing", func(t *testing.T) {
		m := logLine(spanCtx(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{})))
		if _, ok := m[TraceIDKey]; ok {
			t.Fatalf("unexpected trace_id: %v", m)
		}
	})

	t.Run("child span gets its own span_id", func(t *testing.T) {
		tp := sdktrace.NewTracerProvider()
		ctx, parent := tp.Tracer("t").Start(context.Background(), "parent")
		defer parent.End()
		parentLine := logLine(ctx)
		ctx, child := tp.Tracer("t").Start(ctx, "child")
		defer child.End()
		childLine := logLine(ctx)
		if parentLine[TraceIDKey] != childLine[TraceIDKey] {
			t.Fatal("parent and child must share a trace_id")
		}
		if parentLine[SpanIDKey] == childLine[SpanIDKey] {
			t.Fatal("child must have a different span_id")
		}
	})
}

func TestInitRedactsSecrets(t *testing.T) {
	var buf bytes.Buffer
	opts := &slog.HandlerOptions{ReplaceAttr: redactAttr}
	logger := slog.New(slog.NewJSONHandler(&buf, opts))

	logger.Info("connect to postgres://app:hunter2@db:5432/app",
		"password", "hunter2",
		"empty_password", "",
		"dsn", "root:hunter2@tcp(db:3306)/app",
		"uri", "mongodb://u:hunter2@h/db",
		"error", errors.New("open postgres://u:hunter2@h/db failed"),
		"addr", "db:5432",
		slog.Group("db", slog.String("api_key", "hunter2"), slog.String("name", "app")),
	)

	out := buf.String()
	if strings.Contains(out, "hunter2") {
		t.Fatalf("secret leaked into log output: %s", out)
	}
	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatal(err)
	}
	if rec["addr"] != "db:5432" {
		t.Fatalf("non-secret attribute altered: %v", rec["addr"])
	}
	if rec["empty_password"] != "" {
		t.Fatalf("empty secret should stay empty: %v", rec["empty_password"])
	}
	if got := rec["db"].(map[string]any)["name"]; got != "app" {
		t.Fatalf("group attribute altered: %v", got)
	}
}
