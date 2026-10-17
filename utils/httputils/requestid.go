package httputils

import "github.com/gin-gonic/gin"

const (
	// RequestIDHeader carries the request id on requests and responses.
	RequestIDHeader = "X-Request-Id"
	// RequestIDKey is the gin context key under which the framework stores the
	// request id.
	RequestIDKey = "request_id"
)

// RequestID returns the id the framework assigned to the current request. It
// is the caller's X-Request-Id when that was well-formed, otherwise a freshly
// generated one. It returns "" outside a framework-served request.
func RequestID(c *gin.Context) string {
	return c.GetString(RequestIDKey)
}
