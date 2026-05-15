package gorm

import (
	"database/sql"
	"fmt"
	"sync"

	// mysql driver
	_ "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/plugin/opentelemetry/tracing"
)

type DB = gorm.DB

type Session = gorm.Session

type Tx = gorm.Tx

var (
	gormDBs   = make(map[string]*DB)
	gormDBsMu sync.RWMutex
)

// RegisterFromSQL wires a *sql.DB opened by the core SQL store (MySQL driver)
// into a GORM handle named name. It is idempotent per process: the last
// registration for a given name wins. Call during application init only.
func RegisterFromSQL(name string, sqlDB *sql.DB) error {
	if name == "" {
		return fmt.Errorf("gorm: empty store name")
	}
	if sqlDB == nil {
		return fmt.Errorf("gorm: nil sql DB for %q", name)
	}
	dialector := mysql.New(mysql.Config{Conn: sqlDB})
	db, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return fmt.Errorf("gorm: open %q: %w", name, err)
	}
	if err := db.Use(tracing.NewPlugin()); err != nil {
		return fmt.Errorf("gorm: otel plugin %q: %w", name, err)
	}
	gormDBsMu.Lock()
	gormDBs[name] = db
	gormDBsMu.Unlock()
	return nil
}

// NewDB
// MySQL only for now
func NewDB(dsn string) (*DB, error) {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	if err := db.Use(tracing.NewPlugin()); err != nil {
		return nil, err
	}
	return db, nil
}

func GetDB(name string) *DB {
	gormDBsMu.RLock()
	defer gormDBsMu.RUnlock()
	return gormDBs[name]
}
