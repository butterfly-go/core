# Observability

Metrics, health checks, tracing and logging. Metrics and health checks are served by the admin server, a separate listener from your HTTP API so that it never needs to be exposed publicly.

## Prometheus Metrics

The framework automatically exposes the `/metrics` endpoint on port 2223 (change it with `server.admin.addr`):

```go
import (
    "butterfly.orx.me/core/observe/otel"
    "github.com/prometheus/client_golang/prometheus"
)

// Get Prometheus registry
registry := otel.PrometheusRegistry()

// Register custom metrics
var (
    requestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "http_request_duration_seconds",
            Help: "HTTP request latencies in seconds.",
        },
        []string{"method", "endpoint", "status"},
    )
    
    activeUsers = prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "active_users_total",
            Help: "Number of active users.",
        },
    )
)

func init() {
    registry.MustRegister(requestDuration)
    registry.MustRegister(activeUsers)
}

// Use metrics
func measureRequest(c *gin.Context) {
    start := time.Now()
    
    c.Next()
    
    duration := time.Since(start).Seconds()
    requestDuration.WithLabelValues(
        c.Request.Method,
        c.FullPath(),
        fmt.Sprintf("%d", c.Writer.Status()),
    ).Observe(duration)
}
```

Access metrics:
```bash
curl http://localhost:2223/metrics
```

## Health Checks

The admin server (default `:2223`, see `server.admin.addr`) serves two probe endpoints next to `/metrics`:

| Endpoint | Meaning | Status |
|---|---|---|
| `GET /healthz` | Liveness: the process is up. Never looks at dependencies, so a database outage does not get the pod restarted. | Always `200` |
| `GET /readyz` | Readiness: should this instance receive traffic? | `200` or `503` |

`/readyz` depends on the lifecycle state:

- `starting`: initialization has not finished and the servers are not listening yet. Returns `503`.
- `ready`: the dependency checks run. Returns `200` if all pass, `503` if any fails.
- `draining`: shutdown has begun. Returns `503`, so load balancers stop routing here while in-flight requests finish.

```json
{"status":"fail","state":"ready","checks":{"redis/cache":{"status":"ok","duration_ms":1},"sql/main":{"status":"fail","duration_ms":2000}}}
```

Error details are written to the log (only when a check starts failing or recovers) and never put in the response, because they can contain hostnames or credentials.

**Built-in checks** cover every configured Redis (`redis/<name>`), SQL (`sql/<name>`) and Mongo (`mongo/<name>`) client. Note that Mongo and SQL are allowed to start while unreachable (a failed ping at startup is only a warning), so such an instance starts but reports not ready until the dependency comes back. S3 is not checked by default; register your own check if you need one.

**Your own checks** go in `app.Config.HealthChecks`. Names must be unique:

```go
config := &app.Config{
    Service: "my-service",
    HealthChecks: []app.HealthCheck{{
        Name: "payments-api",
        Check: func(ctx context.Context) error {
            // ctx carries the per-check timeout (2s); honor it.
            return paymentsClient.Ping(ctx)
        },
    }},
}
```

Checks run concurrently, each with a 2s timeout, and a check that panics or hangs counts as failed. Results are cached for 1s, so a burst of probes costs one round of pings.

**gRPC:** when `GRPCRegister` is set, the standard `grpc.health.v1.Health` service is registered automatically and reports the same readiness (`SERVING` / `NOT_SERVING`) for every service name, including `Watch`. If your `GRPCRegister` registers its own health service, yours is kept and nothing is added.

**Zero-downtime shutdown:** on a shutdown signal the instance first goes `draining`, then stops the servers. Load balancers need a moment to notice, so you can pause in between:

```yaml
server:
  shutdown_delay: 5s   # default 0; must be smaller than ShutdownTimeout
```

During the delay the servers keep accepting traffic. The delay counts against `ShutdownTimeout` (default 30s).

