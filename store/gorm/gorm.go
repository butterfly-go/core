package gorm

import (
	"time"

	// mysql driver
	_ "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/plugin/opentelemetry/tracing"
)

type (
	DB     = gorm.DB
	Config = gorm.Config
	Option = gorm.Option

	Session   = gorm.Session
	Model     = gorm.Model
	DeletedAt = gorm.DeletedAt

	Association       = gorm.Association
	Statement         = gorm.Statement
	StatementModifier = gorm.StatementModifier

	Dialector                     = gorm.Dialector
	Plugin                        = gorm.Plugin
	ParamsFilter                  = gorm.ParamsFilter
	ConnPool                      = gorm.ConnPool
	SavePointerDialectorInterface = gorm.SavePointerDialectorInterface
	TxBeginner                    = gorm.TxBeginner
	ConnPoolBeginner              = gorm.ConnPoolBeginner
	TxCommitter                   = gorm.TxCommitter
	Tx                            = gorm.Tx
	Valuer                        = gorm.Valuer
	GetDBConnector                = gorm.GetDBConnector
	Rows                          = gorm.Rows
	ErrorTranslator               = gorm.ErrorTranslator

	PreparedStmtDB = gorm.PreparedStmtDB
	PreparedStmtTX = gorm.PreparedStmtTX
	ScanMode       = gorm.ScanMode

	SoftDeleteQueryClause  = gorm.SoftDeleteQueryClause
	SoftDeleteUpdateClause = gorm.SoftDeleteUpdateClause
	SoftDeleteDeleteClause = gorm.SoftDeleteDeleteClause

	ViewOption = gorm.ViewOption
	ColumnType = gorm.ColumnType
	Index      = gorm.Index
	TableType  = gorm.TableType
	Migrator   = gorm.Migrator

	Interface[T any]                  = gorm.Interface[T]
	CreateInterface[T any]            = gorm.CreateInterface[T]
	ChainInterface[T any]             = gorm.ChainInterface[T]
	SetUpdateOnlyInterface[T any]     = gorm.SetUpdateOnlyInterface[T]
	SetCreateOrUpdateInterface[T any] = gorm.SetCreateOrUpdateInterface[T]
	ExecInterface[T any]              = gorm.ExecInterface[T]
	JoinBuilder                       = gorm.JoinBuilder
	PreloadBuilder                    = gorm.PreloadBuilder
)

const (
	ScanInitialized         = gorm.ScanInitialized
	ScanUpdate              = gorm.ScanUpdate
	ScanOnConflictDoNothing = gorm.ScanOnConflictDoNothing
)

var (
	ErrRecordNotFound                = gorm.ErrRecordNotFound
	ErrInvalidTransaction            = gorm.ErrInvalidTransaction
	ErrNotImplemented                = gorm.ErrNotImplemented
	ErrMissingWhereClause            = gorm.ErrMissingWhereClause
	ErrUnsupportedRelation           = gorm.ErrUnsupportedRelation
	ErrPrimaryKeyRequired            = gorm.ErrPrimaryKeyRequired
	ErrModelValueRequired            = gorm.ErrModelValueRequired
	ErrModelAccessibleFieldsRequired = gorm.ErrModelAccessibleFieldsRequired
	ErrSubQueryRequired              = gorm.ErrSubQueryRequired
	ErrInvalidData                   = gorm.ErrInvalidData
	ErrUnsupportedDriver             = gorm.ErrUnsupportedDriver
	ErrRegistered                    = gorm.ErrRegistered
	ErrInvalidField                  = gorm.ErrInvalidField
	ErrEmptySlice                    = gorm.ErrEmptySlice
	ErrDryRunModeUnsupported         = gorm.ErrDryRunModeUnsupported
	ErrInvalidDB                     = gorm.ErrInvalidDB
	ErrInvalidValue                  = gorm.ErrInvalidValue
	ErrInvalidValueOfLength          = gorm.ErrInvalidValueOfLength
	ErrPreloadNotAllowed             = gorm.ErrPreloadNotAllowed
	ErrDuplicatedKey                 = gorm.ErrDuplicatedKey
	ErrForeignKeyViolated            = gorm.ErrForeignKeyViolated
	ErrCheckConstraintViolated       = gorm.ErrCheckConstraintViolated
)

func Open(dialector Dialector, opts ...Option) (*DB, error) {
	return gorm.Open(dialector, opts...)
}

func Expr(expr string, args ...interface{}) clause.Expr {
	return gorm.Expr(expr, args...)
}

func Scan(rows Rows, db *DB, mode ScanMode) {
	gorm.Scan(rows, db, mode)
}

func NewPreparedStmtDB(connPool ConnPool, maxSize int, ttl time.Duration) *PreparedStmtDB {
	return gorm.NewPreparedStmtDB(connPool, maxSize, ttl)
}

func G[T any](db *DB, opts ...clause.Expression) Interface[T] {
	return gorm.G[T](db, opts...)
}

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

func GetDB(_ string) *DB {
	return nil
}
