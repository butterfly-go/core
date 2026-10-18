package tracing

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/url"
	"sort"
	"strings"
	"sync"

	"butterfly.orx.me/core/internal/arg"
	"butterfly.orx.me/core/internal/config"
	"butterfly.orx.me/core/internal/runtime"
	"butterfly.orx.me/core/internal/tlsutil"
	"butterfly.orx.me/core/mod"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	mu             sync.Mutex
	tracerProvider *sdktrace.TracerProvider
)

// exporterConfig is how the OTLP trace exporter connects. It comes from the
// environment because tracing must be set up before the config document is
// trusted to describe it, see exporterConfigFromEnv.
type exporterConfig struct {
	Provider string
	Endpoint string
	// Insecure forces plaintext (true) or TLS (false); nil keeps the exporter
	// default: plaintext for gRPC, TLS for HTTP.
	Insecure *bool
	TLS      mod.TLSConfig
	Headers  map[string]string
}

// exporterConfigFromEnv reads:
//
//	BUTTERFLY_TRACING_PROVIDER   grpc (default) or http
//	BUTTERFLY_TRACING_ENDPOINT   host:port
//	BUTTERFLY_TRACING_INSECURE   true: plaintext, false: TLS. Unset keeps the
//	                             default: plaintext for grpc, TLS for http.
//	BUTTERFLY_TRACING_TLS_*      ENABLED, CA_FILE, CERT_FILE, KEY_FILE,
//	                             SERVER_NAME, INSECURE_SKIP_VERIFY
//	BUTTERFLY_TRACING_HEADERS    k1=v1,k2=v2 sent with every export, for
//	                             collector API keys. Values may be
//	                             percent-encoded.
func exporterConfigFromEnv() (exporterConfig, error) {
	c := exporterConfig{
		Provider: arg.String("tracing-provider"),
		Endpoint: arg.String("tracing-endpoint"),
		TLS:      arg.TLS("tracing.tls"),
	}
	switch v := strings.ToLower(arg.String("tracing-insecure")); v {
	case "":
	case "true", "1":
		t := true
		c.Insecure = &t
	case "false", "0":
		f := false
		c.Insecure = &f
	default:
		return c, fmt.Errorf("BUTTERFLY_TRACING_INSECURE must be true or false, got %q", v)
	}
	headers, err := parseHeaders(arg.String("tracing-headers"))
	if err != nil {
		return c, fmt.Errorf("BUTTERFLY_TRACING_HEADERS: %w", err)
	}
	c.Headers = headers
	return c, c.validate()
}

func (c exporterConfig) validate() error {
	if err := c.TLS.Validate(); err != nil {
		return fmt.Errorf("tracing: %w", err)
	}
	if c.TLS.Enabled && c.Insecure != nil && *c.Insecure {
		return errors.New("tracing: TLS is enabled but BUTTERFLY_TRACING_INSECURE is true")
	}
	return nil
}

// secure reports whether the connection uses TLS.
func (c exporterConfig) secure() bool {
	switch {
	case c.TLS.Enabled:
		return true
	case c.Insecure != nil:
		return !*c.Insecure
	default:
		return c.Provider == "http"
	}
}

// parseHeaders parses "k1=v1,k2=v2". Keys and values are trimmed, values may
// be percent-encoded, and empty entries are skipped.
func parseHeaders(s string) (map[string]string, error) {
	var out map[string]string
	for i, pair := range strings.Split(s, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		k, v, ok := strings.Cut(pair, "=")
		k = strings.TrimSpace(k)
		if !ok || k == "" {
			// The entry itself is not echoed: it may be a bare secret.
			return nil, fmt.Errorf("entry %d is not key=value", i+1)
		}
		val, err := url.PathUnescape(strings.TrimSpace(v))
		if err != nil {
			return nil, fmt.Errorf("header %q has an invalid percent-encoded value", k)
		}
		if out == nil {
			out = make(map[string]string)
		}
		out[k] = val
	}
	return out, nil
}

