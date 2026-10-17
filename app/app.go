package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"butterfly.orx.me/core/internal/admin"
	"butterfly.orx.me/core/internal/config"
	"butterfly.orx.me/core/internal/grpcserver"
	"butterfly.orx.me/core/internal/health"
	"butterfly.orx.me/core/internal/httpserver"
	"butterfly.orx.me/core/internal/log"
	"butterfly.orx.me/core/internal/observe/metric"
	"butterfly.orx.me/core/internal/observe/tracing"
	"butterfly.orx.me/core/internal/runtime"
	"butterfly.orx.me/core/internal/store"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v3"
)

// DefaultShutdownTimeout is the total time budget for graceful shutdown when
// Config.ShutdownTimeout is not set.
const DefaultShutdownTimeout = 30 * time.Second

const (
	// httpReadHeaderTimeout guards against slowloris-style clients.
	httpReadHeaderTimeout = 10 * time.Second
	// serverCount is the number of network servers (HTTP and gRPC) that can
	// report a failure or finish stopping.
	serverCount = 2
)

// HealthCheck is a named readiness check. Check must return nil when the
// dependency is healthy and should honor ctx, which carries a timeout.
type HealthCheck = health.Check

type Config struct {
	Service      string
	Namespace    string
	Config       config.AppConfig
	Router       func(*gin.Engine)
	GRPCRegister func(*grpc.Server)
	// GRPCUnaryInterceptors and GRPCStreamInterceptors are your own gRPC
	// interceptors. They run in order, inside the built-in ones (tracing,
	// logging with trace ids, panic recovery), so they can use
	// log.FromContext and a panic in them is recovered too.
	GRPCUnaryInterceptors  []grpc.UnaryServerInterceptor
	GRPCStreamInterceptors []grpc.StreamServerInterceptor
	InitFunc               []func() error
	// TeardownFunc are called during graceful shutdown, after the HTTP and
	// gRPC servers have stopped accepting work and before the core resources
	// (stores, tracing, metrics) are released. They run in reverse order, like
	// defer, and an error or panic in one does not stop the others.
	TeardownFunc []func() error
	// ShutdownTimeout bounds the whole graceful shutdown sequence. Defaults to
	// DefaultShutdownTimeout when zero or negative.
	ShutdownTimeout time.Duration
	// HealthChecks are extra readiness checks reported on /readyz and gRPC
	// health, in addition to the built-in Redis, SQL and Mongo checks. Names
	// must be unique and must not collide with the built-in ones
	// ("redis/<name>", "sql/<name>", "mongo/<name>").
	HealthChecks []HealthCheck
}

func (c Config) ConfigKey() string {
	if c.Namespace != "" {
		return strings.Trim(c.Namespace, "/") + "/" + c.Service
	}
	return c.Service
}

func (c Config) shutdownTimeout() time.Duration {
	if c.ShutdownTimeout > 0 {
		return c.ShutdownTimeout
	}
	return DefaultShutdownTimeout
}

type App struct {
	config *Config

	// addrs are the listen addresses. They start as the defaults and are
	// replaced by the resolved core config during init (see initServer).
	addrs config.ServerAddrs
	admin *admin.Server
	// shutdownDelay is the pause between going not-ready and stopping servers.
	shutdownDelay time.Duration
	health        *health.Registry
}

func New(cfg *Config) *App {
	return &App{
		config: cfg,
		addrs:  config.DefaultServerAddrs(),
		health: health.New(),
	}
}

// Run initializes the app, serves HTTP (and gRPC when GRPCRegister is set) and
// blocks until SIGINT or SIGTERM, then shuts down gracefully. A second signal
// during shutdown terminates the process immediately.
//
// Run panics if initialization or serving fails.
func (a *App) Run() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		// Restore default signal behavior so a second signal force-quits.
		stop()
	}()

	if err := a.RunContext(ctx); err != nil {
		panic(err)
	}
}

// RunContext is like Run but is driven by ctx instead of OS signals: when ctx
// is cancelled the app shuts down gracefully and RunContext returns nil.
// It returns an error if initialization fails or a server stops unexpectedly.
func (a *App) RunContext(ctx context.Context) error {
	runtime.SetService(a.config.Service)
	runtime.SetConfigKey(a.config.ConfigKey())

	appendFn(
		NewFn(config.Init),
		NewFn(a.InitAppConfig),
		NewFn(config.CoreConfigInit),
		NewFn(config.LogInit),
		NewFn(a.initServer),
		NewFn(metric.Init),
		NewFn(a.initAdmin),
		NewFn(tracing.Init),
		NewFn(store.Init),
		NewFn(a.initHealth),
	)

	for _, fn := range a.config.InitFunc {
		appendFn(NewFn(fn))
	}

	// do func init
	if err := do(); err != nil {
		// Release whatever was partially initialized, but skip user teardown:
		// the user's own resources may never have been created.
		ctx, cancel := a.newShutdownContext()
		defer cancel()
		a.shutdownCore(ctx)
		return err
	}

	return a.serve(ctx)
}

