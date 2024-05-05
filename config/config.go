package config

import (
	"context"

	"butterfly.orx.me/core/internal/config"
)

// Get
func Get(ctx context.Context, key string) ([]byte, error) {
	return config.GetConfig().Get(ctx, key)
}
