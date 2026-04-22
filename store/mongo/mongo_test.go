package mongo

import (
	"testing"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

func TestSetAndGetClient(t *testing.T) {
	// We can't easily create a real mongo.Client without connecting,
	// so test with nil map and empty map.
	Set(map[string]*mongo.Client{})

	if got := GetClient("nonexistent"); got != nil {
		t.Fatalf("expected nil for missing key, got %v", got)
	}
}

func TestGetClient_BeforeSet(t *testing.T) {
	Set(nil)
	if got := GetClient("any"); got != nil {
		t.Fatalf("expected nil before Set, got %v", got)
	}
}
