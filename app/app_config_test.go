package app

import "testing"

func TestConfigKey(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		want   string
	}{
		{
			name: "service only",
			config: Config{
				Service: "order",
			},
			want: "order",
		},
		{
			name: "namespace and service",
			config: Config{
				Service:   "order",
				Namespace: "prod",
			},
			want: "prod/order",
		},
		{
			name: "namespace trims slashes",
			config: Config{
				Service:   "order",
				Namespace: "/prod/",
			},
			want: "prod/order",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.config.ConfigKey(); got != tt.want {
				t.Fatalf("expected config key %q, got %q", tt.want, got)
			}
		})
	}
}
