package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/XSAM/otelsql"
	_ "github.com/jackc/pgx/v5/stdlib"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"

	"butterfly.orx.me/core/internal/config"
	"butterfly.orx.me/core/internal/log"
	"butterfly.orx.me/core/mod"
)

var (
	sqldbClients = make(map[string]*sql.DB)
)

func InitSQLDB() error {
	cfg := config.CoreConfig().Store.DB
	for k, v := range cfg {
		err := setupSQLDB(k, v)
		if err != nil {
			return err
		}
	}
	return nil
}

func GetSQLDB(k string) *sql.DB {
	return sqldbClients[k]
}

func setupSQLDB(k string, v mod.DBConfig) error {
	logger := log.CoreLogger("store.sqldb")
	driver, dsn := buildDriverDSN(v)
	db, err := otelsql.Open(driver, dsn,
		otelsql.WithAttributes(
			semconv.DBSystemNameKey.String(driver),
			semconv.DBNamespace(v.DBName),
		),
	)
	if err != nil {
		logger.Error("open sql db failed", "name", k, "driver", driver, "host", v.Host, "port", v.Port, "database", v.DBName, "error", err.Error())
		return err
	}

	if _, err := otelsql.RegisterDBStatsMetrics(db); err != nil {
		logger.Error("register sql db stats metrics failed", "name", k, "driver", driver, "database", v.DBName, "error", err.Error())
		return fmt.Errorf("register sql db stats metrics %q: %w", k, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Error("ping sql db failed", "name", k, "driver", driver, "host", v.Host, "port", v.Port, "database", v.DBName, "error", err.Error())
		return fmt.Errorf("ping sql db %q: %w", k, err)
	}

	logger.Info("initialize sql db", "name", k, "driver", driver, "host", v.Host, "port", v.Port, "database", v.DBName)
	sqldbClients[k] = db
	return nil
}

func buildDriverDSN(v mod.DBConfig) (driver, dsn string) {
	switch v.Driver {
	case "postgres", "postgresql":
		return "pgx", pgConfigToDSN(v)
	default:
		return "mysql", mysqlConfigToDSN(v)
	}
}

func mysqlConfigToDSN(v mod.DBConfig) string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		v.User, v.Password, v.Host, v.Port, v.DBName)
}

func pgConfigToDSN(v mod.DBConfig) string {
	sslMode := v.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		v.User, v.Password, v.Host, v.Port, v.DBName, sslMode)
}
