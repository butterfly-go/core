package metric

import (
	"butterfly.orx.me/core/internal/arg"
	"butterfly.orx.me/core/internal/runtime"
	"github.com/prometheus/client_golang/prometheus/push"
)

func PrometheusPush() {
	addr := arg.String("prometheus.push.endpoint")
	pusher := push.New(addr, runtime.Service()).Gatherer(registry)
	pusher.Push()
}
