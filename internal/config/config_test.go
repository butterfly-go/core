package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"butterfly.orx.me/core/mod"
)

func TestFileConfig_Get_Success(t *testing.T) {
	// Create a temp file with some YAML content
	tmpfile, err := os.CreateTemp("", "testconfig-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer func() { _ = os.Remove(tmpfile.Name()) }()
	content := []byte("foo: bar\nhello: world\n")
	if _, err := tmpfile.Write(content); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatalf("failed to close temp file: %v", err)
	}

	fc := &FileConfig{path: tmpfile.Name()}
	data, err := fc.Get(context.Background(), "ignored")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("expected %q, got %q", string(content), string(data))
	}
}

func TestFileConfig_Get_FileNotFound(t *testing.T) {
	fc := &FileConfig{path: "/tmp/nonexistent-file.yaml"}
	_, err := fc.Get(context.Background(), "ignored")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestNewFileConfig_MissingPath(t *testing.T) {
	// config.file.path must be empty for arg.String
	t.Setenv("BUTTERFLY_CONFIG_FILE", "")
	t.Setenv("BUTTERFLY_CONFIG_TYPE", "file")
	_, err := NewFileConfig()
	if err == nil {
		t.Error("expected error when config.file.path is not set, got nil")
	}
	if !errors.Is(err, errors.New("config.file.path not set")) && err.Error() != "config.file.path not set" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCoreConfigInit_ValidatesDocument(t *testing.T) {
	oldConfig, oldCore := config, *coreConfig
	t.Cleanup(func() { config, *coreConfig = oldConfig, oldCore })

	write := func(doc string) {
		path := filepath.Join(t.TempDir(), "core.yaml")
		if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
			t.Fatal(err)
		}
		config = &FileConfig{path: path}
		*coreConfig = mod.CoreConfig{}
	}

	write("store:\n  redis:\n    cache:\n      mode: ring\n      addr: \"h:6379\"\nlog:\n  level: loud\n")
	err := CoreConfigInit()
	if err == nil {
		t.Fatal("expected an invalid document to fail startup")
	}
	for _, want := range []string{"invalid core config", "store.redis.cache", `log: level "loud"`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q is missing %q", err, want)
		}
	}

	write("store:\n  redis:\n    cache:\n      addr: \"h:6379\"\n      pool_size: 10\n")
	if err := CoreConfigInit(); err != nil {
		t.Fatalf("valid document: %v", err)
	}
	if coreConfig.Store.Redis["cache"].PoolSize != 10 {
		t.Fatal("document was not loaded")
	}
}
