# Butterfly Core

Go microservice framework providing config, stores, observability, and HTTP/gRPC servers.

## Quick Reference

```
Module:   butterfly.orx.me/core
Go:       1.25+
DI:       Google Wire (github.com/google/wire)
HTTP:     Gin (:8080)
gRPC:     :9090
Metrics:  Prometheus (:2223/metrics)
```

## Project Structure

```
app/           App lifecycle, Wire injector, Dependencies struct
internal/
  config/      Config backend (File/Consul), Wire providers, registry
  store/       Store clients (Redis, Mongo, SQLDB, S3), Wire providers, registry
  observe/     Metrics (Prometheus/OTEL) and Tracing (OTEL)
  runtime/     Service name and config key
  arg/         Env var parsing (BUTTERFLY_ prefix)
  log/         Internal core logger helper
config/        Public config getter
store/         Public store client getters (redis, mongo, sqldb, s3, gorm)
log/           Public logger init and context helpers
mod/           Config structs and Wire types (ConfigKey)
observe/otel/  Public Prometheus registry access
```

## Architecture

See [architecture.md](architecture.md) for full details on:
- Wire dependency graph and data flow
- Package layout with per-file responsibilities
- Registry pattern (`internal/` Set vs public Get)
- How to add a new Wire provider (step-by-step)
- Regenerating Wire code

## Build & Test

```bash
go build ./...
go test ./...
```

## Environment Variables

All env vars use `BUTTERFLY_` prefix. Key separator `.` and `-` are converted to `_`.

| Variable | Purpose |
|---|---|
| BUTTERFLY_CONFIG_TYPE | `file` or `consul` (default: consul) |
| BUTTERFLY_CONFIG_FILE_PATH | Path when using file config |
| BUTTERFLY_CONFIG_CONSUL_ADDRESS | Consul address |
| BUTTERFLY_CONFIG_CONSUL_NAMESPACE | Consul namespace |
| BUTTERFLY_TRACING_DISABLE | `true` to disable tracing |
| BUTTERFLY_TRACING_PROVIDER | `http` or `grpc` (default: grpc) |
| BUTTERFLY_TRACING_ENDPOINT | OTEL collector endpoint |

## Conventions

- Tests use `testing` stdlib + `testify/assert` where helpful
- Config structs live in `mod/` package
- Public API packages in `store/`, `config/`, `observe/` delegate to `internal/` via registry
- Named types for Wire disambiguation (e.g. `store.RedisClients`, `mod.ConfigKey`)
- Provider functions follow `ProvideXxx` naming convention
- Cleanup functions returned from providers for resource management
- `Set*()` never in public packages — only in `internal/.../registry.go`
