package arg

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArgString(t *testing.T) {
	os.Setenv("BUTTERFLY_TRACING_ENDPOINT", "otel")
	assert.Equal(t, String("tracing-endpoint"), "otel")
}
