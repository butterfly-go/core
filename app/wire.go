//go:build wireinject

package app

import (
	"butterfly.orx.me/core/internal/bootstrap"
	"butterfly.orx.me/core/internal/config"
	"butterfly.orx.me/core/internal/store"
	"butterfly.orx.me/core/mod"
	"github.com/google/wire"
)

func initDeps(key mod.ConfigKey) (*bootstrap.Dependencies, func(), error) {
	wire.Build(
		config.ProvideConfig,
		config.ProvideCoreConfig,
		store.ProvideRedisClients,
		store.ProvideMongoClients,
		store.ProvideSQLDBClients,
		store.ProvideS3Store,
		wire.Struct(new(bootstrap.Dependencies), "*"),
	)
	return nil, nil, nil
}
