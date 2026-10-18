package store

import (
	"context"
	"fmt"

	"butterfly.orx.me/core/internal/config"
	"butterfly.orx.me/core/internal/log"
	"butterfly.orx.me/core/internal/redact"
	"butterfly.orx.me/core/internal/tlsutil"
	"butterfly.orx.me/core/mod"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

var (
	redisClients = make(map[string]redis.UniversalClient)
)

func InitRedis() error {
	cfg := config.CoreConfig().Store.Redis
	logger := log.CoreLogger("store.redis")
	for _, k := range sortedKeys(cfg) {
		v := cfg[k]
		secrets := []string{v.Password, v.SentinelPassword}
		mode := v.ResolvedMode()
		attrs := []any{
			"name", k,
			"mode", mode,
			"addrs", v.Nodes(),
			"db", v.DB,
			"tls", v.TLS.Enabled,
			"pool_size", v.PoolSize,
		}

		client, err := newRedisClient(v)
		if err != nil {
			err = redact.Error(err, secrets...)
			logger.Error("create redis client failed", append(attrs, "error", err.Error())...)
			return fmt.Errorf("create redis client %q: %w", k, err)
		}

		if err := redisotel.InstrumentTracing(client); err != nil {
			_ = client.Close()
			logger.Error("instrument redis tracing failed", append(attrs, "error", err.Error())...)
			return fmt.Errorf("instrument redis tracing %q: %w", k, err)
		}

		if err := redisotel.InstrumentMetrics(client); err != nil {
			_ = client.Close()
			logger.Error("instrument redis metrics failed", append(attrs, "error", err.Error())...)
			return fmt.Errorf("instrument redis metrics %q: %w", k, err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		err = client.Ping(ctx).Err()
		cancel()
		if err != nil {
			_ = client.Close()
			err = redact.Error(err, secrets...)
			logger.Error("ping redis failed", append(attrs, "error", err.Error())...)
			return fmt.Errorf("ping redis %q: %w", k, err)
		}
		logger.Info("initialize redis client", attrs...)
		redisClients[k] = client
	}
	return nil
}

// newRedisClient validates v and builds the client for its mode. Nothing
// connects until the first command.
func newRedisClient(v mod.RedisConfig) (redis.UniversalClient, error) {
	opts, err := redisOptions(v)
	if err != nil {
		return nil, err
	}
	switch v.ResolvedMode() {
	case mod.RedisModeSentinel:
		return redis.NewFailoverClient(opts.Failover()), nil
	case mod.RedisModeCluster:
		return redis.NewClusterClient(opts.Cluster()), nil
	default:
		return redis.NewClient(opts.Simple()), nil
	}
}

// redisOptions translates the config into go-redis options. Zero values stay
// zero, which makes go-redis apply its own defaults.
func redisOptions(v mod.RedisConfig) (*redis.UniversalOptions, error) {
	if err := v.Validate(); err != nil {
		return nil, err
	}
	tlsConfig, err := tlsutil.Build(v.TLS)
	if err != nil {
		return nil, err
	}
	opts := &redis.UniversalOptions{
		Addrs:            v.Nodes(),
		Username:         v.Username,
		Password:         v.Password,
		DB:               v.DB,
		SentinelUsername: v.SentinelUsername,
		SentinelPassword: v.SentinelPassword,
		TLSConfig:        tlsConfig,

		PoolSize:        v.PoolSize,
		MinIdleConns:    v.MinIdleConns,
		PoolTimeout:     v.PoolTimeout,
		ConnMaxIdleTime: v.ConnMaxIdleTime,
		ConnMaxLifetime: v.ConnMaxLifetime,
		DialTimeout:     v.DialTimeout,
		ReadTimeout:     v.ReadTimeout,
		WriteTimeout:    v.WriteTimeout,
	}
	if v.ResolvedMode() == mod.RedisModeSentinel {
		opts.MasterName = v.MasterName
	}
	return opts, nil
}

// GetRedisClient returns the standalone or sentinel client registered under k.
// It returns nil for an unknown name and for a cluster entry, which has no
// *redis.Client: use GetRedisUniversalClient or GetRedisClusterClient.
func GetRedisClient(k string) *redis.Client {
	c, _ := redisClients[k].(*redis.Client)
	return c
}

// GetRedisClusterClient returns the cluster client registered under k, or nil
// when k is unknown or not a cluster entry.
func GetRedisClusterClient(k string) *redis.ClusterClient {
	c, _ := redisClients[k].(*redis.ClusterClient)
	return c
}

// GetRedisUniversalClient returns the client registered under k whatever its
// mode, or nil when k is unknown.
func GetRedisUniversalClient(k string) redis.UniversalClient {
	return redisClients[k]
}