func headerNames(h map[string]string) []string {
	names := make([]string, 0, len(h))
	for k := range h {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

func NewTracerProvider(ctx context.Context) (*otlptrace.Exporter, error) {
	cfg, err := exporterConfigFromEnv()
	if err != nil {
		return nil, err
	}
	return newExporter(ctx, cfg)
}

func newExporter(ctx context.Context, cfg exporterConfig) (*otlptrace.Exporter, error) {
	switch cfg.Provider {
	case "http":
		return newHTTPTraceExporter(ctx, cfg)
	default:
		return newGRPCExporter(ctx, cfg)
	}
}

func logExporter(kind string, cfg exporterConfig) {
	slog.Info("tracing "+kind+" endpoint",
		"endpoint", cfg.Endpoint,
		"tls", cfg.secure(),
		"header_names", headerNames(cfg.Headers)) // values are secrets
}

func newHTTPTraceExporter(ctx context.Context, cfg exporterConfig) (*otlptrace.Exporter, error) {
	logExporter("http", cfg)
	opts := []otlptracehttp.Option{otlptracehttp.WithEndpoint(cfg.Endpoint)}
	if cfg.secure() {
		tlsConfig, err := tlsutil.Build(cfg.TLS)
		if err != nil {
			return nil, fmt.Errorf("tracing: %w", err)
		}
		if tlsConfig != nil {
			opts = append(opts, otlptracehttp.WithTLSClientConfig(tlsConfig))
		}
	} else {
		opts = append(opts, otlptracehttp.WithInsecure())
	}
	if len(cfg.Headers) > 0 {
		opts = append(opts, otlptracehttp.WithHeaders(cfg.Headers))
	}
	return otlptracehttp.New(ctx, opts...)
}

func newGRPCExporter(ctx context.Context, cfg exporterConfig) (*otlptrace.Exporter, error) {
	logExporter("grpc", cfg)
	creds := insecure.NewCredentials()
	if cfg.secure() {
		tlsConfig, err := tlsutil.Build(cfg.TLS)
		if err != nil {
			return nil, fmt.Errorf("tracing: %w", err)
		}
		if tlsConfig == nil {
			// TLS requested through INSECURE=false alone: system roots.
			tlsConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		}
		creds = credentials.NewTLS(tlsConfig)
	}
	conn, err := grpc.NewClient(cfg.Endpoint, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, err
	}
	opts := []otlptracegrpc.Option{otlptracegrpc.WithGRPCConn(conn)}
	if len(cfg.Headers) > 0 {
		opts = append(opts, otlptracegrpc.WithHeaders(cfg.Headers))
	}
	return otlptracegrpc.New(ctx, opts...)
}

func Init() error {
	if arg.Bool("tracing-disable") {
		slog.Info("tracing is disabled, skipping initialization")
		return nil
	}

	ctx := context.Background()
	res, err := resource.New(ctx,
		resource.WithAttributes(
			// the service name used to display traces in backends
			semconv.ServiceName(runtime.Service()),
		),
	)
	if err != nil {
		slog.Error("failed to create resource", "error", err.Error())
		return fmt.Errorf("failed to create resource: %w", err)
	}

	// Set up a trace exporter
	traceExporter, err := NewTracerProvider(ctx)
	if err != nil {
		slog.Error("failed to create trace exporter", "error", err.Error())
		return err
	}

	// Register the trace exporter with a TracerProvider, using a batch
	// span processor to aggregate spans before export.
	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
	sampler := traceSamplerFromConfig(config.CoreConfig().Otel)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sampler),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)
	otel.SetTracerProvider(tp)

	mu.Lock()
	tracerProvider = tp
	mu.Unlock()

	// set global propagator to tracecontext (the default is no-op).
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return nil
}

// Shutdown flushes buffered spans and shuts down the tracer provider, which
// also closes the exporter. It is a no-op when tracing was disabled or never
// initialized, and is safe to call more than once.
func Shutdown(ctx context.Context) error {
	mu.Lock()
	tp := tracerProvider
	tracerProvider = nil
	mu.Unlock()
	if tp == nil {
		return nil
	}
	if err := tp.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown tracer provider: %w", err)
	}
	return nil
}

func traceSamplerFromConfig(cfg mod.OtelConfig) sdktrace.Sampler {
	ratio := 1.0
	if cfg.TraceSampleRatio != nil {
		ratio = *cfg.TraceSampleRatio
	}
	switch {
	case math.IsNaN(ratio):
		// Invalid value: fall back to the default (sample everything).
		return sdktrace.AlwaysSample()
	case ratio <= 0:
		return sdktrace.NeverSample()
	case ratio >= 1:
		return sdktrace.AlwaysSample()
	default:
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))
	}
}
