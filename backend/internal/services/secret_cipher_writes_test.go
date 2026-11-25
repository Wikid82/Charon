package services

import (
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/models"
)

func newSecretWriteDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "writes.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.DNSProvider{}, &models.DNSProviderCredential{},
		&models.RemoteStorageTarget{}, &models.SSLCertificate{}, &models.TunnelConfig{},
	))
	return db
}

// TestEncryptedStoreWrites_StaleRowSaveKeepsRotatedSecretColumns covers the
// stale full-row write case for every encrypted table: a row loaded before a
// rotation is saved again with an unrelated field changed. The cipher and the
// version must keep the rotated values.
func TestEncryptedStoreWrites_StaleRowSaveKeepsRotatedSecretColumns(t *testing.T) {
	cases := []struct {
		store  string
		create func(t *testing.T, db *gorm.DB) uint
		load   func(t *testing.T, db *gorm.DB, id uint) any
		rename func(row any)
	}{
		{
			store: "dns_providers",
			create: func(t *testing.T, db *gorm.DB) uint {
				row := models.DNSProvider{UUID: uuid.NewString(), Name: "a", ProviderType: "cloudflare"}
				require.NoError(t, db.Create(&row).Error)
				return row.ID
			},
			load: func(t *testing.T, db *gorm.DB, id uint) any {
				var row models.DNSProvider
				require.NoError(t, db.First(&row, id).Error)
				return &row
			},
			rename: func(row any) { row.(*models.DNSProvider).Name = "renamed" },
		},
		{
			store: "dns_provider_credentials",
			create: func(t *testing.T, db *gorm.DB) uint {
				row := models.DNSProviderCredential{UUID: uuid.NewString(), DNSProviderID: 1, Label: "a"}
				require.NoError(t, db.Create(&row).Error)
				return row.ID
			},
			load: func(t *testing.T, db *gorm.DB, id uint) any {
				var row models.DNSProviderCredential
				require.NoError(t, db.First(&row, id).Error)
				return &row
			},
			rename: func(row any) { row.(*models.DNSProviderCredential).Label = "renamed" },
		},
		{
			store: "remote_storage_targets",
			create: func(t *testing.T, db *gorm.DB) uint {
				row := models.RemoteStorageTarget{UUID: uuid.NewString(), Name: "a", Type: "s3"}
				require.NoError(t, db.Create(&row).Error)
				return row.ID
			},
			load: func(t *testing.T, db *gorm.DB, id uint) any {
				var row models.RemoteStorageTarget
				require.NoError(t, db.First(&row, id).Error)
				return &row
			},
			rename: func(row any) { row.(*models.RemoteStorageTarget).Name = "renamed" },
		},
		{
			store: "certificate_keys",
			create: func(t *testing.T, db *gorm.DB) uint {
				row := models.SSLCertificate{UUID: uuid.NewString(), Name: "a"}
				require.NoError(t, db.Create(&row).Error)
				return row.ID
			},
			load: func(t *testing.T, db *gorm.DB, id uint) any {
				var row models.SSLCertificate
				require.NoError(t, db.First(&row, id).Error)
				return &row
			},
			rename: func(row any) { row.(*models.SSLCertificate).Name = "renamed" },
		},
		{
			store: "tunnels",
			create: func(t *testing.T, db *gorm.DB) uint {
				row := models.TunnelConfig{Name: "a", Provider: models.ProviderCloudflare}
				require.NoError(t, db.Create(&row).Error)
				return row.ID
			},
			load: func(t *testing.T, db *gorm.DB, id uint) any {
				var row models.TunnelConfig
				require.NoError(t, db.First(&row, id).Error)
				return &row
			},
			rename: func(row any) { row.(*models.TunnelConfig).Name = "renamed" },
		},
	}

	for _, tc := range cases {
		t.Run(tc.store, func(t *testing.T) {
			db := newSecretWriteDB(t)
			store := crypto.StoreNamed(tc.store)
			id := tc.create(t, db)

			// Old ciphertext on version 1, as loaded before the rotation.
			require.NoError(t, db.Table(store.Table).Where("id = ?", id).
				UpdateColumns(map[string]any{store.CipherColumn: "old-cipher", store.VersionColumn: 1}).Error)
			stale := tc.load(t, db, id)

			// Rotation rewrites the secret and its tag.
			require.NoError(t, db.Table(store.Table).Where("id = ?", id).
				UpdateColumns(map[string]any{store.CipherColumn: "rotated-cipher", store.VersionColumn: 2}).Error)

			tc.rename(stale)
			require.NoError(t, saveRow(db, store, stale, false))

			var cipher string
			var version int
			require.NoError(t, db.Table(store.Table).Select(store.CipherColumn).Where("id = ?", id).Scan(&cipher).Error)
			require.NoError(t, db.Table(store.Table).Select(store.VersionColumn).Where("id = ?", id).Scan(&version).Error)
			assert.Equal(t, "rotated-cipher", cipher)
			assert.Equal(t, 2, version)
			assert.Equal(t, "renamed", renamedField(t, db, tc.store, id))
		})
	}
}

