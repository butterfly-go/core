package mod

import "time"

type CoreConfig struct {
	Store StoreConfig `yaml:"store"`
	Otel  OtelConfig  `yaml:"otel"`
	Log   LogConfig   `yaml:"log"`
	// Server configures the listen addresses of the built-in servers.
	Server ServerConfig `yaml:"server"`
}

// ServerConfig holds the listen addresses of the servers the framework runs.
// Every field is optional; see the defaults on each.
type ServerConfig struct {
	// HTTP is the business HTTP server. Defaults to $PORT when set, else ":8080".
	HTTP ListenConfig `yaml:"http"`
	// GRPC is the gRPC server, only started when app.Config.GRPCRegister is
	// set. Defaults to ":9090".
	GRPC ListenConfig `yaml:"grpc"`
	// Admin serves operational endpoints: /metrics, /healthz and /readyz.
	// Defaults to ":2223".
	Admin ListenConfig `yaml:"admin"`
	// ShutdownDelay is how long to wait between reporting not-ready on
	// /readyz (and gRPC health) and starting to stop the servers, so load
	// balancers have time to stop routing here. Defaults to 0 (no delay). It
	// counts against app.Config.ShutdownTimeout and must be smaller than it.
	// Accepts Go durations such as "5s".
	ShutdownDelay time.Duration `yaml:"shutdown_delay"`
}

// ListenConfig is a single listen address.
type ListenConfig struct {
	// Addr is a host:port pair such as ":8080" or "127.0.0.1:8080". Port 0
	// picks a free port.
	Addr string `yaml:"addr"`
}

type LogConfig struct {
	Level     string `json:"level" yaml:"level"`
	Format    string `json:"format" yaml:"format"`
	AddSource *bool  `json:"add_source" yaml:"add_source"`
}

type StoreConfig struct {
	Mongo map[string]MongoConfig `yaml:"mongo"`
	Redis map[string]RedisConfig `yaml:"redis"`
	DB    map[string]DBConfig    `yaml:"db"`
	S3    map[string]S3Config    `yaml:"s3"`
}

type OtelConfig struct {
	// TraceSampleRatio is the fraction of root spans to record when tracing
	// is enabled. Valid range is 0.0-1.0. When nil or >= 1, all spans are
	// kept (AlwaysSample). When <= 0, no spans are recorded (NeverSample).
	// Values between 0 and 1 use ParentBased(TraceIDRatioBased(ratio)).
	TraceSampleRatio *float64 `yaml:"trace_sample_ratio"`
}

// TLSConfig configures a TLS client connection. The zero value means no TLS.
type TLSConfig struct {
	// Enabled turns TLS on. The other fields are rejected while it is false,
	// so a CA file is never silently ignored.
	Enabled bool `yaml:"enabled"`
	// CAFile is a PEM bundle of CAs to trust. Defaults to the system roots.
	CAFile string `yaml:"ca_file"`
	// CertFile and KeyFile are the client certificate for mutual TLS. Set both
	// or neither.
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
	// ServerName overrides the name used to verify the server certificate.
	ServerName string `yaml:"server_name"`
	// InsecureSkipVerify disables certificate verification. Testing only.
	InsecureSkipVerify bool `yaml:"insecure_skip_verify"`
}

// MongoConfig configures a MongoDB client.
//
// The pool and timeout fields are optional. A non-zero value overrides the
// matching option in URI; zero leaves the URI or the driver default in effect.
// TLS and credentials belong in the URI (tls=true, tlsCAFile=..., ...).
type MongoConfig struct {
	URI string `yaml:"uri"`
	// MaxPoolSize and MinPoolSize bound the connections per server.
	MaxPoolSize uint64 `yaml:"max_pool_size"`
	MinPoolSize uint64 `yaml:"min_pool_size"`
	// MaxConnecting caps connections being established at once.
	MaxConnecting uint64 `yaml:"max_connecting"`
	// MaxConnIdleTime closes connections idle for longer than this.
	MaxConnIdleTime time.Duration `yaml:"max_conn_idle_time"`
	// ConnectTimeout bounds establishing one connection.
	ConnectTimeout time.Duration `yaml:"connect_timeout"`
	// ServerSelectionTimeout bounds finding a usable server.
	ServerSelectionTimeout time.Duration `yaml:"server_selection_timeout"`
	// Timeout is the default deadline of a whole operation, including server
	// selection, connection checkout and execution. A context deadline wins.
	Timeout time.Duration `yaml:"timeout"`
}

// Redis deployment modes.
const (
	RedisModeStandalone = "standalone"
	RedisModeSentinel   = "sentinel"
	RedisModeCluster    = "cluster"
)

// RedisConfig configures a Redis client in one of three modes, see Mode.
//
// A standalone server only needs Addr. Sentinel adds MasterName and the
// sentinel addresses in Addrs. Cluster lists seed nodes in Addrs. All
// duration and size fields are optional: zero uses the go-redis default.
type RedisConfig struct {
	// Mode is "standalone", "sentinel" or "cluster". When empty it is inferred:
	// master_name set means sentinel, more than one address means cluster,
	// anything else is standalone.
	Mode string `yaml:"mode"`
	// Addr is the server address (host:port) in standalone mode.
	Addr string `yaml:"addr"`
	// Addrs are the sentinel addresses (sentinel) or seed nodes (cluster).
	Addrs []string `yaml:"addrs"`
	// MasterName is the sentinel master name. Sentinel mode only.
	MasterName string `yaml:"master_name"`

	Username string `yaml:"username"`
	Password string `yaml:"password"`
	// DB is the database number. Not supported by cluster.
	DB int `yaml:"db"`
	// SentinelUsername and SentinelPassword authenticate to the sentinels,
	// when they differ from the Redis credentials.
	SentinelUsername string `yaml:"sentinel_username"`
	SentinelPassword string `yaml:"sentinel_password"`

	TLS TLSConfig `yaml:"tls"`

	// PoolSize is the maximum number of connections per node.
	PoolSize int `yaml:"pool_size"`
	// MinIdleConns keeps this many idle connections open.
	MinIdleConns int `yaml:"min_idle_conns"`
	// PoolTimeout is how long to wait for a free connection.
	PoolTimeout time.Duration `yaml:"pool_timeout"`
	// ConnMaxIdleTime and ConnMaxLifetime recycle connections.
	ConnMaxIdleTime time.Duration `yaml:"conn_max_idle_time"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`

	DialTimeout  time.Duration `yaml:"dial_timeout"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
}

type DBConfig struct {
	Driver   string `yaml:"driver"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"db_name"`
	SSLMode  string `yaml:"ssl_mode"`
}

type S3Config struct {
	Endpoint        string `yaml:"endpoint"`
	AccessKeyID     string `yaml:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key"`
	AK              string `yaml:"ak"`
	SK              string `yaml:"sk"`
	SessionToken    string `yaml:"session_token"`
	Region          string `yaml:"region"`
	Bucket          string `yaml:"bucket"`
	UseSSL          bool   `yaml:"use_ssl"`
	UsePathStyle    bool   `yaml:"use_path_style"`
}
