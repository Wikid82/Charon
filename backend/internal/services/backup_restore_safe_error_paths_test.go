package services

import (
	"archive/zip"
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

// --- RestoreBackupSafe: live-db-attached rehydrate FAILURE branches (spec
// §3.5 A2/F3) — distinct from newLiveDBHardeningTestService's happy-path
// tests in backup_restore_safe_coverage_test.go, which only exercise a
// successful rehydrate. Closing the live db's underlying *sql.DB before
// calling RestoreBackupSafe forces every db.Exec/db.Model call from that
// point on to fail with a non-transient "sql: database is closed" error:
// RehydrateLiveDatabase fails on its very first write, the retry loop's
// non-transient branch breaks immediately (no retries), the "rehydrate
// failed" warning logs, F3's pending-restore file is written successfully
// (DataDir/the extracted temp file are both untouched by the closed db),
// and the BackupRecord status update that follows a written pending file
// fails too — exercising all of that branch's own error-warn path.

// newLiveDBRestoreErrorTestService mirrors
// newLiveDBHardeningTestService (backup_restore_safe_coverage_test.go) but
// is kept local to this file so this file's intent (forcing rehydrate
// failure paths) is self-contained.
func newLiveDBRestoreErrorTestService(t *testing.T) (*BackupService, *gorm.DB) {
	t.Helper()

	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o700))
	dbPath := filepath.Join(dataDir, "charon.db")

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("PRAGMA journal_mode=WAL").Error)
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.ProxyHost{}, &models.BackupRecord{}))
	require.NoError(t, db.Create(&models.User{
		UUID: uuid.NewString(), Email: "restore-error@example.com", Name: "Restore Error",
		Role: models.RoleUser, Enabled: true, APIKey: uuid.NewString(),
	}).Error)
	require.NoError(t, db.Create(&models.ProxyHost{
		UUID: uuid.NewString(), DomainNames: "example.com", ForwardHost: "127.0.0.1", ForwardPort: 8080,
	}).Error)

	cfg := &config.Config{DatabasePath: dbPath}
	svc := NewBackupService(cfg, db, nil)
	t.Cleanup(svc.Stop)
	return svc, db
}

// TestRestoreBackupSafe_RehydrateFails_NonTransient_WritesPendingFileAndWarns
// proves the full A2/F3 failure path (spec §3.5): a non-transient rehydrate
// error breaks the retry loop on its first attempt (backup_restore_safe.go
// ~280-281), logs a warning instead of failing the overall restore (~286-
// 287), still successfully persists the durable pending-restore file since
// F3 doesn't depend on the live db connection (~292-298), and then attempts
// (and here, fails, since the same closed connection is used) to mark the
// BackupRecord restore_pending (~300-304) — logging its own warning rather
// than propagating that failure to the caller, since a pending swap must
// never be reported as failed just because a best-effort status label
// couldn't be written.
func TestRestoreBackupSafe_RehydrateFails_NonTransient_WritesPendingFileAndWarns(t *testing.T) {
	svc, db := newLiveDBRestoreErrorTestService(t)

	record, err := svc.CreateBackupWithOptions(BackupOptions{Type: "manual"})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	result, err := svc.RestoreBackupSafe(record.Filename, "")
	require.NoError(t, err, "a rehydrate/status-update failure must degrade to restart_required, not fail the whole restore")
	require.NotNil(t, result)
	assert.False(t, result.LiveRehydrateApplied)
	assert.True(t, result.RestartRequired)
	assert.True(t, result.DatabaseSwapPending, "F3's pending-restore file must still be written even though the live db connection is dead")
	assert.Equal(t, "Backup restored; the database will finish restoring on the next process restart", result.Message)
}

// --- writePendingRestoreFile: direct unit tests of every error branch,
// using the same directory/nonexistent-path techniques already proven for
// cmd/pending-restore-harness (which performs the identical file operation
// in a real separate process — see
// backend/internal/database/pending_restore_process_test.go). ---

