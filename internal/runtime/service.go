package runtime

var (
	service   string
	configKey string
)

func Init(srv string, key string) {
	service = srv
	configKey = key
}

func Service() string {
	return service
}

func ConfigKey() string {
	if configKey != "" {
		return configKey
	}
	return service
}
