package admin

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	prom "github.com/prometheus/client_golang/prometheus"
)

func get(t *testing.T, url string) (int, string) {
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

func TestServerServesMetricsAndCustomHandlers(t *testing.T) {
	reg := prom.NewRegistry()
	c := prom.NewCounter(prom.CounterOpts{Name: "admin_test_total", Help: "test"})
	reg.MustRegister(c)
	c.Inc()

	s := New(reg)
	s.Handle("/custom", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	}))
	if err := s.Start("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	base := "http://" + s.Addr().String()

	code, body := get(t, base+"/metrics")
	if code != http.StatusOK || !strings.Contains(body, "admin_test_total 1") {
		t.Fatalf("/metrics: code=%d body=%q", code, body)
	}
	if code, body := get(t, base+"/custom"); code != http.StatusOK || body != "hello" {
		t.Fatalf("/custom: code=%d body=%q", code, body)
	}

	addr := s.Addr().String()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if err := s.Shutdown(ctx); err != nil {
		t.Fatalf("second Shutdown: %v", err)
	}
	if s.Addr() != nil {
		t.Fatal("Addr should be nil after Shutdown")
	}
	if c, err := net.DialTimeout("tcp", addr, 200*time.Millisecond); err == nil {
		_ = c.Close()
		t.Fatal("still listening after Shutdown")
	}
}

func TestStartReturnsErrorWhenAddressInUse(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()

	if err := New(prom.NewRegistry()).Start(l.Addr().String()); err == nil {
		t.Fatal("expected a listen error")
	}
}

func TestShutdownBeforeStartIsNoop(t *testing.T) {
	if err := New(prom.NewRegistry()).Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
}
