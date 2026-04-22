package app

import (
	"butterfly.orx.me/core/internal/config"
	"butterfly.orx.me/core/internal/store"
	"butterfly.orx.me/core/mod"
)

// Dependencies holds all Wire-injected dependencies.
type Dependencies struct {
	Config     config.Config
	CoreConfig *mod.CoreConfig
	Redis      store.RedisClients
	Mongo      store.MongoClients
	SQLDB      store.SQLDBClients
	S3         *store.S3Store
}
