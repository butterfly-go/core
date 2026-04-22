package sqldb

import (
	"database/sql"
)

var dbs map[string]*sql.DB

// Set sets the SQL database map. Called by the app during initialization.
func Set(d map[string]*sql.DB) {
	dbs = d
}

// GetDB returns a SQL database by name.
func GetDB(name string) *sql.DB {
	return dbs[name]
}
