package arg

import (
	"os"
	"strings"
)

func String(key string) string {
	key = strings.Replace(key, "-", "", -1)
	key = strings.ToUpper(key)
	key = "BUTTERFLY_" + key
	return os.Getenv(key)
}

func Bool(key string) bool {
	v := String(key)
	return v == "true" || v == "1"
}
