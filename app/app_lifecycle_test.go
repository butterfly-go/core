package app

import (
	"errors"
	"reflect"
	"testing"

	"butterfly.orx.me/core/internal/bootstrap"
)

func TestCloseRunsTeardownInReverseOrderAndCleanupOnce(t *testing.T) {
	var order []string
	app := &application{
		config: &Config{
			TeardownFunc: []func() error{
				func() error {
					order = append(order, "teardown-1")
					return nil
				},
				func() error {
					order = append(order, "teardown-2")
					return nil
				},
			},
		},
		cleanup: func() {
			order = append(order, "cleanup")
		},
		deps: &bootstrap.Dependencies{},
	}

	if err := app.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := app.Close(); err != nil {
		t.Fatalf("Close() second call error = %v", err)
	}

	want := []string{"teardown-2", "teardown-1", "cleanup"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("Close() order = %v, want %v", order, want)
	}
	if app.deps != nil {
		t.Fatal("expected deps to be cleared after Close")
	}
}

func TestCloseReturnsJoinedTeardownErrors(t *testing.T) {
	wantErr := errors.New("teardown failed")
	app := &application{
		config: &Config{
			TeardownFunc: []func() error{
				func() error { return wantErr },
			},
		},
	}

	err := app.Close()
	if !errors.Is(err, wantErr) {
		t.Fatalf("Close() error = %v, want joined error containing %v", err, wantErr)
	}
}
