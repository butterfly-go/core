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
| `BUTTERFLY_TRACING_ENDPOINT` | OTLP endpoint, for example `localhost:4317` (gRPC) or `localhost:4318` (HTTP). | none |
| `BUTTERFLY_TRACING_PROVIDER` | `grpc` or `http`. | `grpc` |
| `BUTTERFLY_TRACING_DISABLE` | `true` or `1` turns tracing off. | tracing on |

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

With Consul, the document is the value of a KV key built from the app:

| `app.Config` | Consul key |
|---|---|
| `Service: "user-service"` | `user-service` |
| `Service: "user-service", Namespace: "prod"` | `prod/user-service` |

`app.Config.Namespace` only prefixes the key. Slashes at the ends are trimmed. A missing key is treated as an empty document, so every setting takes its default.

## Core configuration reference

Everything in this section is optional. A service that uses no stores can run with an empty document.

```yaml
store:
  mongo:
    primary:
      uri: "mongodb://localhost:27017"

  redis:
    cache:
      addr: "localhost:6379"
      password: ""
      db: 0

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
