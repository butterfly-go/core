package store

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"butterfly.orx.me/core/internal/config"
	"butterfly.orx.me/core/internal/tlstest"
	"butterfly.orx.me/core/mod"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

func TestNewRedisClientModes(t *testing.T) {
	t.Run("standalone", func(t *testing.T) {
		c, err := newRedisClient(mod.RedisConfig{Addr: "127.0.0.1:6379", Username: "app", Password: "pw", DB: 2})
		if err != nil {
			t.Fatal(err)
		}
		defer c.Close()
		rc, ok := c.(*redis.Client)
		if !ok {
			t.Fatalf("got %T, want *redis.Client", c)
		}
		o := rc.Options()
		if o.Addr != "127.0.0.1:6379" || o.Username != "app" || o.Password != "pw" || o.DB != 2 || o.TLSConfig != nil {
			t.Fatalf("options = %+v", o)
		}
	})

	t.Run("sentinel", func(t *testing.T) {
		c, err := newRedisClient(mod.RedisConfig{
			MasterName: "mymaster", Addrs: []string{"s1:26379", "s2:26379"},
			Password: "pw", SentinelPassword: "spw", DB: 1,
		})
		if err != nil {
			t.Fatal(err)
		}
		defer c.Close()
		// A sentinel-backed client is still a *redis.Client.
		rc, ok := c.(*redis.Client)
		if !ok {
			t.Fatalf("got %T, want *redis.Client", c)
		}
		if rc.Options().DB != 1 || rc.Options().Password != "pw" {
			t.Fatalf("options = %+v", rc.Options())
		}
	})

	t.Run("cluster", func(t *testing.T) {
		c, err := newRedisClient(mod.RedisConfig{Addrs: []string{"a:7000", "b:7001"}, Password: "pw"})
		if err != nil {
			t.Fatal(err)
		}
		defer c.Close()
		cc, ok := c.(*redis.ClusterClient)
		if !ok {
			t.Fatalf("got %T, want *redis.ClusterClient", c)
		}
		if got := cc.Options().Addrs; len(got) != 2 || got[0] != "a:7000" {
			t.Fatalf("addrs = %v", got)
		}
	})
}

func TestRedisOptionsMapping(t *testing.T) {
	b := tlstest.New(t)
	opts, err := redisOptions(mod.RedisConfig{
		Addr:             "h:6380",
		Username:         "u",
		Password:         "p",
		TLS:              mod.TLSConfig{Enabled: true, CAFile: b.CAFile, ServerName: "redis.internal"},
		PoolSize:         25,
		MinIdleConns:     5,
		PoolTimeout:      3 * time.Second,
		ConnMaxIdleTime:  4 * time.Minute,
		ConnMaxLifetime:  time.Hour,
		DialTimeout:      2 * time.Second,
		ReadTimeout:      time.Second,
		WriteTimeout:     time.Second + 1,
		SentinelUsername: "",
	})
	if err != nil {
		t.Fatal(err)
	}
	simple := opts.Simple()
	if simple.PoolSize != 25 || simple.MinIdleConns != 5 || simple.PoolTimeout != 3*time.Second ||
		simple.ConnMaxIdleTime != 4*time.Minute || simple.ConnMaxLifetime != time.Hour ||
		simple.DialTimeout != 2*time.Second || simple.ReadTimeout != time.Second || simple.WriteTimeout != time.Second+1 {
		t.Fatalf("pool/timeouts not mapped: %+v", simple)
	}
	if simple.TLSConfig == nil || simple.TLSConfig.ServerName != "redis.internal" || simple.TLSConfig.RootCAs == nil {
		t.Fatalf("TLS not mapped: %+v", simple.TLSConfig)
	}

	// Sentinel and cluster options carry the same settings.
	if f := opts.Failover(); f.PoolSize != 25 || f.TLSConfig == nil {
		t.Fatalf("failover options: %+v", f)
	}
	if c := opts.Cluster(); c.PoolSize != 25 || c.TLSConfig == nil {
		t.Fatalf("cluster options: %+v", c)
	}
}

