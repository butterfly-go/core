package config

import "context"

var (
	config Config
)

type Config interface {
	Get(ctx context.Context, key string) ([]byte, error)
}

type AppConfig interface {
	Print()
}

func Init() error {
	c, err := NewConsulConfig()
	if err != nil {
		return err
	}
	// set config
	config = c
	return nil
}

func GetConfig() Config {
	return config
}
