package log

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"butterfly.orx.me/core/mod"
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
}
