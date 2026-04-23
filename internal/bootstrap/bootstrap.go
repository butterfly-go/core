package bootstrap

import (
	"butterfly.orx.me/core/internal/config"
	"butterfly.orx.me/core/internal/log"
	"butterfly.orx.me/core/internal/observe/metric"
	"butterfly.orx.me/core/internal/observe/tracing"
	"butterfly.orx.me/core/internal/runtime"
	"butterfly.orx.me/core/internal/store"
	"butterfly.orx.me/core/mod"
)

// Dependencies holds all Wire-injected dependencies used during bootstrap.
type Dependencies struct {
	Config     config.Config
	CoreConfig *mod.CoreConfig
	Redis      store.RedisClients
	Mongo      store.MongoClients
	SQLDB      store.SQLDBClients
	S3         *store.S3Store
}

// Init prepares process-wide runtime state and exposes shared resources.
func Init(service string, configKey string, appConfig config.AppConfig, deps *Dependencies) error {
	runtime.Init(service, configKey)

	if err := config.LoadAppConfig(deps.Config, configKey, appConfig); err != nil {
		return err
	}

	log.Init(deps.CoreConfig.Log)
	if err := metric.Init(); err != nil {
		return err
	}
	if err := tracing.Init(); err != nil {
		return err
	}

	config.SetConfig(deps.Config)
	store.SetRedisClients(deps.Redis)
	store.SetMongoClients(deps.Mongo)
	store.SetSQLDBClients(deps.SQLDB)
	store.SetS3Store(deps.S3)
	return nil
}
