package sqldb

import (
	"testing"
)

func TestSetAndGetDB(t *testing.T) {
	Set(nil)
	if got := GetDB("any"); got != nil {
		t.Fatalf("expected nil before Set, got %v", got)
	}
}
