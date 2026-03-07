package config

import (
	"context"
	"errors"
	"os"
	"testing"
)

func TestFileConfig_Get_Success(t *testing.T) {
	// Create a temp file with some YAML content
	tmpfile, err := os.CreateTemp("", "testconfig-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())
	content := []byte("foo: bar\nhello: world\n")
	if _, err := tmpfile.Write(content); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpfile.Close()

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
	os.Setenv("BUTTERFLY_CONFIG_FILE", "")
	os.Setenv("BUTTERFLY_CONFIG_TYPE", "file")
	// Unset config.file.path for arg.String
	os.Unsetenv("BUTTERFLY_CONFIG_FILE")
	os.Unsetenv("BUTTERFLY_CONFIG_TYPE")
	_, err := NewFileConfig()
	if err == nil {
		t.Error("expected error when config.file.path is not set, got nil")
	}
	if !errors.Is(err, errors.New("config.file.path not set")) && err.Error() != "config.file.path not set" {
		t.Errorf("unexpected error: %v", err)
	}
}
