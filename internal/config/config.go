package config

import (
	"context"
	"errors"
	"os"

	"butterfly.orx.me/core/internal/arg"
	"butterfly.orx.me/core/internal/log"
	"butterfly.orx.me/core/mod"
	"gopkg.in/yaml.v3"
)

// Config is the interface for configuration backends.
type Config interface {
	Get(ctx context.Context, key string) ([]byte, error)
}

// AppConfig is the interface that user application configs must implement.
type AppConfig interface {
	Print()
}

// FileConfig reads configuration from a local file.
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
	return os.ReadFile(f.path)
}

// --- Wire Providers ---

// ProvideConfig creates the config backend based on BUTTERFLY_CONFIG_TYPE env var.
func ProvideConfig() (Config, error) {
	configType := arg.String("config.type")
	if configType == "file" {
		return NewFileConfig()
	}
	return NewConsulConfig()
}

// ProvideCoreConfig loads the core configuration from the config backend.
func ProvideCoreConfig(cfg Config, key mod.ConfigKey) (*mod.CoreConfig, error) {
	ctx := context.Background()
	logger := log.CoreLogger("core.config.init")
	b, err := cfg.Get(ctx, string(key))
	if err != nil {
		logger.Error("get core config error",
			"key", string(key),
			"error", err.Error())
		return nil, err
	}
	cc := new(mod.CoreConfig)
	if err := yaml.Unmarshal(b, cc); err != nil {
		return nil, err
	}
	logger.Info("core config",
		"store_mongo", len(cc.Store.Mongo))
	return cc, nil
}

// LoadAppConfig reads the application config document and unmarshals it into target.
func LoadAppConfig(cfg Config, key string, target AppConfig) error {
	ctx := context.Background()
	logger := log.CoreLogger("app.init.config")
	b, err := cfg.Get(ctx, key)
	if err != nil {
		logger.Error("get app config error",
			"key", key,
			"error", err.Error())
		return err
	}
	if err := yaml.Unmarshal(b, target); err != nil {
		logger.Error("unmarshal failed", "error", err.Error())
		return err
	}
	return nil
}
