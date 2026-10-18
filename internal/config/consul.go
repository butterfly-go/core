package config

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"butterfly.orx.me/core/internal/arg"
	"butterfly.orx.me/core/internal/log"
	"butterfly.orx.me/core/internal/redact"
	capi "github.com/hashicorp/consul/api"
)

// Timing of the watch loop. They are variables of the ConsulConfig so tests
// can shorten them.
const (
	// defaultWatchWait is how long one blocking query waits for a change.
	defaultWatchWait = 5 * time.Minute
	// defaultWatchGap pauses between two queries that saw no change, so a
	// server that answers blocking queries immediately cannot be hammered.
	defaultWatchGap = 250 * time.Millisecond
	// Failed queries are retried with exponential backoff between these.
	watchMinBackoff = time.Second
	watchMaxBackoff = 30 * time.Second
)

type ConsulConfig struct {
	client *capi.Client
	kv     *capi.KV

	watchWait time.Duration
	watchGap  time.Duration
	// backoff bounds, zero means the package defaults.
	minBackoff time.Duration
	maxBackoff time.Duration
}

// NewConsulConfig connects to the Consul agent named by the environment:
//
//	BUTTERFLY_CONFIG_CONSUL_ADDRESS    host:port, or http(s)://host:port
//	BUTTERFLY_CONFIG_CONSUL_NAMESPACE  Enterprise namespace
//	BUTTERFLY_CONFIG_CONSUL_TOKEN      ACL token
//	BUTTERFLY_CONFIG_CONSUL_TOKEN_FILE file holding the ACL token, read once
//	BUTTERFLY_CONFIG_CONSUL_TLS_*      see arg.TLS; TLS_ENABLED selects https
func NewConsulConfig() (*ConsulConfig, error) {
	// Get a new client
	logger := log.CoreLogger("config.consul")
	addr := arg.String("config.consul.address")
	namespace := arg.String("config.consul.namespace")
	token := arg.String("config.consul.token")
	tokenFile := arg.String("config.consul.token_file")
	tlsCfg := arg.TLS("config.consul.tls")

	if err := tlsCfg.Validate(); err != nil {
		return nil, fmt.Errorf("consul config: %w", err)
	}

	cfg := &capi.Config{
		Address:   addr,
		Namespace: namespace,
		Token:     token,
		TokenFile: tokenFile,
	}
	if tlsCfg.Enabled {
		if strings.HasPrefix(addr, "http://") {
			return nil, errors.New("consul config: TLS is enabled but the address uses http://")
		}
		if !strings.HasPrefix(addr, "https://") {
			cfg.Scheme = "https"
		}
		cfg.TLSConfig = capi.TLSConfig{
			Address:            tlsCfg.ServerName,
			CAFile:             tlsCfg.CAFile,
			CertFile:           tlsCfg.CertFile,
			KeyFile:            tlsCfg.KeyFile,
			InsecureSkipVerify: tlsCfg.InsecureSkipVerify,
		}
	}

	logger.Debug("create new consul config",
		"addr", redact.URI(addr),
		"namespace", namespace,
		"tls", tlsCfg.Enabled || strings.HasPrefix(addr, "https://"),
		"token_set", token != "" || tokenFile != "")
	client, err := capi.NewClient(cfg)
	if err != nil {
		return nil, redact.Error(err, token)
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
	logger.Debug("get config",
		"key", key)
	pair, _, err := c.kv.Get(key, nil)
	if err != nil {
		return nil, err
	}
	if pair == nil {
		return []byte(""), nil
	}
	logger.Debug("get key response", "key", key, "len", len(pair.Value))
	return pair.Value, nil
}

// Watch blocks, calling onChange with the new value each time the key's
// value changes, until ctx is cancelled (it then returns nil).
//
// initial is the value the caller already loaded: it is the baseline, so a
// change that landed between that read and the first query is still reported.
// onChange is not called for an unchanged value, for a Consul index bump that
// leaves the value alone, or when the key is deleted (the last good value stays
// in effect and a warning is logged). It runs on the watch goroutine, one call
// at a time, and a panic in it is recovered and logged. The slice passed to
// onChange is owned by the callee.
//
// Query failures are retried with exponential backoff and never end the watch.
func (c *ConsulConfig) Watch(ctx context.Context, key string, initial []byte, onChange func(value []byte)) error {
	logger := log.CoreLogger("config.consul.watch")
	wait, gap := c.watchWait, c.watchGap
	if wait <= 0 {
		wait = defaultWatchWait
	}
	if gap <= 0 {
		gap = defaultWatchGap
	}
	minBackoff, maxBackoff := c.minBackoff, c.maxBackoff
	if minBackoff <= 0 {
		minBackoff = watchMinBackoff
	}
	if maxBackoff <= 0 {
		maxBackoff = watchMaxBackoff
	}

	last := bytes.Clone(initial)
	var index uint64
	backoff := minBackoff

	for ctx.Err() == nil {
		opts := (&capi.QueryOptions{WaitIndex: index, WaitTime: wait}).WithContext(ctx)
		pair, meta, err := c.kv.Get(key, opts)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			logger.Warn("watch consul key failed, retrying", "key", key,
				"retry_in", backoff.String(), "error", err.Error())
			if !sleep(ctx, backoff) {
				return nil
			}
			backoff = min(backoff*2, maxBackoff)
			continue
		}
		backoff = minBackoff

		// Consul's rule for blocking queries: an index that goes backwards
		// means the data was reset, so start over from 0.
		indexChanged := meta.LastIndex != index
		if meta.LastIndex < index {
			index = 0
		} else {
			index = meta.LastIndex
		}
		if !indexChanged {
			if !sleep(ctx, gap) {
				return nil
			}
			continue
		}

		if pair == nil {
			logger.Warn("watched consul key does not exist, keeping the current config", "key", key)
			continue
		}
		if bytes.Equal(pair.Value, last) {
			continue
		}
		last = bytes.Clone(pair.Value)
		logger.Info("consul key changed", "key", key, "len", len(pair.Value), "index", meta.LastIndex)
		deliver(logger, key, onChange, bytes.Clone(pair.Value))
	}
	return nil
}

// deliver calls fn and keeps a panic from taking down the watch goroutine.
func deliver(logger interface{ Error(string, ...any) }, key string, fn func([]byte), value []byte) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("config change callback panicked", "key", key, "panic", fmt.Sprint(r))
		}
	}()
	fn(value)
}

// sleep waits for d or until ctx is done, and reports whether the full wait
// elapsed.
func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
