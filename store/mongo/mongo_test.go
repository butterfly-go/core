package mongo

import (
	"testing"

	"butterfly.orx.me/core/internal/store"
)

func TestGetClient(t *testing.T) {
	store.SetMongoClients(store.MongoClients{})

	if got := GetClient("nonexistent"); got != nil {
		t.Fatalf("expected nil for missing key, got %v", got)
	}
}
