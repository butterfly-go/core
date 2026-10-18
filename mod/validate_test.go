package mod

import (
	"math"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestRedisResolvedMode(t *testing.T) {
	tests := []struct {
		name string
		cfg  RedisConfig
		want string
	}{
		{"single addr", RedisConfig{Addr: "h:6379"}, RedisModeStandalone},
		{"single entry in addrs", RedisConfig{Addrs: []string{"h:6379"}}, RedisModeStandalone},
		{"master name means sentinel", RedisConfig{Addrs: []string{"s1:26379"}, MasterName: "m"}, RedisModeSentinel},
		{"several addrs mean cluster", RedisConfig{Addrs: []string{"a:1", "b:2"}}, RedisModeCluster},
		{"explicit wins", RedisConfig{Mode: "Cluster", Addrs: []string{"a:1"}}, RedisModeCluster},
		{"blank addrs ignored", RedisConfig{Addrs: []string{" ", "a:1"}}, RedisModeStandalone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.ResolvedMode(); got != tt.want {
				t.Fatalf("ResolvedMode = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRedisValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     RedisConfig
		wantErr string // substring, empty means valid
	}{
		{"standalone", RedisConfig{Addr: "localhost:6379"}, ""},
		{"standalone tuned", RedisConfig{Addr: "h:6379", PoolSize: 20, MinIdleConns: 5, PoolTimeout: time.Second}, ""},
		{"sentinel", RedisConfig{MasterName: "m", Addrs: []string{"s1:26379", "s2:26379"}, SentinelPassword: "x"}, ""},
		{"cluster", RedisConfig{Addrs: []string{"a:7000", "b:7001"}}, ""},
		{"tls", RedisConfig{Addr: "h:6380", TLS: TLSConfig{Enabled: true, CAFile: "ca.pem"}}, ""},

		{"no addr", RedisConfig{}, "addr is required"},
		{"bad mode", RedisConfig{Mode: "ring", Addr: "h:1"}, `mode "ring"`},
		{"standalone with several", RedisConfig{Mode: "standalone", Addrs: []string{"a:1", "b:2"}}, "one address"},
		{"missing port", RedisConfig{Addr: "localhost"}, "must be host:port"},
		{"bad port", RedisConfig{Addr: "h:99999"}, "invalid port"},
		{"sentinel without master", RedisConfig{Mode: "sentinel", Addrs: []string{"s:1"}}, "master_name is required"},
		{"sentinel without addrs", RedisConfig{MasterName: "m"}, "at least one sentinel"},
		{"cluster with db", RedisConfig{Addrs: []string{"a:1", "b:2"}, DB: 3}, "db is not supported in cluster"},
		{"master name on standalone", RedisConfig{Mode: "standalone", Addr: "h:1", MasterName: "m"}, "only applies to mode: sentinel"},
		{"sentinel auth off sentinel", RedisConfig{Addr: "h:1", SentinelPassword: "x"}, "only apply to mode: sentinel"},
		{"negative db", RedisConfig{Addr: "h:1", DB: -1}, "db must not be negative"},
		{"negative pool", RedisConfig{Addr: "h:1", PoolSize: -1}, "pool_size must not be negative"},
		{"negative timeout", RedisConfig{Addr: "h:1", DialTimeout: -time.Second}, "dial_timeout must not be negative"},
		{"min idle above pool", RedisConfig{Addr: "h:1", PoolSize: 2, MinIdleConns: 5}, "min_idle_conns (5) must not exceed pool_size (2)"},
		{"tls fields without enabled", RedisConfig{Addr: "h:1", TLS: TLSConfig{CAFile: "ca.pem"}}, "tls.ca_file is set but tls.enabled is false"},
		{"tls cert without key", RedisConfig{Addr: "h:1", TLS: TLSConfig{Enabled: true, CertFile: "c.pem"}}, "must be set together"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestMongoValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     MongoConfig
		wantErr string
	}{
		{"ok", MongoConfig{URI: "mongodb://localhost:27017"}, ""},
		{"srv", MongoConfig{URI: "mongodb+srv://c.example.net/db"}, ""},
		{"tuned", MongoConfig{URI: "mongodb://h", MaxPoolSize: 50, MinPoolSize: 5, Timeout: time.Second}, ""},
		{"no uri", MongoConfig{}, "uri is required"},
		{"wrong scheme", MongoConfig{URI: "http://h"}, "must start with mongodb://"},
		{"min above max", MongoConfig{URI: "mongodb://h", MaxPoolSize: 5, MinPoolSize: 10}, "min_pool_size (10) must not exceed max_pool_size (5)"},
		{"min without max", MongoConfig{URI: "mongodb://h", MinPoolSize: 10}, ""},
		{"negative timeout", MongoConfig{URI: "mongodb://h", ConnectTimeout: -1}, "connect_timeout must not be negative"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestDBValidate(t *testing.T) {
	ok := DBConfig{Driver: "postgres", Host: "h", Port: 5432}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (DBConfig{Host: "h", Port: 3306}).Validate(); err != nil {
		t.Fatalf("empty driver means mysql: %v", err)
	}
	for name, cfg := range map[string]DBConfig{
		"unknown driver": {Driver: "oracle", Host: "h"},
		"no host":        {Driver: "mysql"},
		"bad port":       {Host: "h", Port: 70000},
	} {
		if err := cfg.Validate(); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestLogAndOtelValidate(t *testing.T) {
	if err := (LogConfig{Level: "WARN", Format: "JSON"}).Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (LogConfig{Level: "loud"}).Validate(); err == nil {
		t.Fatal("bad level should fail")
	}
	if err := (LogConfig{Format: "xml"}).Validate(); err == nil {
		t.Fatal("bad format should fail")
	}
	nan := math.NaN()
	if err := (OtelConfig{TraceSampleRatio: &nan}).Validate(); err == nil {
		t.Fatal("NaN ratio should fail")
	}
	big := 5.0
	if err := (OtelConfig{TraceSampleRatio: &big}).Validate(); err != nil {
		t.Fatalf("ratios above 1 are documented as valid: %v", err)
	}
}

func TestCoreConfigValidateReportsEveryProblemWithItsKey(t *testing.T) {
	cfg := &CoreConfig{
		Log: LogConfig{Level: "loud"},
		Store: StoreConfig{
			Redis: map[string]RedisConfig{
				"cache": {Addr: "h:6379"},
				"bad":   {Mode: "ring", Addr: "h:1", PoolSize: -1},
			},
			Mongo: map[string]MongoConfig{"m": {}},
			DB:    map[string]DBConfig{"main": {Driver: "oracle", Host: "h"}},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected errors")
	}
	msg := err.Error()
	for _, want := range []string{
		`log: level "loud"`,
		`store.redis.bad: mode "ring"`,
		"store.redis.bad: pool_size must not be negative",
		"store.mongo.m: uri is required",
		`store.db.main: driver "oracle"`,
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("missing %q in:\n%s", want, msg)
		}
	}
	if strings.Contains(msg, "store.redis.cache") {
		t.Errorf("valid entry reported:\n%s", msg)
	}
	// Each problem sits on its own line, in a stable order.
	if lines := strings.Split(msg, "\n"); len(lines) != 5 {
		t.Errorf("want 5 lines, got %d:\n%s", len(lines), msg)
	}
}

func TestCoreConfigValidateEmptyIsValid(t *testing.T) {
	if err := new(CoreConfig).Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestConfigYAMLRoundTrip(t *testing.T) {
	doc := `
store:
  redis:
    cache:
      addrs: ["s1:26379", "s2:26379"]
      master_name: mymaster
      username: app
      password: pw
      sentinel_password: spw
      pool_size: 30
      min_idle_conns: 5
      pool_timeout: 4s
      conn_max_idle_time: 5m
      dial_timeout: 2s
      tls:
        enabled: true
        ca_file: /ca.pem
        server_name: redis.internal
  mongo:
    primary:
      uri: mongodb://h
      max_pool_size: 100
      min_pool_size: 10
      max_conn_idle_time: 1m
      server_selection_timeout: 3s
      timeout: 10s
`
	var cfg CoreConfig
	if err := yaml.Unmarshal([]byte(doc), &cfg); err != nil {
		t.Fatal(err)
	}
	r := cfg.Store.Redis["cache"]
	if r.ResolvedMode() != RedisModeSentinel || r.MasterName != "mymaster" || len(r.Addrs) != 2 ||
		r.PoolSize != 30 || r.PoolTimeout != 4*time.Second || r.ConnMaxIdleTime != 5*time.Minute ||
		!r.TLS.Enabled || r.TLS.ServerName != "redis.internal" || r.SentinelPassword != "spw" {
		t.Fatalf("redis config not decoded: %+v", r)
	}
	m := cfg.Store.Mongo["primary"]
	if m.MaxPoolSize != 100 || m.MinPoolSize != 10 || m.MaxConnIdleTime != time.Minute ||
		m.ServerSelectionTimeout != 3*time.Second || m.Timeout != 10*time.Second {
		t.Fatalf("mongo config not decoded: %+v", m)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("decoded config should be valid: %v", err)
	}
}

func TestConfigStringersMaskSecrets(t *testing.T) {
	const secret = "hunter2"
	values := []any{
		RedisConfig{Addr: "h:1", Password: secret, SentinelPassword: secret},
		MongoConfig{URI: "mongodb://app:" + secret + "@h/db"},
		DBConfig{Driver: "mysql", Host: "h", User: "u", Password: secret},
		S3Config{AccessKeyID: secret, SecretAccessKey: secret, SessionToken: secret},
	}
	for _, v := range values {
		for _, format := range []string{"%v", "%+v", "%s"} {
			if got := sprintf(format, v); strings.Contains(got, secret) {
				t.Errorf("%T printed with %s leaks the secret: %s", v, format, got)
			}
		}
	}
	// A pointer, as stored in maps of pointers or passed around, is covered too.
	if got := sprintf("%v", &RedisConfig{Addr: "h:1", Password: secret}); strings.Contains(got, secret) {
		t.Errorf("pointer leaks the secret: %s", got)
	}
}