func TestRedisOptionsSentinelFields(t *testing.T) {
	opts, err := redisOptions(mod.RedisConfig{
		MasterName: "mymaster", Addrs: []string{"s1:26379"},
		SentinelUsername: "su", SentinelPassword: "sp", Password: "p",
	})
	if err != nil {
		t.Fatal(err)
	}
	f := opts.Failover()
	if f.MasterName != "mymaster" || len(f.SentinelAddrs) != 1 || f.SentinelAddrs[0] != "s1:26379" ||
		f.SentinelUsername != "su" || f.SentinelPassword != "sp" || f.Password != "p" {
		t.Fatalf("failover options: %+v", f)
	}
}

func TestRedisOptionsRejectsInvalidConfig(t *testing.T) {
	tests := map[string]mod.RedisConfig{
		"no address":      {},
		"cluster with db": {Addrs: []string{"a:1", "b:2"}, DB: 1},
		"missing ca":      {Addr: "h:1", TLS: mod.TLSConfig{Enabled: true, CAFile: "/no/such/ca.pem"}},
	}
	for name, cfg := range tests {
		if _, err := newRedisClient(cfg); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestGetRedisClientByMode(t *testing.T) {
	t.Cleanup(func() { _ = Close(context.Background()) })
	standalone, err := newRedisClient(mod.RedisConfig{Addr: "127.0.0.1:1"})
	if err != nil {
		t.Fatal(err)
	}
	cluster, err := newRedisClient(mod.RedisConfig{Addrs: []string{"127.0.0.1:1", "127.0.0.1:2"}})
	if err != nil {
		t.Fatal(err)
	}
	redisClients["single"], redisClients["cluster"] = standalone, cluster

	if GetRedisClient("single") == nil || GetRedisClusterClient("single") != nil {
		t.Fatal("standalone entry should only be a *redis.Client")
	}
	if GetRedisClient("cluster") != nil || GetRedisClusterClient("cluster") == nil {
		t.Fatal("cluster entry should only be a *redis.ClusterClient")
	}
	if GetRedisUniversalClient("single") == nil || GetRedisUniversalClient("cluster") == nil {
		t.Fatal("universal getter should return every mode")
	}
	if GetRedisClient("nope") != nil || GetRedisClusterClient("nope") != nil || GetRedisUniversalClient("nope") != nil {
		t.Fatal("unknown names return nil")
	}
}

func TestMongoOptionsPoolAndTimeouts(t *testing.T) {
	opts := mongoOptions(mod.MongoConfig{
		URI:                    "mongodb://127.0.0.1:27017",
		MaxPoolSize:            50,
		MinPoolSize:            5,
		MaxConnecting:          3,
		MaxConnIdleTime:        time.Minute,
		ConnectTimeout:         2 * time.Second,
		ServerSelectionTimeout: 3 * time.Second,
		Timeout:                10 * time.Second,
	})
	if err := opts.Validate(); err != nil {
		t.Fatal(err)
	}
	if *opts.MaxPoolSize != 50 || *opts.MinPoolSize != 5 || *opts.MaxConnecting != 3 ||
		*opts.MaxConnIdleTime != time.Minute || *opts.ConnectTimeout != 2*time.Second ||
		*opts.ServerSelectionTimeout != 3*time.Second || *opts.Timeout != 10*time.Second {
		t.Fatalf("options not applied: %+v", opts)
	}
}

func TestMongoOptionsConfigOverridesURI(t *testing.T) {
	opts := mongoOptions(mod.MongoConfig{
		URI:         "mongodb://127.0.0.1/?maxPoolSize=7&minPoolSize=2&serverSelectionTimeoutMS=1500",
		MaxPoolSize: 20, // set: wins over the URI
		// MinPoolSize and the timeouts are unset: the URI stays in effect.
	})
	if *opts.MaxPoolSize != 20 {
		t.Fatalf("MaxPoolSize = %d, want the config value 20", *opts.MaxPoolSize)
	}
	if *opts.MinPoolSize != 2 {
		t.Fatalf("MinPoolSize = %d, want the URI value 2", *opts.MinPoolSize)
	}
	if *opts.ServerSelectionTimeout != 1500*time.Millisecond {
		t.Fatalf("ServerSelectionTimeout = %v, want the URI value", *opts.ServerSelectionTimeout)
	}
}

func TestMongoOptionsZeroValuesLeaveDefaults(t *testing.T) {
	opts := mongoOptions(mod.MongoConfig{URI: "mongodb://127.0.0.1"})
	if opts.MaxPoolSize != nil || opts.MinPoolSize != nil || opts.Timeout != nil || opts.ConnectTimeout != nil {
		t.Fatalf("zero config should not set options: %+v", opts)
	}
}

func TestPostgresDSNEscapesCredentials(t *testing.T) {
	cfg := mod.DBConfig{
		Driver: "postgres", Host: "db.internal", Port: 5433,
		User: "app user", Password: "p@ss/w:rd#1?&", DBName: "my db", SSLMode: "require",
	}
	dsn := pgConfigToDSN(cfg)
	parsed, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("DSN does not parse: %v", err)
	}
	if parsed.User != cfg.User || parsed.Password != cfg.Password || parsed.Host != cfg.Host ||
		parsed.Port != 5433 || parsed.Database != cfg.DBName {
		t.Fatalf("round trip failed: %+v", parsed.Config)
	}
	if parsed.TLSConfig == nil {
		t.Fatal("sslmode=require should enable TLS")
	}
}

func TestPostgresDSNDefaultsSSLModeToDisable(t *testing.T) {
	dsn := pgConfigToDSN(mod.DBConfig{Host: "h", Port: 5432, User: "u", Password: "p", DBName: "d"})
	if dsn != "postgres://u:p@h:5432/d?sslmode=disable" {
		t.Fatalf("dsn = %q", dsn)
	}
}

// captureLogs routes the default slog logger into a buffer for the test.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(old) })
	return &buf
}

