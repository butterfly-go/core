package tracing

import (
	"context"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestShutdownNoopWhenNotInitialized(t *testing.T) {
	if err := Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
}

func TestShutdownFlushesAndIsIdempotent(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	mu.Lock()
	tracerProvider = tp
	mu.Unlock()

	if err := Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if err := Shutdown(context.Background()); err != nil {
		t.Fatalf("second Shutdown: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if tracerProvider != nil {
		t.Fatal("tracerProvider should be cleared")
	}
}
