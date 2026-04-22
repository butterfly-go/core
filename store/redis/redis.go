package redis

import (
	"github.com/redis/go-redis/v9"
)

type Client = redis.Client

var clients map[string]*redis.Client

// Set sets the Redis clients map. Called by the app during initialization.
func Set(c map[string]*redis.Client) {
	clients = c
}

// GetClient returns a Redis client by name.
func GetClient(k string) *redis.Client {
	return clients[k]
}
