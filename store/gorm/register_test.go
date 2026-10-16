package gorm

import (
	"database/sql"
	"testing"

	_ "github.com/go-sql-driver/mysql"
)

func TestRegisterFromSQL(t *testing.T) {
	t.Run("rejects empty name", func(t *testing.T) {
		if err := RegisterFromSQL("", &sql.DB{}); err == nil {
			t.Fatal("expected error for empty name")
		}
	})

	t.Run("rejects nil sql db", func(t *testing.T) {
		if err := RegisterFromSQL("nil-db", nil); err == nil {
			t.Fatal("expected error for nil sql db")
		}
		if GetDB("nil-db") != nil {
			t.Fatal("nothing should be registered on error")
		}
	})

	t.Run("registers without connecting", func(t *testing.T) {
		// Port 1 is unreachable: registration must not dial the database.
		sqlDB, err := sql.Open("mysql", "u:p@tcp(127.0.0.1:1)/db")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = sqlDB.Close() }()

		if err := RegisterFromSQL("lazy", sqlDB); err != nil {
			t.Fatalf("RegisterFromSQL: %v", err)
		}
		db := GetDB("lazy")
		if db == nil {
			t.Fatal("GetDB returned nil after registration")
		}
		if got, err := db.DB(); err != nil || got != sqlDB {
			t.Fatalf("handle does not share the sql pool: %v", err)
		}
	})

	t.Run("unknown name returns nil", func(t *testing.T) {
		if GetDB("does-not-exist") != nil {
			t.Fatal("expected nil for unknown name")
		}
	})
}
