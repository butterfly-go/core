package config

import (
	"context"

	"butterfly.orx.me/core/internal/config"
)

var cfg config.Config

// Set sets the config backend. Called by the app during initialization.
func Set(c config.Config) {
	cfg = c
}

// Get retrieves configuration data by key from the config backend.
func Get(ctx context.Context, key string) ([]byte, error) {
	return cfg.Get(ctx, key)
}