func TestWritePendingRestoreFile_SourceDoesNotExist(t *testing.T) {
	dir := t.TempDir()
	svc := &BackupService{DataDir: dir, DatabaseName: "charon.db"}

	err := svc.writePendingRestoreFile(filepath.Join(dir, "no-such-source.db"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open validated restore database")
}

func TestWritePendingRestoreFile_DestinationCannotBeCreated(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "source.db")
	require.NoError(t, os.WriteFile(sourcePath, []byte("data"), 0o600))

	// DataDir points at a path whose parent doesn't exist, so
	// DataDir/charon.db.pending-restore can never be created.
	svc := &BackupService{DataDir: filepath.Join(dir, "no-such-parent"), DatabaseName: "charon.db"}

	err := svc.writePendingRestoreFile(sourcePath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create pending-restore file")
}

func TestWritePendingRestoreFile_CopyFails(t *testing.T) {
	dir := t.TempDir()
	// A directory opens successfully via os.Open but fails on Read, so
	// io.Copy fails distinctly from the "open" and "create" branches above.
	sourceDir := filepath.Join(dir, "source-is-a-dir")
	require.NoError(t, os.Mkdir(sourceDir, 0o755))

	svc := &BackupService{DataDir: dir, DatabaseName: "charon.db"}

	err := svc.writePendingRestoreFile(sourceDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "write pending-restore file")
}

// --- readBackupManifest: the malformed-JSON branch, distinct from the
// already-tested "no manifest.json entry at all" (legacy) and "archive
// isn't a zip at all" cases. ---

func TestReadBackupManifest_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "malformed_manifest.zip")
	out, err := os.Create(zipPath) // #nosec G304 -- test-controlled path
	require.NoError(t, err)
	w := zip.NewWriter(out)

	mw, err := w.Create("manifest.json")
	require.NoError(t, err)
	_, err = mw.Write([]byte("{not valid json"))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	require.NoError(t, out.Close())

	manifest, legacy, err := readBackupManifest(zipPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse manifest.json")
	assert.Nil(t, manifest)
	assert.False(t, legacy)
}

// --- verifyManifestChecksums: direct unit tests of every branch, since
// RestoreBackupSafe/ValidateBackup only exercise this indirectly through a
// full archive round trip. ---

func TestVerifyManifestChecksums_ArchiveNotAZip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "not-a-zip.bin")
	require.NoError(t, os.WriteFile(path, []byte("garbage"), 0o600))

	err := verifyManifestChecksums(path, &BackupManifest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open archive")
}

func TestVerifyManifestChecksums_EntryMissingFromArchive(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "missing_entry.zip")
	out, err := os.Create(zipPath) // #nosec G304 -- test-controlled path
	require.NoError(t, err)
	w := zip.NewWriter(out)
	require.NoError(t, w.Close())
	require.NoError(t, out.Close())

	manifest := &BackupManifest{Contents: []ManifestEntry{{Path: "charon.db", Size: 10, SHA256: "deadbeef"}}}
	err = verifyManifestChecksums(zipPath, manifest)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `manifest entry "charon.db" is missing from the archive`)
}

func TestVerifyManifestChecksums_EntryExceedsDeclaredSizePlusSlack(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "oversized_entry.zip")
	out, err := os.Create(zipPath) // #nosec G304 -- test-controlled path
	require.NoError(t, err)
	w := zip.NewWriter(out)
	fw, err := w.Create("charon.db")
	require.NoError(t, err)
	// Actual content is far larger than declared Size + manifestEntrySizeSlack.
	_, err = fw.Write(make([]byte, manifestEntrySizeSlack+4096))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	require.NoError(t, out.Close())

	manifest := &BackupManifest{Contents: []ManifestEntry{{Path: "charon.db", Size: 1, SHA256: "irrelevant"}}}
	err = verifyManifestChecksums(zipPath, manifest)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds its declared size plus slack")
}

func TestVerifyManifestChecksums_SizeMismatch(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "size_mismatch.zip")
	out, err := os.Create(zipPath) // #nosec G304 -- test-controlled path
	require.NoError(t, err)
	w := zip.NewWriter(out)
	fw, err := w.Create("charon.db")
	require.NoError(t, err)
	content := []byte("actual-content-is-this-many-bytes-long")
	_, err = fw.Write(content)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	require.NoError(t, out.Close())

	// Declares a different (but still within-slack) size than the real
	// content, so the "exceeds slack" guard passes but the exact-size
	// comparison must still fail.
	manifest := &BackupManifest{Contents: []ManifestEntry{{Path: "charon.db", Size: int64(len(content)) - 1, SHA256: "irrelevant"}}}
	err = verifyManifestChecksums(zipPath, manifest)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "size mismatch")
}

// --- RestoreBackupSafe: S1 pre-restore safety backup creation failure. ---

// TestRestoreBackupSafe_PreRestoreBackupFails_ReturnsWrappedError proves
// that when createBackupLocked (S1) itself fails, RestoreBackupSafe returns
// a wrapped error without ever reaching A1/A2. Validation (V1-V6) only
// reads from the archive being restored (BackupDir), never from the live
// database at DataDir/DatabaseName, so removing the live database file
// after the archive to restore already exists — but before calling
// RestoreBackupSafe — lets validation succeed normally while S1's own
// os.Stat(dbPath) guard (backup_service.go's createBackupLocked,
// "database file not found") fails, proving this is specifically S1's
// failure and not a validation rejection.
func TestRestoreBackupSafe_PreRestoreBackupFails_ReturnsWrappedError(t *testing.T) {
	svc := newHardeningTestService(t)

	record, err := svc.CreateBackupWithOptions(BackupOptions{Type: "manual"})
	require.NoError(t, err)

	liveDBPath := filepath.Join(svc.DataDir, svc.DatabaseName)
	require.NoError(t, os.Remove(liveDBPath))

	result, err := svc.RestoreBackupSafe(record.Filename, "")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "create pre-restore safety backup")
	assert.Contains(t, err.Error(), "database file not found")
}
