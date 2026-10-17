package metric

import (
	"context"
	"testing"
)

func TestShutdownNoopWhenNotInitialized(t *testing.T) {
	if err := Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
}
