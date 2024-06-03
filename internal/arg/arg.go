package arg

import (
	"log/slog"
	"os"
	"strings"
)

func String(key string) string {
	envKey := strings.Replace(key, "-", "_", -1)
	envKey = strings.ToUpper(envKey)
	envKey = "BUTTERFLY_" + envKey
	slog.Debug("arg get string", "key", key, "env_key", envKey)
	return os.Getenv(envKey)
}

func Bool(key string) bool {
	v := String(key)
	return v == "true" || v == "1"
}
