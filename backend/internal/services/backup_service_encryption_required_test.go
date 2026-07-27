package services

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newRemoteStorageTestService builds a BackupService with a real *gorm.DB
// (mirroring newLiveDBHardeningTestService's construction style), with
// models.RemoteStorageTarget additionally auto-migrated alongside the usual
// User/ProxyHost/BackupRecord tables, so computeEncryptionKeyRequired (M5)
// can be exercised end-to-end via CreateBackupWithOptions/ValidateBackup.
func newRemoteStorageTestService(t *testing.T) *BackupService {
	t.Helper()

	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o700))
	dbPath := filepath.Join(dataDir, "charon.db")

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.ProxyHost{}, &models.BackupRecord{}, &models.RemoteStorageTarget{}))

	require.NoError(t, db.Create(&models.User{
		UUID: uuid.NewString(), Email: "encryption-required@example.com", Name: "Encryption Required",
		Role: models.RoleUser, Enabled: true, APIKey: uuid.NewString(),
	}).Error)
	require.NoError(t, db.Create(&models.ProxyHost{
		UUID: uuid.NewString(), DomainNames: "example.com", ForwardHost: "127.0.0.1", ForwardPort: 8080,
	}).Error)

	cfg := &config.Config{DatabasePath: dbPath}
	svc := NewBackupService(cfg, db, nil)
	t.Cleanup(svc.Stop)
	return svc
}

// TestComputeEncryptionKeyRequired_PositiveAndNegative proves M5: the
// positive (true) detection branch of computeEncryptionKeyRequired
// (backup_service.go:743-745) actually returns true when a
// remote_storage_targets row exists, propagates end-to-end into
// BackupManifest.EncryptionKeyRequired via CreateBackupWithOptions, and
// survives the round trip through ValidateBackup. The negative path (no
// seeded row) is kept alongside as a regression guard anchoring the
// positive/negative pair in one readable test.
func TestComputeEncryptionKeyRequired_PositiveAndNegative(t *testing.T) {
	t.Run("true when a remote_storage_targets row exists", func(t *testing.T) {
		svc := newRemoteStorageTestService(t)

		require.NoError(t, svc.db.Create(&models.RemoteStorageTarget{
			UUID: uuid.NewString(), Name: "test", Type: "s3",
		}).Error)

		// Direct: the low-level detection branch itself.
		assert.True(t, svc.computeEncryptionKeyRequired())

		// End-to-end: CreateBackupWithOptions bakes the flag into the
		// manifest at creation time, and ValidateBackup must read it back.
		record, err := svc.CreateBackupWithOptions(BackupOptions{Type: "manual"})
		require.NoError(t, err)

		result, err := svc.ValidateBackup(record.Filename, "")
		require.NoError(t, err)
		assert.Equal(t, 2, result.FormatVersion)
		assert.True(t, result.EncryptionKeyRequired, "EncryptionKeyRequired must survive the manifest round trip via ValidateBackup")
	})

	t.Run("false when remote_storage_targets is empty", func(t *testing.T) {
		svc := newRemoteStorageTestService(t)

		assert.False(t, svc.computeEncryptionKeyRequired())

		record, err := svc.CreateBackupWithOptions(BackupOptions{Type: "manual"})
		require.NoError(t, err)

		result, err := svc.ValidateBackup(record.Filename, "")
		require.NoError(t, err)
		assert.False(t, result.EncryptionKeyRequired)
	})
}
