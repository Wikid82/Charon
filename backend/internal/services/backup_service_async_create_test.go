package services

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newAsyncJobsTestDB opens an in-memory gorm DB, migrated with every model
// the async-job pipeline touches (BackupJob, BackupRecord + its FK
// dependents, SecurityAudit for the audit-logging tests).
func newAsyncJobsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.BackupJob{},
		&models.BackupRecord{},
		&models.BackupRemoteCopy{},
		&models.RemoteStorageTarget{},
		&models.SecurityAudit{},
	))
	return db
}

func TestStartCreateBackupJob_CompletesSuccessfully(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o750))
	dbPath := filepath.Join(dataDir, "charon.db")
	createSQLiteTestDB(t, dbPath)

	db := newAsyncJobsTestDB(t)
	svc := &BackupService{
		DataDir:      dataDir,
		BackupDir:    filepath.Join(tmpDir, "backups"),
		DatabaseName: "charon.db",
		db:           db,
	}

	audit := RequestAuditInfo{Actor: "1", IPAddress: "127.0.0.1", UserAgent: "test"}
	job, err := svc.StartCreateBackupJob(BackupOptions{Type: "manual"}, audit)
	require.NoError(t, err)
	require.NotNil(t, job)
	assert.Equal(t, "create", job.Type)
	// The job must be returned immediately, before the goroutine's work is
	// done — status is "pending" at the moment StartCreateBackupJob returns
	// (spec §3.2.1's literal 202 response body; the goroutine flips it to
	// "running" shortly after).
	assert.Equal(t, "pending", job.Status)
	assert.NotEmpty(t, job.UUID)

	svc.WaitForJobs()

	finished, err := svc.GetBackupJob(job.UUID)
	require.NoError(t, err)
	assert.Equal(t, "completed", finished.Status)
	assert.NotEmpty(t, finished.Filename)
	assert.NotEmpty(t, finished.ResultUUID)
	assert.NotNil(t, finished.StartedAt)
	assert.NotNil(t, finished.FinishedAt)
	// Last checkpoint reached before s.db.Create(record) succeeds (spec
	// §3.3.2) — the terminal status update never touches Stage.
	assert.Equal(t, "finalizing", finished.Stage)

	var record models.BackupRecord
	require.NoError(t, db.Where("uuid = ?", finished.ResultUUID).First(&record).Error)
	assert.Equal(t, finished.Filename, record.Filename)
}

func TestStartCreateBackupJob_EncryptWithoutPassphrase_SynchronousError_NoJobRow(t *testing.T) {
	db := newAsyncJobsTestDB(t)
	svc := &BackupService{db: db}

	job, err := svc.StartCreateBackupJob(BackupOptions{Encrypt: true}, RequestAuditInfo{})
	require.Error(t, err)
	assert.Nil(t, job)

	var count int64
	require.NoError(t, db.Model(&models.BackupJob{}).Count(&count).Error)
	assert.Zero(t, count, "no job row should be created for a synchronous pre-check failure")

	// The lock must not have been acquired at all for this synchronous
	// rejection.
	locked := svc.mu.TryLock()
	assert.True(t, locked)
	if locked {
		svc.mu.Unlock()
	}
}

