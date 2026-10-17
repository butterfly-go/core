package grpcserver

import (
	"context"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/wrapperspb"

	corelog "butterfly.orx.me/core/log"
	"butterfly.orx.me/core/testsuite"
)

type fixture struct {
	conn  *grpc.ClientConn
	mock  *testsuite.MockLog
	spans *tracetest.InMemoryExporter
}

type fixtureOpts struct {
	unary  []grpc.UnaryServerInterceptor
	stream []grpc.StreamServerInterceptor
}

func newFixture(t *testing.T, fo fixtureOpts) *fixture {
	t.Helper()

	_, mock := testsuite.NewMockLog()
	t.Cleanup(mock.SetAsDefault())

	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	srv := New(Options{
		Unary:  fo.unary,
		Stream: fo.stream,
		otel: []otelgrpc.Option{
			otelgrpc.WithTracerProvider(tp),
			otelgrpc.WithPropagators(propagation.TraceContext{}),
		},
	})
	registerTestService(srv)
	healthpb.RegisterHealthServer(srv, health.NewServer())

	lis := bufconn.Listen(1 << 16)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return &fixture{conn: conn, mock: mock, spans: exp}
}

func (f *fixture) call(method string, md metadata.MD) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if md != nil {
		ctx = metadata.NewOutgoingContext(ctx, md)
	}
	out := new(wrapperspb.StringValue)
	err := f.conn.Invoke(ctx, "/"+svcName+"/"+method, wrapperspb.String("in"), out)
	return out.GetValue(), err
}

// entry returns the first captured log entry with the message, failing if none.
func (f *fixture) entry(t *testing.T, msg string) testsuite.LogEntry {
	t.Helper()
	for _, e := range f.mock.Entries() {
		if e.Message == msg {
			return e
		}
	}
	t.Fatalf("no log entry %q; got %v", msg, f.mock.Messages())
	return testsuite.LogEntry{}
}

func TestUnaryLogsCallWithTraceIDsAndContextLogger(t *testing.T) {
	f := newFixture(t, fixtureOpts{})
	if _, err := f.call("Echo", nil); err != nil {
		t.Fatal(err)
	}

	spans := f.spans.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("spans = %d, want 1", len(spans))
	}
	wantTrace := spans[0].SpanContext.TraceID().String()

	// A handler using log.FromContext gets the call identity and trace ids.
	h := f.entry(t, "in handler")
	if h.Attrs["grpc.service"] != svcName || h.Attrs["grpc.method"] != "Echo" || h.Attrs["component"] != "grpc" {
		t.Fatalf("handler log attrs = %v", h.Attrs)
	}
	if h.Attrs["trace_id"] != wantTrace || h.Attrs["span_id"] == nil {
		t.Fatalf("handler log lacks trace ids: %v (want trace %s)", h.Attrs, wantTrace)
	}

	c := f.entry(t, "grpc call")
	if c.Level != slog.LevelDebug || c.Attrs["grpc.code"] != "OK" || c.Attrs["grpc.type"] != "unary" {
		t.Fatalf("call log = %+v", c)
	}
	if c.Attrs["trace_id"] != wantTrace {
		t.Fatalf("call log trace_id = %v, want %s", c.Attrs["trace_id"], wantTrace)
	}
	if _, ok := c.Attrs["duration"]; !ok {
		t.Fatalf("call log lacks duration: %v", c.Attrs)
	}
}

func TestIncomingTraceContextIsPropagated(t *testing.T) {
	f := newFixture(t, fixtureOpts{})
	const traceID = "0af7651916cd43dd8448eb211c80319c"
	md := metadata.Pairs("traceparent", "00-"+traceID+"-b7ad6b7169203331-01")
	if _, err := f.call("Echo", md); err != nil {
		t.Fatal(err)
	}
	if got := f.entry(t, "grpc call").Attrs["trace_id"]; got != traceID {
		t.Fatalf("trace_id = %v, want the caller's %s", got, traceID)
	}
	spans := f.spans.GetSpans()
	if len(spans) != 1 || spans[0].SpanContext.TraceID().String() != traceID || !spans[0].Parent.IsValid() {
		t.Fatalf("server span should continue the caller's trace: %+v", spans)
	}
}

func TestLogLevelsByStatusCode(t *testing.T) {
	tests := []struct {
		method string
		code   codes.Code
		level  slog.Level
	}{
		{"NotFound", codes.NotFound, slog.LevelInfo},
		{"InvalidArgument", codes.InvalidArgument, slog.LevelInfo},
		{"Unavailable", codes.Unavailable, slog.LevelWarn},
		{"DeadlineExceeded", codes.DeadlineExceeded, slog.LevelWarn},
		{"Internal", codes.Internal, slog.LevelError},
		{"Unknown", codes.Unknown, slog.LevelError},
	}
	f := newFixture(t, fixtureOpts{})
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			f.mock.Reset()
			_, err := f.call(tt.method, nil)
			if status.Code(err) != tt.code {
				t.Fatalf("code = %v, want %v", status.Code(err), tt.code)
			}
			c := f.entry(t, "grpc call")
			if c.Level != tt.level || c.Attrs["grpc.code"] != tt.code.String() {
				t.Fatalf("log = %+v, want level %v", c, tt.level)
			}
			if _, ok := c.Attrs["error"]; !ok {
				t.Fatalf("error attr missing: %v", c.Attrs)
			}
		})
	}
}

