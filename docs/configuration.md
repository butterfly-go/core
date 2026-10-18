# Configuration

Butterfly reads one YAML document per service. That single document holds both the **core configuration** (stores, logging, tracing, listen addresses; documented below) and **your own settings** (see [Your application config](#your-application-config)).

Two things are decided before the document is read, through environment variables: where it comes from (a file or Consul), and how tracing exports.

## Environment variables

Every variable has the prefix `BUTTERFLY_`.

| Variable | Meaning | Default |
|---|---|---|
| `BUTTERFLY_CONFIG_TYPE` | Where the document comes from: `file` or `consul`. Any other value means Consul. | `consul` |
| `BUTTERFLY_CONFIG_FILE_PATH` | Path of the YAML file. Required when the type is `file`. | none |
| `BUTTERFLY_CONFIG_CONSUL_ADDRESS` | Consul address, for example `consul:8500`. | the Consul client default |
| `BUTTERFLY_CONFIG_CONSUL_NAMESPACE` | A [Consul Enterprise namespace](https://developer.hashicorp.com/consul/docs/enterprise/namespaces). This is not the same as `app.Config.Namespace`, see below. | none |
| `BUTTERFLY_CONFIG_CONSUL_TOKEN` | Consul ACL token. | none |
| `BUTTERFLY_CONFIG_CONSUL_TOKEN_FILE` | File holding the ACL token. Read once at startup. Ignored when `..._TOKEN` is set. | none |
| `BUTTERFLY_CONFIG_CONSUL_TLS_ENABLED` | `true` talks HTTPS to Consul. Needed before any other `..._TLS_*` variable. | off |
| `BUTTERFLY_CONFIG_CONSUL_TLS_CA_FILE` | PEM bundle of CAs that signed the Consul certificate. | system roots |
| `BUTTERFLY_CONFIG_CONSUL_TLS_CERT_FILE`, `..._TLS_KEY_FILE` | Client certificate for mutual TLS. Set both or neither. | none |
| `BUTTERFLY_CONFIG_CONSUL_TLS_SERVER_NAME` | Name to verify the Consul certificate against. | the address host |
| `BUTTERFLY_CONFIG_CONSUL_TLS_INSECURE_SKIP_VERIFY` | `true` skips certificate verification. Testing only. | off |
| `BUTTERFLY_TRACING_ENDPOINT` | OTLP endpoint, for example `localhost:4317` (gRPC) or `localhost:4318` (HTTP). | none |
| `BUTTERFLY_TRACING_PROVIDER` | `grpc` or `http`. | `grpc` |
| `BUTTERFLY_TRACING_DISABLE` | `true` or `1` turns tracing off. | tracing on |
| `BUTTERFLY_TRACING_INSECURE` | `true` sends plaintext, `false` uses TLS. | plaintext for `grpc`, TLS for `http` |
| `BUTTERFLY_TRACING_TLS_ENABLED` | `true` uses TLS with the settings below. | off |
| `BUTTERFLY_TRACING_TLS_CA_FILE`, `..._CERT_FILE`, `..._KEY_FILE`, `..._SERVER_NAME`, `..._INSECURE_SKIP_VERIFY` | Same meaning as for Consul. | none |
| `BUTTERFLY_TRACING_HEADERS` | Headers sent with every export, as `key=value,key=value`. Values may be percent-encoded. Use it for collector API keys. | none |

A `..._TLS_CA_FILE` (or any other TLS file) without `..._TLS_ENABLED=true` is an error rather than being ignored, so a CA you meant to use is never silently skipped. Secrets passed this way (`..._TOKEN`, `..._HEADERS`) are never logged: only the header names are.

```bash
# Run locally with a file
export BUTTERFLY_CONFIG_TYPE=file
export BUTTERFLY_CONFIG_FILE_PATH=./config.yaml
export BUTTERFLY_TRACING_DISABLE=true
```

Tracing is on by default. Set `BUTTERFLY_TRACING_ENDPOINT`, or set `BUTTERFLY_TRACING_DISABLE=true` for local development.

## Where the document comes from

### File

With `BUTTERFLY_CONFIG_TYPE=file`, the framework reads the file at `BUTTERFLY_CONFIG_FILE_PATH`. The service name and namespace play no role. The file is read at startup, so a change needs a restart.

### Consul

With Consul, the document is the value of a KV key built from the app (see [Consul with a token and TLS](#consul-with-a-token-and-tls) for secured clusters, and [Reloading configuration](#reloading-configuration) for live updates):

| `app.Config` | Consul key |
|---|---|
| `Service: "user-service"` | `user-service` |
| `Service: "user-service", Namespace: "prod"` | `prod/user-service` |

`app.Config.Namespace` only prefixes the key. Slashes at the ends are trimmed. A missing key is treated as an empty document, so every setting takes its default.

### Consul with a token and TLS

```bash
export BUTTERFLY_CONFIG_CONSUL_ADDRESS=consul.internal:8501
export BUTTERFLY_CONFIG_CONSUL_TOKEN_FILE=/var/run/secrets/consul/token
export BUTTERFLY_CONFIG_CONSUL_TLS_ENABLED=true
export BUTTERFLY_CONFIG_CONSUL_TLS_CA_FILE=/etc/consul/ca.pem
# Mutual TLS, when the agent verifies clients:
export BUTTERFLY_CONFIG_CONSUL_TLS_CERT_FILE=/etc/consul/client.pem
export BUTTERFLY_CONFIG_CONSUL_TLS_KEY_FILE=/etc/consul/client-key.pem
```

`https://consul.internal:8501` as the address also selects HTTPS. Combining `http://` with `..._TLS_ENABLED=true` fails at startup.

## Core configuration reference

Everything in this section is optional. A service that uses no stores can run with an empty document.

```yaml
store:
  mongo:
    primary:
      uri: "mongodb://localhost:27017"
      # Optional. A non-zero value overrides the same option in the URI.
      max_pool_size: 100
      min_pool_size: 5
      max_connecting: 2
      max_conn_idle_time: 5m
      connect_timeout: 10s
      server_selection_timeout: 5s
      timeout: 30s               # default deadline of a whole operation

  redis:
    cache:
      addr: "localhost:6379"
      username: ""             # Redis 6 ACL user, optional
      password: ""
      db: 0
      # Optional. Zero means the go-redis default.
      pool_size: 20
      min_idle_conns: 2
      pool_timeout: 4s
      conn_max_idle_time: 5m
      conn_max_lifetime: 1h
      dial_timeout: 5s
      read_timeout: 3s
      write_timeout: 3s
      tls:
        enabled: false
        ca_file: ""
        cert_file: ""
        key_file: ""
        server_name: ""
        insecure_skip_verify: false

  db:
    main:
      driver: "mysql"          # mysql (default), or postgres / postgresql
      host: "localhost"
      port: 3306
      user: "root"
      password: "password"
      db_name: "myapp"
      ssl_mode: "disable"      # PostgreSQL only, default: disable

  s3:
    assets:
      endpoint: "s3.amazonaws.com"
      region: "us-east-1"                        # default: us-east-1
      bucket: "my-assets"
      access_key_id: "AKIAIOSFODNN7EXAMPLE"      # or "ak"
      secret_access_key: "wJalrXUtnFEMI/K7MDENG" # or "sk"
      session_token: ""                          # optional
      use_ssl: true
      use_path_style: false                      # true for MinIO and similar

log:
  level: "info"        # debug, info, warn, error. Default: info
  format: "json"       # json or text. Default: text
  add_source: true     # include file:line. Default: true

otel:
  trace_sample_ratio: 1.0   # see below

server:
  http:  { addr: ":8080" }
  grpc:  { addr: ":9090" }
  admin: { addr: ":2223" }
  shutdown_delay: 0s
```

Stores are keyed by a name you choose (`primary`, `cache`, `main`, `assets`). You get a client by that name, see [Storage](storage.md).

### `store.redis`

One entry is one client, in one of three modes. `mode` is inferred when left out: `master_name` set means sentinel, more than one address means cluster, otherwise standalone.

```yaml
store:
  redis:
    single:                       # standalone
      addr: "redis:6379"

    ha:                           # sentinel: failover to the current master
      master_name: "mymaster"
      addrs: ["sentinel-1:26379", "sentinel-2:26379", "sentinel-3:26379"]
      password: "redis-password"
      sentinel_password: "sentinel-password"   # when sentinels have their own

    sharded:                      # cluster: seed nodes, the rest is discovered
      addrs: ["node-1:6379", "node-2:6379", "node-3:6379"]
      password: "redis-password"
      tls: { enabled: true, ca_file: "/etc/redis/ca.pem" }
```

- `tls.enabled: true` turns TLS on, with the system CAs unless `ca_file` is set. `cert_file` and `key_file` add a client certificate for mutual TLS (both or neither). With sentinel, TLS applies to the Redis nodes and the sentinels. The other `tls.*` fields without `enabled: true` are an error.
- A sentinel client is a regular `*redis.Client`. A cluster has no `*redis.Client`, see [Redis in storage](storage.md#redis-modes-tls-and-pools).
- `db` is not supported in cluster mode. `username` and `password` apply to every node.
- Pool and timeout fields apply per node and are all optional.

### `store.mongo`

Credentials, replica set, TLS (`tls=true&tlsCAFile=...`) and the rest of the connection options stay in the `uri`. The pool and timeout fields above are conveniences that **override** the same option when the URI sets it and are non-zero, and leave the URI or driver default alone when zero.

### `log`

`level`, `format` and `add_source` configure the global `slog` logger. See [Logging](observability.md#logging).

### `otel`

`trace_sample_ratio` is the fraction of root traces to record when tracing is on.

| Value | Behavior |
|---|---|
| unset, or `>= 1` | record every trace |
| `<= 0` | record none |
| between 0 and 1 | record that fraction of new traces, and follow the caller's decision for incoming ones |

### `server`

Listen addresses and the shutdown delay.

- Each `addr` is a `host:port` pair, such as `":8081"` or `"127.0.0.1:8081"`. Port `0` picks a free port, and the real address is logged at startup.
- `http` is resolved in this order: `server.http.addr`, then the `PORT` environment variable, then `:8080`.
- `grpc` is only used when the app sets `GRPCRegister`.
- `admin` serves `/metrics`, `/healthz` and `/readyz`, see [Observability](observability.md).
- A malformed address, or two servers that would bind the same port, makes startup fail with a message naming the keys. The gRPC address is not checked for conflicts when no gRPC server is registered.
- `shutdown_delay` is how long to wait after reporting not-ready and before stopping the servers, for example `5s`. It must be smaller than `ShutdownTimeout`. See [Application lifecycle](application.md#graceful-shutdown).

## Validation

The core settings (`store.redis`, `store.mongo`, `store.db`, `log`, `otel`) are checked when the document is loaded. Startup fails with every problem found, each named by the key it belongs to, instead of the first symptom:

```text
invalid core config: store.redis.cache: mode "ring" is not one of standalone, sentinel, cluster
store.redis.cache: pool_size must not be negative
store.mongo.primary: uri is required
```

Typical checks: required fields (`addr`, `uri`, `host`), valid `host:port` addresses, `master_name` for sentinel, unknown `mode`, `driver` or `log.level`, negative sizes and timeouts, `min_pool_size` above `max_pool_size`, and inconsistent TLS fields. Files named in `tls` are read when the client is created, so a missing CA fails startup there. The `server` section is checked separately when the listen addresses are resolved.

Your own settings can take part. Give the struct set as `app.Config.Config` a `Validate() error` method, and it runs right after the document is parsed. A returned error stops startup:

```go
func (c *MyConfig) Validate() error {
    if c.MaxRetries < 0 {
        return errors.New("max_retries must not be negative")
    }
    return nil
}
```

`Validate` may also fill in defaults, since it runs on the value that is then handed to your code.

## Reloading configuration

With Consul, set `app.Config.OnConfigChange` to be told when the document changes. The framework watches the key with Consul blocking queries, so a change arrives within moments and an idle service costs one long-lived request.

```go
var current atomic.Pointer[MyConfig]

cfg := &MyConfig{}
current.Store(cfg)

app.New(&app.Config{
    Service: "user-service",
    Config:  cfg,
    OnConfigChange: func(c app.AppConfig) {
        current.Store(c.(*MyConfig)) // already parsed and validated
    },
})
```

How it behaves:

- The callback receives a **new** `*MyConfig`, decoded from the new document alone. Fields start at their zero value, not at what `cfg` holds, so put defaults in `Validate`. `cfg` itself is never written after startup. Read the live value through your own `atomic.Pointer`, as above.
- The new document is parsed and validated first, both the core settings and your `Validate`. If anything fails, the reload is rejected, an error is logged, and the service keeps the previous settings. The callback is not called.
- The callback runs on one goroutine, one change at a time. A panic in it is recovered and logged.
- Unchanged values (a rewrite with identical content) and key deletion do not trigger the callback. After a deletion the last good settings stay in effect and a warning is logged.
- Failed queries are retried with exponential backoff (1s up to 30s) and never stop the watch.
- Core settings (`store`, `server`, `otel`) are read at startup only. Changing them logs a warning that a restart is needed.
- `OnConfigChange` needs `Config` to be set. With the file backend it logs a warning and nothing is watched.
- The watch stops before the stores are closed during shutdown.

To watch some other key, use `config.Watch`, which blocks until the context ends:

```go
go func() {
    err := config.Watch(ctx, "shared/feature-flags", func(value []byte) {
        // new content of the key
    })
    if errors.Is(err, config.ErrWatchUnsupported) {
        // file backend
    }
}()
```

## Logging and secrets

Credentials are kept out of the logs in two ways.

- The framework never logs passwords, tokens or DSNs: store startup messages carry addresses, database names and pool sizes only, and driver errors are scrubbed before they are logged or returned. Printing a config struct (`%v`, or as a `slog` attribute) shows `password: ***`.
- A safety net on the global logger masks any attribute whose key looks like a credential (`password`, `token`, `dsn`, `secret`, `api_key`, `authorization`, and keys ending in `_password`, `_secret`, `_token`), and the password part of any `scheme://user:password@host` URI or MySQL DSN inside a message, attribute or error. It applies once `log.Init` has run, which the framework does at startup.

The net is a backstop, not a licence to log secrets: it works from key names and URL shapes and cannot recognise an arbitrary token in a free-form message.

## Your application config

Set `app.Config.Config` to a pointer to your own struct. The framework unmarshals **the same YAML document** into it, so your fields sit at the top level next to `store`, `log` and the rest. Keys your struct does not declare are ignored, including the core keys.

```go
type MyConfig struct {
    APIKey     string `yaml:"api_key"`
    MaxRetries int    `yaml:"max_retries"`
}

// Print is required by the app.Config interface. The framework does not call it.
func (c *MyConfig) Print() {}

cfg := &MyConfig{}
application := app.New(&app.Config{
    Service: "user-service",
    Config:  cfg,
})
```

```yaml
# Your settings
api_key: "secret"
max_retries: 3

# Core settings
store:
  redis:
    cache: { addr: "localhost:6379" }
```

`cfg` is filled in before your `InitFunc`s run, so they can read it. `Config` is optional: leave it unset when you have nothing of your own to configure.

To read another key from the configuration backend yourself, use `butterfly.orx.me/core/config`:

```go
import "butterfly.orx.me/core/config"

b, err := config.Get(ctx, "shared/feature-flags")
```

With the file backend the key is ignored and you get the same file.

## Environment variable names

The names are derived from dotted or dashed keys: `config.file.path` becomes `BUTTERFLY_CONFIG_FILE_PATH`, `tracing-endpoint` becomes `BUTTERFLY_TRACING_ENDPOINT`.
