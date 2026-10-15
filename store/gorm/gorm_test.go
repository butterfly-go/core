package gorm

import (
	"errors"
	"testing"

	"gorm.io/gorm"
)

func TestErrorAliases(t *testing.T) {
	pairs := []struct {
		name     string
		got, org error
	}{
		{"ErrRecordNotFound", ErrRecordNotFound, gorm.ErrRecordNotFound},
		{"ErrDuplicatedKey", ErrDuplicatedKey, gorm.ErrDuplicatedKey},
		{"ErrForeignKeyViolated", ErrForeignKeyViolated, gorm.ErrForeignKeyViolated},
		{"ErrCheckConstraintViolated", ErrCheckConstraintViolated, gorm.ErrCheckConstraintViolated},
		{"ErrInvalidTransaction", ErrInvalidTransaction, gorm.ErrInvalidTransaction},
		{"ErrMissingWhereClause", ErrMissingWhereClause, gorm.ErrMissingWhereClause},
		{"ErrNotImplemented", ErrNotImplemented, gorm.ErrNotImplemented},
	}
	for _, p := range pairs {
		if !errors.Is(p.got, p.org) {
			t.Errorf("%s does not match the gorm error", p.name)
		}
	}
}

func TestTypeAliasesAreInterchangeable(t *testing.T) {
	// Aliases are identical types, so values pass to gorm APIs without casts.
	takesGorm := func(gorm.Model, gorm.DeletedAt, gorm.Config, *gorm.DB) bool { return true }
	if !takesGorm(Model{}, DeletedAt{}, Config{}, new(DB)) {
		t.Fatal("unreachable")
	}
}
