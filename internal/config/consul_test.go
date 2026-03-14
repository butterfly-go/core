package config

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

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
