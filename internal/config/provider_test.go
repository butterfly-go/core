package config

import (
	"context"
	"os"
	"testing"

	"butterfly.orx.me/core/mod"
)

func TestProvideConfig_File(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "butterfly-cfg-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	tmpfile.Close()

	t.Setenv("BUTTERFLY_CONFIG_TYPE", "file")
	t.Setenv("BUTTERFLY_CONFIG_FILE_PATH", tmpfile.Name())

	cfg, err := ProvideConfig()
	if err != nil {
		t.Fatalf("ProvideConfig() error: %v", err)
	}
	if _, ok := cfg.(*FileConfig); !ok {
		t.Fatalf("expected *FileConfig, got %T", cfg)
	}
}

func TestProvideConfig_FileMissingPath(t *testing.T) {
	t.Setenv("BUTTERFLY_CONFIG_TYPE", "file")
	t.Setenv("BUTTERFLY_CONFIG_FILE_PATH", "")

	_, err := ProvideConfig()
	if err == nil {
		t.Fatal("expected error when file path is not set")
	}
}

func TestProvideCoreConfig_Success(t *testing.T) {
	yaml := `
store:
  redis:
    default:
      addr: "localhost:6379"
  mongo:
    primary:
      uri: "mongodb://localhost:27017"
log:
  level: "debug"
  format: "json"
`
	mock := &mockConfig{data: []byte(yaml)}
	cc, err := ProvideCoreConfig(mock, mod.ConfigKey("test-service"))
	if err != nil {
		t.Fatalf("ProvideCoreConfig() error: %v", err)
	}

	if len(cc.Store.Redis) != 1 {
		t.Fatalf("expected 1 redis config, got %d", len(cc.Store.Redis))
	}
	if cc.Store.Redis["default"].Addr != "localhost:6379" {
		t.Fatalf("unexpected redis addr: %s", cc.Store.Redis["default"].Addr)
	}
	if len(cc.Store.Mongo) != 1 {
		t.Fatalf("expected 1 mongo config, got %d", len(cc.Store.Mongo))
	}
	if cc.Log.Level != "debug" {
		t.Fatalf("expected log level debug, got %s", cc.Log.Level)
	}
	if cc.Log.Format != "json" {
		t.Fatalf("expected log format json, got %s", cc.Log.Format)
	}
}

func TestProvideCoreConfig_InvalidYAML(t *testing.T) {
	mock := &mockConfig{data: []byte("invalid: [yaml: broken")}
	_, err := ProvideCoreConfig(mock, mod.ConfigKey("test"))
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestProvideCoreConfig_GetError(t *testing.T) {
	mock := &mockConfig{err: context.DeadlineExceeded}
	_, err := ProvideCoreConfig(mock, mod.ConfigKey("test"))
	if err == nil {
		t.Fatal("expected error when config backend fails")
	}
}

func TestProvideCoreConfig_EmptyConfig(t *testing.T) {
	mock := &mockConfig{data: []byte("")}
	cc, err := ProvideCoreConfig(mock, mod.ConfigKey("test"))
	if err != nil {
		t.Fatalf("ProvideCoreConfig() error: %v", err)
	}
	if len(cc.Store.Redis) != 0 {
		t.Fatalf("expected empty redis config, got %d", len(cc.Store.Redis))
	}
}

// mockConfig implements Config for testing.
type mockConfig struct {
	data []byte
	err  error
}

func (m *mockConfig) Get(_ context.Context, _ string) ([]byte, error) {
	return m.data, m.err
}
