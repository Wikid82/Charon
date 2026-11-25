package crypto

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// openStoreRowsDB opens a real SQLite database for HasRows checks.
func openStoreRowsDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "has_rows.db")), &gorm.Config{})
	require.NoError(t, err)
	return db
}

// TestEncryptedStore_HasRowsIgnoresClearedSetting checks that a settings row left
// with an empty value, as a cleared passphrase does, does not mark the store in use.
func TestEncryptedStore_HasRowsIgnoresClearedSetting(t *testing.T) {
	db := openStoreRowsDB(t)
	require.NoError(t, db.Exec("CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT)").Error)
	store := storeByName(t, "setting:backup.encryption_passphrase_enc")
	const key = "backup.encryption_passphrase_enc"

	require.NoError(t, db.Exec("INSERT INTO settings (key, value) VALUES (?, ?)", key, "").Error)
	has, err := store.HasRows(db)
	require.NoError(t, err)
	assert.False(t, has, "an empty setting value is not a stored secret")

	require.NoError(t, db.Exec("UPDATE settings SET value = ? WHERE key = ?", "ciphertext", key).Error)
	has, err = store.HasRows(db)
	require.NoError(t, err)
	assert.True(t, has, "a stored setting value marks the store in use")
}

// TestEncryptedStore_HasRowsCountsEmptyTableRows checks that table stores keep
// counting a row without a ciphertext, since that row still records use of the store.
func TestEncryptedStore_HasRowsCountsEmptyTableRows(t *testing.T) {
	db := openStoreRowsDB(t)
	require.NoError(t, db.Exec("CREATE TABLE dns_providers (id INTEGER PRIMARY KEY, credentials_encrypted TEXT, key_version INTEGER)").Error)
	store := storeByName(t, "dns_providers")

	has, err := store.HasRows(db)
	require.NoError(t, err)
	assert.False(t, has, "an empty table has no rows")

	require.NoError(t, db.Exec("INSERT INTO dns_providers (credentials_encrypted, key_version) VALUES ('', 1)").Error)
	has, err = store.HasRows(db)
	require.NoError(t, err)
	assert.True(t, has)
}
