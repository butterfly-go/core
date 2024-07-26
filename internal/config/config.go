package config

import (
	"context"

	"butterfly.orx.me/core/internal/log"
	"butterfly.orx.me/core/internal/runtime"
	"butterfly.orx.me/core/mod"
	"gopkg.in/yaml.v3"
)

var (
	config Config

	coreConfig = new(mod.CoreConfig)
)

type Config interface {
	Get(ctx context.Context, key string) ([]byte, error)
}

func CoreConfig() *mod.CoreConfig {
	return coreConfig
}

type AppConfig interface {
	Print()
}

func Init() error {
	c, err := NewConsulConfig()
	if err != nil {
		return err
	}
	// set config
	config = c
	return nil
}

func GetConfig() Config {
	return config
}

func CoreConfigInit() error {
	ctx := context.Background()
	logger := log.CoreLogger("core.config.init")
	configKey := runtime.Service()
	b, err := GetConfig().Get(ctx, configKey)
	if err != nil {
		logger.Error("get app config error",
			"key", configKey,
			"error", err.Error())
		return err
	}
	err = yaml.Unmarshal(b, coreConfig)
	if err != nil {
		return err
	}
	logger.Info("core config",
		"store_mongo", len(coreConfig.Store.Mongo))
	return nil
}
