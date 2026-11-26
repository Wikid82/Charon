package database

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// openSecureDeleteTestDB opens an in-memory database with one pinned connection, so the pragma state seen inside
// WithSecureDelete is the state of the connection it runs on.
func openSecureDeleteTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:secure_delete_test?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func TestWithSecureDeleteEnablesTheModeAndRestoresThePrevious(t *testing.T) {
	db := openSecureDeleteTestDB(t)
	require.NoError(t, db.Exec(secureDeletePragmas[0]).Error)

	var seen int
	err := WithSecureDelete(db, func(conn *gorm.DB) error {
		var err error
		seen, err = readSecureDelete(conn)
		return err
	})

	require.NoError(t, err)
	require.Equal(t, 1, seen, "the mode is on while fn runs")
	after, err := readSecureDelete(db)
	require.NoError(t, err)
	require.Equal(t, 0, after, "the previous mode is restored")
}

func TestWithSecureDeleteRestoresThePreviousModeWhenFnFails(t *testing.T) {
	db := openSecureDeleteTestDB(t)
	require.NoError(t, db.Exec(secureDeletePragmas[0]).Error)
	boom := errors.New("write failed")

	err := WithSecureDelete(db, func(*gorm.DB) error { return boom })

	require.ErrorIs(t, err, boom)
	after, readErr := readSecureDelete(db)
	require.NoError(t, readErr)
	require.Equal(t, 0, after)
}

func TestValidSecureDeleteModeAcceptsOnlyKnownModes(t *testing.T) {
	for mode := range secureDeletePragmas {
		got, err := validSecureDeleteMode(mode)
		require.NoError(t, err)
		require.Equal(t, mode, got)
	}
	for _, mode := range []int{-1, len(secureDeletePragmas)} {
		_, err := validSecureDeleteMode(mode)
		require.Error(t, err, mode)
	}
}
