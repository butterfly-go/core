package httpserver

import (
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	corelog "butterfly.orx.me/core/log"
	"butterfly.orx.me/core/testsuite"
	"butterfly.orx.me/core/utils/httputils"
)

type fixture struct {
	engine *gin.Engine
	mock   *testsuite.MockLog
	spans  *tracetest.InMemoryExporter
}

func newFixture(t *testing.T, routes func(*gin.Engine)) *fixture {
	t.Helper()
	gin.SetMode(gin.TestMode)

	_, mock := testsuite.NewMockLog()
	t.Cleanup(mock.SetAsDefault())

	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	t.Cleanup(func() { _ = tp.Shutdown(t.Context()) })

	r := NewEngine("test-service",
		otelgin.WithTracerProvider(tp),
		otelgin.WithPropagators(propagation.TraceContext{}),
	)
	routes(r)
	return &fixture{engine: r, mock: mock, spans: exp}
}

func (f *fixture) do(method, path string, headers ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req.RemoteAddr = "192.0.2.7:1234"
	req.Header.Set("User-Agent", "test-agent")
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	f.engine.ServeHTTP(rec, req)
	return rec
}

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

func routesForTests(r *gin.Engine) {
	r.GET("/users/:id", func(c *gin.Context) {
		corelog.FromContext(c.Request.Context()).Info("in handler")
		c.String(http.StatusOK, "hello "+c.Param("id")+" "+httputils.RequestID(c))
	})
	r.GET("/boom", func(*gin.Context) { panic("kaboom secret") })
	r.GET("/fail", func(c *gin.Context) { c.Status(http.StatusInternalServerError) })
	r.GET("/bad", func(c *gin.Context) { c.Status(http.StatusBadRequest) })
	r.GET("/pipe", func(*gin.Context) {
		panic(&net.OpError{Op: "write", Err: os.NewSyscallError("write", syscall.EPIPE)})
	})
	r.GET("/abort", func(*gin.Context) { panic(http.ErrAbortHandler) })
}

func TestRequestIDGeneratedAndEchoed(t *testing.T) {
	f := newFixture(t, routesForTests)
	rec := f.do(http.MethodGet, "/users/42")
	id := rec.Header().Get(httputils.RequestIDHeader)
	if len(id) != 32 {
		t.Fatalf("generated id %q, want 32 hex chars", id)
	}
	if !strings.HasSuffix(rec.Body.String(), " "+id) {
		t.Fatalf("handler saw a different id: body %q, header %q", rec.Body.String(), id)
	}
	if other := f.do(http.MethodGet, "/users/42").Header().Get(httputils.RequestIDHeader); other == id {
		t.Fatal("ids must differ between requests")
	}
}

func TestRequestIDFromCallerIsValidated(t *testing.T) {
	f := newFixture(t, routesForTests)
	tests := []struct {
		name string
		in   string
		keep bool
	}{
		{"uuid", "123e4567-e89b-12d3-a456-426614174000", true},
		{"trace id style", "0af7651916cd43dd8448eb211c80319c", true},
		{"with separators", "req_1.2:3-x", true},
		{"too long", strings.Repeat("a", maxRequestIDLen+1), false},
		{"space", "bad id", false},
		{"quote injection", `x" level=ERROR msg=forged`, false},
		{"unicode", "ключ", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := f.do(http.MethodGet, "/users/1", httputils.RequestIDHeader, tt.in).Header().Get(httputils.RequestIDHeader)
			if tt.keep && got != tt.in {
				t.Fatalf("id = %q, want the caller's %q", got, tt.in)
			}
			if !tt.keep && (got == tt.in || !validRequestID(got)) {
				t.Fatalf("id = %q, want a fresh valid id", got)
			}
		})
	}
	// An empty header is replaced too.
	if got := f.do(http.MethodGet, "/users/1", httputils.RequestIDHeader, "").Header().Get(httputils.RequestIDHeader); got == "" {
		t.Fatal("empty id must be replaced")
	}
}

func TestHandlerLoggerAndAccessLogCarryRequestAndTraceIDs(t *testing.T) {
	f := newFixture(t, routesForTests)
	rec := f.do(http.MethodGet, "/users/42?token=secret", httputils.RequestIDHeader, "rid-1")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	spans := f.spans.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("spans = %d, want 1", len(spans))
	}
	wantTrace := spans[0].SpanContext.TraceID().String()

	h := f.entry(t, "in handler")
	want := map[string]any{
		"component": "http", "request_id": "rid-1", "http.method": "GET",
		"http.path": "/users/42", "http.route": "/users/:id", "trace_id": wantTrace,
	}
	for k, v := range want {
		if h.Attrs[k] != v {
			t.Errorf("handler log %s = %v, want %v (all: %v)", k, h.Attrs[k], v, h.Attrs)
		}
	}
	if h.Attrs["span_id"] == nil {
		t.Error("handler log lacks span_id")
	}

	a := f.entry(t, "http request")
	if a.Level != slog.LevelDebug {
		t.Errorf("200 logged at %v, want Debug", a.Level)
	}
	for k, v := range want {
		if a.Attrs[k] != v {
			t.Errorf("access log %s = %v, want %v", k, a.Attrs[k], v)
		}
	}
	if a.Attrs["http.status"] != int64(200) && a.Attrs["http.status"] != 200 {
		t.Errorf("http.status = %v (%T)", a.Attrs["http.status"], a.Attrs["http.status"])
	}
	if a.Attrs["client_ip"] != "192.0.2.7" || a.Attrs["user_agent"] != "test-agent" {
		t.Errorf("client attrs = %v", a.Attrs)
	}
	if _, ok := a.Attrs["duration"]; !ok {
		t.Error("access log lacks duration")
	}
	// The query string can hold secrets; only the path is logged.
	for k, v := range a.Attrs {
		if s, ok := v.(string); ok && strings.Contains(s, "secret") {
			t.Errorf("query leaked into %s = %q", k, s)
		}
	}
}

