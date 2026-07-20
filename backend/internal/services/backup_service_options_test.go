package services

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// boolPtr returns a pointer to b — used throughout this file to build
// BackupOptions.UploadToRemote/*bool request fields (spec §12.2.2's
// "nil = default, non-nil = explicit override" convention).
func boolPtr(b bool) *bool {
	return &b
}

// newOptionsTestService builds a *BackupService backed by a real on-disk
// database (so StartCreateBackupJob's actual archive pipeline succeeds) and
// a gorm DB migrated with every model the create-backup async-job path plus
// BackupSettings touch: Setting (resolveBackupPassphrase/UpdateSettings),
// BackupJob/BackupRecord/BackupRemoteCopy/RemoteStorageTarget (the async
// job + CreateBackupWithOptions persistence path), SecurityAudit (job
// failure auditing, unused here but part of the shared migration set per
// backup_service_async_create_test.go's newAsyncJobsTestDB convention).
func newOptionsTestService(t *testing.T, enc *crypto.EncryptionService) *BackupService {
	t.Helper()

	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o750))
	dbPath := filepath.Join(dataDir, "charon.db")
	createSQLiteTestDB(t, dbPath)

	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.Setting{},
		&models.BackupJob{},
		&models.BackupRecord{},
		&models.BackupRemoteCopy{},
		&models.RemoteStorageTarget{},
		&models.SecurityAudit{},
	))

	cfg := &config.Config{DatabasePath: dbPath}
	svc := NewBackupService(cfg, db, enc)
	t.Cleanup(svc.Stop)
	return svc
}

// TestStartCreateBackupJob_UsesStoredPassphraseWhenEncryptRequestedWithoutOne
// is test 1 of spec §12.2.4: a stored, encrypted-at-rest passphrase (set via
// the real UpdateSettings encrypt round-trip, not a raw DB write) must be
// transparently reused when the Create dialog requests encryption without
// supplying its own passphrase.
func TestStartCreateBackupJob_UsesStoredPassphraseWhenEncryptRequestedWithoutOne(t *testing.T) {
	enc, err := crypto.NewEncryptionService(testEncryptionKeyBase64(t))
	require.NoError(t, err)
	svc := newOptionsTestService(t, enc)

	passphrase := "correct-horse-battery-staple"
	require.NoError(t, svc.UpdateSettings(BackupSettingsUpdate{EncryptionPassphrase: &passphrase}))

	job, err := svc.StartCreateBackupJob(BackupOptions{Encrypt: true}, RequestAuditInfo{})
	require.NoError(t, err)
	require.NotNil(t, job)

	svc.WaitForJobs()

	finished, err := svc.GetBackupJob(job.UUID)
	require.NoError(t, err)
	require.Equal(t, "completed", finished.Status, "error_message=%q error_code=%q", finished.ErrorMessage, finished.ErrorCode)

	var record models.BackupRecord
	require.NoError(t, svc.db.Where("uuid = ?", finished.ResultUUID).First(&record).Error)
	assert.True(t, record.Encrypted)
}

// TestStartCreateBackupJob_EncryptWithNoPassphraseAndNoneStored_Fails is
// test 2: with no stored passphrase and none supplied, the pre-existing
// synchronous rejection must still fire — proving resolveBackupPassphrase's
// no-op path doesn't swallow the guard.
func TestStartCreateBackupJob_EncryptWithNoPassphraseAndNoneStored_Fails(t *testing.T) {
	svc := newOptionsTestService(t, nil)

	job, err := svc.StartCreateBackupJob(BackupOptions{Encrypt: true}, RequestAuditInfo{})
	require.Error(t, err)
	assert.Nil(t, job)

	var count int64
	require.NoError(t, svc.db.Model(&models.BackupJob{}).Count(&count).Error)
	assert.Zero(t, count, "no job row should be created for a synchronous pre-check failure")
}

// TestCreateBackupWithOptions_UploadToRemoteFalse_SkipsHook is test 3: the
// synchronous legacy/scheduled path (CreateBackupWithOptions) must skip the
// remote-upload hook when UploadToRemote is an explicit false.
func TestCreateBackupWithOptions_UploadToRemoteFalse_SkipsHook(t *testing.T) {
	svc := newOptionsTestService(t, nil)

	var called atomic.Bool
	svc.SetRemoteUploadHook(func(ctx context.Context, record *models.BackupRecord) {
		called.Store(true)
	})

	record, err := svc.CreateBackupWithOptions(BackupOptions{Type: "manual", UploadToRemote: boolPtr(false)})
	require.NoError(t, err)
	require.NotNil(t, record)

	svc.uploadWG.Wait()
	assert.False(t, called.Load(), "remote-upload hook must not fire when UploadToRemote is explicitly false")
}

