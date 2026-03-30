package log

import (
	"log/slog"
	"testing"
)

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
		cfg  LogConfig
	}{
		{
			name: "json format with debug level and source",
			cfg:  LogConfig{Level: "debug", Format: "json", AddSource: true},
		},
		{
			name: "text format with error level",
			cfg:  LogConfig{Level: "error", Format: "text", AddSource: false},
		},
		{
			name: "default format and level",
			cfg:  LogConfig{},
		},
		{
			name: "unknown format falls back to text",
			cfg:  LogConfig{Format: "xml"},
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
