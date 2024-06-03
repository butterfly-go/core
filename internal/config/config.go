package config

import (
	"context"

	"butterfly.orx.me/core/mod"
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