func TestAccessLogLevelsAndUnmatchedRoutes(t *testing.T) {
	f := newFixture(t, routesForTests)
	tests := []struct {
		path  string
		level slog.Level
	}{
		{"/users/1", slog.LevelDebug},
		{"/bad", slog.LevelInfo},
		{"/nope", slog.LevelInfo}, // 404
		{"/fail", slog.LevelError},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			f.mock.Reset()
			f.do(http.MethodGet, tt.path)
			if got := f.entry(t, "http request"); got.Level != tt.level {
				t.Fatalf("level = %v, want %v", got.Level, tt.level)
			}
		})
	}

	f.mock.Reset()
	f.do(http.MethodGet, "/nope")
	a := f.entry(t, "http request")
	if _, ok := a.Attrs["http.route"]; ok {
		t.Errorf("unmatched route must not report a route: %v", a.Attrs)
	}
	if a.Attrs["http.path"] != "/nope" {
		t.Errorf("path = %v", a.Attrs["http.path"])
	}
}

func TestPanicIsRecoveredAndLogged(t *testing.T) {
	f := newFixture(t, routesForTests)
	rec := f.do(http.MethodGet, "/boom", httputils.RequestIDHeader, "rid-panic")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "kaboom") {
		t.Fatalf("panic value leaked to the client: %q", rec.Body.String())
	}
	if rec.Header().Get(httputils.RequestIDHeader) != "rid-panic" {
		t.Fatal("the request id header should survive a panic so the client can quote it")
	}

	p := f.entry(t, "panic recovered in http handler")
	if p.Level != slog.LevelError || !strings.Contains(p.Attrs["panic"].(string), "kaboom") {
		t.Fatalf("panic log = %+v", p)
	}
	if !strings.Contains(p.Attrs["stack"].(string), "goroutine") {
		t.Fatal("panic log lacks a stack")
	}
	for _, k := range []string{"request_id", "trace_id", "http.route"} {
		if p.Attrs[k] == nil {
			t.Errorf("panic log lacks %s: %v", k, p.Attrs)
		}
	}
	// The access log sees the 500 written by recovery.
	if a := f.entry(t, "http request"); a.Level != slog.LevelError {
		t.Fatalf("access log level = %v, want Error", a.Level)
	}
	spans := f.spans.GetSpans()
	if len(spans) != 1 || len(spans[0].Events) == 0 {
		t.Fatalf("panic should be recorded on the span: %+v", spans)
	}

	// The engine keeps serving.
	if rec := f.do(http.MethodGet, "/users/1"); rec.Code != http.StatusOK {
		t.Fatalf("after panic: status = %d", rec.Code)
	}
}

func TestBrokenPipeIsNotAServerError(t *testing.T) {
	f := newFixture(t, routesForTests)
	rec := f.do(http.MethodGet, "/pipe")
	if rec.Code == http.StatusInternalServerError {
		t.Fatal("no 500 should be written to a connection that is gone")
	}
	if f.mock.ContainsMessage("panic recovered in http handler") {
		t.Fatal("a lost connection is not a panic worth an Error log")
	}
	w := f.entry(t, "client connection lost")
	if w.Level != slog.LevelWarn {
		t.Fatalf("level = %v, want Warn", w.Level)
	}
}

func TestAbortHandlerIsPropagatedButStillLogged(t *testing.T) {
	f := newFixture(t, routesForTests)
	var recovered any
	func() {
		defer func() { recovered = recover() }()
		f.do(http.MethodGet, "/abort")
	}()
	if recovered != http.ErrAbortHandler {
		t.Fatalf("recovered = %v, want http.ErrAbortHandler to reach net/http", recovered)
	}
	if f.mock.ContainsMessage("panic recovered in http handler") {
		t.Fatal("ErrAbortHandler is not a crash and must not be logged as one")
	}
	f.entry(t, "http request") // the deferred access log still ran
}

func TestUserMiddlewareRunsInsideBuiltIns(t *testing.T) {
	f := newFixture(t, func(r *gin.Engine) {
		r.Use(func(c *gin.Context) {
			corelog.FromContext(c.Request.Context()).Info("user middleware")
			c.Next()
		})
		r.GET("/x", func(*gin.Context) { panic("from handler") })
	})
	if rec := f.do(http.MethodGet, "/x", httputils.RequestIDHeader, "rid-u"); rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
	if u := f.entry(t, "user middleware"); u.Attrs["request_id"] != "rid-u" || u.Attrs["trace_id"] == nil {
		t.Fatalf("user middleware should see the request logger: %v", u.Attrs)
	}
}
