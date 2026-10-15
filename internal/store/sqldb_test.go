package store

import "testing"

func TestDBSystemName(t *testing.T) {
	tests := map[string]string{
		"pgx":     "postgresql",
		"mysql":   "mysql",
		"unknown": "unknown",
	}
	for driver, want := range tests {
		got := dbSystemName(driver)
		if string(got.Key) != "db.system.name" {
			t.Fatalf("%s: key = %q", driver, got.Key)
		}
		if got.Value.AsString() != want {
			t.Fatalf("%s: value = %q, want %q", driver, got.Value.AsString(), want)
		}
	}
}
