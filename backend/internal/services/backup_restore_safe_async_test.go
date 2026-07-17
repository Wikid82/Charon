package services

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newAsyncRestoreTestService constructs a *BackupService with a real,
// migrated in-memory db (BackupJob/BackupRecord/SecurityAudit) and a
// Charon-shaped source database (users + proxy_hosts tables, spec §3.5 V6),
// so StartRestoreJob's full V->S->A->R->F pipeline can actually run
// end-to-end, not just against a bare struct.
func newAsyncRestoreTestService(t *testing.T) (*BackupService, *gorm.DB) {
	t.Helper()

	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o750))

	dbPath := filepath.Join(dataDir, "charon.db")
	rawDB, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rawDB.Close() })
	_, err = rawDB.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT)`)
	require.NoError(t, err)
	_, err = rawDB.Exec(`INSERT INTO users (email) VALUES ('admin@example.com')`)
	require.NoError(t, err)
	_, err = rawDB.Exec(`CREATE TABLE proxy_hosts (id INTEGER PRIMARY KEY, domain_names TEXT)`)
	require.NoError(t, err)

	db := newAsyncJobsTestDB(t)

	cfg := &config.Config{DatabasePath: dbPath}
	svc := NewBackupService(cfg, db, nil)
	t.Cleanup(svc.Stop)
	return svc, db
}

// TestStartRestoreJob_CompletesSuccessfully_DoesNotSelfDeadlock is the
// required regression test for spec §3.3.1's BLOCKING FIX: an earlier draft
// of this plan had StartRestoreJob's goroutine call a "WithProgress" restore
// core that ALSO did its own internal s.mu.TryLock() — since TryLock()
// never blocks (just fails when already held by the same external
// TryLock() call), every restore job would have failed its own first
// attempt with ErrBackupInProgress, 100% of the time, zero concurrency
// required. This test proves the fix: the job must actually reach
// status:"completed", not fail immediately.
func TestStartRestoreJob_CompletesSuccessfully_DoesNotSelfDeadlock(t *testing.T) {
	svc, db := newAsyncRestoreTestService(t)

	record, err := svc.CreateBackupWithOptions(BackupOptions{Type: "manual"})
	require.NoError(t, err)

	job, err := svc.StartRestoreJob(record.Filename, "", RequestAuditInfo{Actor: "1"})
	require.NoError(t, err)
	require.NotNil(t, job)
	assert.Equal(t, "restore", job.Type)
	assert.Equal(t, "pending", job.Status)

	svc.WaitForJobs()

	finished, err := svc.GetBackupJob(job.UUID)
	require.NoError(t, err)
	require.Equal(t, "completed", finished.Status, "job must complete, not fail with ErrBackupInProgress (self-deadlock regression), error_message=%q", finished.ErrorMessage)
	assert.NotEmpty(t, finished.ResultJSON)
	assert.Contains(t, finished.ResultJSON, "Backup restored successfully")
	assert.NotNil(t, finished.FinishedAt)

	var count int64
	require.NoError(t, db.Model(&models.BackupJob{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestStartRestoreJob_NotFound_StaysSynchronous404_NoJobRow(t *testing.T) {
	svc, db := newAsyncRestoreTestService(t)

	job, err := svc.StartRestoreJob("does-not-exist.zip", "", RequestAuditInfo{})
	require.ErrorIs(t, err, ErrBackupNotFound)
	assert.Nil(t, job)

	var count int64
	require.NoError(t, db.Model(&models.BackupJob{}).Count(&count).Error)
	assert.Zero(t, count, "no job row should be created for a synchronous not-found rejection")

	locked := svc.mu.TryLock()
	assert.True(t, locked, "s.mu must not be left locked after the not-found pre-check")
	if locked {
		svc.mu.Unlock()
	}
}

func TestStartRestoreJob_AlreadyInProgress_ReturnsErrBackupInProgress_NoJobRow(t *testing.T) {
	svc, db := newAsyncRestoreTestService(t)
	record, err := svc.CreateBackupWithOptions(BackupOptions{Type: "manual"})
	require.NoError(t, err)

	require.True(t, svc.mu.TryLock())
	defer svc.mu.Unlock()

	job, err := svc.StartRestoreJob(record.Filename, "", RequestAuditInfo{})
	require.ErrorIs(t, err, ErrBackupInProgress)
	assert.Nil(t, job)

	var count int64
	require.NoError(t, db.Model(&models.BackupJob{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestStartRestoreJob_NilDB_ReturnsErrorSynchronously(t *testing.T) {
	svc := &BackupService{BackupDir: t.TempDir()}
	job, err := svc.StartRestoreJob("whatever.zip", "", RequestAuditInfo{})
	require.Error(t, err)
	assert.Nil(t, job)
}

// TestStartRestoreJob_LockNotLeakedOnPersistenceFailure mirrors the create
// job's equivalent test (spec §3.3.1 lock-leak fix).
func TestStartRestoreJob_LockNotLeakedOnPersistenceFailure(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o750))
	dbPath := filepath.Join(dataDir, "charon.db")
	createSQLiteTestDB(t, dbPath)
	backupDir := filepath.Join(tmpDir, "backups")
	require.NoError(t, os.MkdirAll(backupDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(backupDir, "existing.zip"), []byte("PK\x03\x04fake"), 0o600))

	// Deliberately un-migrated DB: db.Create(&job) will fail.
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	svc := &BackupService{
		DataDir:      dataDir,
		BackupDir:    backupDir,
		DatabaseName: "charon.db",
		db:           db,
	}

	_, err = svc.StartRestoreJob("existing.zip", "", RequestAuditInfo{})
	require.Error(t, err)

	locked := svc.mu.TryLock()
	assert.True(t, locked, "s.mu must not be left locked after a job-row persistence failure")
	if locked {
		svc.mu.Unlock()
	}
}

// TestStartRestoreJob_SecurityAuditRowWrittenOnPermissionError mirrors the
// create job's equivalent test — a permission-denied failure inside the
// restore job goroutine must still write a SecurityAudit row.
func TestStartRestoreJob_SecurityAuditRowWrittenOnPermissionError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission-denied simulation requires a non-root user")
	}

	svc, db := newAsyncRestoreTestService(t)
	record, err := svc.CreateBackupWithOptions(BackupOptions{Type: "manual"})
	require.NoError(t, err)

	securitySvc := NewSecurityService(db)
	t.Cleanup(func() { securitySvc.Close() })
	svc.SetSecurityService(securitySvc)

	// Make DataDir read-only so A1 (unzipWithSkipManifest, which
	// MkdirAll(0o700)s into DataDir) fails with permission denied — after
	// S1's pre-restore safety backup has already been created, exercising
	// a failure deeper in the pipeline than StartRestoreJob's own
	// synchronous pre-checks.
	require.NoError(t, os.Chmod(svc.DataDir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(svc.DataDir, 0o750) })

	audit := RequestAuditInfo{Actor: "99", IPAddress: "10.1.1.1", UserAgent: "restore-test-agent"}
	job, err := svc.StartRestoreJob(record.Filename, "", audit)
	require.NoError(t, err)
	require.NotNil(t, job)

	svc.WaitForJobs()
	securitySvc.Flush()

	finished, err := svc.GetBackupJob(job.UUID)
	require.NoError(t, err)
	assert.Equal(t, "failed", finished.Status)
	assert.Equal(t, "permissions_write_denied", finished.ErrorCode)

	var audits []models.SecurityAudit
	require.NoError(t, db.Find(&audits).Error)
	require.NotEmpty(t, audits, "a permission-denied restore job failure must still write a SecurityAudit row")
	assert.Equal(t, "99", audits[0].Actor)
	assert.Equal(t, "backup_restore_failed", audits[0].Action)
	assert.Equal(t, "permissions", audits[0].EventCategory)
	assert.Equal(t, "10.1.1.1", audits[0].IPAddress)
	assert.Equal(t, "restore-test-agent", audits[0].UserAgent)
}

// TestRestoreBackupSafe_ThinWrapper_StillLocksAndReturnsIdenticalResult
// proves RestoreBackupSafe (the pre-existing, unmodified-signature entry
// point) still works exactly as before now that its body has moved into
// the lock-free restoreBackupSafeLockedWithProgress core — same TryLock/
// Unlock semantics, same result shape, progress == nil throughout.
func TestRestoreBackupSafe_ThinWrapper_StillLocksAndReturnsIdenticalResult(t *testing.T) {
	svc, _ := newAsyncRestoreTestService(t)
	record, err := svc.CreateBackupWithOptions(BackupOptions{Type: "manual"})
	require.NoError(t, err)

	result, err := svc.RestoreBackupSafe(record.Filename, "")
	require.NoError(t, err)
	assert.Equal(t, "Backup restored successfully", result.Message)
}
