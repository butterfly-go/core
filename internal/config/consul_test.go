package config

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"butterfly.orx.me/core/internal/tlstest"
	capi "github.com/hashicorp/consul/api"
)

func TestConsulConfig_Get_UsesNamespace(t *testing.T) {
	c, err := newTestConsulConfig("team-a", func(r *http.Request) (*http.Response, error) {
		if got := r.URL.Query().Get("ns"); got != "team-a" {
			t.Fatalf("expected namespace query %q, got %q", "team-a", got)
		}
		if r.URL.Path != "/v1/kv/service-a" {
			t.Fatalf("expected request path %q, got %q", "/v1/kv/service-a", r.URL.Path)
		}
		return newJSONResponse(http.StatusOK, `[{ "Key": "service-a", "Value": "Y29uZmlnOiB0cnVlCg==" }]`), nil
	})
	if err != nil {
		t.Fatalf("expected no error creating consul config, got %v", err)
	}

	data, err := c.Get(context.Background(), "service-a")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if string(data) != "config: true\n" {
		t.Fatalf("expected decoded consul value, got %q", string(data))
	}
}

func TestConsulConfig_Get_WithoutNamespace(t *testing.T) {
	c, err := newTestConsulConfig("", func(r *http.Request) (*http.Response, error) {
		if got := r.URL.Query().Get("ns"); got != "" {
			t.Fatalf("expected no namespace query, got %q", got)
		}
		return newJSONResponse(http.StatusNotFound, ``), nil
	})
	if err != nil {
		t.Fatalf("expected no error creating consul config, got %v", err)
	}

	data, err := c.Get(context.Background(), "missing-service")
	if err != nil {
		t.Fatalf("expected no error for missing key, got %v", err)
	}
	if len(data) != 0 {
		t.Fatalf("expected empty data for missing key, got %q", string(data))
	}
}

func TestNewConsulConfig_InvalidAddress(t *testing.T) {
	t.Setenv("BUTTERFLY_CONFIG_CONSUL_ADDRESS", "://bad-address")
	t.Setenv("BUTTERFLY_CONFIG_CONSUL_NAMESPACE", "team-a")

	if _, err := NewConsulConfig(); err == nil {
		t.Fatal("expected error for invalid consul address, got nil")
	}
}

func newTestConsulConfig(namespace string, fn func(*http.Request) (*http.Response, error)) (*ConsulConfig, error) {
	client, err := capi.NewClient(&capi.Config{
		Address:   "http://consul.test",
		Namespace: namespace,
		HttpClient: &http.Client{
			Transport: roundTripperFunc(fn),
		},
	})
	if err != nil {
		return nil, err
	}

	return &ConsulConfig{
		client: client,
		kv:     client.KV(),
	}, nil
}

func newJSONResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

func TestNewConsulConfig_SendsToken(t *testing.T) {
	kv := newFakeKV("a: 1")
	srv := httptest.NewServer(kv)
	defer srv.Close()
	t.Setenv("BUTTERFLY_CONFIG_CONSUL_ADDRESS", srv.Listener.Addr().String())
	t.Setenv("BUTTERFLY_CONFIG_CONSUL_TOKEN", "s3cr3t-token")

	c, err := NewConsulConfig()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Get(context.Background(), "svc"); err != nil {
		t.Fatal(err)
	}
	if kv.token != "s3cr3t-token" {
		t.Fatalf("X-Consul-Token = %q", kv.token)
	}
}

func TestNewConsulConfig_ReadsTokenFile(t *testing.T) {
	kv := newFakeKV("a: 1")
	srv := httptest.NewServer(kv)
	defer srv.Close()
	tokenFile := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenFile, []byte("file-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BUTTERFLY_CONFIG_CONSUL_ADDRESS", srv.Listener.Addr().String())
	t.Setenv("BUTTERFLY_CONFIG_CONSUL_TOKEN_FILE", tokenFile)

	c, err := NewConsulConfig()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Get(context.Background(), "svc"); err != nil {
		t.Fatal(err)
	}
	if kv.token != "file-token" {
		t.Fatalf("X-Consul-Token = %q", kv.token)
	}
}

func newTLSConsulServer(t *testing.T, b tlstest.Bundle, requireClientCert bool) (*fakeKV, string) {
	t.Helper()
	kv := newFakeKV("a: 1")
	srv := httptest.NewUnstartedServer(kv)
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{b.ServerCert}}
	if requireClientCert {
		srv.TLS.ClientAuth = tls.RequireAndVerifyClientCert
		srv.TLS.ClientCAs = b.CAPool
	}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return kv, srv.Listener.Addr().String()
}

