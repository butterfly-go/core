package store

import (
	"database/sql"
	"fmt"

	"butterfly.orx.me/core/mod"
)

// Legacy global for backward compatibility.
var sqldbClients = make(map[string]*sql.DB)

// ProvideSQLDBClients creates SQL database connections from config.
func ProvideSQLDBClients(cc *mod.CoreConfig) (SQLDBClients, func(), error) {
	clients := make(SQLDBClients)
	for k, v := range cc.Store.DB {
		db, err := sql.Open("mysql", dbConfigToDSN(v))
		if err != nil {
			for _, d := range clients {
				_ = d.Close()
			}
			return nil, nil, err
		}
		clients[k] = db
	}
	cleanup := func() {
		for _, d := range clients {
			_ = d.Close()
		}
	}
	return clients, cleanup, nil
}

// SetLegacySQLDBClients populates the legacy global.
func SetLegacySQLDBClients(clients SQLDBClients) {
	sqldbClients = map[string]*sql.DB(clients)
}

// GetSQLDB returns a SQL database by name from the legacy global.
func GetSQLDB(k string) *sql.DB {
	return sqldbClients[k]
}

func dbConfigToDSN(v mod.DBConfig) string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local", v.User, v.Password, v.Host, v.Port, v.DBName)
}
