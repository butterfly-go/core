package tracing

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"butterfly.orx.me/core/internal/tlstest"
	"butterfly.orx.me/core/mod"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	coltrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

func boolp(b bool) *bool { return &b }

func TestParseHeaders(t *testing.T) {
	tests := []struct {
		in      string
		want    map[string]string
		wantErr bool
	}{
		{in: "", want: nil},
		{in: " , ,", want: nil},
		{in: "api-key=abc", want: map[string]string{"api-key": "abc"}},
		{in: " a = 1 , b=2 ", want: map[string]string{"a": "1", "b": "2"}},
		{in: "token=a=b==", want: map[string]string{"token": "a=b=="}},
		{in: "auth=Bearer%20xyz", want: map[string]string{"auth": "Bearer xyz"}},
		{in: "empty=", want: map[string]string{"empty": ""}},
		{in: "novalue", wantErr: true},
		{in: "=v", wantErr: true},
		{in: "k=%zz", wantErr: true},
	}
	for _, tt := range tests {
		got, err := parseHeaders(tt.in)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseHeaders(%q) error = %v, wantErr %v", tt.in, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
			t.Errorf("parseHeaders(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestParseHeadersErrorDoesNotEchoSecret(t *testing.T) {
	_, err := parseHeaders("a=1,my-bare-secret-token")
	if err == nil || strings.Contains(err.Error(), "my-bare-secret-token") {
		t.Fatalf("error should name the entry position, not its content: %v", err)
	}
}

func TestExporterConfigSecure(t *testing.T) {
	tests := []struct {
		name string
		cfg  exporterConfig
		want bool
	}{
		{"grpc defaults to plaintext", exporterConfig{Provider: "grpc"}, false},
		{"unset provider is grpc", exporterConfig{}, false},
		{"http defaults to TLS", exporterConfig{Provider: "http"}, true},
		{"http insecure", exporterConfig{Provider: "http", Insecure: boolp(true)}, false},
		{"grpc insecure=false", exporterConfig{Provider: "grpc", Insecure: boolp(false)}, true},
		{"tls enabled", exporterConfig{Provider: "grpc", TLS: mod.TLSConfig{Enabled: true}}, true},
	}
	for _, tt := range tests {
		if got := tt.cfg.secure(); got != tt.want {
			t.Errorf("%s: secure() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestExporterConfigFromEnv(t *testing.T) {
	t.Setenv("BUTTERFLY_TRACING_PROVIDER", "http")
	t.Setenv("BUTTERFLY_TRACING_ENDPOINT", "collector:4318")
	t.Setenv("BUTTERFLY_TRACING_INSECURE", "false")
	t.Setenv("BUTTERFLY_TRACING_HEADERS", "api-key=abc,x-team=core")
	t.Setenv("BUTTERFLY_TRACING_TLS_ENABLED", "true")
	t.Setenv("BUTTERFLY_TRACING_TLS_CA_FILE", "/ca.pem")

	cfg, err := exporterConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Provider != "http" || cfg.Endpoint != "collector:4318" || cfg.Insecure == nil || *cfg.Insecure ||
		!cfg.TLS.Enabled || cfg.TLS.CAFile != "/ca.pem" || cfg.Headers["api-key"] != "abc" || cfg.Headers["x-team"] != "core" {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestExporterConfigFromEnvErrors(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"bad insecure", map[string]string{"BUTTERFLY_TRACING_INSECURE": "maybe"}, "INSECURE"},
		{"bad headers", map[string]string{"BUTTERFLY_TRACING_HEADERS": "oops"}, "HEADERS"},
		{"tls fields without enabled", map[string]string{"BUTTERFLY_TRACING_TLS_CA_FILE": "/ca.pem"}, "tls.enabled is false"},
		{"tls and insecure conflict", map[string]string{"BUTTERFLY_TRACING_TLS_ENABLED": "true", "BUTTERFLY_TRACING_INSECURE": "true"}, "INSECURE is true"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			if _, err := exporterConfigFromEnv(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestExporterConfigFromEnvDefaults(t *testing.T) {
	cfg, err := exporterConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Insecure != nil || cfg.TLS.Enabled || cfg.Headers != nil {
		t.Fatalf("defaults changed: %+v", cfg)
	}
}

func exportOneSpan(t *testing.T, cfg exporterConfig) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	exp, err := newExporter(ctx, cfg)
	if err != nil {
		t.Fatalf("newExporter: %v", err)
	}
	defer func() { _ = exp.Shutdown(context.Background()) }()
	return exp.ExportSpans(ctx, tracetest.SpanStubs{{Name: "test"}}.Snapshots())
}

// httpCollector records the requests an OTLP/HTTP exporter sends.
type httpCollector struct {
	mu      sync.Mutex
	headers []http.Header
	certs   int // requests that carried a verified client certificate
}

func (c *httpCollector) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.mu.Lock()
	c.headers = append(c.headers, r.Header.Clone())
	if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
		c.certs++
	}
	c.mu.Unlock()
	w.Header().Set("Content-Type", "application/x-protobuf")
	w.WriteHeader(http.StatusOK)
}

func (c *httpCollector) last(t *testing.T) http.Header {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.headers) == 0 {
		t.Fatal("collector received no request")
	}
	return c.headers[len(c.headers)-1]
}

func TestHTTPExporterSendsHeadersOverTLS(t *testing.T) {
	b := tlstest.New(t)
	col := &httpCollector{}
	srv := httptest.NewUnstartedServer(col)
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{b.ServerCert}}
	srv.StartTLS()
	defer srv.Close()

	err := exportOneSpan(t, exporterConfig{
		Provider: "http",
		Endpoint: srv.Listener.Addr().String(),
		TLS:      mod.TLSConfig{Enabled: true, CAFile: b.CAFile},
		Headers:  map[string]string{"Api-Key": "abc123", "X-Team": "core"},
	})
	if err != nil {
		t.Fatalf("export over TLS: %v", err)
	}
	h := col.last(t)
	if h.Get("Api-Key") != "abc123" || h.Get("X-Team") != "core" {
		t.Fatalf("headers not sent: %v", h)
	}
}

func TestHTTPExporterMutualTLS(t *testing.T) {
	b := tlstest.New(t)
	col := &httpCollector{}
	srv := httptest.NewUnstartedServer(col)
	srv.TLS = &tls.Config{
		Certificates: []tls.Certificate{b.ServerCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    b.CAPool,
	}
	srv.StartTLS()
	defer srv.Close()

	err := exportOneSpan(t, exporterConfig{
		Provider: "http",
		Endpoint: srv.Listener.Addr().String(),
		TLS:      mod.TLSConfig{Enabled: true, CAFile: b.CAFile, CertFile: b.ClientCertFile, KeyFile: b.ClientKeyFile},
	})
	if err != nil {
		t.Fatalf("export with a client certificate: %v", err)
	}
	if col.certs == 0 {
		t.Fatal("server did not see the client certificate")
	}
}

func TestHTTPExporterRejectsUntrustedServer(t *testing.T) {
	b := tlstest.New(t)
	srv := httptest.NewUnstartedServer(&httpCollector{})
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{b.ServerCert}}
	srv.StartTLS()
	defer srv.Close()

	// The default for http is TLS with system roots, which do not know the test CA.
	if err := exportOneSpan(t, exporterConfig{Provider: "http", Endpoint: srv.Listener.Addr().String()}); err == nil {
		t.Fatal("expected a certificate verification error")
	}
}

func TestHTTPExporterPlaintext(t *testing.T) {
	col := &httpCollector{}
	srv := httptest.NewServer(col)
	defer srv.Close()

	err := exportOneSpan(t, exporterConfig{
		Provider: "http",
		Endpoint: srv.Listener.Addr().String(),
		Insecure: boolp(true),
		Headers:  map[string]string{"Authorization": "Bearer t"},
	})
	if err != nil {
		t.Fatalf("plaintext export: %v", err)
	}
	if got := col.last(t).Get("Authorization"); got != "Bearer t" {
		t.Fatalf("Authorization = %q", got)
	}
}

// grpcCollector records the metadata of each Export call.
type grpcCollector struct {
	coltrace.UnimplementedTraceServiceServer
	mu  sync.Mutex
	mds []metadata.MD
}

func (c *grpcCollector) Export(ctx context.Context, _ *coltrace.ExportTraceServiceRequest) (*coltrace.ExportTraceServiceResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	c.mu.Lock()
	c.mds = append(c.mds, md)
	c.mu.Unlock()
	return &coltrace.ExportTraceServiceResponse{}, nil
}

func startGRPCCollector(t *testing.T, opts ...grpc.ServerOption) (*grpcCollector, string) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	col := &grpcCollector{}
	srv := grpc.NewServer(opts...)
	coltrace.RegisterTraceServiceServer(srv, col)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	return col, lis.Addr().String()
}

func TestGRPCExporterDefaultIsPlaintextWithHeaders(t *testing.T) {
	col, addr := startGRPCCollector(t)
	err := exportOneSpan(t, exporterConfig{
		Endpoint: addr,
		Headers:  map[string]string{"x-api-key": "k1"},
	})
	if err != nil {
		t.Fatalf("plaintext gRPC export: %v", err)
	}
	col.mu.Lock()
	defer col.mu.Unlock()
	if len(col.mds) != 1 || col.mds[0].Get("x-api-key")[0] != "k1" {
		t.Fatalf("metadata = %v", col.mds)
	}
}

func TestGRPCExporterTLS(t *testing.T) {
	b := tlstest.New(t)
	col, addr := startGRPCCollector(t, grpc.Creds(credentials.NewServerTLSFromCert(&b.ServerCert)))

	err := exportOneSpan(t, exporterConfig{
		Endpoint: addr,
		TLS:      mod.TLSConfig{Enabled: true, CAFile: b.CAFile},
		Headers:  map[string]string{"x-api-key": "k2"},
	})
	if err != nil {
		t.Fatalf("gRPC export over TLS: %v", err)
	}
	col.mu.Lock()
	defer col.mu.Unlock()
	if len(col.mds) != 1 || col.mds[0].Get("x-api-key")[0] != "k2" {
		t.Fatalf("metadata = %v", col.mds)
	}
}

func TestGRPCExporterTLSRejectsUntrustedServer(t *testing.T) {
	b := tlstest.New(t)
	_, addr := startGRPCCollector(t, grpc.Creds(credentials.NewServerTLSFromCert(&b.ServerCert)))
	// INSECURE=false alone means TLS with system roots.
	err := exportOneSpan(t, exporterConfig{Endpoint: addr, Insecure: boolp(false)})
	if err == nil {
		t.Fatal("expected a certificate verification error")
	}
}

func TestGRPCExporterMutualTLS(t *testing.T) {
	b := tlstest.New(t)
	serverTLS := &tls.Config{
		Certificates: []tls.Certificate{b.ServerCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    b.CAPool,
	}
	col, addr := startGRPCCollector(t, grpc.Creds(credentials.NewTLS(serverTLS)))

	err := exportOneSpan(t, exporterConfig{
		Endpoint: addr,
		TLS:      mod.TLSConfig{Enabled: true, CAFile: b.CAFile, CertFile: b.ClientCertFile, KeyFile: b.ClientKeyFile},
	})
	if err != nil {
		t.Fatalf("gRPC export with a client certificate: %v", err)
	}
	col.mu.Lock()
	defer col.mu.Unlock()
	if len(col.mds) != 1 {
		t.Fatalf("collector saw %d exports", len(col.mds))
	}
}

func TestNewExporterBadTLSFiles(t *testing.T) {
	for _, provider := range []string{"grpc", "http"} {
		_, err := newExporter(context.Background(), exporterConfig{
			Provider: provider,
			Endpoint: "localhost:4317",
			TLS:      mod.TLSConfig{Enabled: true, CAFile: "/no/such/ca.pem"},
		})
		if err == nil || !strings.Contains(err.Error(), "tls.ca_file") {
			t.Errorf("%s: error = %v", provider, err)
		}
	}
}
