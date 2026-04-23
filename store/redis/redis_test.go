package redis

import (
	"testing"

	"butterfly.orx.me/core/internal/store"
	goredis "github.com/redis/go-redis/v9"
)

func TestGetClient(t *testing.T) {
	client := goredis.NewClient(&goredis.Options{Addr: "localhost:6379"})
	store.SetRedisClients(store.RedisClients{"default": client})

	if got := GetClient("default"); got != client {
		t.Fatal("expected the same client instance back")
	}
	if got := GetClient("nonexistent"); got != nil {
		t.Fatalf("expected nil for missing key, got %v", got)
	}
}
