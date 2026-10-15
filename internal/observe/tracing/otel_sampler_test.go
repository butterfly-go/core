package tracing

import (
	"math"
	"strings"
	"testing"

	"butterfly.orx.me/core/mod"
)

func float64Ptr(v float64) *float64 { return &v }

func TestTraceSamplerFromConfig(t *testing.T) {
	tests := []struct {
		name     string
		cfg      mod.OtelConfig
		wantDesc string
	}{
		{"nil ratio defaults to always", mod.OtelConfig{}, "AlwaysOnSampler"},
		{"explicit always", mod.OtelConfig{TraceSampleRatio: float64Ptr(1)}, "AlwaysOnSampler"},
		{"above one treated as always", mod.OtelConfig{TraceSampleRatio: float64Ptr(2)}, "AlwaysOnSampler"},
		{"NaN falls back to always", mod.OtelConfig{TraceSampleRatio: float64Ptr(math.NaN())}, "AlwaysOnSampler"},
		{"zero is never", mod.OtelConfig{TraceSampleRatio: float64Ptr(0)}, "AlwaysOffSampler"},
		{"negative is never", mod.OtelConfig{TraceSampleRatio: float64Ptr(-0.5)}, "AlwaysOffSampler"},
		{"fraction uses parent based ratio", mod.OtelConfig{TraceSampleRatio: float64Ptr(0.25)}, "ParentBased{root:TraceIDRatioBased{0.25}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := traceSamplerFromConfig(tt.cfg)
			if s == nil {
				t.Fatal("sampler is nil")
			}
			if got := s.Description(); !strings.HasPrefix(got, tt.wantDesc) {
				t.Fatalf("sampler description = %q, want prefix %q", got, tt.wantDesc)
			}
		})
	}
}
