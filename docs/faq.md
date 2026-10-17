# FAQ

## How do I run a service locally without Consul and a tracing backend?

Use the file backend and turn tracing off:

```bash
export BUTTERFLY_CONFIG_TYPE=file
export BUTTERFLY_CONFIG_FILE_PATH=./config.yaml
export BUTTERFLY_TRACING_DISABLE=true
go run .
```

An empty `config.yaml` is enough for a service with no stores. See [Configuration](configuration.md).

## How do I configure multiple databases?

Add one entry per database under `store.db`, and fetch each by its key:

```yaml
store:
  db:
    primary:
      driver: mysql
      host: primary.db.example.com
      port: 3306
      user: app
      password: secret
      db_name: main
    analytics:
      driver: postgres
      host: analytics.db.example.com
      port: 5432
      user: reader
      password: secret
      db_name: analytics
```

```go
primary := sqldb.GetDB("primary")     // *sql.DB
reports := sqldb.GetDB("analytics")   // *sql.DB, PostgreSQL
orm := gorm.GetDB("primary")          // *gorm.DB, MySQL entries only
```

See [Storage](storage.md).

## Why is my store client `nil`?

Either the name does not match a key in the configuration, or you asked for it too early. The getters return `nil` for an unknown name instead of panicking. Stores are connected by `Run`, before your `InitFunc`s, so calling a getter in `main()` before `Run`, or in a package-level `var x = redis.GetClient(...)`, returns `nil`. Fetch clients inside handlers or `InitFunc`s. See [Startup order](application.md#startup-order).

## Why does `/readyz` return 503?

Look at the response body, which names each failing check, and at the logs, which have the error. The usual causes:

- `"state":"starting"` or `"draining"`: the instance has not finished starting, or has begun shutting down.
- A `redis/...`, `sql/...` or `mongo/...` check fails: the dependency is unreachable. Redis, SQL and Mongo are allowed to start while unreachable (a failed ping at startup only logs a warning), so such an instance starts but stays unready until the dependency comes back.

See [Health checks](observability.md#health-checks).

## How do I change the log format?

Set it in the config file:

```yaml
log:
  level: debug       # debug, info, warn, error
  format: json       # json or text
  add_source: true   # include source file and line
```

The framework applies this during startup by calling `slog.SetDefault`. To install your own handler, do it in an `InitFunc`, which runs after the framework's setup. Setting it earlier, in `main()` or `init()`, is overwritten.

```go
InitFunc: []func() error{
    func() error {
        slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
            Level: slog.LevelDebug,
        })))
        return nil
    },
},
```

## How do I tune the connection pool?

`sqldb.GetDB` returns a standard `*sql.DB`, so use its methods, in an `InitFunc`:

```go
db := sqldb.GetDB("primary")
db.SetMaxOpenConns(100)
db.SetMaxIdleConns(10)
db.SetConnMaxLifetime(time.Hour)
```

For GORM entries this is the same pool: `gorm.GetDB(name)` shares it.

## How do I add rate limiting?

Add it as gin middleware in `Router`:

```go
import "golang.org/x/time/rate"

func rateLimit(rps int) gin.HandlerFunc {
    limiter := rate.NewLimiter(rate.Limit(rps), rps)
    return func(c *gin.Context) {
        if !limiter.Allow() {
            c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many requests"})
            return
        }
        c.Next()
    }
}

// in Router:
r.Use(rateLimit(100))
```

The limiter above is per process, so with N replicas the total is N times higher.

## How do I add my own metrics?

Register them on the framework's registry and they appear on `/metrics`. See [Prometheus metrics](observability.md#prometheus-metrics).

## How do I shut down gracefully?

You do not have to do anything. See [Graceful shutdown](application.md#graceful-shutdown).
