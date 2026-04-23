package sqldb

import (
	"testing"

	"butterfly.orx.me/core/internal/store"
)

func TestGetDB(t *testing.T) {
	store.SetSQLDBClients(nil)

	if got := GetDB("any"); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}
