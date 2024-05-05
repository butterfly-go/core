package config

import "context"

type Config interface {
	Get(ctx context.Context, key string) ([]byte, error)
}
