package app

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"

	"butterfly.orx.me/core/internal/config"
	"butterfly.orx.me/core/internal/log"
	"butterfly.orx.me/core/mod"
	"gopkg.in/yaml.v3"
)

// Validator can be implemented by the struct set as Config.Config. Validate is
// called right after the document is parsed, at startup and again on every
// reload; an error stops startup, or rejects the reload and keeps the previous
// settings. It may also fill in defaults.
//
// The same check runs on the core settings (store, log, otel) through
// mod.CoreConfig.Validate, so a bad value fails fast with the key it came from.
type Validator = config.Validator

// parseAppConfig unmarshals doc into target and validates it.
func parseAppConfig(doc []byte, target config.AppConfig) error {
	if err := yaml.Unmarshal(doc, target); err != nil {
		return fmt.Errorf("unmarshal app config: %w", err)
	}
	if v, ok := target.(Validator); ok {
		if err := v.Validate(); err != nil {
			return fmt.Errorf("invalid app config: %w", err)
		}
	}
	return nil
}

// newConfigLike returns a pointer to a new zero value of the type cfg points
// to. Reloads decode into it instead of into cfg, so the value the application
// is already reading from is never written to concurrently.
func newConfigLike(cfg config.AppConfig) (config.AppConfig, error) {
	rv := reflect.ValueOf(cfg)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return nil, fmt.Errorf("Config must be a non-nil pointer, got %T", cfg)
	}
	fresh, ok := reflect.New(rv.Type().Elem()).Interface().(config.AppConfig)
	if !ok {
		return nil, fmt.Errorf("cannot create a new %T", cfg)
	}
	return fresh, nil
}

// reload handles a new configuration document: it parses and validates it
// completely and only then hands the result to OnConfigChange. A document that
// fails any step is rejected and the application keeps running on the previous
// settings.
func (a *App) reload(doc []byte) error {
	logger := log.CoreLogger("app.reload")

	core := new(mod.CoreConfig)
	if err := yaml.Unmarshal(doc, core); err != nil {
		return fmt.Errorf("unmarshal core config: %w", err)
	}
	if err := core.Validate(); err != nil {
		return fmt.Errorf("invalid core config: %w", err)
	}

	fresh, err := newConfigLike(a.config.Config)
	if err != nil {
		return err
	}
	if err := parseAppConfig(doc, fresh); err != nil {
		return err
	}

	// Stores, listeners and tracing are built once at startup.
	current := config.CoreConfig()
	if !reflect.DeepEqual(core.Store, current.Store) ||
		!reflect.DeepEqual(core.Server, current.Server) ||
		!reflect.DeepEqual(core.Otel, current.Otel) {
		logger.Warn("core config (store, server, otel) changed but is only read at startup, restart to apply it")
	}

	a.config.OnConfigChange(fresh)
	logger.Info("config reloaded")
	return nil
}

// configWatch is a running watch on the config document.
type configWatch struct {
	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once
}

// Stop ends the watch and waits for the goroutine, so no callback runs after
// it returns, unless ctx ends first (a callback that never returns must not
// block shutdown). It is safe to call more than once.
func (w *configWatch) Stop(ctx context.Context) {
	w.once.Do(func() {
		w.cancel()
		select {
		case <-w.done:
		case <-ctx.Done():
		}
	})
}

// startConfigWatch begins watching the config document when OnConfigChange is
// set. initial is the document the app started with. It returns nil when
// there is nothing to watch.
func (a *App) startConfigWatch(ctx context.Context, backend config.Config, initial []byte) *configWatch {
	if a.config.OnConfigChange == nil {
		return nil
	}
	logger := log.CoreLogger("app.reload")
	watcher, ok := backend.(config.Watcher)
	if !ok {
		logger.Warn("OnConfigChange is set but the config backend cannot watch for changes, config will not reload",
			"backend", fmt.Sprintf("%T", backend))
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	w := &configWatch{cancel: cancel, done: make(chan struct{})}
	key := a.config.ConfigKey()
	go func() {
		defer close(w.done)
		err := watcher.Watch(ctx, key, initial, func(doc []byte) {
			if err := a.reload(doc); err != nil {
				logger.Error("config reload rejected, keeping the previous config", "key", key, "error", err.Error())
			}
		})
		if err != nil {
			logger.Error("config watch stopped", "key", key, "error", err.Error())
		}
	}()
	logger.Info("watching config for changes", "key", key)
	return w
}

// checkReloadConfig reports a configuration that cannot work before anything
// is started.
func (a *App) checkReloadConfig() error {
	if a.config.OnConfigChange != nil && a.config.Config == nil {
		return errors.New("OnConfigChange needs Config to be set: it receives the freshly parsed Config")
	}
	if a.config.OnConfigChange != nil {
		if _, err := newConfigLike(a.config.Config); err != nil {
			return fmt.Errorf("OnConfigChange: %w", err)
		}
	}
	return nil
}
