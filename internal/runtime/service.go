package runtime

var (
	service   string
	configKey string
)

func Service() string {
	return service
}

func SetService(srv string) {
	service = srv
}

func ConfigKey() string {
	if configKey != "" {
		return configKey
	}
	return service
}

func SetConfigKey(key string) {
	configKey = key
}
