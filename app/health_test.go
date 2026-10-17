package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"butterfly.orx.me/core/internal/config"
	"butterfly.orx.me/core/internal/health"
	"butterfly.orx.me/core/mod"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func httpGet(t *testing.T, url string) (int, string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, string(b)
}

func TestReadinessLifecycleAroundServe(t *testing.T) {
	a := New(&Config{Service: "health-lifecycle-test", ShutdownTimeout: 5 * time.Second})
	a.addrs.HTTP = freeAddr(t)
	if st := a.health.State(); st != health.Starting {
		t.Fatalf("initial state = %v", st)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- a.serve(ctx) }()

	eventually(t, "ready", func() bool { return a.health.State() == health.Ready })
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if st := a.health.State(); st != health.Draining {
		t.Fatalf("state after shutdown = %v, want draining", st)
	}
}

func TestShutdownDelayKeepsServingWhileNotReady(t *testing.T) {
	a := New(&Config{Service: "shutdown-delay-test", ShutdownTimeout: 10 * time.Second})
	a.addrs.HTTP = freeAddr(t)
	a.shutdownDelay = 400 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- a.serve(ctx) }()
	eventually(t, "ready", func() bool { return a.health.State() == health.Ready })

	start := time.Now()
	cancel()
	eventually(t, "draining", func() bool { return a.health.State() == health.Draining })

	// During the delay the app reports not-ready but still accepts traffic.
	conn, err := (&http.Client{Timeout: time.Second}).Get("http://" + a.addrs.HTTP + "/")
	if err != nil {
		t.Fatalf("server should still accept connections during the delay: %v", err)
	}
	_ = conn.Body.Close()

	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed < 350*time.Millisecond {
		t.Fatalf("shutdown finished after %v, before the delay elapsed", elapsed)
	}
}

func TestStartupFailureIsNotSlowedByShutdownDelay(t *testing.T) {
	// A listen error fails before the signal-wait, so the delay must not apply.
	a := New(&Config{Service: "delay-skip-test"})
	a.shutdownDelay = time.Hour
	a.addrs.HTTP = "127.0.0.1:-1"
	done := make(chan error, 1)
	go func() { done <- a.serve(context.Background()) }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected an error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve blocked on the shutdown delay after a startup failure")
	}
}

