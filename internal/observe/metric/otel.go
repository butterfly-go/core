package metric

import (
	"fmt"
	"log/slog"
	"net/http"

	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
)

var (
	registry = prom.NewRegistry()
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
	go serveMetrics()
	return nil
}

func serveMetrics() {
	slog.Info("serving metrics", "addr", "localhost:2223", "path", "/metrics")
	http.Handle("/metrics", promhttp.InstrumentMetricHandler(
		registry,
		promhttp.HandlerFor(registry, promhttp.HandlerOpts{})),
	)
	err := http.ListenAndServe(":2223", nil) //nolint:gosec // Ignoring G114: Use of net/http serve function that has no support for setting timeouts.
	if err != nil {
		slog.Error("error serving metrics", "error", err.Error())
		return
	}
}
