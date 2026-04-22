package store

import (
	"database/sql"
	"fmt"

	"butterfly.orx.me/core/mod"
)

// ProvideSQLDBClients creates SQL database connections from config.
func ProvideSQLDBClients(cc *mod.CoreConfig) (SQLDBClients, func(), error) {
	clients := make(SQLDBClients)
	for k, v := range cc.Store.DB {
		db, err := sql.Open("mysql", DBConfigToDSN(v))
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

// DBConfigToDSN converts a DBConfig to a MySQL DSN string.
func DBConfigToDSN(v mod.DBConfig) string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local", v.User, v.Password, v.Host, v.Port, v.DBName)
}
