package httputils

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// TwirpHandler
type TwirpHandler interface {
	PathPrefix() string
	ServeHTTP(http.ResponseWriter, *http.Request)
}

// RegisterTwirpHandler
func RegisterTwirpHandler(m *gin.Engine, handler TwirpHandler) {
	m.Any(handler.PathPrefix()+":method", func(c *gin.Context) {
		handler.ServeHTTP(c.Writer, c.Request)
	})
}
