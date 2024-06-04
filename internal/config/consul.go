package config

import (
	"context"

	"butterfly.orx.me/core/internal/arg"
	"butterfly.orx.me/core/internal/log"
	capi "github.com/hashicorp/consul/api"
)

type ConsulConfig struct {
	client *capi.Client
	kv     *capi.KV
}

func NewConsulConfig() (*ConsulConfig, error) {
	// Get a new client
	logger := log.CoreLogger("config.consul")
	addr := arg.String("config.consul.address")
	logger.Info("create new consul config",
		"addr", addr)
	client, err := capi.NewClient(&capi.Config{
		Address: addr,
	})
	if err != nil {
		return nil, err
	}
	// Get a handle to the KV API
	kv := client.KV()
	return &ConsulConfig{
		kv:     kv,
		client: client,
	}, nil
}

func (c *ConsulConfig) Get(_ context.Context, key string) ([]byte, error) {
	logger := log.CoreLogger("config.consul")
	logger.Info("get config",
		"key", key)
	pair, _, err := c.kv.Get(key, nil)
	if err != nil {
		return nil, err
	}
	if pair == nil {
		return []byte(""), nil
	}
	return pair.Value, nil
}
