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
  config/      Config backend (File/Consul), Wire providers
  store/       Store clients (Redis, Mongo, SQLDB, S3), Wire providers
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

## Wire Dependency Injection

The framework uses Google Wire for compile-time dependency injection. The dependency graph:

```
ConfigKey → ProvideConfig() → config.Config
config.Config + ConfigKey → ProvideCoreConfig() → *mod.CoreConfig
*mod.CoreConfig → ProvideRedisClients() → store.RedisClients (+ cleanup)
*mod.CoreConfig → ProvideMongoClients() → store.MongoClients (+ cleanup)
*mod.CoreConfig → ProvideSQLDBClients() → store.SQLDBClients (+ cleanup)
*mod.CoreConfig → ProvideS3Store() → *store.S3Store
```

Key files:
- `app/wire.go` — injector definition (build tag: wireinject)
- `app/wire_gen.go` — generated code (DO NOT EDIT)
- `app/deps.go` — Dependencies struct

After Wire init, legacy globals are set via `config.SetLegacy()` and `store.SetLegacyClients()` for backward compatibility with public getter packages.

### Regenerating Wire Code

```bash
go install github.com/google/wire/cmd/wire@latest
wire ./app/...
```

Run this after changing any Wire provider signatures or the injector.

### Adding a New Wire Provider

1. Create `ProvideXxx(deps...) (Type, func(), error)` in the relevant internal package
2. Add it to the `wire.Build()` call in `app/wire.go`
3. Add the field to `Dependencies` in `app/deps.go`
4. Run `wire ./app/...`
5. Use the dependency in `app/app.go` `Run()` method

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
- Public API packages in `store/`, `config/`, `observe/` delegate to `internal/` implementations
- Named types for Wire disambiguation (e.g. `store.RedisClients`, `mod.ConfigKey`)
- Provider functions follow `ProvideXxx` naming convention
- Cleanup functions returned from providers for resource management
