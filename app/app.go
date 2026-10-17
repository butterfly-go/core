package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"butterfly.orx.me/core/internal/config"
	"butterfly.orx.me/core/internal/log"
	"butterfly.orx.me/core/internal/observe/metric"
	"butterfly.orx.me/core/internal/observe/tracing"
	"butterfly.orx.me/core/internal/runtime"
	"butterfly.orx.me/core/internal/store"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
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

type Config struct {
	Service      string
	Namespace    string
	Config       config.AppConfig
	Router       func(*gin.Engine)
	GRPCRegister func(*grpc.Server)
	InitFunc     []func() error
	// TeardownFunc are called during graceful shutdown, after the HTTP and
	// gRPC servers have stopped accepting work and before the core resources
	// (stores, tracing, metrics) are released. They run in reverse order, like
	// defer, and an error or panic in one does not stop the others.
	TeardownFunc []func() error
	// ShutdownTimeout bounds the whole graceful shutdown sequence. Defaults to
	// DefaultShutdownTimeout when zero or negative.
	ShutdownTimeout time.Duration
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

	// Listen addresses. Empty means the default (see httpListenAddr and
	// grpcListenAddr); tests override them.
	httpAddr string
	grpcAddr string
}

func New(config *Config) *App {
	return &App{
		config: config,
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
		NewFn(metric.Init),
		NewFn(tracing.Init),
		NewFn(store.Init),
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

	var serveErr error
	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received, shutting down gracefully")
	case serveErr = <-srvs.errCh:
		logger.Error("server stopped unexpectedly, shutting down", "error", serveErr.Error())
	}

	shutdownCtx, cancel := a.newShutdownContext()
	defer cancel()
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
// attempted regardless of earlier failures. Metrics go last so they remain
// scrapeable while everything else drains.
func (a *App) shutdownCore(ctx context.Context) {
	logger := log.CoreLogger("app.shutdown")
	steps := []struct {
		name string
		fn   func(context.Context) error
	}{
		{"store", store.Close},
		{"tracing", tracing.Shutdown},
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

func (a *App) httpListenAddr() string {
	if a.httpAddr != "" {
		return a.httpAddr
	}
	// Same default as gin's Engine.Run: $PORT, falling back to 8080.
	if port := os.Getenv("PORT"); port != "" {
		return ":" + port
	}
	return ":8080"
}

func (a *App) grpcListenAddr() string {
	if a.grpcAddr != "" {
		return a.grpcAddr
	}
	return ":9090"
}

func (a *App) newHTTPServer() *http.Server {
	r := gin.New()
	// Disable log by default
	r.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		Output: io.Discard,
	}))

	// Recovery middleware recovers from any panics and writes a 500 if there was one.
	r.Use(gin.Recovery())

	r.Use(otelgin.Middleware(a.config.Service))
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

	httpLis, err := net.Listen("tcp", a.httpListenAddr())
	if err != nil {
		return nil, fmt.Errorf("listen http: %w", err)
	}
	var grpcLis net.Listener
	if a.config.GRPCRegister != nil {
		grpcLis, err = net.Listen("tcp", a.grpcListenAddr())
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
		s.grpc = grpc.NewServer()
		a.config.GRPCRegister(s.grpc)
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
