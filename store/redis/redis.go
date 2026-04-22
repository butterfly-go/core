package redis

import (
	"butterfly.orx.me/core/internal/store"
	"github.com/redis/go-redis/v9"
)

type Client = redis.Client

// GetClient returns a Redis client by name.
func GetClient(k string) *redis.Client {
	return store.GetRedisClient(k)
}
