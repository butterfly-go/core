package config

import (
	"context"
	"errors"
	"os"

	"butterfly.orx.me/core/internal/arg"
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

type FileConfig struct {
	path string
}

func NewFileConfig() (*FileConfig, error) {
	path := arg.String("config.file.path")
	if path == "" {
		return nil, errors.New("config.file.path not set")
	}
	return &FileConfig{path: path}, nil
}

func (f *FileConfig) Get(_ context.Context, key string) ([]byte, error) {
	// For file config, ignore key and just read the file
	return os.ReadFile(f.path)
}

func Init() error {
	configType := arg.String("config.type")
	if configType == "file" {
		c, err := NewFileConfig()
		if err != nil {
			return err
		}
		config = c
		return nil
	}
	c, err := NewConsulConfig()
	if err != nil {
		return err
	}
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
