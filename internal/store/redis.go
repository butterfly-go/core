package store

import (
	"context"

	"butterfly.orx.me/core/internal/config"
	"github.com/redis/go-redis/v9"
)

var (
	redisClients = make(map[string]*redis.Client)
)

func InitRedis() error {
	config := config.CoreConfig().Store.Redis
	for k, v := range config {
		client := redis.NewClient(&redis.Options{
			Addr:     v.Addr,
			Password: v.Password,
			DB:       v.DB,
		})
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		err := client.Ping(ctx).Err()
		if err != nil {
			return err
		}
		redisClients[k] = client
	}
	return nil
}

func GetRedisClient(k string) *redis.Client {
	return redisClients[k]
}
