package testsuite

import (
	"log/slog"
	"testing"
)

func TestMockLog_CaptureAndAssert(t *testing.T) {
	logger, mock := NewMockLog()

	logger.Info("startup", "service", "order")
	logger.Error("db failed", "retry", true)

	if !mock.ContainsMessage("startup") {
		t.Fatal("expected startup log to be captured")
	}

	if got := mock.CountLevel(slog.LevelError); got != 1 {
		t.Fatalf("expected 1 error log, got %d", got)
	}

	entries := mock.Entries()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	if got := entries[0].Attrs["service"]; got != "order" {
		t.Fatalf("expected service attr order, got %#v", got)
	}
}

func TestMockLog_SetAsDefault(t *testing.T) {
	logger, mock := NewMockLog()
	restore := mock.SetAsDefault()
	t.Cleanup(restore)

	slog.Info("from default")
	logger.Warn("from custom logger")

	messages := mock.Messages()
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
}

func TestMockLog_Reset(t *testing.T) {
	logger, mock := NewMockLog()
	logger.Info("before reset")

	mock.Reset()

	if got := len(mock.Entries()); got != 0 {
		t.Fatalf("expected no entries after reset, got %d", got)
	}
}

func TestMockLog_WithAttrsAndGroups(t *testing.T) {
	logger, mock := NewMockLog()
	logger.With("component", "worker").WithGroup("request").Info("accepted", "id", "req-1")

	entries := mock.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	if got := entries[0].Attrs["component"]; got != "worker" {
		t.Fatalf("expected component attr worker, got %#v", got)
	}

	if got := entries[0].Attrs["request.id"]; got != "req-1" {
		t.Fatalf("expected grouped attr request.id=req-1, got %#v", got)
	}
}
