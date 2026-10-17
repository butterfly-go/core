// Package admin runs the operational HTTP server: /metrics today, and health
// endpoints later. It is separate from the business HTTP server so that user
// routes can never collide with it and so that it stays up while the business
// server drains during shutdown.
package admin

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"butterfly.orx.me/core/internal/log"
)

const readHeaderTimeout = 10 * time.Second

// Server is the admin HTTP server.
type Server struct {
	mux *http.ServeMux

	mu   sync.Mutex
	http *http.Server
	addr net.Addr
}

// New returns a Server that serves /metrics from registry.
func New(registry *prom.Registry) *Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.InstrumentMetricHandler(
		registry,
		promhttp.HandlerFor(registry, promhttp.HandlerOpts{})),
	)
	return &Server{mux: mux}
}

// Handle registers an additional handler. It is safe to call before or after
// Start.
func (s *Server) Handle(pattern string, h http.Handler) {
	s.mux.Handle(pattern, h)
}

// Start binds addr synchronously, so an address conflict is reported to the
// caller, then serves in the background.
func (s *Server) Start(addr string) error {
	logger := log.CoreLogger("admin")
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen admin: %w", err)
	}
	srv := &http.Server{
		Handler:           s.mux,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	s.mu.Lock()
	s.http, s.addr = srv, lis.Addr()
	s.mu.Unlock()

	logger.Info("admin server listening", "addr", lis.Addr().String(), "metrics_path", "/metrics")
	go func() {
		if err := srv.Serve(lis); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("admin server failed", "error", err.Error())
		}
	}()
	return nil
}

// Addr returns the address the server is listening on, or nil before Start.
func (s *Server) Addr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.addr
}

// Shutdown gracefully stops the server. It is a no-op if the server was never
// started and is safe to call more than once.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	srv := s.http
	s.http, s.addr = nil, nil
	s.mu.Unlock()
	if srv == nil {
		return nil
	}
	if err := srv.Shutdown(ctx); err != nil {
		_ = srv.Close()
		return fmt.Errorf("shutdown admin server: %w", err)
	}
	return nil
}
