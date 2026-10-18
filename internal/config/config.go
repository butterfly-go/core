package config

import (
	"context"
	"errors"
	"fmt"
	"os"

	"butterfly.orx.me/core/internal/arg"
	"butterfly.orx.me/core/internal/log"
	"butterfly.orx.me/core/internal/runtime"
	corelog "butterfly.orx.me/core/log"
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

// Watcher is implemented by backends that can report changes to a key. Consul
// does; the file backend does not.
type Watcher interface {
	// Watch blocks until ctx is cancelled, calling onChange with the new value
	// whenever the key's value changes. initial is the value already loaded,
	// the baseline changes are compared against.
	Watch(ctx context.Context, key string, initial []byte, onChange func(value []byte)) error
}

// ErrWatchUnsupported is returned by Watch when the configured backend cannot
// watch for changes.
var ErrWatchUnsupported = errors.New("config backend does not support watching")

// Watch reads key once as the baseline and then calls onChange whenever its
// value changes, until ctx is cancelled. It returns ErrWatchUnsupported for
// backends that cannot watch.
func Watch(ctx context.Context, key string, onChange func(value []byte)) error {
	w, ok := GetConfig().(Watcher)
	if !ok {
		return ErrWatchUnsupported
	}
	initial, err := GetConfig().Get(ctx, key)
	if err != nil {
		return err
	}
	return w.Watch(ctx, key, initial, onChange)
}

// Validator is implemented by configuration structs that can check
// themselves. See mod.CoreConfig and app.Config.Config.
type Validator interface {
	Validate() error
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

func (f *FileConfig) Get(_ context.Context, _ string) ([]byte, error) {
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
	configKey := runtime.ConfigKey()
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
	if err := coreConfig.Validate(); err != nil {
		logger.Error("invalid core config", "key", configKey, "error", err.Error())
		return fmt.Errorf("invalid core config: %w", err)
	}
	logger.Info("core config",
		"store_mongo", len(coreConfig.Store.Mongo))
	return nil
}

func LogInit() error {
	corelog.Init(coreConfig.Log)
	return nil
}
