package health

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"
)

func okCheck(name string) Check {
	return Check{Name: name, Check: func(context.Context) error { return nil }}
}

func TestStatesShortCircuitWithoutRunningChecks(t *testing.T) {
	var runs atomic.Int32
	r := New()
	if err := r.Add(Check{Name: "c", Check: func(context.Context) error { runs.Add(1); return nil }}); err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		state State
		want  string
	}{{Starting, "starting"}, {Draining, "draining"}} {
		r.SetState(tt.state)
		rep := r.Readiness(context.Background())
		if rep.OK() || rep.State != tt.want || len(rep.Checks) != 0 {
			t.Fatalf("%v: report = %+v", tt.state, rep)
		}
	}
	if runs.Load() != 0 {
		t.Fatal("checks must not run outside the Ready state")
	}

	r.SetState(Ready)
	if rep := r.Readiness(context.Background()); !rep.OK() || rep.State != "ready" || rep.Checks["c"].Status != StatusOK {
		t.Fatalf("ready: report = %+v", rep)
	}
}

func TestReadyWithNoChecks(t *testing.T) {
	r := New()
	r.SetState(Ready)
	if rep := r.Readiness(context.Background()); !rep.OK() {
		t.Fatalf("report = %+v", rep)
	}
}

func TestFailingCheckMakesReadinessFail(t *testing.T) {
	r := New(WithCacheTTL(0))
	_ = r.Add(
		okCheck("good"),
		Check{Name: "bad", Check: func(context.Context) error { return errors.New("secret-host:5432 refused") }},
	)
	r.SetState(Ready)

	rep := r.Readiness(context.Background())
	if rep.OK() || rep.Checks["good"].Status != StatusOK || rep.Checks["bad"].Status != StatusFail {
		t.Fatalf("report = %+v", rep)
	}
	b, _ := json.Marshal(rep)
	if strings.Contains(string(b), "secret-host") {
		t.Fatalf("error details leaked into the report: %s", b)
	}
}

func TestCheckTimeoutPanicAndRecovery(t *testing.T) {
	var healthy atomic.Bool
	r := New(WithCheckTimeout(50*time.Millisecond), WithCacheTTL(0))
	_ = r.Add(
		Check{Name: "hang", Check: func(context.Context) error { select {} }}, // ignores ctx entirely
		Check{Name: "panic", Check: func(context.Context) error { panic("boom") }},
		Check{Name: "flaky", Check: func(context.Context) error {
			if healthy.Load() {
				return nil
			}
			return errors.New("down")
		}},
	)
	r.SetState(Ready)

	start := time.Now()
	rep := r.Readiness(context.Background())
	if time.Since(start) > 2*time.Second {
		t.Fatal("a hanging check must not block past its timeout")
	}
	for _, n := range []string{"hang", "panic", "flaky"} {
		if rep.Checks[n].Status != StatusFail {
			t.Fatalf("%s should fail: %+v", n, rep)
		}
	}

	healthy.Store(true)
	rep = r.Readiness(context.Background())
	if rep.Checks["flaky"].Status != StatusOK || rep.OK() {
		t.Fatalf("flaky should recover while others still fail: %+v", rep)
	}
}

func TestResultsAreCachedAndConcurrentCallersShareARound(t *testing.T) {
	var runs atomic.Int32
	r := New(WithCacheTTL(time.Hour))
	_ = r.Add(Check{Name: "c", Check: func(context.Context) error {
		runs.Add(1)
		time.Sleep(50 * time.Millisecond)
		return nil
	}})
	r.SetState(Ready)

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() { defer wg.Done(); r.Readiness(context.Background()) }()
	}
	wg.Wait()
	if got := runs.Load(); got != 1 {
		t.Fatalf("runs = %d, want 1", got)
	}
}

func TestCacheExpires(t *testing.T) {
	var runs atomic.Int32
	r := New(WithCacheTTL(30 * time.Millisecond))
	_ = r.Add(Check{Name: "c", Check: func(context.Context) error { runs.Add(1); return nil }})
	r.SetState(Ready)

	r.Readiness(context.Background())
	r.Readiness(context.Background())
	if runs.Load() != 1 {
		t.Fatalf("runs = %d, want 1 within TTL", runs.Load())
	}
	time.Sleep(60 * time.Millisecond)
	r.Readiness(context.Background())
	if runs.Load() != 2 {
		t.Fatalf("runs = %d, want 2 after TTL", runs.Load())
	}
}

func TestDrainingIsNotBlockedByRunningChecksOrCache(t *testing.T) {
	r := New(WithCacheTTL(time.Hour))
	_ = r.Add(okCheck("c"))
	r.SetState(Ready)
	if !r.Readiness(context.Background()).OK() {
		t.Fatal("should be ready")
	}
	// A cached "ok" must not outlive the flip to Draining.
	r.SetState(Draining)
	if rep := r.Readiness(context.Background()); rep.OK() || rep.State != "draining" {
		t.Fatalf("report = %+v", rep)
	}
}

func TestCancelledProberDoesNotPoisonSharedResult(t *testing.T) {
	r := New(WithCacheTTL(time.Hour))
	_ = r.Add(Check{Name: "c", Check: func(ctx context.Context) error { return ctx.Err() }})
	r.SetState(Ready)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if rep := r.Readiness(ctx); !rep.OK() {
		t.Fatalf("a cancelled caller context must not fail the checks: %+v", rep)
	}
}

