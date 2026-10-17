package app

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	return addr
}

func waitForTCP(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			_ = c.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("%s did not start listening", addr)
}

func TestShutdownTimeoutDefault(t *testing.T) {
	if got := (Config{}).shutdownTimeout(); got != DefaultShutdownTimeout {
		t.Fatalf("default = %v, want %v", got, DefaultShutdownTimeout)
	}
	if got := (Config{ShutdownTimeout: -1}).shutdownTimeout(); got != DefaultShutdownTimeout {
		t.Fatalf("negative = %v, want %v", got, DefaultShutdownTimeout)
	}
	if got := (Config{ShutdownTimeout: time.Second}).shutdownTimeout(); got != time.Second {
		t.Fatalf("custom = %v, want %v", got, time.Second)
	}
}

func TestServeDrainsInFlightRequestsAndRunsTeardownInReverse(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})

	var mu sync.Mutex
	var order []int
	record := func(i int, err error) func() error {
		return func() error {
			mu.Lock()
			defer mu.Unlock()
			order = append(order, i)
			return err
		}
	}

	httpAddr := freeAddr(t)
	a := New(&Config{
		Service: "shutdown-test",
		Router: func(r *gin.Engine) {
			r.GET("/slow", func(c *gin.Context) {
				close(started)
				<-release
				c.String(http.StatusOK, "done")
			})
		},
		TeardownFunc: []func() error{
			record(0, nil),
			record(1, errors.New("boom")), // an error must not stop the rest
			func() error { // nor must a panic
				_ = record(2, nil)()
				panic("teardown panic")
			},
		},
		ShutdownTimeout: 10 * time.Second,
	})
	a.addrs.HTTP = httpAddr

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serveDone := make(chan error, 1)
	go func() { serveDone <- a.serve(ctx) }()
	waitForTCP(t, httpAddr)

	type result struct {
		body string
		err  error
	}
	respCh := make(chan result, 1)
	go func() {
		resp, err := http.Get("http://" + httpAddr + "/slow")
		if err != nil {
			respCh <- result{err: err}
			return
		}
		defer func() { _ = resp.Body.Close() }()
		b, err := io.ReadAll(resp.Body)
		respCh <- result{body: string(b), err: err}
	}()

	<-started
	cancel()

	// Shutdown must wait for the in-flight request.
	select {
	case err := <-serveDone:
		t.Fatalf("serve returned before in-flight request finished: %v", err)
	case <-time.After(200 * time.Millisecond):
	}

	close(release)

	select {
	case res := <-respCh:
		if res.err != nil || res.body != "done" {
			t.Fatalf("in-flight request: body=%q err=%v", res.body, res.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("in-flight request did not complete")
	}
	select {
	case err := <-serveDone:
		if err != nil {
			t.Fatalf("serve returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return after shutdown")
	}

	mu.Lock()
	defer mu.Unlock()
	if want := []int{2, 1, 0}; !slices.Equal(order, want) {
		t.Fatalf("teardown order = %v, want %v", order, want)
	}

	// The listener must be released.
	if c, err := net.DialTimeout("tcp", httpAddr, 200*time.Millisecond); err == nil {
		_ = c.Close()
		t.Fatal("http server still accepting connections after shutdown")
	}
}

func TestServeForcesShutdownAfterTimeout(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	defer close(release)

	httpAddr := freeAddr(t)
	a := New(&Config{
		Service: "shutdown-timeout-test",
		Router: func(r *gin.Engine) {
			r.GET("/stuck", func(_ *gin.Context) {
				close(started)
				<-release
			})
		},
		ShutdownTimeout: 200 * time.Millisecond,
	})
	a.addrs.HTTP = httpAddr

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serveDone := make(chan error, 1)
	go func() { serveDone <- a.serve(ctx) }()
	waitForTCP(t, httpAddr)

	go func() {
		resp, err := http.Get("http://" + httpAddr + "/stuck")
		if err == nil {
			_ = resp.Body.Close()
		}
	}()
	<-started
	cancel()

	select {
	case err := <-serveDone:
		if err != nil {
			t.Fatalf("serve returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return after shutdown timeout")
	}
}

func TestServeStopsGRPCServer(t *testing.T) {
	httpAddr, grpcAddr := freeAddr(t), freeAddr(t)
	a := New(&Config{
		Service: "grpc-shutdown-test",
		GRPCRegister: func(s *grpc.Server) {
			healthpb.RegisterHealthServer(s, health.NewServer())
		},
		ShutdownTimeout: 5 * time.Second,
	})
	a.addrs.HTTP, a.addrs.GRPC = httpAddr, grpcAddr

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serveDone := make(chan error, 1)
	go func() { serveDone <- a.serve(ctx) }()
	waitForTCP(t, grpcAddr)

	conn, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	callCtx, callCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer callCancel()
	if _, err := healthpb.NewHealthClient(conn).Check(callCtx, &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatalf("grpc health check: %v", err)
	}

	cancel()
	select {
	case err := <-serveDone:
		if err != nil {
			t.Fatalf("serve returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return after shutdown")
	}
	if c, err := net.DialTimeout("tcp", grpcAddr, 200*time.Millisecond); err == nil {
		_ = c.Close()
		t.Fatal("grpc server still accepting connections after shutdown")
	}
}

func TestServeReturnsErrorWhenAddressInUse(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()

	var tornDown bool
	a := New(&Config{
		Service:      "listen-error-test",
		TeardownFunc: []func() error{func() error { tornDown = true; return nil }},
	})
	a.addrs.HTTP = l.Addr().String()

	done := make(chan error, 1)
	go func() { done <- a.serve(context.Background()) }()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected a listen error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return on listen error")
	}
	if !tornDown {
		t.Fatal("teardown funcs should run when serving fails to start")
	}
}
