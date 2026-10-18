# Application structure and lifecycle

## `app.Config`

An application is an `app.Config` handed to `app.New(...).Run()`.

| Field | Purpose |
|---|---|
| `Service` | Service name. Used for the config key, tracing and the HTTP middleware. |
| `Namespace` | Optional prefix of the Consul key (`Namespace/Service`). See [Configuration](configuration.md). |
| `Config` | Optional pointer to your own settings struct. May have a `Validate() error` method, see [Validation](configuration.md#validation). See [Configuration](configuration.md#your-application-config). |
| `OnConfigChange` | `func(app.AppConfig)`. Called with a freshly parsed and validated `Config` when the Consul document changes. See [Reloading configuration](configuration.md#reloading-configuration). |
| `Router` | `func(*gin.Engine)`. Registers HTTP routes and middleware. See [HTTP](http.md). |
| `GRPCRegister` | `func(*grpc.Server)`. Registers gRPC services. Setting it starts the gRPC server. See [gRPC](grpc.md). |
| `GRPCUnaryInterceptors`, `GRPCStreamInterceptors` | Your own gRPC interceptors. |
| `InitFunc` | Functions run at startup, in order, after the framework's own setup. A returned error stops startup. |
| `TeardownFunc` | Functions run at shutdown, in reverse order. |
| `ShutdownTimeout` | Time budget of the whole shutdown. Default 30s. |
| `HealthChecks` | Extra readiness checks. See [Health checks](observability.md#health-checks). |

## A complete application

```go
package main

import (
    "context"
    "net/http"

    "butterfly.orx.me/core/app"
    "butterfly.orx.me/core/log"
    "butterfly.orx.me/core/store/redis"
    "github.com/gin-gonic/gin"
    "google.golang.org/grpc"
    pb "your-service/proto"
)

type MyConfig struct {
    APIKey string `yaml:"api_key"`
}

func (c *MyConfig) Print() {}

var cfg = &MyConfig{}

func main() {
    app.New(&app.Config{
        Service:   "user-service",
        Namespace: "my-namespace", // Consul key: my-namespace/user-service
        Config:    cfg,

        Router:       setupHTTPRoutes,
        GRPCRegister: setupGRPCServer,

        InitFunc: []func() error{
            warmCache,
        },
        TeardownFunc: []func() error{
            flushMetrics,
        },
    }).Run()
}

func setupHTTPRoutes(r *gin.Engine) {
    api := r.Group("/api/v1")
    {
        api.GET("/users/:id", getUser)
        api.POST("/users", createUser)
    }
    // /healthz, /readyz and /metrics are served by the framework on the admin
    // port. Do not add them here.
}

func setupGRPCServer(s *grpc.Server) {
    pb.RegisterUserServiceServer(s, &userServiceServer{})
}

func getUser(c *gin.Context) {
    log.FromContext(c.Request.Context()).Info("loading user", "id", c.Param("id"))
    c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

// InitFunc: runs after the stores are connected, so redis.GetClient works.
func warmCache() error {
    return redis.GetClient("cache").Set(context.Background(), "warm", "1", 0).Err()
}

func flushMetrics() error { return nil }
```

## Startup order

`Run` does this, in order. A failure at any step stops startup (`Run` panics with the error, `RunContext` returns it).

1. Choose the configuration backend (file or Consul).
2. Load your application config into `Config` and run its `Validate`, if it has one.
3. Load the core configuration and validate it.
4. Configure logging (`log.level`, `log.format`, `log.add_source`).
5. Resolve and validate the listen addresses (`server.*`).
6. Set up metrics.
7. Start the admin server (`/metrics`, `/healthz`, `/readyz`). `/readyz` answers `503 starting` from now until the servers are up.
8. Set up tracing.
9. Connect the stores: Redis, SQL, GORM, Mongo, S3.
10. Register the health checks.
11. Run your `InitFunc`s, in order.
12. Start watching the config document, when `OnConfigChange` is set.
13. Start the HTTP server and, if `GRPCRegister` is set, the gRPC server. `/readyz` now reports `ready`.

Because the stores are connected before your `InitFunc`s run, you can call `redis.GetClient(...)`, `sqldb.GetDB(...)` and the like from them.

## Graceful shutdown

`Run` handles `SIGINT` and `SIGTERM`. On a signal, it shuts down in this order:

1. Reports not-ready on `/readyz` and gRPC health, so load balancers stop routing here. If `server.shutdown_delay` is set, it then waits that long. The servers keep serving during the wait.
2. Stops accepting new connections and drains in-flight HTTP requests and gRPC calls.
3. Runs your `TeardownFunc`s in **reverse** order, like `defer`. An error or panic in one is logged and does not stop the others.
4. Closes the framework's own resources: the Redis, SQL and Mongo clients.
5. Flushes and shuts down tracing, stops the admin server, and shuts down metrics.

The whole sequence shares one time budget, `ShutdownTimeout` (default 30s). When it runs out, remaining connections are cut. A second signal during shutdown ends the process immediately.

```go
app.New(&app.Config{
    Service:         "my-service",
    ShutdownTimeout: 20 * time.Second,
    TeardownFunc: []func() error{
        func() error { return messageQueue.Close() }, // registered first, runs last
        func() error { return myCache.Flush() },      // registered last, runs first
    },
})
```

You do not need to close clients obtained from `store/*`: the framework closes them. Resources you create yourself still need a teardown function.

### Driving the app from a context

`RunContext(ctx)` is `Run` driven by a context instead of OS signals. Cancelling `ctx` triggers the same graceful shutdown, and `RunContext` returns `nil` when it is done. It returns an error instead of panicking if startup fails.

```go
ctx, cancel := context.WithCancel(context.Background())
done := make(chan error, 1)
go func() { done <- application.RunContext(ctx) }()
// ...
cancel()
err := <-done
```

Run one application per process. It registers process-wide state such as the metrics registry and the admin port, so a second `Run` in the same process (in a test, say) is not supported. To test routes, see [Testing](testing.md).
