package app

import (
	"context"

	"butterfly.orx.me/core/internal/observe/tracing"
	"butterfly.orx.me/core/internal/runtime"
	"github.com/gin-gonic/gin"
)

type Config struct {
	Service string
	Router  func(*gin.Engine)
}

type App struct {
	config *Config
}

func (a *App) Run() {
	ctx := context.Background()
	runtime.SetService(a.config.Service)
	tracing.Init(ctx)
	a.HTTPServer()
}

func (a *App) HTTPServer() error {
	r := gin.Default()
	a.config.Router(r)
	return r.Run()
}
