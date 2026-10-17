package metric

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
)

const readHeaderTimeout = 10 * time.Second

var (
	registry = prom.NewRegistry()

	mu            sync.Mutex
	meterProvider *metric.MeterProvider
	metricsServer *http.Server
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

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.InstrumentMetricHandler(
		registry,
		promhttp.HandlerFor(registry, promhttp.HandlerOpts{})),
	)
	srv := &http.Server{
		Addr:              ":2223",
		Handler:           mux,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	mu.Lock()
	meterProvider = provider
	metricsServer = srv
	mu.Unlock()

	go serveMetrics(srv)
	return nil
}

func serveMetrics(srv *http.Server) {
	slog.Info("serving metrics", "addr", "localhost:2223", "path", "/metrics")
	err := srv.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("error serving metrics", "error", err.Error())
		return
	}
}

// Shutdown stops the metrics HTTP endpoint and shuts down the OpenTelemetry
// meter provider. It is safe to call when Init was never called, and safe to
// call more than once.
func Shutdown(ctx context.Context) error {
	mu.Lock()
	srv, provider := metricsServer, meterProvider
	metricsServer, meterProvider = nil, nil
	mu.Unlock()

	var errs []error
	if srv != nil {
		if err := srv.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("shutdown metrics server: %w", err))
		}
	}
	if provider != nil {
		if err := provider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("shutdown meter provider: %w", err))
		}
	}
	return errors.Join(errs...)
}
