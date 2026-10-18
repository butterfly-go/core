package mod

import (
	"errors"
	"fmt"
	"math"
	"net"
	"sort"
	"strconv"
	"strings"
)

// Validate checks the core configuration and reports every problem it finds,
// each prefixed with the key it belongs to, for example
// `store.redis.cache: mode "foo" is not one of ...`.
//
// It does not check the server section, which is validated where the
// addresses are resolved, and it does not touch the network or the file
// system.
func (c *CoreConfig) Validate() error {
	var errs []error
	errs = append(errs, prefixed("log", c.Log.Validate())...)
	errs = append(errs, prefixed("otel", c.Otel.Validate())...)
	errs = append(errs, c.Store.validate()...)
	return errors.Join(errs...)
}

func (c StoreConfig) validate() []error {
	var errs []error
	for _, name := range sortedNames(c.Redis) {
		errs = append(errs, prefixed("store.redis."+name, c.Redis[name].Validate())...)
	}
	for _, name := range sortedNames(c.Mongo) {
		errs = append(errs, prefixed("store.mongo."+name, c.Mongo[name].Validate())...)
	}
	for _, name := range sortedNames(c.DB) {
		errs = append(errs, prefixed("store.db."+name, c.DB[name].Validate())...)
	}
	return errs
}

// Validate checks the log section.
func (c LogConfig) Validate() error {
	var errs []error
	switch strings.ToLower(c.Level) {
	case "", "debug", "info", "warn", "warning", "error":
	default:
		errs = append(errs, fmt.Errorf("level %q is not one of debug, info, warn, error", c.Level))
	}
	switch strings.ToLower(c.Format) {
	case "", "json", "text":
	default:
		errs = append(errs, fmt.Errorf("format %q is not one of json, text", c.Format))
	}
	return errors.Join(errs...)
}

// Validate checks the otel section.
func (c OtelConfig) Validate() error {
	if c.TraceSampleRatio != nil && math.IsNaN(*c.TraceSampleRatio) {
		return errors.New("trace_sample_ratio must be a number")
	}
	return nil
}

// Validate checks that TLS settings are consistent. It does not read files.
func (c TLSConfig) Validate() error {
	var errs []error
	if !c.Enabled {
		for name, set := range map[string]bool{
			"ca_file":              c.CAFile != "",
			"cert_file":            c.CertFile != "",
			"key_file":             c.KeyFile != "",
			"server_name":          c.ServerName != "",
			"insecure_skip_verify": c.InsecureSkipVerify,
		} {
			if set {
				errs = append(errs, fmt.Errorf("tls.%s is set but tls.enabled is false", name))
			}
		}
		sort.Slice(errs, func(i, j int) bool { return errs[i].Error() < errs[j].Error() })
		return errors.Join(errs...)
	}
	if (c.CertFile == "") != (c.KeyFile == "") {
		errs = append(errs, errors.New("tls.cert_file and tls.key_file must be set together"))
	}
	return errors.Join(errs...)
}

// ResolvedMode returns Mode, or the inferred mode when Mode is empty.
func (c RedisConfig) ResolvedMode() string {
	if m := strings.ToLower(strings.TrimSpace(c.Mode)); m != "" {
		return m
	}
	switch {
	case c.MasterName != "":
		return RedisModeSentinel
	case len(c.nodes()) > 1:
		return RedisModeCluster
	default:
		return RedisModeStandalone
	}
}

// Nodes returns the configured addresses: Addrs when set, else Addr.
func (c RedisConfig) Nodes() []string { return c.nodes() }

func (c RedisConfig) nodes() []string {
	var out []string
	for _, a := range c.Addrs {
		if a = strings.TrimSpace(a); a != "" {
			out = append(out, a)
		}
	}
	if len(out) == 0 {
		if a := strings.TrimSpace(c.Addr); a != "" {
			out = append(out, a)
		}
	}
	return out
}

