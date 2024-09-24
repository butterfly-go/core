package httputils

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// TwirpHandler
type TwirpHandler interface {
	ServeHTTP(http.ResponseWriter, *http.Request)
}

// RegisterTwirpHandler
func RegisterTwirpHandler(m *gin.Engine, prefix string, handler TwirpHandler) {
	m.POST(prefix+":method", func(c *gin.Context) {
		handler.ServeHTTP(c.Writer, c.Request)
	})
}
