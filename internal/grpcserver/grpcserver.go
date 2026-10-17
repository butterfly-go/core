// Package grpcserver builds the framework's gRPC server with its default
// observability and safety net:
//
//   - OpenTelemetry tracing and RPC metrics (via the otelgrpc stats handler),
//     with trace context propagated from callers. Health checks are not traced.
//   - A logging interceptor that logs every call and stores a request-scoped
//     logger in the context (see log.FromContext).
//   - A recovery interceptor that turns handler panics into codes.Internal.
//
// Interceptors supplied by the application run after (inside) these.
package grpcserver

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc/filters"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	corelog "butterfly.orx.me/core/log"
)

// Options configures the server.
type Options struct {
	// Unary and Stream are the application's interceptors. They run in order,
	// inside the built-in logging and recovery interceptors.
	Unary  []grpc.UnaryServerInterceptor
	Stream []grpc.StreamServerInterceptor

	// otel overrides the otelgrpc options; tests use it to inject providers.
	otel []otelgrpc.Option
}

// New returns a gRPC server with the built-in handlers and interceptors.
func New(opts Options) *grpc.Server {
	return grpc.NewServer(serverOptions(opts)...)
}

func serverOptions(opts Options) []grpc.ServerOption {
	// Logging is outermost so that it sees the Internal error produced by the
	// recovery interceptor, and so the logger it stores in the context is
	// available to recovery and to the application.
	unary := append([]grpc.UnaryServerInterceptor{unaryLogging(), unaryRecovery()}, opts.Unary...)
	stream := append([]grpc.StreamServerInterceptor{streamLogging(), streamRecovery()}, opts.Stream...)

	otelOpts := append([]otelgrpc.Option{
		// Kubelet and load balancers probe health every few seconds; tracing
		// each probe would drown out real traffic.
		otelgrpc.WithFilter(filters.Not(filters.HealthCheck())),
	}, opts.otel...)

	return []grpc.ServerOption{
		grpc.StatsHandler(otelgrpc.NewServerHandler(otelOpts...)),
		grpc.ChainUnaryInterceptor(unary...),
		grpc.ChainStreamInterceptor(stream...),
	}
}

// splitMethod splits "/pkg.Service/Method" into its service and method.
func splitMethod(full string) (service, method string) {
	full = strings.TrimPrefix(full, "/")
	service, method, ok := strings.Cut(full, "/")
	if !ok {
		return "", full
	}
	return service, method
}

// withCallLogger stores a logger carrying the call's identity in ctx, so any
// code that uses log.FromContext gets grpc.service and grpc.method (plus the
// trace ids, which FromContext adds).
func withCallLogger(ctx context.Context, fullMethod string) context.Context {
	service, method := splitMethod(fullMethod)
	logger := slog.Default().With(
		"component", "grpc",
		"grpc.service", service,
		"grpc.method", method,
	)
	return corelog.WithLogger(ctx, logger)
}

// levelFor maps a status code to a log level. Successful calls are Debug so
// that busy services are not flooded at the default level; errors caused by
// the caller are Info; transient server-side problems are Warn; and bugs or
// data problems are Error.
func levelFor(code grpccodes.Code) slog.Level {
	switch code {
	case grpccodes.OK:
		return slog.LevelDebug
	case grpccodes.Unknown, grpccodes.Internal, grpccodes.DataLoss, grpccodes.Unimplemented:
		return slog.LevelError
	case grpccodes.DeadlineExceeded, grpccodes.ResourceExhausted, grpccodes.Unavailable, grpccodes.Aborted:
		return slog.LevelWarn
	default: // Canceled, InvalidArgument, NotFound, AlreadyExists, PermissionDenied, ...
		return slog.LevelInfo
	}
}

func logCall(ctx context.Context, kind string, start time.Time, err error) {
	code := status.Code(err)
	attrs := []any{
		"grpc.type", kind,
		"grpc.code", code.String(),
		"duration", time.Since(start),
	}
	if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
		attrs = append(attrs, "peer", p.Addr.String())
	}
	if err != nil {
		attrs = append(attrs, "error", err.Error())
	}
	corelog.FromContext(ctx).Log(ctx, levelFor(code), "grpc call", attrs...)
}

func unaryLogging() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ctx = withCallLogger(ctx, info.FullMethod)
		start := time.Now()
		resp, err := handler(ctx, req)
		logCall(ctx, "unary", start, err)
		return resp, err
	}
}

// contextStream overrides a stream's context.
type contextStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *contextStream) Context() context.Context { return s.ctx }

func streamLogging() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := withCallLogger(ss.Context(), info.FullMethod)
		start := time.Now()
		err := handler(srv, &contextStream{ServerStream: ss, ctx: ctx})
		logCall(ctx, "stream", start, err)
		return err
	}
}

func unaryRecovery() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if p := recover(); p != nil {
				resp, err = nil, recovered(ctx, p)
			}
		}()
		return handler(ctx, req)
	}
}

func streamRecovery() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		defer func() {
			if p := recover(); p != nil {
				err = recovered(ss.Context(), p)
			}
		}()
		return handler(srv, ss)
	}
}

// recovered logs a recovered panic and returns the error sent to the caller.
// The panic value is logged but never sent: it may contain internal details.
func recovered(ctx context.Context, p any) error {
	corelog.FromContext(ctx).Error("panic recovered in grpc handler",
		"panic", fmt.Sprint(p),
		"stack", string(debug.Stack()),
	)
	span := trace.SpanFromContext(ctx)
	span.RecordError(fmt.Errorf("panic: %v", p))
	span.SetStatus(codes.Error, "panic")
	return status.Error(grpccodes.Internal, "internal error")
}
