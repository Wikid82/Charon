package services

import (
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/database"
)

// WithWriteLock delegates to database.WithWriteLock. The helper lives in the
// database package so non-service packages can use it without an import cycle.
func WithWriteLock(db *gorm.DB, lockModel any, fn func(tx *gorm.DB) error) error {
	return database.WithWriteLock(db, lockModel, fn)
}
