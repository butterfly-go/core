package sqldb

import (
	"database/sql"

	"butterfly.orx.me/core/internal/store"
)

// GetDB returns a SQL database by name.
func GetDB(name string) *sql.DB {
	return store.GetSQLDB(name)
}
