# Butterfly

A Go microservice framework. You supply routes, gRPC services and settings. Butterfly provides the rest of what a production service needs: configuration, tracing, metrics, structured logging, data store clients, health checks and graceful shutdown.

```go
package main

import (
    "butterfly.orx.me/core/app"
    "github.com/gin-gonic/gin"
)

func main() {
    app.New(&app.Config{
        Service: "hello",
        Router: func(r *gin.Engine) {
            r.GET("/ping", func(c *gin.Context) { c.JSON(200, gin.H{"message": "pong"}) })
        },
    }).Run()
}
```

## What you get

- **HTTP and gRPC servers** ([Gin](https://github.com/gin-gonic/gin), grpc-go, optional Twirp), each with tracing, request ids, access or call logs and panic recovery already installed.
- **Configuration** from a YAML file or Consul, with your own settings in the same document.
- **Stores** configured by name: Redis, MySQL and PostgreSQL (`database/sql` and GORM), MongoDB, S3-compatible object storage. Instrumented, health-checked and closed on shutdown.
- **Observability**: OpenTelemetry tracing, Prometheus metrics, `log/slog` logging with trace correlation.
- **Operations**: `/healthz` and `/readyz` plus gRPC health on a separate admin port, and a graceful shutdown that drains traffic and releases resources in order.

## Requirements

Go 1.26 or newer.

```bash
go get butterfly.orx.me/core
```

## Try it

Save the program above as `main.go`, then run it with a local config file and tracing off:

```bash
go mod init hello && go mod tidy

echo '{}' > config.yaml
export BUTTERFLY_CONFIG_TYPE=file
export BUTTERFLY_CONFIG_FILE_PATH=./config.yaml
export BUTTERFLY_TRACING_DISABLE=true

go run .
```

```bash
curl localhost:8080/ping      # your API
curl localhost:2223/readyz    # readiness, on the admin port
curl localhost:2223/metrics   # Prometheus metrics
```

Press Ctrl-C and the service drains in-flight requests and shuts down cleanly.

The default configuration backend is Consul, so without the `BUTTERFLY_CONFIG_*` variables above the service tries to read its configuration from there.

## Documentation

| Topic | |
|---|---|
| [Application](docs/application.md) | `app.Config`, startup order, graceful shutdown |
| [Configuration](docs/configuration.md) | Environment variables, config file, core config reference, your own settings |
| [HTTP](docs/http.md) | Gin integration, built-in middleware, Twirp |
| [gRPC](docs/grpc.md) | gRPC server, built-in behavior, interceptors |
| [Storage](docs/storage.md) | Redis, SQL, GORM, MongoDB, S3 |
| [Observability](docs/observability.md) | Metrics, health checks, tracing, logging |
| [Testing](docs/testing.md) | Testing handlers, capturing logs |
| [Deployment](docs/deployment.md) | Docker, Kubernetes, rolling updates |
| [Dependency injection](docs/wire.md) | Using Google Wire with Butterfly |
| [Example](docs/examples.md) | A complete user service |
| [FAQ](docs/faq.md) | Common questions |
