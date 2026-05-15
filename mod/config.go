package mod

type CoreConfig struct {
	Store StoreConfig `yaml:"store"`
	Otel  OtelConfig  `yaml:"otel"`
	Log   LogConfig   `yaml:"log"`
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
	// is enabled. Valid range is 0.0–1.0. When nil or >= 1, all spans are
	// kept (AlwaysSample). When <= 0, no spans are recorded (NeverSample).
	// Values between 0 and 1 use ParentBased(TraceIDRatioBased(ratio)).
	TraceSampleRatio *float64 `yaml:"trace_sample_ratio"`
}

type MongoConfig struct {
	URI string `yaml:"uri"`
}

type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
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
