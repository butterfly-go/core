package redis

import (
	"testing"

	goredis "github.com/redis/go-redis/v9"
)

func TestSetAndGetClient(t *testing.T) {
	client := goredis.NewClient(&goredis.Options{Addr: "localhost:6379"})
	Set(map[string]*goredis.Client{"default": client})

	if got := GetClient("default"); got != client {
		t.Fatal("expected the same client instance back")
	}
	if got := GetClient("nonexistent"); got != nil {
		t.Fatalf("expected nil for missing key, got %v", got)
	}
}

func TestGetClient_BeforeSet(t *testing.T) {
	// Reset to nil
	Set(nil)
	if got := GetClient("any"); got != nil {
		t.Fatalf("expected nil before Set, got %v", got)
	}
}
