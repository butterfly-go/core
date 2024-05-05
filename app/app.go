package app

import (
	"context"

	"butterfly.orx.me/core/internal/config"
	"butterfly.orx.me/core/internal/observe/metric"
	"butterfly.orx.me/core/internal/observe/tracing"
	"butterfly.orx.me/core/internal/runtime"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Service      string
	Config       config.ConfigObject
	Router       func(*gin.Engine)
	GRPCRegister func(*grpc.Server)
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
	ctx := context.Background()
	runtime.SetService(a.config.Service)

	_ = a.InitAppConfig()
	_ = tracing.Init(ctx)
	_ = metric.Init()

	if a.config.GRPCRegister != nil {
		go a.GRPCServer()
	}

	_ = a.HTTPServer()
}

func (a *App) InitAppConfig() error {
	ctx := context.Background()
	b, err := config.GetConfig().Get(ctx, a.config.Service)

	if err != nil {
		return err
	}
	err = yaml.Unmarshal(b, a.config.Config)
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
	server := grpc.NewServer()
	a.config.GRPCRegister(server)
}