// renamedField returns the non-secret field the stale save changed.
func renamedField(t *testing.T, db *gorm.DB, store string, id uint) string {
	t.Helper()
	switch store {
	case "dns_provider_credentials":
		var row models.DNSProviderCredential
		require.NoError(t, db.First(&row, id).Error)
		return row.Label
	case "dns_providers":
		var row models.DNSProvider
		require.NoError(t, db.First(&row, id).Error)
		return row.Name
	case "remote_storage_targets":
		var row models.RemoteStorageTarget
		require.NoError(t, db.First(&row, id).Error)
		return row.Name
	case "certificate_keys":
		var row models.SSLCertificate
		require.NoError(t, db.First(&row, id).Error)
		return row.Name
	default:
		var row models.TunnelConfig
		require.NoError(t, db.First(&row, id).Error)
		return row.Name
	}
}

// TestSaveFields_WritesOnlyNamedColumns proves a stats-style write leaves the
// secret columns alone even when the loaded struct holds old values.
func TestSaveFields_WritesOnlyNamedColumns(t *testing.T) {
	db := newSecretWriteDB(t)
	row := models.TunnelConfig{Name: "a", Provider: models.ProviderCloudflare}
	require.NoError(t, db.Create(&row).Error)
	require.NoError(t, db.Model(&row).UpdateColumns(map[string]any{"encrypted_credentials": "rotated", "key_version": 2}).Error)

	row.EncryptedCredentials = "stale"
	row.KeyVersion = 1
	row.Name = "renamed"
	require.NoError(t, saveFields(db, &row, "Name"))

	var reloaded models.TunnelConfig
	require.NoError(t, db.First(&reloaded, row.ID).Error)
	assert.Equal(t, "renamed", reloaded.Name)
	assert.Equal(t, "rotated", reloaded.EncryptedCredentials)
	assert.Equal(t, 2, reloaded.KeyVersion)
}

// TestEncryptSecret_TypedNilRotationServiceIsNoKey: a typed nil key service must
// report "no key" as an error, never panic.
func TestEncryptSecret_TypedNilRotationServiceIsNoKey(t *testing.T) {
	var rs *crypto.RotationService
	_, _, err := encryptSecret(rs, []byte("x"))
	require.ErrorIs(t, err, ErrEncryptionFailed)

	_, err = decryptSecret(rs, "abc", 1)
	require.Error(t, err)
}

// TestSettingKeyMatchesRegistry keeps the duplicated passphrase settings key in
// step with the encrypted-settings registry in crypto.
func TestSettingKeyMatchesRegistry(t *testing.T) {
	assert.Contains(t, crypto.EncryptedSettingKeys, SettingKeyBackupEncryptionPassphraseEnc)
}
