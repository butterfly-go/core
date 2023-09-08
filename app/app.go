package app

import (
	"github.com/gin-gonic/gin"
)

type Config struct {
	Router func(*gin.Engine)
}

type App struct {
}

func (a *App) Run() {

}
