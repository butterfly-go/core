package mod

type CoreConfig struct {
	Store StoreConfig `yaml:"store"`
	Otel  OtelConfig  `yaml:"otel"`
}

type StoreConfig struct {
	Mongo map[string]MongoConfig `yaml:"mongo"`
}

type OtelConfig struct {
}

type MongoConfig struct {
	URI string `yaml:"uri"`
}
