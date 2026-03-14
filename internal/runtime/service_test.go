package runtime

import "testing"

func TestConfigKey(t *testing.T) {
	SetService("billing")
	SetConfigKey("")
	if got := ConfigKey(); got != "billing" {
		t.Fatalf("expected service fallback config key %q, got %q", "billing", got)
	}

	SetConfigKey("prod/billing")
	if got := ConfigKey(); got != "prod/billing" {
		t.Fatalf("expected explicit config key %q, got %q", "prod/billing", got)
	}
}