// serve starts the servers, waits for ctx to be done (or a server to fail) and
// then runs the graceful shutdown sequence.
func (a *App) serve(ctx context.Context) error {
	logger := log.CoreLogger("app.shutdown")

	srvs, err := a.startServers()
	if err != nil {
		shutdownCtx, cancel := a.newShutdownContext()
		defer cancel()
		a.shutdown(shutdownCtx)
		return err
	}

	a.health.SetState(health.Ready)

	var serveErr error
	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received, shutting down gracefully")
	case serveErr = <-srvs.errCh:
		logger.Error("server stopped unexpectedly, shutting down", "error", serveErr.Error())
	}

	// The shutdown budget starts now, so the delay below counts against it.
	shutdownCtx, cancel := a.newShutdownContext()
	defer cancel()

	// Report not-ready first so load balancers stop routing here, and give
	// them shutdownDelay to notice. Skipped when a server already failed:
	// there is nothing worth waiting for.
	a.health.SetState(health.Draining)
	if serveErr == nil && a.shutdownDelay > 0 {
		logger.Info("not ready, waiting before stopping servers", "delay", a.shutdownDelay.String())
		select {
		case <-time.After(a.shutdownDelay):
		case <-shutdownCtx.Done():
		}
	}
	srvs.stop(shutdownCtx)
	a.shutdown(shutdownCtx)
	return serveErr
}

func (a *App) newShutdownContext() (context.Context, context.CancelFunc) {
	// Deliberately not derived from the run context: that one is already
	// cancelled by the time we shut down.
	return context.WithTimeout(context.Background(), a.config.shutdownTimeout())
}

// shutdown runs user teardown functions and then releases core resources.
func (a *App) shutdown(ctx context.Context) {
	a.runTeardown()
	a.shutdownCore(ctx)
}

func (a *App) runTeardown() {
	logger := log.CoreLogger("app.shutdown")
	fns := a.config.TeardownFunc
	for i := len(fns) - 1; i >= 0; i-- {
		runTeardownFunc(logger, i, fns[i])
	}
}

func runTeardownFunc(logger *slog.Logger, index int, fn func() error) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("teardown func panicked", "index", index, "panic", fmt.Sprint(r))
		}
	}()
	if err := fn(); err != nil {
		logger.Error("teardown func failed", "index", index, "error", err.Error())
	}
}

// shutdownCore releases the resources owned by the framework. Each step is
// attempted regardless of earlier failures. The admin server goes after the
// stores and tracing so its endpoints stay reachable while they drain.
func (a *App) shutdownCore(ctx context.Context) {
	logger := log.CoreLogger("app.shutdown")
	steps := []struct {
		name string
		fn   func(context.Context) error
	}{
		{"store", store.Close},
		{"tracing", tracing.Shutdown},
		{"admin", a.shutdownAdmin},
		{"metrics", metric.Shutdown},
	}
	for _, step := range steps {
		if err := step.fn(ctx); err != nil {
			logger.Error("shutdown step failed", "step", step.name, "error", err.Error())
		}
	}
	logger.Info("shutdown complete")
}

func (a *App) InitAppConfig() error {
	// The application config is optional: services with nothing of their own
	// to configure can leave Config unset.
	if a.config.Config == nil {
		return nil
	}
	ctx := context.Background()
	logger := log.CoreLogger("app.init.config")
	b, err := config.GetConfig().Get(ctx, a.config.ConfigKey())
	if err != nil {
		logger.Error("get app config error",
			"key", a.config.ConfigKey(),
			"error", err.Error())
		return err
	}
	err = yaml.Unmarshal(b, a.config.Config)
	if err != nil {
		logger.Error("unmarshal failed", "error", err.Error())
	}
	return err
}

