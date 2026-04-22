package config

import (
	"context"

	"butterfly.orx.me/core/internal/config"
)

// Get retrieves configuration data by key from the config backend.
func Get(ctx context.Context, key string) ([]byte, error) {
	return config.GetConfig().Get(ctx, key)
}
