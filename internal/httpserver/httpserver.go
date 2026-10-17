// Package httpserver builds the framework's gin engine with its default
// middleware, mirroring what grpcserver does for gRPC:
//
//   - OpenTelemetry tracing (otelgin).
//   - A request id, taken from X-Request-Id or generated, echoed on the response.
//   - A request-scoped logger in the request context (see log.FromContext).
//   - An access log line per request.
//   - Panic recovery that logs with the request id and trace id.
//
// Routes and middleware added by the application run after (inside) these.
package httpserver

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	corelog "butterfly.orx.me/core/log"
	"butterfly.orx.me/core/utils/httputils"
)

// maxRequestIDLen bounds an accepted incoming request id.
const maxRequestIDLen = 128

// NewEngine returns a gin engine with the built-in middleware installed.
// otelOpts customizes the tracing middleware; tests use it to inject providers.
func NewEngine(service string, otelOpts ...otelgin.Option) *gin.Engine {
	r := gin.New()
	// Order matters. Tracing is outermost so the span exists for everything
	// below. The access log wraps recovery so it sees the 500 that recovery
	// writes.
	r.Use(
		otelgin.Middleware(service, otelOpts...),
		requestScope(),
		accessLog(),
		recovery(),
	)
	return r
}

// requestScope assigns the request id and stores a request-scoped logger in
// the request context.
func requestScope() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(httputils.RequestIDHeader)
		if !validRequestID(id) {
			id = newRequestID()
		}
		c.Set(httputils.RequestIDKey, id)
		c.Header(httputils.RequestIDHeader, id)

		attrs := []any{
			"component", "http",
			"request_id", id,
			"http.method", c.Request.Method,
			"http.path", c.Request.URL.Path,
		}
		// The matched route template: low-cardinality, unlike the path.
		if route := c.FullPath(); route != "" {
			attrs = append(attrs, "http.route", route)
		}
		logger := slog.Default().With(attrs...)
		c.Request = c.Request.WithContext(corelog.WithLogger(c.Request.Context(), logger))
		c.Next()
	}
}

// validRequestID accepts the id formats seen in practice (UUID, ULID, hex trace
// ids, ...) and rejects anything that could forge or break a log line.
func validRequestID(id string) bool {
	if id == "" || len(id) > maxRequestIDLen {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-', r == '_', r == '.', r == ':':
		default:
			return false
		}
	}
	return true
}

func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand does not fail on supported platforms; fall back to the
		// clock rather than dropping the request.
		return fmt.Sprintf("t%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

// levelFor maps a response status to a log level. Like gRPC, successful
// requests are Debug so enabling the middleware does not flood the default
// Info level; client errors are Info and server errors are Error.
func levelFor(status int) slog.Level {
	switch {
	case status >= http.StatusInternalServerError:
		return slog.LevelError
	case status >= http.StatusBadRequest:
		return slog.LevelInfo
	default:
		return slog.LevelDebug
	}
}

// accessLog logs one line per request. It logs from a defer so that a panic
// that is deliberately propagated (see recovery) is still recorded.
func accessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		defer func() {
			status := c.Writer.Status()
			size := max(c.Writer.Size(), 0)
			ctx := c.Request.Context()
			corelog.FromContext(ctx).Log(ctx, levelFor(status), "http request",
				"http.status", status,
				"duration", time.Since(start),
				"bytes", size,
				"client_ip", c.ClientIP(),
				"user_agent", c.Request.UserAgent(),
			)
		}()
		c.Next()
	}
}

// recovery turns handler panics into a logged 500. The panic value is logged
// but never sent to the client.
func recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			p := recover()
			if p == nil {
				return
			}
			// http.ErrAbortHandler is the standard way to abort a response (it
			// is what httputil.ReverseProxy uses). Let net/http handle it.
			if err, ok := p.(error); ok && errors.Is(err, http.ErrAbortHandler) {
				panic(p)
			}

			ctx := c.Request.Context()
			logger := corelog.FromContext(ctx)

			// The client is gone; there is nobody to send a 500 to.
			if isBrokenPipe(p) {
				logger.Warn("client connection lost", "error", fmt.Sprint(p))
				c.Abort()
				return
			}

			logger.Error("panic recovered in http handler",
				"panic", fmt.Sprint(p),
				"stack", string(debug.Stack()),
			)
			span := trace.SpanFromContext(ctx)
			span.RecordError(fmt.Errorf("panic: %v", p))
			span.SetStatus(codes.Error, "panic")
			c.AbortWithStatus(http.StatusInternalServerError)
		}()
		c.Next()
	}
}

// isBrokenPipe reports whether the panic is a write to a closed connection.
func isBrokenPipe(p any) bool {
	err, ok := p.(error)
	return ok && (errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET))
}