func TestNewConsulConfig_TLS(t *testing.T) {
	b := tlstest.New(t)
	_, addr := newTLSConsulServer(t, b, false)
	t.Setenv("BUTTERFLY_CONFIG_CONSUL_ADDRESS", addr)
	t.Setenv("BUTTERFLY_CONFIG_CONSUL_TLS_ENABLED", "true")
	t.Setenv("BUTTERFLY_CONFIG_CONSUL_TLS_CA_FILE", b.CAFile)

	c, err := NewConsulConfig()
	if err != nil {
		t.Fatal(err)
	}
	data, err := c.Get(context.Background(), "svc")
	if err != nil {
		t.Fatalf("Get over TLS: %v", err)
	}
	if string(data) != "a: 1" {
		t.Fatalf("data = %q", data)
	}
}

func TestNewConsulConfig_TLSRejectsUntrustedServer(t *testing.T) {
	b := tlstest.New(t)
	_, addr := newTLSConsulServer(t, b, false)
	t.Setenv("BUTTERFLY_CONFIG_CONSUL_ADDRESS", addr)
	t.Setenv("BUTTERFLY_CONFIG_CONSUL_TLS_ENABLED", "true") // no CA: the test CA is not trusted

	c, err := NewConsulConfig()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Get(context.Background(), "svc"); err == nil {
		t.Fatal("expected a certificate verification error")
	}
}

func TestNewConsulConfig_TLSSkipVerifyAndServerName(t *testing.T) {
	b := tlstest.New(t)
	_, addr := newTLSConsulServer(t, b, false)
	t.Setenv("BUTTERFLY_CONFIG_CONSUL_ADDRESS", addr)
	t.Setenv("BUTTERFLY_CONFIG_CONSUL_TLS_ENABLED", "true")
	t.Setenv("BUTTERFLY_CONFIG_CONSUL_TLS_INSECURE_SKIP_VERIFY", "true")
	c, err := NewConsulConfig()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Get(context.Background(), "svc"); err != nil {
		t.Fatalf("skip-verify Get: %v", err)
	}

	// A wrong server name must fail verification against the trusted CA.
	t.Setenv("BUTTERFLY_CONFIG_CONSUL_TLS_INSECURE_SKIP_VERIFY", "")
	t.Setenv("BUTTERFLY_CONFIG_CONSUL_TLS_CA_FILE", b.CAFile)
	t.Setenv("BUTTERFLY_CONFIG_CONSUL_TLS_SERVER_NAME", "other.example.com")
	c, err = NewConsulConfig()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Get(context.Background(), "svc"); err == nil {
		t.Fatal("expected a server name mismatch")
	}
}

func TestNewConsulConfig_MutualTLS(t *testing.T) {
	b := tlstest.New(t)
	_, addr := newTLSConsulServer(t, b, true)
	t.Setenv("BUTTERFLY_CONFIG_CONSUL_ADDRESS", addr)
	t.Setenv("BUTTERFLY_CONFIG_CONSUL_TLS_ENABLED", "true")
	t.Setenv("BUTTERFLY_CONFIG_CONSUL_TLS_CA_FILE", b.CAFile)

	c, err := NewConsulConfig()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Get(context.Background(), "svc"); err == nil {
		t.Fatal("expected the server to reject a client without a certificate")
	}

	t.Setenv("BUTTERFLY_CONFIG_CONSUL_TLS_CERT_FILE", b.ClientCertFile)
	t.Setenv("BUTTERFLY_CONFIG_CONSUL_TLS_KEY_FILE", b.ClientKeyFile)
	c, err = NewConsulConfig()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Get(context.Background(), "svc"); err != nil {
		t.Fatalf("Get with a client certificate: %v", err)
	}
}

func TestNewConsulConfig_TLSMisconfiguration(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"ca file without enabled",
			map[string]string{"BUTTERFLY_CONFIG_CONSUL_TLS_CA_FILE": "/ca.pem"}, "tls.enabled is false"},
		{"cert without key",
			map[string]string{"BUTTERFLY_CONFIG_CONSUL_TLS_ENABLED": "true", "BUTTERFLY_CONFIG_CONSUL_TLS_CERT_FILE": "/c.pem"}, "must be set together"},
		{"http address with tls",
			map[string]string{"BUTTERFLY_CONFIG_CONSUL_TLS_ENABLED": "true", "BUTTERFLY_CONFIG_CONSUL_ADDRESS": "http://consul:8500"}, "http://"},
		{"missing ca file",
			map[string]string{"BUTTERFLY_CONFIG_CONSUL_TLS_ENABLED": "true", "BUTTERFLY_CONFIG_CONSUL_TLS_CA_FILE": "/no/such/ca.pem"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			_, err := NewConsulConfig()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want substring %q", err, tt.want)
			}
		})
	}
}
