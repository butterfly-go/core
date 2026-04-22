package app

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"

	"butterfly.orx.me/core/internal/config"
	"butterfly.orx.me/core/internal/log"
	"butterfly.orx.me/core/internal/observe/metric"
	"butterfly.orx.me/core/internal/observe/tracing"
	"butterfly.orx.me/core/internal/runtime"
	"butterfly.orx.me/core/internal/store"
	corelog "butterfly.orx.me/core/log"
	"butterfly.orx.me/core/mod"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v3"
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

func (c Config) ConfigKey() string {
	if c.Namespace != "" {
		return strings.Trim(c.Namespace, "/") + "/" + c.Service
	}
	return c.Service
}

type App struct {
	config  *Config
	deps    *Dependencies
	cleanup func()
}

func New(config *Config) *App {
	return &App{
		config: config,
	}
}

func (a *App) Run() {
	runtime.SetService(a.config.Service)
	runtime.SetConfigKey(a.config.ConfigKey())

	// Wire-generated dependency initialization
	deps, cleanup, err := initDeps(mod.ConfigKey(a.config.ConfigKey()))
	if err != nil {
		panic(err)
	}
	a.deps = deps
	a.cleanup = cleanup

	// Initialize app-specific config (user config struct)
	if err := a.initAppConfig(deps.Config); err != nil {
		panic(err)
	}

	// Side-effect initialization
	corelog.Init(deps.CoreConfig.Log)
	if err := metric.Init(); err != nil {
		panic(err)
	}
	if err := tracing.Init(); err != nil {
		panic(err)
	}

	// Populate internal registry for public package access
	config.SetConfig(deps.Config)
	store.SetRedisClients(deps.Redis)
	store.SetMongoClients(deps.Mongo)
	store.SetSQLDBClients(deps.SQLDB)
	store.SetS3Store(deps.S3)

	// User-provided init functions
	for _, fn := range a.config.InitFunc {
		if err := fn(); err != nil {
			panic(err)
		}
	}

	// Start servers
	if a.config.GRPCRegister != nil {
		go a.GRPCServer()
	}

	_ = a.HTTPServer()
}

func (a *App) initAppConfig(cfg config.Config) error {
	ctx := context.Background()
	logger := log.CoreLogger("app.init.config")
	b, err := cfg.Get(ctx, a.config.ConfigKey())
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

func (a *App) HTTPServer() error {
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

func (a *App) GRPCServer() {
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