func TestAddValidation(t *testing.T) {
	r := New()
	if err := r.Add(Check{Check: func(context.Context) error { return nil }}); err == nil {
		t.Fatal("empty name should be rejected")
	}
	if err := r.Add(Check{Name: "x"}); err == nil {
		t.Fatal("nil func should be rejected")
	}
	if err := r.Add(okCheck("a"), okCheck("a")); err == nil {
		t.Fatal("duplicate in one call should be rejected")
	}
	if len(r.Names()) != 0 {
		t.Fatal("a rejected Add must register nothing")
	}
	if err := r.Add(okCheck("a")); err != nil {
		t.Fatal(err)
	}
	if err := r.Add(okCheck("a")); err == nil {
		t.Fatal("duplicate across calls should be rejected")
	}
	if err := r.Add(okCheck("b"), okCheck("a2")); err != nil {
		t.Fatal(err)
	}
	if got := r.Names(); len(got) != 3 || got[0] != "a" || got[1] != "a2" || got[2] != "b" {
		t.Fatalf("names = %v", got)
	}
}

func serve(t *testing.T, r *Registry, path string) (int, Report, http.Header) {
	t.Helper()
	mux := http.NewServeMux()
	r.Mount(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	var rep Report
	if err := json.Unmarshal(rec.Body.Bytes(), &rep); err != nil {
		t.Fatalf("body %q: %v", rec.Body.String(), err)
	}
	return rec.Code, rep, rec.Header()
}

func TestHTTPHandlers(t *testing.T) {
	var down atomic.Bool
	r := New(WithCacheTTL(0))
	_ = r.Add(Check{Name: "dep", Check: func(context.Context) error {
		if down.Load() {
			return errors.New("down")
		}
		return nil
	}})

	// Liveness is 200 in every state; readiness follows the lifecycle.
	for _, tt := range []struct {
		state     State
		wantReady int
	}{{Starting, 503}, {Ready, 200}, {Draining, 503}} {
		r.SetState(tt.state)
		if code, _, _ := serve(t, r, "/healthz"); code != 200 {
			t.Fatalf("%v: /healthz = %d", tt.state, code)
		}
		code, rep, h := serve(t, r, "/readyz")
		if code != tt.wantReady {
			t.Fatalf("%v: /readyz = %d, report %+v", tt.state, code, rep)
		}
		if h.Get("Content-Type") != "application/json" || h.Get("Cache-Control") != "no-store" {
			t.Fatalf("headers = %v", h)
		}
	}

	r.SetState(Ready)
	down.Store(true)
	code, rep, _ := serve(t, r, "/readyz")
	if code != 503 || rep.Checks["dep"].Status != StatusFail {
		t.Fatalf("failing dep: code=%d report=%+v", code, rep)
	}
	// Liveness must ignore dependencies.
	if code, _, _ := serve(t, r, "/healthz"); code != 200 {
		t.Fatalf("/healthz = %d with a failing dependency", code)
	}
}

func grpcClient(t *testing.T, g *GRPCServer) healthpb.HealthClient {
	t.Helper()
	lis := bufconn.Listen(1 << 16)
	s := grpc.NewServer()
	if !g.Register(s) {
		t.Fatal("expected Register to add the service")
	}
	go func() { _ = s.Serve(lis) }()
	t.Cleanup(s.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return healthpb.NewHealthClient(conn)
}

func TestGRPCCheckFollowsReadiness(t *testing.T) {
	r := New(WithCacheTTL(0))
	var down atomic.Bool
	_ = r.Add(Check{Name: "dep", Check: func(context.Context) error {
		if down.Load() {
			return errors.New("down")
		}
		return nil
	}})
	client := grpcClient(t, NewGRPCServer(r))

	check := func(service string) healthpb.HealthCheckResponse_ServingStatus {
		t.Helper()
		resp, err := client.Check(context.Background(), &healthpb.HealthCheckRequest{Service: service})
		if err != nil {
			t.Fatal(err)
		}
		return resp.Status
	}

	if got := check(""); got != healthpb.HealthCheckResponse_NOT_SERVING {
		t.Fatalf("starting: %v", got)
	}
	r.SetState(Ready)
	if got := check(""); got != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("ready: %v", got)
	}
	if got := check("any.Service"); got != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("named service: %v", got)
	}
	down.Store(true)
	if got := check(""); got != healthpb.HealthCheckResponse_NOT_SERVING {
		t.Fatalf("dep down: %v", got)
	}
	down.Store(false)
	r.SetState(Draining)
	if got := check(""); got != healthpb.HealthCheckResponse_NOT_SERVING {
		t.Fatalf("draining: %v", got)
	}
}

func TestGRPCWatchStreamsChanges(t *testing.T) {
	r := New(WithCacheTTL(0))
	g := NewGRPCServer(r)
	g.interval = 10 * time.Millisecond
	client := grpcClient(t, g)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stream, err := client.Watch(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatal(err)
	}
	recv := func() healthpb.HealthCheckResponse_ServingStatus {
		t.Helper()
		resp, err := stream.Recv()
		if err != nil {
			t.Fatal(err)
		}
		return resp.Status
	}

	if got := recv(); got != healthpb.HealthCheckResponse_NOT_SERVING {
		t.Fatalf("initial: %v", got)
	}
	r.SetState(Ready)
	if got := recv(); got != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("after ready: %v", got)
	}
	r.SetState(Draining)
	if got := recv(); got != healthpb.HealthCheckResponse_NOT_SERVING {
		t.Fatalf("after draining: %v", got)
	}
}

func TestGRPCRegisterDoesNotOverrideApplicationService(t *testing.T) {
	s := grpc.NewServer()
	healthpb.RegisterHealthServer(s, healthpb.UnimplementedHealthServer{})
	if NewGRPCServer(New()).Register(s) {
		t.Fatal("must not register a second health service")
	}
}
