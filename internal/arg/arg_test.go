package arg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArgString(t *testing.T) {
	t.Setenv("BUTTERFLY_TRACING_ENDPOINT", "otel")
	t.Setenv("BUTTERFLY_CONFIG_CONSUL_ADDRESS", "consul:8500")
	assert.Equal(t, String("tracing-endpoint"), "otel")
	assert.Equal(t, String("config.consul.address"), "consul:8500")
}
