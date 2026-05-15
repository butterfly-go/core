package tracing

import (
	"context"
	"fmt"
	"log/slog"

	"butterfly.orx.me/core/internal/arg"
	"butterfly.orx.me/core/internal/config"
	"butterfly.orx.me/core/internal/runtime"
	"butterfly.orx.me/core/mod"
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
	case "http":
		return newHTTPTraceExporter(ctx)
	default:
		return newGRPCExporter(ctx)
	}
}

func newHTTPTraceExporter(ctx context.Context) (*otlptrace.Exporter, error) {
	endpoint := arg.String("tracing-endpoint")
	slog.Info("tracing http endpoint", "endpoint", endpoint)
	traceExporter, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpoint(endpoint))
	return traceExporter, err
}

func newGRPCExporter(ctx context.Context) (*otlptrace.Exporter, error) {
	endpoint := arg.String("tracing-endpoint")
	slog.Info("tracing grpc endpoint", "endpoint", endpoint)
	conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	return traceExporter, err
}

func Init() error {
	if arg.Bool("tracing-disable") {
		slog.Info("tracing is disabled, skipping initialization")
		return nil
	}

	ctx := context.Background()
	res, err := resource.New(ctx,
		resource.WithAttributes(
			// the service name used to display traces in backends
			semconv.ServiceName(runtime.Service()),
		),
	)
	if err != nil {
		slog.Error("failed to create resource", "error", err.Error())
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
	sampler := traceSamplerFromConfig(config.CoreConfig().Otel)
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sampler),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)
	otel.SetTracerProvider(tracerProvider)

	// set global propagator to tracecontext (the default is no-op).
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return nil
}

func traceSamplerFromConfig(cfg mod.OtelConfig) sdktrace.Sampler {
	ratio := 1.0
	if cfg.TraceSampleRatio != nil {
		ratio = *cfg.TraceSampleRatio
	}
	switch {
	case ratio <= 0:
		return sdktrace.NeverSample()
	case ratio >= 1:
		return sdktrace.AlwaysSample()
	default:
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))
	}
}
