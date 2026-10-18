package mod

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

func sprintf(format string, a ...any) string { return fmt.Sprintf(format, a...) }

func TestConfigLogValueMasksSecrets(t *testing.T) {
	const secret = "hunter2"
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	logger.Info("cfg",
		"redis", RedisConfig{Addr: "h:1", Password: secret, SentinelPassword: secret},
		"mongo", MongoConfig{URI: "mongodb://app:" + secret + "@h/db"},
		"db", DBConfig{Host: "h", Password: secret},
		"s3", S3Config{SecretAccessKey: secret, SK: secret, SessionToken: secret},
	)
	out := buf.String()
	if strings.Contains(out, secret) {
		t.Fatalf("secret leaked: %s", out)
	}
	for _, want := range []string{"redis.password=***", "mongo.uri=mongodb://app:***@h/db", "db.password=***"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %s", want, out)
		}
	}
}
