package store

import (
	"context"

	"butterfly.orx.me/core/mod"
	"github.com/redis/go-redis/v9"
)

// Legacy global for backward compatibility.
var redisClients = make(map[string]*redis.Client)

// ProvideRedisClients creates Redis clients from config.
func ProvideRedisClients(cc *mod.CoreConfig) (RedisClients, func(), error) {
	clients := make(RedisClients)
	for k, v := range cc.Store.Redis {
		client := redis.NewClient(&redis.Options{
			Addr:     v.Addr,
			Password: v.Password,
			DB:       v.DB,
		})
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		if err := client.Ping(ctx).Err(); err != nil {
			for _, c := range clients {
				_ = c.Close()
			}
			return nil, nil, err
		}
		clients[k] = client
	}
	cleanup := func() {
		for _, c := range clients {
			_ = c.Close()
		}
	}
	return clients, cleanup, nil
}

// SetLegacyRedisClients populates the legacy global.
func SetLegacyRedisClients(clients RedisClients) {
	redisClients = map[string]*redis.Client(clients)
}

// GetRedisClient returns a Redis client by name from the legacy global.
func GetRedisClient(k string) *redis.Client {
	return redisClients[k]
}
