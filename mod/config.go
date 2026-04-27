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
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"db_name"`
}

type S3Config struct {
	Provider        string `yaml:"provider"`
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
