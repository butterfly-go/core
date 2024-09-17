package store

import (
	"database/sql"
	"fmt"

	"butterfly.orx.me/core/internal/config"
	"butterfly.orx.me/core/mod"
)

var (
	sqldbClients = make(map[string]*sql.DB)
)

func InitSqlDB() error {
	config := config.CoreConfig().Store.DB
	for k, v := range config {
		err := setupSqlDB(k, v)
		if err != nil {
			return err
		}
	}
	return nil
}

func GetSqlDB(k string) *sql.DB {
	return sqldbClients[k]
}

func setupSqlDB(k string, v mod.DBConfig) error {
	db, err := sql.Open("mysql", dbConfigToDSN(v))
	if err != nil {
		return err
	}
	sqldbClients[k] = db
	return nil
}

func dbConfigToDSN(v mod.DBConfig) string {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local", v.User, v.Password, v.Host, v.Port, v.DBName)
	return dsn
}