// Validate checks a Redis entry.
func (c RedisConfig) Validate() error {
	var errs []error
	mode := c.ResolvedMode()
	nodes := c.nodes()

	switch mode {
	case RedisModeStandalone:
		switch {
		case len(nodes) == 0:
			errs = append(errs, errors.New("addr is required"))
		case len(nodes) > 1:
			errs = append(errs, errors.New("standalone mode takes one address, got several; use mode: cluster or sentinel"))
		}
		if c.MasterName != "" {
			errs = append(errs, errors.New("master_name only applies to mode: sentinel"))
		}
	case RedisModeSentinel:
		if c.MasterName == "" {
			errs = append(errs, errors.New("master_name is required in sentinel mode"))
		}
		if len(nodes) == 0 {
			errs = append(errs, errors.New("addrs must list at least one sentinel"))
		}
	case RedisModeCluster:
		if len(nodes) == 0 {
			errs = append(errs, errors.New("addrs must list at least one cluster node"))
		}
		if c.DB != 0 {
			errs = append(errs, errors.New("db is not supported in cluster mode"))
		}
		if c.MasterName != "" {
			errs = append(errs, errors.New("master_name only applies to mode: sentinel"))
		}
	default:
		errs = append(errs, fmt.Errorf("mode %q is not one of standalone, sentinel, cluster", c.Mode))
	}

	for _, n := range nodes {
		if err := validateHostPort(n); err != nil {
			errs = append(errs, fmt.Errorf("address %q: %w", n, err))
		}
	}
	if mode != RedisModeSentinel && (c.SentinelUsername != "" || c.SentinelPassword != "") {
		errs = append(errs, errors.New("sentinel_username and sentinel_password only apply to mode: sentinel"))
	}
	if c.DB < 0 {
		errs = append(errs, errors.New("db must not be negative"))
	}
	errs = append(errs, nonNegative(map[string]int64{
		"pool_size":          int64(c.PoolSize),
		"min_idle_conns":     int64(c.MinIdleConns),
		"pool_timeout":       int64(c.PoolTimeout),
		"conn_max_idle_time": int64(c.ConnMaxIdleTime),
		"conn_max_lifetime":  int64(c.ConnMaxLifetime),
		"dial_timeout":       int64(c.DialTimeout),
		"read_timeout":       int64(c.ReadTimeout),
		"write_timeout":      int64(c.WriteTimeout),
	})...)
	if c.PoolSize > 0 && c.MinIdleConns > c.PoolSize {
		errs = append(errs, fmt.Errorf("min_idle_conns (%d) must not exceed pool_size (%d)", c.MinIdleConns, c.PoolSize))
	}
	if err := c.TLS.Validate(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// Validate checks a Mongo entry.
func (c MongoConfig) Validate() error {
	var errs []error
	switch uri := strings.TrimSpace(c.URI); {
	case uri == "":
		errs = append(errs, errors.New("uri is required"))
	case !strings.HasPrefix(uri, "mongodb://") && !strings.HasPrefix(uri, "mongodb+srv://"):
		errs = append(errs, errors.New("uri must start with mongodb:// or mongodb+srv://"))
	}
	if c.MaxPoolSize > 0 && c.MinPoolSize > c.MaxPoolSize {
		errs = append(errs, fmt.Errorf("min_pool_size (%d) must not exceed max_pool_size (%d)", c.MinPoolSize, c.MaxPoolSize))
	}
	errs = append(errs, nonNegative(map[string]int64{
		"max_conn_idle_time":       int64(c.MaxConnIdleTime),
		"connect_timeout":          int64(c.ConnectTimeout),
		"server_selection_timeout": int64(c.ServerSelectionTimeout),
		"timeout":                  int64(c.Timeout),
	})...)
	return errors.Join(errs...)
}

// Validate checks a SQL entry.
func (c DBConfig) Validate() error {
	var errs []error
	switch strings.ToLower(strings.TrimSpace(c.Driver)) {
	case "", "mysql", "postgres", "postgresql":
	default:
		errs = append(errs, fmt.Errorf("driver %q is not one of mysql, postgres, postgresql", c.Driver))
	}
	if strings.TrimSpace(c.Host) == "" {
		errs = append(errs, errors.New("host is required"))
	}
	if c.Port < 0 || c.Port > 65535 {
		errs = append(errs, fmt.Errorf("port %d is out of range", c.Port))
	}
	return errors.Join(errs...)
}

func validateHostPort(addr string) error {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return errors.New("must be host:port")
	}
	if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		return fmt.Errorf("invalid port %q", port)
	}
	return nil
}

func nonNegative(m map[string]int64) []error {
	var errs []error
	for _, name := range sortedNames(m) {
		if m[name] < 0 {
			errs = append(errs, fmt.Errorf("%s must not be negative", name))
		}
	}
	return errs
}

func prefixed(prefix string, err error) []error {
	var out []error
	for _, e := range flatten(err) {
		out = append(out, fmt.Errorf("%s: %w", prefix, e))
	}
	return out
}

// flatten expands joined errors into their leaves so each gets its own prefix.
func flatten(err error) []error {
	if err == nil {
		return nil
	}
	if multi, ok := err.(interface{ Unwrap() []error }); ok {
		var out []error
		for _, e := range multi.Unwrap() {
			out = append(out, flatten(e)...)
		}
		return out
	}
	return []error{err}
}

func sortedNames[V any](m map[string]V) []string {
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
