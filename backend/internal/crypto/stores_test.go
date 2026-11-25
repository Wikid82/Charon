package crypto

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestEncryptedStores_RegistryListsEverySecretStore(t *testing.T) {
	names := map[string]EncryptedStore{}
	for _, s := range EncryptedStores {
		names[s.Name] = s
	}
	assert.Len(t, names, len(EncryptedStores), "store names must be unique")
	for _, want := range []string{
		"dns_providers", "dns_provider_credentials", "remote_storage_targets",
		"certificate_keys", "tunnels", "setting:backup.encryption_passphrase_enc",
	} {
		assert.Contains(t, names, want)
	}
	assert.Equal(t, "settings", names["setting:backup.encryption_passphrase_enc"].Table)
	assert.Equal(t, "value", names["setting:backup.encryption_passphrase_enc"].CipherColumn)
	assert.Empty(t, names["setting:backup.encryption_passphrase_enc"].VersionColumn)
}

func TestEncryptedStore_ColumnsNeverIncludeOtherFields(t *testing.T) {
	versioned := storeByNameForStoreTest(t, "tunnels")
	assert.Equal(t, []string{"encrypted_credentials", "key_version"}, versioned.Columns())

	setting := storeByNameForStoreTest(t, "setting:backup.encryption_passphrase_enc")
	assert.Equal(t, []string{"value"}, setting.Columns())
}

func TestEncryptedStore_SettingScopeFiltersOnKey(t *testing.T) {
	db := dryRunDB(t)
	store := storeByNameForStoreTest(t, "setting:backup.encryption_passphrase_enc")
	stmt := store.Scope(db.Table(store.Table)).Find(&[]map[string]any{}).Statement
	assert.Contains(t, stmt.SQL.String(), "`key`")
	assert.Contains(t, stmt.Vars, any("backup.encryption_passphrase_enc"))
}

func TestEncryptedStore_RowQueriesUseOnlyRegistryColumns(t *testing.T) {
	db := dryRunDB(t)
	store := storeByNameForStoreTest(t, "dns_providers")
	stmt := store.rowsQuery(db).Find(&[]map[string]any{}).Statement
	sql := stmt.SQL.String()
	assert.Contains(t, sql, "SELECT")
	assert.Contains(t, sql, "credentials_encrypted")
	assert.Contains(t, sql, "key_version")
	assert.NotContains(t, sql, "*")
}

// storeByNameForStoreTest returns a registry entry for a test in this file.
func storeByNameForStoreTest(t *testing.T, name string) EncryptedStore {
	t.Helper()
	return storeByName(t, name)
}

// dryRunDB returns a handle that builds SQL without executing it.
func dryRunDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "dry.db")), &gorm.Config{DryRun: true})
	require.NoError(t, err)
	return db
}