// TestCreateBackupWithOptions_UploadToRemoteNil_FiresHookAsBefore is test
// 4: the explicit regression guard for "nil means unchanged" on the
// CreateBackupWithOptions call site — the pre-existing unconditional-upload
// behavior must be preserved for every caller that never sets the field.
func TestCreateBackupWithOptions_UploadToRemoteNil_FiresHookAsBefore(t *testing.T) {
	svc := newOptionsTestService(t, nil)

	var called atomic.Bool
	svc.SetRemoteUploadHook(func(ctx context.Context, record *models.BackupRecord) {
		called.Store(true)
	})

	record, err := svc.CreateBackupWithOptions(BackupOptions{Type: "manual"})
	require.NoError(t, err)
	require.NotNil(t, record)

	svc.uploadWG.Wait()
	assert.True(t, called.Load(), "remote-upload hook must still fire unconditionally when UploadToRemote is nil")
}

// TestStartCreateBackupJob_UploadToRemoteFalse_SkipsHook is test 5 — the
// test that actually covers the bug fix (§12.2.1's corrected call-graph
// finding): the REAL manual-create request path is StartCreateBackupJob ->
// runCreateBackupJob, which never calls CreateBackupWithOptions at all. This
// exercises that exact path (not CreateBackupWithOptions directly) and would
// have failed against the old, unfixed implementation where
// runCreateBackupJob never fired the hook under any circumstances.
func TestStartCreateBackupJob_UploadToRemoteFalse_SkipsHook(t *testing.T) {
	svc := newOptionsTestService(t, nil)

	var called atomic.Bool
	svc.SetRemoteUploadHook(func(ctx context.Context, record *models.BackupRecord) {
		called.Store(true)
	})

	job, err := svc.StartCreateBackupJob(BackupOptions{UploadToRemote: boolPtr(false)}, RequestAuditInfo{})
	require.NoError(t, err)
	require.NotNil(t, job)

	svc.WaitForJobs()
	svc.uploadWG.Wait()

	finished, err := svc.GetBackupJob(job.UUID)
	require.NoError(t, err)
	require.Equal(t, "completed", finished.Status)
	assert.False(t, called.Load(), "remote-upload hook must not fire from the async job path when UploadToRemote is explicitly false")
}

// TestStartCreateBackupJob_UploadToRemoteNil_FiresHook is test 6 — the
// regression test proving the previously-latent "manual creates never
// upload" gap (§12.2.1) is actually closed. Before this fix, this exact
// scenario would have failed (spy never called) even though nothing about
// UploadToRemote was involved: runCreateBackupJob never called
// CreateBackupWithOptions (the hook's only call site) at all.
func TestStartCreateBackupJob_UploadToRemoteNil_FiresHook(t *testing.T) {
	svc := newOptionsTestService(t, nil)

	var called atomic.Bool
	svc.SetRemoteUploadHook(func(ctx context.Context, record *models.BackupRecord) {
		called.Store(true)
	})

	job, err := svc.StartCreateBackupJob(BackupOptions{}, RequestAuditInfo{})
	require.NoError(t, err)
	require.NotNil(t, job)

	svc.WaitForJobs()
	svc.uploadWG.Wait()

	finished, err := svc.GetBackupJob(job.UUID)
	require.NoError(t, err)
	require.Equal(t, "completed", finished.Status)
	assert.True(t, called.Load(), "remote-upload hook must fire from the async job path when UploadToRemote is nil (default true)")
}

// TestStartCreateBackupJob_CorruptedStoredPassphrase_ReturnsError is test 7
// (non-blocking finding #3 from review): a corrupted/unparseable stored
// ciphertext (bit-rot, key-rotation mismatch) must surface as a real
// synchronous error from StartCreateBackupJob via
// resolveBackupPassphrase's Decrypt failure, not silently fall through to
// "passphrase is required".
func TestStartCreateBackupJob_CorruptedStoredPassphrase_ReturnsError(t *testing.T) {
	enc, err := crypto.NewEncryptionService(testEncryptionKeyBase64(t))
	require.NoError(t, err)
	svc := newOptionsTestService(t, enc)

	// Seed corrupted ciphertext directly, bypassing UpdateSettings's normal
	// Encrypt call, to simulate bit-rot or a key-rotation mismatch.
	require.NoError(t, upsertBackupSetting(svc.db, SettingKeyBackupEncryptionPassphraseEnc, "not-valid-ciphertext", "string"))

	job, err := svc.StartCreateBackupJob(BackupOptions{Encrypt: true}, RequestAuditInfo{})
	require.Error(t, err)
	assert.Nil(t, job)

	var count int64
	require.NoError(t, svc.db.Model(&models.BackupJob{}).Count(&count).Error)
	assert.Zero(t, count, "no job row should be created when the stored passphrase fails to decrypt")
}