## OpenTelemetry Tracing

The framework automatically configures OpenTelemetry tracing:

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
)

func processOrder(ctx context.Context, orderID string) error {
    // Create new span
    tracer := otel.Tracer("order-service")
    ctx, span := tracer.Start(ctx, "processOrder")
    defer span.End()
    
    // Add attributes
    span.SetAttributes(
        attribute.String("order.id", orderID),
        attribute.String("order.status", "processing"),
    )
    
    // Call other services
    if err := validateOrder(ctx, orderID); err != nil {
        span.RecordError(err)
        return err
    }
    
    // Add event
    span.AddEvent("order validated")
    
    return nil
}
```

Tracing is on by default and exports over OTLP. Pick the exporter with environment variables (see [Configuration](configuration.md#environment-variables)):

```bash
# OTLP over HTTP
export BUTTERFLY_TRACING_PROVIDER=http
export BUTTERFLY_TRACING_ENDPOINT=localhost:4318

# OTLP over gRPC (the default provider)
export BUTTERFLY_TRACING_PROVIDER=grpc
export BUTTERFLY_TRACING_ENDPOINT=localhost:4317

# No tracing, for local development
export BUTTERFLY_TRACING_DISABLE=true
```

Sampling is controlled by `otel.trace_sample_ratio` in the config file. See [Configuration](configuration.md#otel). HTTP requests, gRPC calls, Redis, SQL, GORM and Mongo are traced without any code on your side. Trace context is propagated in the W3C `traceparent` header.

## Logging

The framework provides a structured logging system based on Go's `log/slog`. Logging is automatically configured during initialization via the `log` section in the config file.

### Log configuration

Configure logging in your YAML config:

```yaml
log:
  level: "info"        # debug, info, warn/warning, error (default: info)
  format: "json"       # json or text (default: text)
  add_source: false    # include source file/line in log entries
```

### Logger in the context

Use `log.FromContext(ctx)` instead of the global `slog` functions inside handlers. It returns the logger carried by the context, or `slog.Default()` when there is none, so it never returns nil:

```go
import "butterfly.orx.me/core/log"

func handler(c *gin.Context) {
    logger := log.FromContext(c.Request.Context())
    logger.Info("handling request", "path", c.Request.URL.Path)
    // {"msg":"handling request","request_id":"...","trace_id":"...","span_id":"...","path":"/users"}
}
```

For HTTP requests the built-in middleware has already put a logger with `request_id` into the context (see [HTTP](http.md#gin-framework-integration)), and `FromContext` adds the trace ids. In gRPC handlers the context carries the trace ids as well.

To add your own fields for the rest of a call, attach an extended logger and pass the context on:

```go
func processOrder(ctx context.Context, order *Order) error {
    logger := log.FromContext(ctx).With("order_id", order.ID)
    ctx = log.WithLogger(ctx, logger)

    return charge(ctx, order) // log.FromContext(ctx) in charge includes order_id
}
```

### Correlating logs with traces

When the context carries an OpenTelemetry span, `log.FromContext(ctx)` adds `trace_id` and `span_id` to the logger, so you can jump from a log line to its trace (and back). This works for HTTP requests (via the built-in `otelgin` middleware) and gRPC calls, and for any span you start yourself:

```go
func handler(c *gin.Context) {
    ctx := c.Request.Context()
    log.FromContext(ctx).Info("start")           // trace_id=T span_id=A

    ctx, span := tracer.Start(ctx, "load-profile")
    defer span.End()
    log.FromContext(ctx).Info("loading profile") // trace_id=T span_id=B (the child span)
}
```

The ids are read when you call `FromContext`, so call it again after starting a child span. A logger created earlier, or one obtained from `slog.Default()` directly, will not pick up the new span. Without a span in the context, `FromContext` returns the logger unchanged. The attribute names are available as `log.TraceIDKey` and `log.SpanIDKey`.
