package config

import (
	"context"
	"testing"

	iconfig "butterfly.orx.me/core/internal/config"
)

type mockBackend struct {
	data []byte
	err  error
}

func (m *mockBackend) Get(_ context.Context, _ string) ([]byte, error) {
	return m.data, m.err
}

func TestGet(t *testing.T) {
	mock := &mockBackend{data: []byte("hello: world")}
	iconfig.SetConfig(mock)

	data, err := Get(context.Background(), "test-key")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if string(data) != "hello: world" {
		t.Fatalf("expected 'hello: world', got %q", string(data))
	}
}

func TestGet_Error(t *testing.T) {
	mock := &mockBackend{err: context.DeadlineExceeded}
	iconfig.SetConfig(mock)

	_, err := Get(context.Background(), "test-key")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
