package tracing

import (
	"context"
	"fmt"
	"log/slog"

	"butterfly.orx.me/core/internal/arg"
	"butterfly.orx.me/core/internal/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func NewTracerProvider(ctx context.Context) (*otlptrace.Exporter, error) {
	provider := arg.String("tracing-provider")
	switch provider {
	default:
		return newGRPCExporter(ctx)
	}
}

func newHTTPTraceExporter(ctx context.Context) (*otlptrace.Exporter, error) {
	endpoint := arg.String("tracing-endpoint")
	traceExporter, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpoint(endpoint))
	return traceExporter, err
}

func newGRPCExporter(ctx context.Context) (*otlptrace.Exporter, error) {
	endpoint := arg.String("tracing-endpoint")
	conn, err := grpc.Dial(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	return traceExporter, err
}

func Init(ctx context.Context) error {

	res, err := resource.New(ctx,
		resource.WithAttributes(
			// the service name used to display traces in backends
			semconv.ServiceName(runtime.Service()),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	// Set up a trace exporter
	traceExporter, err := NewTracerProvider(ctx)
	if err != nil {
		slog.Error("failed to create trace exporter", "error", err.Error())
		return err
	}

	// Register the trace exporter with a TracerProvider, using a batch
	// span processor to aggregate spans before export.
	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)
	otel.SetTracerProvider(tracerProvider)

	// set global propagator to tracecontext (the default is no-op).
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Shutdown will flush any remaining spans and shut down the exporter.
	return err
}
