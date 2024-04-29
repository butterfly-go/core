package otel

import (
	"butterfly.orx.me/core/internal/observe/metric"
	"github.com/prometheus/client_golang/prometheus"
)

func PrometheusRegistry() prometheus.Registerer {
	return metric.PrometheusRegister()
}
