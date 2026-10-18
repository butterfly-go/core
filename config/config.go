package config

import (
	"context"
	"errors"

	"butterfly.orx.me/core/internal/config"
)

// ErrWatchUnsupported is returned by Watch when the configuration backend
// cannot report changes. Only Consul can; the file backend cannot.
var ErrWatchUnsupported = config.ErrWatchUnsupported

// Get
func Get(ctx context.Context, key string) ([]byte, error) {
	return config.GetConfig().Get(ctx, key)
}

// Watch reads key once as the baseline and then calls onChange with the new
// value each time it changes, until ctx is cancelled. It blocks, so run it in
// a goroutine.
//
// onChange is not called for the baseline, for an unchanged value, or when the
// key is deleted. It runs on the watching goroutine, one call at a time, and
// owns the slice it receives. Query errors are retried with backoff.
//
// It returns ErrWatchUnsupported with the file backend.
func Watch(ctx context.Context, key string, onChange func(value []byte)) error {
	if onChange == nil {
		return errors.New("config: Watch needs an onChange callback")
	}
	return config.Watch(ctx, key, onChange)
}
