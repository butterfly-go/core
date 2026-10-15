package arg

import (
	"log/slog"
	"os"
	"strings"
)

func String(key string) string {
	envKey := strings.ReplaceAll(key, "-", "_")
	envKey = strings.ReplaceAll(envKey, ".", "_")
	envKey = strings.ToUpper(envKey)
	envKey = "BUTTERFLY_" + envKey
	slog.Debug("arg get string", "key", key, "env_key", envKey)
	return os.Getenv(envKey)
}

func Bool(key string) bool {
	v := String(key)
	return v == "true" || v == "1"
}