func withCoreStore(t *testing.T, s mod.StoreConfig) {
	t.Helper()
	old := config.CoreConfig().Store
	config.CoreConfig().Store = s
	t.Cleanup(func() {
		config.CoreConfig().Store = old
		// Bounded: disconnecting Mongo from a server that is gone can
		// otherwise wait for its default 30s server selection timeout.
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = Close(ctx)
	})
}

func TestInitDoesNotLeakCredentials(t *testing.T) {
	const secret = "hunter2-never-log-me"
	logs := captureLogs(t)
	withCoreStore(t, mod.StoreConfig{
		DB: map[string]mod.DBConfig{
			"pg":    {Driver: "postgres", Host: "127.0.0.1", Port: 1, User: "app", Password: secret, DBName: "d"},
			"mysql": {Driver: "mysql", Host: "127.0.0.1", Port: 1, User: "app", Password: secret, DBName: "d"},
		},
		Mongo: map[string]mod.MongoConfig{
			"m": {URI: "mongodb://app:" + secret + "@127.0.0.1:1/?directConnection=true", ServerSelectionTimeout: 100 * time.Millisecond},
		},
	})
	// SQL and Mongo pings are best-effort: startup succeeds, and each failure
	// is logged. That is the path that used to print connection details.
	if err := InitSQLDB(); err != nil {
		t.Fatal(err)
	}
	if err := InitMongo(); err != nil {
		t.Fatal(err)
	}
	if out := logs.String(); strings.Contains(out, secret) {
		t.Fatalf("password leaked into the logs:\n%s", out)
	} else if !strings.Contains(out, "ping sql db failed") || !strings.Contains(out, "ping mongo failed") {
		t.Fatalf("expected the failed pings to be logged:\n%s", out)
	}
}

func TestInitRedisFailureDoesNotLeakPassword(t *testing.T) {
	const secret = "hunter2-never-log-me"
	logs := captureLogs(t)
	withCoreStore(t, mod.StoreConfig{
		Redis: map[string]mod.RedisConfig{"cache": {Addr: "127.0.0.1:1", Password: secret, DialTimeout: 200 * time.Millisecond}},
	})
	err := InitRedis()
	if err == nil {
		t.Fatal("expected a connection error")
	}
	if strings.Contains(err.Error(), secret) || strings.Contains(logs.String(), secret) {
		t.Fatalf("password leaked:\nerror: %v\nlogs: %s", err, logs.String())
	}
}

func TestInitRedisRejectsInvalidConfig(t *testing.T) {
	withCoreStore(t, mod.StoreConfig{
		Redis: map[string]mod.RedisConfig{"bad": {Mode: "ring", Addr: "h:1"}},
	})
	err := InitRedis()
	if err == nil || !strings.Contains(err.Error(), `"bad"`) {
		t.Fatalf("error = %v, want one naming the entry", err)
	}
}
