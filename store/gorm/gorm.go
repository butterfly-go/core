package gorm

import (
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/plugin/opentelemetry/tracing"
)

type DB = gorm.DB

// NewDB
// MySQL only for now
func NewDB(dsn string) (*DB, error) {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	if err := db.Use(tracing.NewPlugin()); err != nil {
		return nil, err
	}
	return db, nil
}
