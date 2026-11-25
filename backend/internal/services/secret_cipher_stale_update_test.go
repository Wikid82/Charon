package services

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/crypto/cryptotest"
	"github.com/Wikid82/charon/backend/internal/models"
)

// rotateDuringUpdate simulates a rotation that lands between the service loading a
// row and saving it: before the named table's UPDATE runs, the row's cipher and tag
// are rewritten directly in the database. The writer's own UPDATE must not undo it.
func rotateDuringUpdate(t *testing.T, db *gorm.DB, table string, id uint) {
	t.Helper()
	inside := false
	const name = "test:rotate-before-stale-update"
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register(name, func(tx *gorm.DB) {
		if inside || tx.Statement.Table != table {
			return
		}
		inside = true
		defer func() { inside = false }()
		_ = tx.Session(&gorm.Session{NewDB: true}).Table(table).
			Where("id = ?", id).
			UpdateColumns(map[string]any{"key_version": 2})
		_ = tx.Session(&gorm.Session{NewDB: true}).Table(table).
			Where("id = ?", id).
			UpdateColumns(map[string]any{store(table).CipherColumn: "rotated-cipher"})
	}))
	t.Cleanup(func() { _ = db.Callback().Update().Remove(name) })
}

// store returns the registry entry for a table (test helper).
func store(table string) crypto.EncryptedStore {
	for _, s := range crypto.EncryptedStores {
		if s.Table == table {
			return s
		}
	}
	panic("no store for table " + table)
}

// TestEncryptedWriters_UpdateLeavesRotatedCiphertextAlone is the E30 check for the
// re-encrypting update paths that do not write new ciphertext. The writer changes
// only its request fields; the rotated cipher and tag survive the save.
func TestEncryptedWriters_UpdateLeavesRotatedCiphertextAlone(t *testing.T) {
	cipher := cryptoCipherForTest(t)

	t.Run("dns provider update", func(t *testing.T) {
		db := newSecretWriteDB(t)
		row := models.DNSProvider{UUID: uuid.NewString(), Name: "before", ProviderType: "cloudflare"}
		require.NoError(t, db.Create(&row).Error)
		svc := NewDNSProviderService(db, cipher)
		rotateDuringUpdate(t, db, "dns_providers", row.ID)

		newName := "after"
		_, err := svc.Update(context.Background(), row.ID, UpdateDNSProviderRequest{Name: &newName})
		require.NoError(t, err)
		assertRotatedSecret(t, db, "dns_providers", row.ID, "after")
	})

	t.Run("credential update", func(t *testing.T) {
		db := newSecretWriteDB(t)
		provider := models.DNSProvider{UUID: uuid.NewString(), Name: "p", ProviderType: "cloudflare"}
		require.NoError(t, db.Create(&provider).Error)
		credential := models.DNSProviderCredential{UUID: uuid.NewString(), DNSProviderID: provider.ID, Label: "c"}
		require.NoError(t, db.Create(&credential).Error)
		svc := NewCredentialService(db, cipher)
		rotateDuringUpdate(t, db, "dns_provider_credentials", credential.ID)

		zone := "example.com"
		_, err := svc.Update(context.Background(), provider.ID, credential.ID, UpdateCredentialRequest{ZoneFilter: &zone})
		require.NoError(t, err)
		assertRotatedSecret(t, db, "dns_provider_credentials", credential.ID, "")
		var reloaded models.DNSProviderCredential
		require.NoError(t, db.First(&reloaded, credential.ID).Error)
		assert.Equal(t, "example.com", reloaded.ZoneFilter)
	})

	t.Run("remote target update", func(t *testing.T) {
		db := newSecretWriteDB(t)
		target := models.RemoteStorageTarget{UUID: uuid.NewString(), Name: "before", Type: "s3"}
		require.NoError(t, db.Create(&target).Error)
		svc := NewBackupRemoteService(db, cipher, t.TempDir())
		rotateDuringUpdate(t, db, "remote_storage_targets", target.ID)

		newName := "after"
		_, err := svc.Update(target.UUID, &newName, nil, nil, nil)
		require.NoError(t, err)
		assertRotatedSecret(t, db, "remote_storage_targets", target.ID, "after")
	})
}

// assertRotatedSecret checks the row kept the rotated cipher and tag, and (when
// wantName is set) that the writer changed the name.
func assertRotatedSecret(t *testing.T, db *gorm.DB, table string, id uint, wantName string) {
	t.Helper()
	s := store(table)
	var cipherText string
	var version int
	require.NoError(t, db.Table(table).Select(s.CipherColumn).Where("id = ?", id).Scan(&cipherText).Error)
	require.NoError(t, db.Table(table).Select(s.VersionColumn).Where("id = ?", id).Scan(&version).Error)
	assert.Equal(t, "rotated-cipher", cipherText)
	assert.Equal(t, 2, version)
	if wantName != "" {
		var name string
		require.NoError(t, db.Table(table).Select("name").Where("id = ?", id).Scan(&name).Error)
		assert.Equal(t, wantName, name)
	}
}

// cryptoCipherForTest returns a single-key cipher for writer tests.
func cryptoCipherForTest(t *testing.T) crypto.VersionedCipher {
	t.Helper()
	key, err := crypto.GenerateNewKey()
	require.NoError(t, err)
	svc, err := crypto.NewEncryptionService(key)
	require.NoError(t, err)
	return cryptotest.SingleKey(svc)
}
