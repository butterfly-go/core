package tlsutil

import (
	"crypto/tls"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"butterfly.orx.me/core/internal/tlstest"
	"butterfly.orx.me/core/mod"
)

func TestBuildDisabledReturnsNil(t *testing.T) {
	cfg, err := Build(mod.TLSConfig{})
	if err != nil || cfg != nil {
		t.Fatalf("Build(zero) = %v, %v; want nil, nil", cfg, err)
	}
}

func TestBuildRejectsFieldsWithoutEnabled(t *testing.T) {
	if _, err := Build(mod.TLSConfig{CAFile: "ca.pem"}); err == nil {
		t.Fatal("a CA file with TLS disabled should be an error, not silently ignored")
	}
}

func TestBuildLoadsCAAndClientCert(t *testing.T) {
	b := tlstest.New(t)
	cfg, err := Build(mod.TLSConfig{
		Enabled:    true,
		CAFile:     b.CAFile,
		CertFile:   b.ClientCertFile,
		KeyFile:    b.ClientKeyFile,
		ServerName: "redis.internal",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RootCAs == nil || len(cfg.Certificates) != 1 {
		t.Fatalf("CA or client certificate missing: %+v", cfg)
	}
	if cfg.ServerName != "redis.internal" || cfg.MinVersion != tls.VersionTLS12 || cfg.InsecureSkipVerify {
		t.Fatalf("unexpected settings: %+v", cfg)
	}
}

func TestBuildEnabledWithoutFilesUsesSystemRoots(t *testing.T) {
	cfg, err := Build(mod.TLSConfig{Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RootCAs != nil || len(cfg.Certificates) != 0 {
		t.Fatalf("expected the defaults: %+v", cfg)
	}
}

func TestBuildInsecureSkipVerify(t *testing.T) {
	cfg, err := Build(mod.TLSConfig{Enabled: true, InsecureSkipVerify: true})
	if err != nil || !cfg.InsecureSkipVerify {
		t.Fatalf("cfg = %+v, err = %v", cfg, err)
	}
}

func TestBuildErrors(t *testing.T) {
	b := tlstest.New(t)
	notPEM := filepath.Join(t.TempDir(), "junk.pem")
	if err := os.WriteFile(notPEM, []byte("not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		cfg  mod.TLSConfig
		want string
	}{
		{"missing CA file", mod.TLSConfig{Enabled: true, CAFile: "/no/such/ca.pem"}, "tls.ca_file"},
		{"CA without certificates", mod.TLSConfig{Enabled: true, CAFile: notPEM}, "no valid PEM"},
		{"cert without key", mod.TLSConfig{Enabled: true, CertFile: b.ClientCertFile}, "must be set together"},
		{"bad key pair", mod.TLSConfig{Enabled: true, CertFile: b.ClientCertFile, KeyFile: b.ServerKeyFile}, "tls.cert_file"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Build(tt.cfg)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want substring %q", err, tt.want)
			}
		})
	}
}