func TestAdminServesHealthEndpointsAndGRPCHealth(t *testing.T) {
	dep := errors.New("dependency down")
	var depErr error
	a := New(&Config{
		Service:         "admin-health-test",
		ShutdownTimeout: 5 * time.Second,
		GRPCRegister:    func(*grpc.Server) {},
		HealthChecks: []HealthCheck{{
			Name:  "custom",
			Check: func(context.Context) error { return depErr },
		}},
	})
	a.addrs = config.ServerAddrs{HTTP: freeAddr(t), GRPC: freeAddr(t), Admin: "127.0.0.1:0"}
	if err := a.initHealth(); err != nil {
		t.Fatal(err)
	}
	if err := a.initAdmin(); err != nil {
		t.Fatal(err)
	}
	admin := "http://" + a.admin.Addr().String()

	// Starting: alive but not ready.
	if code, _ := httpGet(t, admin+"/healthz"); code != 200 {
		t.Fatalf("/healthz while starting = %d", code)
	}
	if code, body := httpGet(t, admin+"/readyz"); code != 503 || !strings.Contains(body, `"starting"`) {
		t.Fatalf("/readyz while starting = %d %s", code, body)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- a.serve(ctx) }()
	waitForTCP(t, a.addrs.GRPC)
	eventually(t, "ready", func() bool { return a.health.State() == health.Ready })

	code, body := httpGet(t, admin+"/readyz")
	var rep health.Report
	if err := json.Unmarshal([]byte(body), &rep); err != nil {
		t.Fatal(err)
	}
	if code != 200 || rep.Checks["custom"].Status != health.StatusOK {
		t.Fatalf("/readyz when ready = %d %s", code, body)
	}

	conn, err := grpc.NewClient(a.addrs.GRPC, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	hc := healthpb.NewHealthClient(conn)
	status := func() healthpb.HealthCheckResponse_ServingStatus {
		resp, err := hc.Check(context.Background(), &healthpb.HealthCheckRequest{})
		if err != nil {
			t.Fatal(err)
		}
		return resp.Status
	}
	if got := status(); got != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("grpc health when ready = %v", got)
	}

	// A failing custom check turns both probes red (after the 1s cache TTL).
	depErr = dep
	eventually(t, "readyz 503", func() bool { c, _ := httpGet(t, admin+"/readyz"); return c == 503 })
	if got := status(); got != healthpb.HealthCheckResponse_NOT_SERVING {
		t.Fatalf("grpc health with failing dep = %v", got)
	}
	if code, _ := httpGet(t, admin+"/healthz"); code != 200 {
		t.Fatalf("/healthz must ignore dependencies, got %d", code)
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	// The admin server is torn down by shutdownCore, which serve ran.
	if resp, err := http.Get(admin + "/healthz"); err == nil {
		_ = resp.Body.Close()
		t.Fatal("admin server should be stopped after shutdown")
	}
}

func TestInitHealthRejectsDuplicateNames(t *testing.T) {
	ok := func(context.Context) error { return nil }
	a := New(&Config{
		Service:      "dup-test",
		HealthChecks: []HealthCheck{{Name: "x", Check: ok}, {Name: "x", Check: ok}},
	})
	if err := a.initHealth(); err == nil {
		t.Fatal("expected duplicate name error")
	}
}

func TestInitServerShutdownDelayValidation(t *testing.T) {
	orig := config.CoreConfig().Server
	t.Cleanup(func() { config.CoreConfig().Server = orig })

	tests := []struct {
		name    string
		delay   time.Duration
		timeout time.Duration
		wantErr bool
	}{
		{"unset", 0, 0, false},
		{"ok", 5 * time.Second, 0, false},
		{"negative", -time.Second, 0, true},
		{"equals default timeout", DefaultShutdownTimeout, 0, true},
		{"exceeds custom timeout", 10 * time.Second, 5 * time.Second, true},
		{"below custom timeout", 2 * time.Second, 5 * time.Second, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.CoreConfig().Server = mod.ServerConfig{ShutdownDelay: tt.delay}
			a := New(&Config{Service: "delay-validation", ShutdownTimeout: tt.timeout})
			err := a.initServer()
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && a.shutdownDelay != tt.delay {
				t.Fatalf("shutdownDelay = %v, want %v", a.shutdownDelay, tt.delay)
			}
		})
	}
}

func TestGRPCInterceptorsFromConfigAreInstalled(t *testing.T) {
	var unaryCalls, streamCalls atomic.Int32
	a := New(&Config{
		Service:         "grpc-interceptor-test",
		ShutdownTimeout: 5 * time.Second,
		GRPCRegister:    func(*grpc.Server) {},
		GRPCUnaryInterceptors: []grpc.UnaryServerInterceptor{
			func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
				unaryCalls.Add(1)
				return h(ctx, req)
			},
		},
		GRPCStreamInterceptors: []grpc.StreamServerInterceptor{
			func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, h grpc.StreamHandler) error {
				streamCalls.Add(1)
				return h(srv, ss)
			},
		},
	})
	a.addrs = config.ServerAddrs{HTTP: freeAddr(t), GRPC: freeAddr(t), Admin: "127.0.0.1:0"}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- a.serve(ctx) }()
	waitForTCP(t, a.addrs.GRPC)

	conn, err := grpc.NewClient(a.addrs.GRPC, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	callCtx, callCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer callCancel()
	hc := healthpb.NewHealthClient(conn)
	if _, err := hc.Check(callCtx, &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatal(err)
	}
	// Watch is a server-streaming RPC.
	watchCtx, watchCancel := context.WithCancel(callCtx)
	stream, err := hc.Watch(watchCtx, &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Recv(); err != nil {
		t.Fatal(err)
	}
	watchCancel()

	if got := unaryCalls.Load(); got != 1 {
		t.Fatalf("unary interceptor calls = %d, want 1", got)
	}
	eventually(t, "stream interceptor", func() bool { return streamCalls.Load() == 1 })

	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
