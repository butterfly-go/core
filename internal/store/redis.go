package store

import (
	"context"
	"fmt"

	"butterfly.orx.me/core/internal/config"
	"butterfly.orx.me/core/internal/log"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

var (
	redisClients = make(map[string]*redis.Client)
)

func InitRedis() error {
	cfg := config.CoreConfig().Store.Redis
	logger := log.CoreLogger("store.redis")
	for k, v := range cfg {
		client := redis.NewClient(&redis.Options{
			Addr:     v.Addr,
			Password: v.Password,
			DB:       v.DB,
		})

		if err := redisotel.InstrumentTracing(client); err != nil {
			logger.Error("instrument redis tracing failed", "name", k, "addr", v.Addr, "db", v.DB, "error", err.Error())
			return fmt.Errorf("instrument redis tracing %q: %w", k, err)
		}

		if err := redisotel.InstrumentMetrics(client); err != nil {
			logger.Error("instrument redis metrics failed", "name", k, "addr", v.Addr, "db", v.DB, "error", err.Error())
			return fmt.Errorf("instrument redis metrics %q: %w", k, err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		err := client.Ping(ctx).Err()
		cancel()
		if err != nil {
			logger.Error("ping redis failed", "name", k, "addr", v.Addr, "db", v.DB, "error", err.Error())
			return fmt.Errorf("ping redis %q: %w", k, err)
		}
		logger.Info("initialize redis client", "name", k, "addr", v.Addr, "db", v.DB)
		redisClients[k] = client
	}
	return nil
}

func GetRedisClient(k string) *redis.Client {
	return redisClients[k]
}
