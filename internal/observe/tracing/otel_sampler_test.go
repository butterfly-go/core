package tracing

import (
	"testing"

	"butterfly.orx.me/core/mod"
)

func float64Ptr(v float64) *float64 { return &v }

func TestTraceSamplerFromConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  mod.OtelConfig
	}{
		{"nil ratio defaults to always", mod.OtelConfig{}},
		{"explicit always", mod.OtelConfig{TraceSampleRatio: float64Ptr(1)}},
		{"above one treated as always", mod.OtelConfig{TraceSampleRatio: float64Ptr(2)}},
		{"zero is never", mod.OtelConfig{TraceSampleRatio: float64Ptr(0)}},
		{"negative is never", mod.OtelConfig{TraceSampleRatio: float64Ptr(-0.5)}},
		{"fraction uses parent based ratio", mod.OtelConfig{TraceSampleRatio: float64Ptr(0.25)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := traceSamplerFromConfig(tt.cfg)
			if s == nil {
				t.Fatal("sampler is nil")
			}
		})
	}
}
