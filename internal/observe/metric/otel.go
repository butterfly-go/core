package metric

import (
	"context"
	"fmt"
	"sync"

	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
)

var (
	registry = prom.NewRegistry()

	mu            sync.Mutex
	meterProvider *metric.MeterProvider
)

func PrometheusRegister() *prom.Registry {
	return registry
}

func Init() error {
	// The exporter embeds a default OpenTelemetry Reader and
	// implements prometheus.Collector, allowing it to be used as
	// both a Reader and Collector.
	exporter, err := prometheus.New(prometheus.WithRegisterer(registry))
	if err != nil {
		return fmt.Errorf("create prometheus exporter: %w", err)
	}

	// Add go runtime metrics and process collectors.
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		// requestDurations,
	)

	provider := metric.NewMeterProvider(metric.WithReader(exporter))
	otel.SetMeterProvider(provider)

	mu.Lock()
	meterProvider = provider
	mu.Unlock()
	return nil
}

// Shutdown shuts down the OpenTelemetry meter provider. The /metrics HTTP
// endpoint is served by the admin server, which is stopped separately. It is
// safe to call when Init was never called, and safe to call more than once.
func Shutdown(ctx context.Context) error {
	mu.Lock()
	provider := meterProvider
	meterProvider = nil
	mu.Unlock()

	if provider == nil {
		return nil
	}
	if err := provider.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown meter provider: %w", err)
	}
	return nil
}
