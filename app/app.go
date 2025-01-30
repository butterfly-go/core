package app

import (
	"context"
	"fmt"
	"net"

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

type Config struct {
	Service      string
	Config       config.AppConfig
	Router       func(*gin.Engine)
	GRPCRegister func(*grpc.Server)
	InitFunc     []func() error
	TeardownFunc []func() error
}

func (c Config) ConfigKey() string {
	// @todo
	return c.Service
}

type App struct {
	config *Config
}

func New(config *Config) *App {
	return &App{
		config: config,
	}
}

func (a *App) Run() {
	runtime.SetService(a.config.Service)

	appendFn(
		NewFn(config.Init),
		NewFn(a.InitAppConfig),
		NewFn(config.CoreConfigInit),
		NewFn(metric.Init),
		NewFn(tracing.Init),
		NewFn(store.Init),
	)

	for _, fn := range a.config.InitFunc {
		appendFn(NewFn(fn))
	}

	// do func init
	err := do()
	if err != nil {
		panic(err)
	}

	if a.config.GRPCRegister != nil {
		go a.GRPCServer()
	}

	_ = a.HTTPServer()
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

func (a *App) HTTPServer() error {
	r := gin.Default()
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
	// run grpc server
	log.CoreLogger("grpc").Info("grpc server listening ", "addr", lis.Addr())
	if err := server.Serve(lis); err != nil {
		panic(err)
	}
}
