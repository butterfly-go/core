package store

import (
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"

	"butterfly.orx.me/core/internal/config"
	"butterfly.orx.me/core/mod"
)

var (
	sqldbClients = make(map[string]*sql.DB)
)

func InitSQLDB() error {
	config := config.CoreConfig().Store.DB
	for k, v := range config {
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
	driver, dsn := buildDriverDSN(v)
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return err
	}
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
