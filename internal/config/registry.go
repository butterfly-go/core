package config

var configBackend Config

func SetConfig(c Config)  { configBackend = c }
func GetConfig() Config   { return configBackend }
