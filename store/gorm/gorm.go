package gorm

import (
	"database/sql"
	"errors"
	"fmt"
	"sync"

	// mysql driver
	"github.com/XSAM/otelsql"
	mysqldriver "github.com/go-sql-driver/mysql"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/plugin/opentelemetry/tracing"

	"butterfly.orx.me/core/internal/log"
)

// Type aliases re-exported from gorm so callers do not need to import
// gorm.io/gorm directly. Only the commonly used surface is exported on
// purpose; add more here when a real use case appears.
type (
	// DB is gorm.DB.
	DB = gorm.DB
	// Config is gorm.Config.
	Config = gorm.Config
	// Option is gorm.Option.
	Option = gorm.Option
	// Session is gorm.Session.
	Session = gorm.Session
	// Tx is gorm.Tx.
	Tx = gorm.Tx

	// Model is gorm.Model (ID, CreatedAt, UpdatedAt, DeletedAt).
	Model = gorm.Model
	// DeletedAt is gorm.DeletedAt, used for soft delete.
	DeletedAt = gorm.DeletedAt

	// Dialector is gorm.Dialector.
	Dialector = gorm.Dialector
	// Plugin is gorm.Plugin.
	Plugin = gorm.Plugin

	// Interface is the generics API returned by G.
	Interface[T any] = gorm.Interface[T]
)

// Common gorm errors, for use with errors.Is.
var (
	ErrRecordNotFound          = gorm.ErrRecordNotFound
	ErrDuplicatedKey           = gorm.ErrDuplicatedKey
	ErrForeignKeyViolated      = gorm.ErrForeignKeyViolated
	ErrCheckConstraintViolated = gorm.ErrCheckConstraintViolated
	ErrInvalidTransaction      = gorm.ErrInvalidTransaction
	ErrMissingWhereClause      = gorm.ErrMissingWhereClause
	ErrNotImplemented          = gorm.ErrNotImplemented
)

// Expr builds a raw SQL expression, see gorm.Expr.
func Expr(expr string, args ...any) clause.Expr {
	return gorm.Expr(expr, args...)
}

// G returns the generics API for type T, see gorm.G.
// Requires gorm >= 1.30.
func G[T any](db *DB, opts ...clause.Expression) Interface[T] {
	return gorm.G[T](db, opts...)
}

// NOTE: gorm.Open is intentionally not re-exported. Use NewDB, which installs
// the tracing plugin and registers DB stats metrics.

var (
	gormDBs   = make(map[string]*DB)
	gormDBsMu sync.RWMutex
)

// RegisterFromSQL wires a *sql.DB opened by the core SQL store (MySQL driver)
// into a GORM handle named name. It is idempotent per process: the last
// registration for a given name wins. Call during application init only.
func RegisterFromSQL(name string, sqlDB *sql.DB) error {
	if name == "" {
		return errors.New("gorm: empty store name")
	}
	if sqlDB == nil {
		return fmt.Errorf("gorm: nil sql DB for %q", name)
	}
	// Do not touch the network here: the pool is shared with the core SQL
	// store, whose connectivity check is best-effort, so an unreachable
	// database must not prevent the service from starting.
	dialector := mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true})
	db, err := gorm.Open(dialector, &gorm.Config{DisableAutomaticPing: true})
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

// NewDB opens a MySQL connection with tracing and DB stats metrics enabled.
// MySQL only for now.
func NewDB(dsn string) (*DB, error) {
	logger := log.CoreLogger("store.gorm")
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		logger.Error("open gorm db failed", "driver", "mysql", "error", err.Error())
		return nil, err
	}

	if err := db.Use(tracing.NewPlugin()); err != nil {
		logger.Error("install gorm tracing plugin failed", "driver", "mysql", "error", err.Error())
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		logger.Error("get gorm sql db failed", "driver", "mysql", "error", err.Error())
		return nil, err
	}
	attrs := []attribute.KeyValue{semconv.DBSystemNameMySQL}
	if cfg, perr := mysqldriver.ParseDSN(dsn); perr == nil && cfg.DBName != "" {
		attrs = append(attrs, semconv.DBNamespace(cfg.DBName))
	}
	if _, err := otelsql.RegisterDBStatsMetrics(sqlDB, otelsql.WithAttributes(attrs...)); err != nil {
		logger.Error("register gorm db stats metrics failed", "driver", "mysql", "error", err.Error())
		return nil, err
	}

	logger.Info("initialize gorm db", "driver", "mysql")
	return db, nil
}

// GetDB returns the GORM handle registered for the store entry name, or nil
// if there is none (unknown name, or a non-MySQL entry).
func GetDB(name string) *DB {
	gormDBsMu.RLock()
	defer gormDBsMu.RUnlock()
	return gormDBs[name]
}
