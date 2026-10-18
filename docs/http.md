# HTTP

## Gin Framework Integration

The framework integrates the Gin Web framework and installs this middleware before your `Router` runs, outermost first:

1. **OpenTelemetry tracing** (`otelgin`).
2. **Request id and request-scoped logger.** The id is taken from the caller's `X-Request-Id` header, or generated when the header is missing or malformed (only letters, digits and `-_.:`, at most 128 characters, so a client cannot forge log lines). It is echoed in the `X-Request-Id` response header, including on a 500 from a panic, so users can quote it in a bug report.
3. **Access log**, one `http request` line per request.
4. **Panic recovery**, which logs and returns `500`.

Your routes and middleware run inside these.

```go
func setupHTTPRoutes(r *gin.Engine) {
    // Add custom middleware: it runs after the built-in ones, so it can use
    // log.FromContext and a panic in it is recovered too.
    r.Use(customAuthMiddleware())

    // Register routes
    r.GET("/", homeHandler)

    // API versioning
    v1 := r.Group("/api/v1")
    v1.Use(rateLimitMiddleware())
    {
        v1.GET("/resources", listResources)
        v1.POST("/resources", createResource)
    }
}

func homeHandler(c *gin.Context) {
    // The logger already carries request_id, http.method, http.path, http.route,
    // trace_id and span_id.
    log.FromContext(c.Request.Context()).Info("serving home")

    // The request id, if you need it in a response body or another system:
    id := httputils.RequestID(c)

    c.JSON(http.StatusOK, gin.H{
        "service":    "user-service",
        "version":    "1.0.0",
        "request_id": id,
    })
}
```

### Access log

Each request produces one line with `http.status`, `duration`, `bytes`, `client_ip` and `user_agent`, on top of the request attributes above. The level follows the status, so enabling this does not flood the default `info` level:

| Status | Level |
|---|---|
| 1xx to 3xx | `debug` |
| 4xx | `info` |
| 5xx | `error` |

Set `log.level: debug` to see every request. Only the path is logged, never the query string, because query strings often carry tokens. `http.route` is the matched route template (`/users/:id`), which is safe to group by. It is omitted for unmatched routes (404).

### Panic recovery

A panic in a handler or middleware is logged at `error` with the stack, `request_id` and `trace_id`, recorded on the span, and answered with an empty `500`. The panic value is never sent to the client. Two cases are treated differently: a write to a client that has already disconnected is logged as a `warn` without trying to write a response, and `http.ErrAbortHandler` (used by `httputil.ReverseProxy`) is passed on to `net/http`, which aborts the connection quietly.

## Twirp RPC support

`httputils.RegisterTwirpHandler` mounts a Twirp server on the gin engine. Pass the server's own `PathPrefix()`, which includes the package and service name (`/twirp/<package>.<Service>/`). Twirp calls look like `POST /twirp/<package>.<Service>/<Method>`, and the handler matches the last segment as the method.

```go
import (
    "butterfly.orx.me/core/utils/httputils"
    "your-service/rpc/userservice"
)

func setupHTTPRoutes(r *gin.Engine) {
    twirpServer := userservice.NewUserServiceServer(&userServiceImpl{}, nil)

    httputils.RegisterTwirpHandler(r, twirpServer.PathPrefix(), twirpServer)
}
```

Twirp requests go through the same built-in middleware as every other route: tracing, request id, access log and panic recovery.
