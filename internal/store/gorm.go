package store

import (
	"strings"

	"butterfly.orx.me/core/internal/config"
	gormstore "butterfly.orx.me/core/store/gorm"
)

// InitGORM registers GORM handles for MySQL entries in core store config,
// reusing the *sql.DB pools created by InitSQLDB. PostgreSQL configs are
// skipped until a GORM postgres driver is added to the module.
func InitGORM() error {
	for name, dbCfg := range config.CoreConfig().Store.DB {
		d := strings.ToLower(strings.TrimSpace(dbCfg.Driver))
		if d == "postgres" || d == "postgresql" {
			continue
		}
		sqlDB := GetSQLDB(name)
		if sqlDB == nil {
			continue
		}
		if err := gormstore.RegisterFromSQL(name, sqlDB); err != nil {
			return err
		}
	}
	return nil
}
