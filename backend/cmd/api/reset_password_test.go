package main

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestResetUserPassword_MovesSessionVersionAndSetsPassword(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+filepath.Join(t.TempDir(), "reset.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.User{}))

	user := models.User{Email: "admin@example.com", Name: "Admin", Enabled: true, SessionVersion: 3}
	require.NoError(t, user.SetPassword("old-password-1"))
	require.NoError(t, db.Create(&user).Error)

	require.NoError(t, resetUserPassword(db, "admin@example.com", "new-password-1"))

	var after models.User
	require.NoError(t, db.First(&after, user.ID).Error)
	require.Equal(t, uint(4), after.SessionVersion, "reset must end sessions issued before it")
	require.True(t, after.CheckPassword("new-password-1"))
	require.False(t, after.CheckPassword("old-password-1"))
}

func TestResetUserPassword_UnknownEmail(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+filepath.Join(t.TempDir(), "reset-missing.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.User{}))

	require.Error(t, resetUserPassword(db, "nobody@example.com", "new-password-1"))
}

func TestResetUserPassword_RejectsUnhashablePassword(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+filepath.Join(t.TempDir(), "reset-long.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.User{}))
	user := models.User{Email: "admin@example.com", Name: "Admin", Enabled: true}
	require.NoError(t, db.Create(&user).Error)

	// bcrypt refuses passwords longer than 72 bytes.
	err = resetUserPassword(db, "admin@example.com", strings.Repeat("p", 73))
	require.ErrorContains(t, err, "hash password")
}

func TestResetUserPassword_SaveFailureIsReturned(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+filepath.Join(t.TempDir(), "reset-save.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.User{}))
	user := models.User{Email: "admin@example.com", Name: "Admin", Enabled: true, SessionVersion: 1}
	require.NoError(t, db.Create(&user).Error)

	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("test:fail_update", func(tx *gorm.DB) {
		_ = tx.AddError(errors.New("forced update failure"))
	}))

	err = resetUserPassword(db, "admin@example.com", "new-password-1")
	require.ErrorContains(t, err, "save password")

	var after models.User
	require.NoError(t, db.First(&after, user.ID).Error)
	require.Equal(t, uint(1), after.SessionVersion, "a failed reset must not move the session version")
}
