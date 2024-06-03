package metric

import (
	"log/slog"

	"butterfly.orx.me/core/internal/arg"
	"butterfly.orx.me/core/internal/runtime"
	"github.com/prometheus/client_golang/prometheus/push"
)

func PrometheusPush() {
	addr := arg.String("prometheus.push.endpoint")
	pusher := push.New(addr, runtime.Service()).Gatherer(registry)
	err := pusher.Push()
	if err != nil {
		slog.Error("PrometheusPush failed", "error", err.Error())
	}
}
