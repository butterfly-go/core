package redact

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestString(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		secrets []string
		want    string
	}{
		{"empty", "", nil, ""},
		{"plain", "dial tcp 127.0.0.1:5432: connection refused", nil, "dial tcp 127.0.0.1:5432: connection refused"},
		{"postgres uri", "parse postgres://app:s3cret@db:5432/app?sslmode=disable", nil,
			"parse postgres://app:***@db:5432/app?sslmode=disable"},
		{"mongodb srv", "mongodb+srv://u:p%40ss@cluster0.example.net/db", nil,
			"mongodb+srv://u:***@cluster0.example.net/db"},
		{"unescaped at sign", "postgres://u:p@ss@h:5432/db", nil, "postgres://u:***@h:5432/db"},
		{"no password", "mongodb://host1:27017,host2:27017/db", nil, "mongodb://host1:27017,host2:27017/db"},
		{"user only", "redis://user@host:6379", nil, "redis://user@host:6379"},
		{"empty user", "redis://:hunter2@host:6379/0", nil, "redis://:***@host:6379/0"},
		{"mysql dsn", "root:p4ss@tcp(127.0.0.1:3306)/app?charset=utf8mb4", nil,
			"root:***@tcp(127.0.0.1:3306)/app?charset=utf8mb4"},
		{"kv password", "host=db user=u password=hunter2 dbname=x", nil, "host=db user=u password=*** dbname=x"},
		{"query password", "mongodb://h/db?authSource=admin&sslpassword=x&tlsCertificateKeyFilePassword=y", nil,
			"mongodb://h/db?authSource=admin&sslpassword=***&tlsCertificateKeyFilePassword=***"},
		{"literal secret", "auth failed for hunter2", []string{"hunter2"}, "auth failed for ***"},
		{"empty secret ignored", "plain text", []string{""}, "plain text"},
		{"multiple", "a=pw1 b=pw2", []string{"pw1", "pw2"}, "a=*** b=***"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := String(tt.in, tt.secrets...); got != tt.want {
				t.Fatalf("String(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSecret(t *testing.T) {
	if Secret("") != "" {
		t.Fatal("empty secret should stay empty")
	}
	if Secret("x") != Mask {
		t.Fatal("non-empty secret should be masked")
	}
}

func TestErrorKeepsChain(t *testing.T) {
	base := errors.New("cannot parse postgres://u:pw@h/db")
	wrapped := fmt.Errorf("open: %w", base)
	got := Error(wrapped)
	if strings.Contains(got.Error(), "pw") {
		t.Fatalf("password leaked: %v", got)
	}
	if !errors.Is(got, base) {
		t.Fatal("errors.Is should still find the original error")
	}

	clean := errors.New("connection refused")
	if Error(clean) != clean {
		t.Fatal("an error without secrets should be returned unchanged")
	}
	if Error(nil) != nil {
		t.Fatal("nil should stay nil")
	}
	if got := Error(errors.New("bad token abc"), "abc"); strings.Contains(got.Error(), "abc") {
		t.Fatalf("literal secret leaked: %v", got)
	}
}

func TestIsSensitiveKey(t *testing.T) {
	for _, k := range []string{"password", "Password", "db_password", "api-key", "secret_access_key", "token", "csrf_token", "dsn", "authorization"} {
		if !IsSensitiveKey(k) {
			t.Errorf("%q should be sensitive", k)
		}
	}
	for _, k := range []string{"key", "name", "addr", "token_len", "access_key_len", "config_key", "uri", "msg", "error"} {
		if IsSensitiveKey(k) {
			t.Errorf("%q should not be sensitive", k)
		}
	}
}
