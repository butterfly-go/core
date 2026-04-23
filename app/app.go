package app

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"

	"butterfly.orx.me/core/internal/bootstrap"
	"butterfly.orx.me/core/internal/config"
	"butterfly.orx.me/core/internal/log"
	"butterfly.orx.me/core/mod"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"google.golang.org/grpc"
)

type Config struct {
	Service      string
	Namespace    string
	Config       config.AppConfig
	Router       func(*gin.Engine)
	GRPCRegister func(*grpc.Server)
	InitFunc     []func() error
	TeardownFunc []func() error
}

// Application is the public lifecycle contract returned by New.
type Application interface {
	Run()
	Close() error
}

func (c Config) ConfigKey() string {
	if c.Namespace != "" {
		return strings.Trim(c.Namespace, "/") + "/" + c.Service
	}
	return c.Service
}

type application struct {
	config    *Config
	deps      *bootstrap.Dependencies
	cleanup   func()
	closeOnce sync.Once
}

var _ Application = (*application)(nil)

func New(config *Config) Application {
	return &application{
		config: config,
	}
}

func (a *application) Run() {
	// Wire-generated dependency initialization
	deps, cleanup, err := initDeps(mod.ConfigKey(a.config.ConfigKey()))
	if err != nil {
		panic(err)
	}
	a.deps = deps
	a.cleanup = cleanup
	defer func() {
		closeErr := a.Close()
		if r := recover(); r != nil {
			panic(r)
		}
		if closeErr != nil {
			panic(closeErr)
		}
	}()

	if err := bootstrap.Init(a.config.Service, a.config.ConfigKey(), a.config.Config, deps); err != nil {
		panic(err)
	}

	// User-provided init functions
	for _, fn := range a.config.InitFunc {
		if err := fn(); err != nil {
			panic(err)
		}
	}

	// Start servers
	if a.config.GRPCRegister != nil {
		go a.grpcServer()
	}

	if err := a.httpServer(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}

func (a *application) Close() error {
	var err error
	a.closeOnce.Do(func() {
		var errs []error
		if a.config != nil {
			teardowns := slices.Clone(a.config.TeardownFunc)
			slices.Reverse(teardowns)
			for _, fn := range teardowns {
				if fn == nil {
					continue
				}
				if teardownErr := fn(); teardownErr != nil {
					errs = append(errs, teardownErr)
				}
			}
		}
		if a.cleanup != nil {
			a.cleanup()
			a.cleanup = nil
		}
		a.deps = nil
		err = errors.Join(errs...)
	})
	return err
}

func (a *application) httpServer() error {
	r := gin.New()
	r.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		Output: io.Discard,
	}))
	r.Use(gin.Recovery())
	r.Use(otelgin.Middleware(a.config.Service))
	if a.config.Router != nil {
		a.config.Router(r)
	}
	return r.Run()
}

func (a *application) grpcServer() {
	var port = 9090
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		panic(err)
	}
	server := grpc.NewServer()
	a.config.GRPCRegister(server)
	log.CoreLogger("grpc").Info("grpc server listening ", "addr", lis.Addr())
	if err := server.Serve(lis); err != nil {
		panic(err)
	}
}