func TestStartCreateBackupJob_AlreadyInProgress_ReturnsErrBackupInProgress_NoJobRow(t *testing.T) {
	db := newAsyncJobsTestDB(t)
	svc := &BackupService{db: db}
	require.True(t, svc.mu.TryLock())
	defer svc.mu.Unlock()

	job, err := svc.StartCreateBackupJob(BackupOptions{Type: "manual"}, RequestAuditInfo{})
	require.ErrorIs(t, err, ErrBackupInProgress)
	assert.Nil(t, job)

	var count int64
	require.NoError(t, db.Model(&models.BackupJob{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestStartCreateBackupJob_NilDB_ReturnsErrorSynchronously(t *testing.T) {
	svc := &BackupService{}
	job, err := svc.StartCreateBackupJob(BackupOptions{Type: "manual"}, RequestAuditInfo{})
	require.Error(t, err)
	assert.Nil(t, job)
}

// TestStartCreateBackupJob_LockNotLeakedOnPersistenceFailure is the
// required regression test for spec §3.3.1's lock-leak fix: if
// s.db.Create(&job) fails immediately after s.mu.TryLock() already
// succeeded, no goroutine has been spawned yet to own Unlock() — so
// StartCreateBackupJob itself must unlock before returning.
func TestStartCreateBackupJob_LockNotLeakedOnPersistenceFailure(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o750))
	dbPath := filepath.Join(dataDir, "charon.db")
	createSQLiteTestDB(t, dbPath)

	// Deliberately un-migrated DB: db.Create(&job) will fail with "no such
	// table: backup_jobs", simulating a persistence failure that happens
	// AFTER the lock succeeds but before any goroutine exists.
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	svc := &BackupService{
		DataDir:      dataDir,
		BackupDir:    filepath.Join(tmpDir, "backups"),
		DatabaseName: "charon.db",
		db:           db,
	}

	_, err = svc.StartCreateBackupJob(BackupOptions{Type: "manual"}, RequestAuditInfo{})
	require.Error(t, err)

	// A subsequent call must succeed in acquiring the lock — it must not
	// spuriously observe ErrBackupInProgress just because the prior
	// attempt's persistence failed after TryLock succeeded.
	locked := svc.mu.TryLock()
	assert.True(t, locked, "s.mu must not be left locked after a job-row persistence failure")
	if locked {
		svc.mu.Unlock()
	}
}

// TestStartCreateBackupJob_SecurityAuditRowWrittenOnPermissionError is the
// required regression test for spec §3.3.1's security-audit fix: a
// permission-denied failure inside the job goroutine (simulated via a
// read-only BackupDir parent) must still produce a models.SecurityAudit row
// via the captured RequestAuditInfo, matching what
// respondPermissionError/logPermissionAudit produce synchronously today.
func TestStartCreateBackupJob_SecurityAuditRowWrittenOnPermissionError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission-denied simulation requires a non-root user")
	}

	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o750))
	dbPath := filepath.Join(dataDir, "charon.db")
	createSQLiteTestDB(t, dbPath)

	restrictedParent := filepath.Join(tmpDir, "restricted")
	require.NoError(t, os.MkdirAll(restrictedParent, 0o750))
	backupDir := filepath.Join(restrictedParent, "backups")

	db := newAsyncJobsTestDB(t)
	securitySvc := NewSecurityService(db)
	t.Cleanup(func() { securitySvc.Close() })

	svc := &BackupService{
		DataDir:      dataDir,
		BackupDir:    backupDir,
		DatabaseName: "charon.db",
		db:           db,
	}
	svc.SetSecurityService(securitySvc)

	require.NoError(t, os.Chmod(restrictedParent, 0o500))
	t.Cleanup(func() { _ = os.Chmod(restrictedParent, 0o750) })

	audit := RequestAuditInfo{Actor: "42", IPAddress: "10.0.0.5", UserAgent: "test-agent"}
	job, err := svc.StartCreateBackupJob(BackupOptions{Type: "manual"}, audit)
	require.NoError(t, err)
	require.NotNil(t, job)

	svc.WaitForJobs()
	securitySvc.Flush()

	finished, err := svc.GetBackupJob(job.UUID)
	require.NoError(t, err)
	assert.Equal(t, "failed", finished.Status)
	assert.Equal(t, "permissions_write_denied", finished.ErrorCode)
	assert.NotEmpty(t, finished.ErrorMessage)

	var audits []models.SecurityAudit
	require.NoError(t, db.Find(&audits).Error)
	require.NotEmpty(t, audits, "a permission-denied job failure must still write a SecurityAudit row")
	assert.Equal(t, "42", audits[0].Actor)
	assert.Equal(t, "backup_create_failed", audits[0].Action)
	assert.Equal(t, "permissions", audits[0].EventCategory)
	assert.Equal(t, "permissions", audits[0].EventCategory)
	assert.Equal(t, "10.0.0.5", audits[0].IPAddress)
	assert.Equal(t, "test-agent", audits[0].UserAgent)
	assert.Contains(t, audits[0].Details, "permissions_write_denied")
}

