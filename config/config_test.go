package config

import (
	"context"
	"testing"
)

type mockBackend struct {
	data []byte
	err  error
}

func (m *mockBackend) Get(_ context.Context, _ string) ([]byte, error) {
	return m.data, m.err
}

func TestSetAndGet(t *testing.T) {
	mock := &mockBackend{data: []byte("hello: world")}
	Set(mock)

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
	Set(mock)

	_, err := Get(context.Background(), "test-key")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
