package arg

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArgString(t *testing.T) {
	os.Setenv("BUTTERFLY_TRACING_ENDPOINT", "otel")
	os.Setenv("BUTTERFLY_CONFIG_CONSUL_ADDRESS", "consul:8500")
	assert.Equal(t, String("tracing-endpoint"), "otel")
	assert.Equal(t, String("config.consul.address"), "consul:8500")
}
