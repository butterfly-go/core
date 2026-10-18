# gRPC

The framework automatically starts a gRPC server on port 9090 (change it with `server.grpc.addr`):

```go
func setupGRPCServer(s *grpc.Server) {
    // Register multiple gRPC services
    pb.RegisterUserServiceServer(s, &userServer{})
    pb.RegisterAuthServiceServer(s, &authServer{})
    
    // Register gRPC reflection service (for debugging)
    reflection.Register(s)
}

// Implement gRPC service
type userServer struct {
    pb.UnimplementedUserServiceServer
}

func (s *userServer) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {
    // Implement business logic
    return &pb.User{
        Id:   req.Id,
        Name: "John Doe",
    }, nil
}
```

## Built-in gRPC behavior

Every gRPC server the framework creates comes with:

- **Tracing and metrics.** OpenTelemetry spans and `rpc.server.*` metrics (served on `/metrics`) for every call, continuing the trace of the caller through the `traceparent` header. Health checks are not traced, because probes run every few seconds and would bury real traffic.
- **Call logging.** One `grpc call` log line per RPC with `grpc.service`, `grpc.method`, `grpc.type` (`unary` or `stream`), `grpc.code`, `duration`, `peer`, `error` (if any), and `trace_id` / `span_id`. The level depends on the outcome, so busy services are not flooded at the default `info` level:

  | Result | Level |
  |---|---|
  | `OK` | `debug` |
  | Caller errors: `InvalidArgument`, `NotFound`, `AlreadyExists`, `PermissionDenied`, `Unauthenticated`, `FailedPrecondition`, `Canceled`, ... | `info` |
  | Transient problems: `DeadlineExceeded`, `ResourceExhausted`, `Unavailable`, `Aborted` | `warn` |
  | Bugs and data problems: `Unknown`, `Internal`, `DataLoss`, `Unimplemented` | `error` |

- **Panic recovery.** A panic in a handler or interceptor is logged at `error` with the stack and `trace_id`, recorded on the span, and returned to the caller as `codes.Internal` with the message `internal error`. The panic value is never sent to the caller. The server keeps running.
- **A request-scoped logger.** Inside a handler, `log.FromContext(ctx)` returns a logger that already has `grpc.service`, `grpc.method`, `trace_id` and `span_id`, so your own log lines line up with the call log and the trace.

```go
func (s *userServer) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {
    log.FromContext(ctx).Info("loading user", "id", req.Id)
    // {"msg":"loading user","grpc.method":"GetUser","trace_id":"...","span_id":"...","id":42}
    ...
}
```

### Your own interceptors

Add interceptors with `GRPCUnaryInterceptors` and `GRPCStreamInterceptors`. They run in order, **inside** the built-in ones. That means they can use `log.FromContext`, and a panic in them is recovered as well:

```go
config := &app.Config{
    Service:      "my-service",
    GRPCRegister: setupGRPCServer,
    GRPCUnaryInterceptors: []grpc.UnaryServerInterceptor{
        authInterceptor,
        rateLimitInterceptor,
    },
    GRPCStreamInterceptors: []grpc.StreamServerInterceptor{
        streamAuthInterceptor,
    },
}
```

## Health

The standard `grpc.health.v1.Health` service is registered automatically, so Kubernetes gRPC probes and `grpc_health_probe` work without code. It follows the same readiness state as `/readyz`. See [Health checks](observability.md#health-checks).
