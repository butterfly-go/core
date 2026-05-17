package gorm

import (
	// mysql driver
	"github.com/XSAM/otelsql"
	_ "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/plugin/opentelemetry/tracing"

	"butterfly.orx.me/core/internal/log"
)

type DB = gorm.DB

type Session = gorm.Session

type Tx = gorm.Tx

// NewDB
// MySQL only for now
func NewDB(dsn string) (*DB, error) {
	logger := log.CoreLogger("store.gorm")
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		logger.Error("open gorm db failed", "driver", "mysql", "error", err.Error())
		return nil, err
	}

	if err := db.Use(tracing.NewPlugin()); err != nil {
		logger.Error("install gorm tracing plugin failed", "driver", "mysql", "error", err.Error())
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		logger.Error("get gorm sql db failed", "driver", "mysql", "error", err.Error())
		return nil, err
	}
	if _, err := otelsql.RegisterDBStatsMetrics(sqlDB); err != nil {
		logger.Error("register gorm db stats metrics failed", "driver", "mysql", "error", err.Error())
		return nil, err
	}

	logger.Info("initialize gorm db", "driver", "mysql")
	return db, nil
}

func GetDB(_ string) *DB {
	return nil
}