func TestUnaryPanicIsRecovered(t *testing.T) {
	f := newFixture(t, fixtureOpts{})
	_, err := f.call("Panic", nil)
	if status.Code(err) != codes.Internal {
		t.Fatalf("code = %v, want Internal", status.Code(err))
	}
	if strings.Contains(err.Error(), "kaboom") {
		t.Fatalf("panic value leaked to the caller: %v", err)
	}

	p := f.entry(t, "panic recovered in grpc handler")
	if p.Level != slog.LevelError || !strings.Contains(p.Attrs["panic"].(string), "kaboom") {
		t.Fatalf("panic log = %+v", p)
	}
	if !strings.Contains(p.Attrs["stack"].(string), "goroutine") || p.Attrs["grpc.method"] != "Panic" || p.Attrs["trace_id"] == nil {
		t.Fatalf("panic log lacks stack/method/trace_id: %v", p.Attrs)
	}
	// The logging interceptor sees the converted error.
	if c := f.entry(t, "grpc call"); c.Attrs["grpc.code"] != "Internal" || c.Level != slog.LevelError {
		t.Fatalf("call log = %+v", c)
	}
	// The panic is recorded on the span.
	spans := f.spans.GetSpans()
	if len(spans) != 1 || len(spans[0].Events) == 0 {
		t.Fatalf("expected the panic recorded as a span event: %+v", spans)
	}

	// The server survives.
	if got, err := f.call("Echo", nil); err != nil || got != "ok" {
		t.Fatalf("server should keep serving: %q %v", got, err)
	}
}

func TestStreamLoggingAndPanicRecovery(t *testing.T) {
	f := newFixture(t, fixtureOpts{})
	run := func(method string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		stream, err := f.conn.NewStream(ctx, &grpc.StreamDesc{ServerStreams: true}, "/"+svcName+"/"+method)
		if err != nil {
			return err
		}
		if err := stream.SendMsg(wrapperspb.String("in")); err != nil {
			return err
		}
		if err := stream.CloseSend(); err != nil {
			return err
		}
		out := new(wrapperspb.StringValue)
		for {
			if err := stream.RecvMsg(out); err != nil {
				return err
			}
		}
	}

	f.mock.Reset()
	if err := run("StreamOK"); err == nil || err.Error() != "EOF" {
		t.Fatalf("StreamOK err = %v, want EOF", err)
	}
	c := f.entry(t, "grpc call")
	if c.Attrs["grpc.type"] != "stream" || c.Attrs["grpc.code"] != "OK" || c.Attrs["grpc.method"] != "StreamOK" {
		t.Fatalf("stream log = %+v", c)
	}
	// The wrapped stream's context carries the logger, so the handler's own
	// log line has the call identity and a trace id.
	if h := f.entry(t, "in stream handler"); h.Attrs["grpc.method"] != "StreamOK" || h.Attrs["trace_id"] == nil {
		t.Fatalf("stream handler log = %v", h.Attrs)
	}

	f.mock.Reset()
	err := run("StreamPanic")
	if status.Code(err) != codes.Internal || strings.Contains(err.Error(), "stream kaboom") {
		t.Fatalf("StreamPanic err = %v", err)
	}
	if p := f.entry(t, "panic recovered in grpc handler"); !strings.Contains(p.Attrs["panic"].(string), "stream kaboom") {
		t.Fatalf("panic log = %+v", p)
	}
	if c := f.entry(t, "grpc call"); c.Attrs["grpc.code"] != "Internal" {
		t.Fatalf("stream call log = %+v", c)
	}
}

func TestHealthChecksAreNotTraced(t *testing.T) {
	f := newFixture(t, fixtureOpts{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := healthpb.NewHealthClient(f.conn).Check(ctx, &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatal(err)
	}
	if n := len(f.spans.GetSpans()); n != 0 {
		t.Fatalf("health check produced %d spans, want 0", n)
	}
	if _, err := f.call("Echo", nil); err != nil {
		t.Fatal(err)
	}
	if n := len(f.spans.GetSpans()); n != 1 {
		t.Fatalf("regular call produced %d spans, want 1", n)
	}
}

func TestUserInterceptorsRunInsideBuiltIns(t *testing.T) {
	var order []string
	var sawLogger bool
	f := newFixture(t, fixtureOpts{
		unary: []grpc.UnaryServerInterceptor{
			func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
				order = append(order, "user1")
				// The built-in logging interceptor has already stored its logger.
				corelog.FromContext(ctx).Info("user interceptor")
				return h(ctx, req)
			},
			func(ctx context.Context, req any, info *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
				order = append(order, "user2")
				if info.FullMethod == "/"+svcName+"/UserPanic" {
					panic("from user interceptor")
				}
				return h(ctx, req)
			},
		},
	})

	if _, err := f.call("Echo", nil); err != nil {
		t.Fatal(err)
	}
	if strings.Join(order, ",") != "user1,user2" {
		t.Fatalf("order = %v", order)
	}
	if u := f.entry(t, "user interceptor"); u.Attrs["grpc.method"] == "Echo" {
		sawLogger = true
	}
	if !sawLogger {
		t.Fatal("user interceptor should see the built-in call logger")
	}

	// A panic in a user interceptor is recovered too.
	if _, err := f.call("UserPanic", nil); status.Code(err) != codes.Internal {
		t.Fatalf("code = %v, want Internal", status.Code(err))
	}
	if !f.mock.ContainsMessage("panic recovered in grpc handler") {
		t.Fatal("panic in user interceptor was not recovered")
	}
}

func TestSplitMethod(t *testing.T) {
	tests := []struct{ in, svc, method string }{
		{"/pkg.Svc/Do", "pkg.Svc", "Do"},
		{"pkg.Svc/Do", "pkg.Svc", "Do"},
		{"/weird", "", "weird"},
	}
	for _, tt := range tests {
		if s, m := splitMethod(tt.in); s != tt.svc || m != tt.method {
			t.Errorf("splitMethod(%q) = %q, %q", tt.in, s, m)
		}
	}
}
