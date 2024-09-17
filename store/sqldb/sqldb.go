package sqldb

import (
	"database/sql"

	"butterfly.orx.me/core/internal/store"
)

func GetDB(name string) *sql.DB {
	return store.GetSqlDB(name)
}