// initServer resolves and validates the listen addresses from the core config.
func (a *App) initServer() error {
	addrs, err := config.ResolveServerAddrs(
		config.CoreConfig().Server, os.Getenv("PORT"), a.config.GRPCRegister != nil)
	if err != nil {
		return fmt.Errorf("server config: %w", err)
	}
	a.addrs = addrs

	delay := config.CoreConfig().Server.ShutdownDelay
	if delay < 0 {
		return fmt.Errorf("server config: shutdown_delay %s must not be negative", delay)
	}
	if timeout := a.config.shutdownTimeout(); delay >= timeout {
		return fmt.Errorf("server config: shutdown_delay %s must be smaller than the shutdown timeout %s", delay, timeout)
	}
	a.shutdownDelay = delay
	return nil
}

// initHealth registers the built-in store checks and the user's own.
func (a *App) initHealth() error {
	if err := a.health.Add(store.HealthChecks()...); err != nil {
		return fmt.Errorf("register store health checks: %w", err)
	}
	if err := a.health.Add(a.config.HealthChecks...); err != nil {
		return fmt.Errorf("register health checks: %w", err)
	}
	return nil
}

// initAdmin starts the admin server (/metrics, /healthz, /readyz).
func (a *App) initAdmin() error {
	a.admin = admin.New(metric.PrometheusRegister())
	a.health.Mount(a.admin)
	return a.admin.Start(a.addrs.Admin)
}

func (a *App) shutdownAdmin(ctx context.Context) error {
	if a.admin == nil {
		return nil
	}
	return a.admin.Shutdown(ctx)
}

func (a *App) newHTTPServer() *http.Server {
	r := httpserver.NewEngine(a.config.Service)
	if a.config.Router != nil {
		a.config.Router(r)
	}
	return &http.Server{
		Handler:           r,
		ReadHeaderTimeout: httpReadHeaderTimeout,
	}
}

// servers holds the running network servers.
type servers struct {
	http *http.Server
	grpc *grpc.Server
	// errCh receives the first unexpected server failure.
	errCh chan error
}

// startServers binds the listeners synchronously, so address conflicts are
// reported to the caller, and then serves in the background.
func (a *App) startServers() (*servers, error) {
	logger := log.CoreLogger("app.serve")
	s := &servers{errCh: make(chan error, serverCount)}

	httpLis, err := net.Listen("tcp", a.addrs.HTTP)
	if err != nil {
		return nil, fmt.Errorf("listen http: %w", err)
	}
	var grpcLis net.Listener
	if a.config.GRPCRegister != nil {
		grpcLis, err = net.Listen("tcp", a.addrs.GRPC)
		if err != nil {
			_ = httpLis.Close()
			return nil, fmt.Errorf("listen grpc: %w", err)
		}
	}

	s.http = a.newHTTPServer()
	logger.Info("http server listening", "addr", httpLis.Addr().String())
	go func() {
		if err := s.http.Serve(httpLis); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.errCh <- fmt.Errorf("http server: %w", err)
		}
	}()

	if grpcLis != nil {
		s.grpc = grpcserver.New(grpcserver.Options{
			Unary:  a.config.GRPCUnaryInterceptors,
			Stream: a.config.GRPCStreamInterceptors,
		})
		a.config.GRPCRegister(s.grpc)
		// Standard gRPC health, tied to readiness, unless the app has its own.
		health.NewGRPCServer(a.health).Register(s.grpc)
		logger.Info("grpc server listening", "addr", grpcLis.Addr().String())
		go func() {
			if err := s.grpc.Serve(grpcLis); err != nil {
				s.errCh <- fmt.Errorf("grpc server: %w", err)
			}
		}()
	}
	return s, nil
}

// stop gracefully drains both servers concurrently. If ctx expires first, the
// remaining connections are cut.
func (s *servers) stop(ctx context.Context) {
	logger := log.CoreLogger("app.shutdown")
	done := make(chan struct{}, serverCount)
	pending := 1

	go func() {
		defer func() { done <- struct{}{} }()
		if err := s.http.Shutdown(ctx); err != nil {
			logger.Warn("http graceful shutdown incomplete, closing", "error", err.Error())
			_ = s.http.Close()
		}
	}()

	if s.grpc != nil {
		pending++
		go func() {
			defer func() { done <- struct{}{} }()
			stopped := make(chan struct{})
			go func() {
				s.grpc.GracefulStop()
				close(stopped)
			}()
			select {
			case <-stopped:
			case <-ctx.Done():
				logger.Warn("grpc graceful stop timed out, forcing stop")
				s.grpc.Stop()
				<-stopped
			}
		}()
	}

	for range pending {
		<-done
	}
}