func TestGetBackupJob_NotFound(t *testing.T) {
	db := newAsyncJobsTestDB(t)
	svc := &BackupService{db: db}

	_, err := svc.GetBackupJob("does-not-exist")
	require.Error(t, err)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestReconcileStuckBackupJobs_MarksRunningAndPendingAsFailed(t *testing.T) {
	db := newAsyncJobsTestDB(t)

	running := models.BackupJob{Type: "create", Status: "running"}
	pending := models.BackupJob{Type: "restore", Status: "pending"}
	completed := models.BackupJob{Type: "create", Status: "completed"}
	require.NoError(t, db.Create(&running).Error)
	require.NoError(t, db.Create(&pending).Error)
	require.NoError(t, db.Create(&completed).Error)

	require.NoError(t, reconcileStuckBackupJobs(db))

	var reconciledRunning, reconciledPending, untouchedCompleted models.BackupJob
	require.NoError(t, db.First(&reconciledRunning, running.ID).Error)
	require.NoError(t, db.First(&reconciledPending, pending.ID).Error)
	require.NoError(t, db.First(&untouchedCompleted, completed.ID).Error)

	assert.Equal(t, "failed", reconciledRunning.Status)
	assert.NotEmpty(t, reconciledRunning.ErrorMessage)
	assert.NotNil(t, reconciledRunning.FinishedAt)
	assert.Equal(t, "failed", reconciledPending.Status)
	assert.Equal(t, "completed", untouchedCompleted.Status, "an already-terminal job must never be touched")
}

func TestReconcileStuckBackupJobs_NilDB_NoOp(t *testing.T) {
	assert.NoError(t, reconcileStuckBackupJobs(nil))
}

func TestCheckDatabaseIntegrity(t *testing.T) {
	t.Run("healthy database returns nil", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "healthy.db")
		createSQLiteTestDB(t, dbPath)
		assert.NoError(t, checkDatabaseIntegrity(dbPath))
	})

	t.Run("non-sqlite file returns ErrDatabaseCorrupted", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "not-a-db.db")
		require.NoError(t, os.WriteFile(dbPath, []byte("not-a-sqlite-db"), 0o600))

		err := checkDatabaseIntegrity(dbPath)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrDatabaseCorrupted)
	})
}

func TestBackupErrorCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, ""},
		{"insufficient space", ErrInsufficientSpace, "backup_insufficient_space"},
		{"passphrase required", ErrPassphraseRequired, "backup_passphrase_required"},
		{"passphrase invalid", ErrPassphraseInvalid, "backup_passphrase_invalid"},
		{"newer format", ErrNewerBackupFormat, "backup_validation_failed"},
		{"validation failed", ErrBackupValidationFailed, "backup_validation_failed"},
		{"restore unrecoverable", ErrRestoreUnrecoverable, "backup_restore_unrecoverable"},
		{"database corrupted sentinel", ErrDatabaseCorrupted, "backup_database_corrupted"},
		{"permission denied fallback", &os.PathError{Op: "open", Path: "/x", Err: syscall.EACCES}, "permissions_write_denied"},
		{"raw corruption message fallback", errors.New("database disk image is malformed"), "backup_database_corrupted"},
		{"unclassified", errors.New("something else entirely"), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, backupErrorCode(tt.err))
		})
	}
}

func TestBackupService_SetSecurityService(t *testing.T) {
	db := newAsyncJobsTestDB(t)
	svc := &BackupService{}
	assert.Nil(t, svc.securityService)

	secSvc := NewSecurityService(db)
	t.Cleanup(func() { secSvc.Close() })
	svc.SetSecurityService(secSvc)
	assert.Same(t, secSvc, svc.securityService)
}

func TestBackupService_Stop_WaitsForInFlightJobs(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o750))
	dbPath := filepath.Join(dataDir, "charon.db")
	createSQLiteTestDB(t, dbPath)

	cfg := &config.Config{DatabasePath: dbPath}
	db := newAsyncJobsTestDB(t)
	svc := NewBackupService(cfg, db, nil)

	job, err := svc.StartCreateBackupJob(BackupOptions{Type: "manual"}, RequestAuditInfo{})
	require.NoError(t, err)
	require.NotNil(t, job)

	svc.Stop()

	finished, err := svc.GetBackupJob(job.UUID)
	require.NoError(t, err)
	assert.Equal(t, "completed", finished.Status)
}
