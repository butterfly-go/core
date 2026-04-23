package core

import "butterfly.orx.me/core/app"

func New(c *app.Config) app.Application {
	return app.New(c)
}
