package store

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"strconv"

	"github.com/XSAM/otelsql"
	_ "github.com/jackc/pgx/v5/stdlib"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"

	"butterfly.orx.me/core/internal/config"
	"butterfly.orx.me/core/internal/log"
	"butterfly.orx.me/core/internal/redact"
	"butterfly.orx.me/core/mod"
)

var (
	sqldbClients = make(map[string]*sql.DB)
)

func InitSQLDB() error {
	cfg := config.CoreConfig().Store.DB
	for _, k := range sortedKeys(cfg) {
		if err := setupSQLDB(k, cfg[k]); err != nil {
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
	// The DSN carries the password. It is never logged; errors from the
	// driver are scrubbed because they sometimes quote it.
	secrets := []string{v.Password, dsn}
	db, err := otelsql.Open(driver, dsn,
		otelsql.WithAttributes(
			dbSystemName(driver),
			semconv.DBNamespace(v.DBName),
		),
	)
	if err != nil {
		err = redact.Error(err, secrets...)
		logger.Error("open sql db failed", "name", k, "driver", driver, "host", v.Host, "port", v.Port, "database", v.DBName, "error", err.Error())
		return fmt.Errorf("open sql db %q: %w", k, err)
	}

	if _, err := otelsql.RegisterDBStatsMetrics(db); err != nil {
		logger.Error("register sql db stats metrics failed", "name", k, "driver", driver, "database", v.DBName, "error", err.Error())
		_ = db.Close()
		return fmt.Errorf("register sql db stats metrics %q: %w", k, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	// Ping is best-effort: sql.Open never connects eagerly, so an unreachable
	// database must not prevent the service from starting.
	if err := db.PingContext(ctx); err != nil {
		logger.Warn("ping sql db failed", "name", k, "driver", driver, "host", v.Host, "port", v.Port, "database", v.DBName, "error", redact.Error(err, secrets...).Error())
	}

	logger.Info("initialize sql db", "name", k, "driver", driver, "host", v.Host, "port", v.Port, "database", v.DBName)
	sqldbClients[k] = db
	return nil
}

// dbSystemName maps a database/sql driver name to the OTel db.system.name attribute.
func dbSystemName(driver string) attribute.KeyValue {
	switch driver {
	case "pgx":
		return semconv.DBSystemNamePostgreSQL
	case "mysql":
		return semconv.DBSystemNameMySQL
	default:
		return semconv.DBSystemNameKey.String(driver)
	}
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
	// url.URL escapes the user and password, so a password containing '@',
	// '/' or '#' does not corrupt the DSN.
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(v.User, v.Password),
		Host:     net.JoinHostPort(v.Host, strconv.Itoa(v.Port)),
		Path:     "/" + v.DBName,
		RawQuery: url.Values{"sslmode": {sslMode}}.Encode(),
	}
	return u.String()
}
