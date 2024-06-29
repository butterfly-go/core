package mod

type CoreConfig struct {
	Store StoreConfig `yaml:"store"`
	Otel  OtelConfig  `yaml:"otel"`
}

type StoreConfig struct {
	Mongo map[string]MongoConfig `yaml:"mongo"`
	Redis map[string]RedisConfig `yaml:"redis"`
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
