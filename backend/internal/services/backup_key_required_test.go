package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestComputeEncryptionKeyRequired_ClearedPassphraseDoesNotRequireKey checks that a
// backup passphrase setting cleared to an empty value no longer makes the restore
// warning ask for the encryption key, while a stored passphrase still does.
func TestComputeEncryptionKeyRequired_ClearedPassphraseDoesNotRequireKey(t *testing.T) {
	db := newSecretWriteDB(t)
	require.NoError(t, db.Exec("CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT)").Error)
	const key = "backup.encryption_passphrase_enc"
	svc := &BackupService{db: db}

	require.NoError(t, db.Exec("INSERT INTO settings (key, value) VALUES (?, ?)", key, "").Error)
	assert.False(t, svc.computeEncryptionKeyRequired(), "a cleared passphrase needs no key")

	require.NoError(t, db.Exec("UPDATE settings SET value = ? WHERE key = ?", "ciphertext", key).Error)
	assert.True(t, svc.computeEncryptionKeyRequired(), "a stored passphrase needs the key")
}
