package database

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// writeLockRow is a throwaway table used as the lock target and as the table
// the callback writes to.
type writeLockRow struct {
	ID   uint `gorm:"primaryKey"`
	Name string
}

func openWriteLockDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "write_lock.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	t.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	require.NoError(t, db.AutoMigrate(&writeLockRow{}))
	return db
}

func TestWithWriteLock_CommitsCallbackWrites(t *testing.T) {
	db := openWriteLockDB(t)

	err := WithWriteLock(db, &writeLockRow{}, func(tx *gorm.DB) error {
		return tx.Create(&writeLockRow{Name: "kept"}).Error
	})
	require.NoError(t, err)

	var count int64
	require.NoError(t, db.Model(&writeLockRow{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestWithWriteLock_PropagatesCallbackError(t *testing.T) {
	db := openWriteLockDB(t)

	sentinel := errors.New("callback failed")
	err := WithWriteLock(db, &writeLockRow{}, func(tx *gorm.DB) error {
		if createErr := tx.Create(&writeLockRow{Name: "rolled back"}).Error; createErr != nil {
			return createErr
		}
		return sentinel
	})
	require.ErrorIs(t, err, sentinel)

	var count int64
	require.NoError(t, db.Model(&writeLockRow{}).Count(&count).Error)
	assert.Equal(t, int64(0), count, "a failed callback must roll back its writes")
}

func TestWithWriteLock_LockStatementErrorSkipsCallback(t *testing.T) {
	db := openWriteLockDB(t)

	// A model whose table does not exist makes the lock statement fail.
	type missingTable struct{ ID uint }
	called := false
	err := WithWriteLock(db, &missingTable{}, func(*gorm.DB) error {
		called = true
		return nil
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "acquire write lock")
	assert.False(t, called)
}
