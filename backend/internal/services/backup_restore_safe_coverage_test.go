package services

import (
	"archive/zip"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/version"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// --- validateBackupArchive / ValidateBackup: early guard branches ---

// TestValidateBackup_PathTraversal_FilenameWithDirectoryComponent proves the
// first path-traversal guard (filename != filepath.Base(filename)) rejects
// a filename containing directory components.
func TestValidateBackup_PathTraversal_FilenameWithDirectoryComponent(t *testing.T) {
	svc := newHardeningTestService(t)

	_, err := svc.ValidateBackup("../etc/passwd", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path traversal attempt detected")
}

// TestValidateBackup_PathTraversal_DotDot proves the second path-traversal
// guard: a filename of ".." is its own filepath.Base (so the first check
// passes) but joins to a path outside BackupDir, which the
// strings.HasPrefix guard must reject.
func TestValidateBackup_PathTraversal_DotDot(t *testing.T) {
	svc := newHardeningTestService(t)

	_, err := svc.ValidateBackup("..", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path traversal attempt detected")
}

// TestValidateBackup_FileNotFound proves a nonexistent (but validly-named)
// backup filename surfaces the raw os.Stat error.
func TestValidateBackup_FileNotFound(t *testing.T) {
	svc := newHardeningTestService(t)

	_, err := svc.ValidateBackup("does-not-exist.zip", "")
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

// TestValidateBackup_NotAZipFile proves a file that isn't a valid zip
// archive at all (garbage bytes, no PK magic) is rejected by
// readBackupManifest's zip.OpenReader failure, wrapped as
// ErrBackupValidationFailed.
func TestValidateBackup_NotAZipFile(t *testing.T) {
	svc := newHardeningTestService(t)

	filename := "garbage.zip"
	require.NoError(t, os.WriteFile(filepath.Join(svc.BackupDir, filename), []byte("not a zip file at all"), 0o600))

	_, err := svc.ValidateBackup(filename, "")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrBackupValidationFailed)
}

// TestValidateBackup_NewerFormatVersion_Rejected proves an archive whose
// manifest declares a format_version this build doesn't understand is
// rejected with ErrNewerBackupFormat before any extraction is attempted.
func TestValidateBackup_NewerFormatVersion_Rejected(t *testing.T) {
	svc := newHardeningTestService(t)

	filename := "future_format.zip"
	zipPath := filepath.Join(svc.BackupDir, filename)
	out, err := os.Create(zipPath) // #nosec G304 -- test-controlled path
	require.NoError(t, err)
	w := zip.NewWriter(out)

	manifestBytes, err := json.Marshal(BackupManifest{FormatVersion: 3})
	require.NoError(t, err)
	mw, err := w.Create("manifest.json")
	require.NoError(t, err)
	_, err = mw.Write(manifestBytes)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	require.NoError(t, out.Close())

	_, err = svc.ValidateBackup(filename, "")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNewerBackupFormat)
}

// TestValidateBackup_MissingDatabaseEntry_Rejected proves V5 extraction
// fails cleanly (ErrBackupValidationFailed) when the archive's manifest
// declares no problematic checksums (empty Contents) but the database entry
// itself is simply absent from the archive.
func TestValidateBackup_MissingDatabaseEntry_Rejected(t *testing.T) {
	svc := newHardeningTestService(t)

	filename := "no_db_entry.zip"
	zipPath := filepath.Join(svc.BackupDir, filename)
	out, err := os.Create(zipPath) // #nosec G304 -- test-controlled path
	require.NoError(t, err)
	w := zip.NewWriter(out)

	// A manifest with zero Contents entries trivially passes V4 (nothing to
	// checksum), but V5 must still fail: there is no charon.db entry at all.
	manifestBytes, err := json.Marshal(BackupManifest{FormatVersion: 2, DatabaseName: svc.DatabaseName})
	require.NoError(t, err)
	mw, err := w.Create("manifest.json")
	require.NoError(t, err)
	_, err = mw.Write(manifestBytes)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	require.NoError(t, out.Close())

	_, err = svc.ValidateBackup(filename, "")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrBackupValidationFailed)
	assert.Contains(t, err.Error(), "extract database from backup")
}

// TestValidateBackup_SanityCheckFailure_MissingCharonTables proves V6
// rejects an extracted database that opens fine and passes
// PRAGMA integrity_check but doesn't look like a Charon database (missing
// the users/proxy_hosts tables).
func TestValidateBackup_SanityCheckFailure_MissingCharonTables(t *testing.T) {
	svc := newHardeningTestService(t)

	// A syntactically valid, but empty (no users/proxy_hosts tables)
	// sqlite database.
	notCharonDBPath := filepath.Join(t.TempDir(), "not-charon.db")
	rawDB, err := sql.Open("sqlite3", notCharonDBPath)
	require.NoError(t, err)
	_, err = rawDB.Exec(`CREATE TABLE something_else (id INTEGER PRIMARY KEY)`)
	require.NoError(t, err)
	require.NoError(t, rawDB.Close())

	filename := "not_charon.zip"
	zipPath := filepath.Join(svc.BackupDir, filename)
	out, err := os.Create(zipPath) // #nosec G304 -- test-controlled path
	require.NoError(t, err)
	w := zip.NewWriter(out)
	require.NoError(t, svc.addToZipTracked(w, notCharonDBPath, svc.DatabaseName, nil))
	require.NoError(t, w.Close())
	require.NoError(t, out.Close())

	_, err = svc.ValidateBackup(filename, "")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrBackupValidationFailed)
	assert.Contains(t, err.Error(), "does not look like a Charon database")
}

// TestValidateBackup_ManifestPresent_AppVersionMismatch_WarnsButValid
// proves ValidateBackup's manifest != nil branch populates FormatVersion/
// AppVersion/CreatedAt/EncryptionKeyRequired, and that a manifest whose
// AppVersion differs from the running version produces a non-fatal warning
// rather than a validation failure.
func TestValidateBackup_ManifestPresent_AppVersionMismatch_WarnsButValid(t *testing.T) {
	svc := newHardeningTestService(t)

	dbPath := filepath.Join(svc.DataDir, svc.DatabaseName)

	filename := "old_version.zip"
	zipPath := filepath.Join(svc.BackupDir, filename)
	out, err := os.Create(zipPath) // #nosec G304 -- test-controlled path
	require.NoError(t, err)
	w := zip.NewWriter(out)

	var entries []ManifestEntry
	require.NoError(t, svc.addToZipTracked(w, dbPath, svc.DatabaseName, &entries))

	manifest := BackupManifest{
		FormatVersion: 2,
		AppVersion:    "0.0.1-ancient",
		CreatedAt:     time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		DatabaseName:  svc.DatabaseName,
		Contents:      entries,
	}
	manifestBytes, err := json.Marshal(manifest)
	require.NoError(t, err)
	mw, err := w.Create("manifest.json")
	require.NoError(t, err)
	_, err = mw.Write(manifestBytes)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	require.NoError(t, out.Close())

	result, err := svc.ValidateBackup(filename, "")
	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.False(t, result.LegacyFormat)
	assert.Equal(t, 2, result.FormatVersion)
	assert.Equal(t, "0.0.1-ancient", result.AppVersion)
	require.Len(t, result.Warnings, 1)
	assert.Contains(t, result.Warnings[0], "0.0.1-ancient")
	assert.Contains(t, result.Warnings[0], version.Version)
}

// --- RestoreBackupSafe: F2 rollback-on-apply-failure ---

// TestRestoreBackupSafe_ApplyFailure_RollsBackToPreRestoreState is required
// coverage for F2 (spec §3.5): when unzipWithSkipManifest fails partway
// through applying the archive's non-database entries, RestoreBackupSafe
// must re-apply the just-taken pre_restore safety backup and fail loudly,
// rather than leaving DataDir half-migrated. A legacy (no-manifest) archive
// containing a zip-slip path traversal entry inside caddy/** is used to
// force the apply step to fail after validation (V1-V6) has already
// succeeded, since a legacy archive has no per-entry checksums to catch
// this earlier.
func TestRestoreBackupSafe_ApplyFailure_RollsBackToPreRestoreState(t *testing.T) {
	svc := newHardeningTestService(t)

	dbContent, err := os.ReadFile(filepath.Join(svc.DataDir, svc.DatabaseName))
	require.NoError(t, err)

	filename := "malicious_legacy.zip"
	zipPath := filepath.Join(svc.BackupDir, filename)
	out, err := os.Create(zipPath) // #nosec G304 -- test-controlled path
	require.NoError(t, err)
	w := zip.NewWriter(out)

	dbWriter, err := w.Create("charon.db")
	require.NoError(t, err)
	_, err = dbWriter.Write(dbContent)
	require.NoError(t, err)

	// A zip-slip entry: extractZip must reject this during A1's apply step.
	evilWriter, err := w.Create("caddy/../../evil.txt")
	require.NoError(t, err)
	_, err = evilWriter.Write([]byte("escaped"))
	require.NoError(t, err)

	require.NoError(t, w.Close())
	require.NoError(t, out.Close())

	before, err := svc.ListBackups()
	require.NoError(t, err)
	beforeCount := len(before)

	result, err := svc.RestoreBackupSafe(filename, "")
	require.Error(t, err)
	require.Nil(t, result)
	assert.Contains(t, err.Error(), "restore failed, previous state restored")

	// A pre_restore safety backup was created (S1) even though the overall
	// restore failed and was rolled back (F2).
	after, err := svc.ListBackups()
	require.NoError(t, err)
	assert.Equal(t, beforeCount+1, len(after), "S1's pre_restore safety backup must exist even after an F2 rollback")
}

// --- RestoreBackupSafe: live-db rehydrate + Caddy reload branches ---

// fakeConfigurableCaddyReloader lets tests toggle ApplyConfig's outcome,
// unlike the fixed-success fakeCaddyReloader already used elsewhere in this
// package's tests.
type fakeConfigurableCaddyReloader struct {
	err   error
	calls int
}

func (f *fakeConfigurableCaddyReloader) ApplyConfig(_ context.Context) error {
	f.calls++
	return f.err
}

// newLiveDBHardeningTestService builds a BackupService whose constructor
// db argument is a real, WAL-mode *gorm.DB opened against the same on-disk
// charon.db RestoreBackupSafe will restore — exercising the `s.db != nil`
// live-rehydrate branch inside RestoreBackupSafe itself (distinct from the
// standalone RehydrateLiveDatabase tests in backup_service_rehydrate_test.go,
// which never go through RestoreBackupSafe's own db-attached branch).
func newLiveDBHardeningTestService(t *testing.T) *BackupService {
	t.Helper()

	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o700))
	dbPath := filepath.Join(dataDir, "charon.db")

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("PRAGMA journal_mode=WAL").Error)
	require.NoError(t, db.Exec("PRAGMA wal_autocheckpoint=0").Error)
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.ProxyHost{}, &models.BackupRecord{}))

	require.NoError(t, db.Create(&models.User{
		UUID: uuid.NewString(), Email: "restore-user@example.com", Name: "Restore User",
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

// TestRestoreBackupSafe_LiveDBAttached_RehydratesAndReloadsCaddy proves the
// db != nil branch (live rehydrate) and the caddyReloader != nil branch
// (R1) both run to their success outcomes when RestoreBackupSafe is called
// on a service wired with a real live database and a fake Caddy reloader.
func TestRestoreBackupSafe_LiveDBAttached_RehydratesAndReloadsCaddy(t *testing.T) {
	svc := newLiveDBHardeningTestService(t)

	reloader := &fakeConfigurableCaddyReloader{}
	svc.SetCaddyReloader(reloader)

	record, err := svc.CreateBackupWithOptions(BackupOptions{Type: "manual"})
	require.NoError(t, err)

	result, err := svc.RestoreBackupSafe(record.Filename, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.LiveRehydrateApplied, "a healthy live db must rehydrate successfully in-process")
	assert.False(t, result.RestartRequired)
	assert.False(t, result.DatabaseSwapPending)
	assert.True(t, result.CaddyReloaded)
	assert.Equal(t, "Backup restored successfully", result.Message)
	assert.Equal(t, 1, reloader.calls)
}

// TestRestoreBackupSafe_CaddyReloadFailure_ReportedNotFatal proves a
// caddyReloader failure (R1/F4) is logged and reflected as
// caddy_reloaded=false without failing the overall restore, since Caddy's
// own manager already rolled itself back.
func TestRestoreBackupSafe_CaddyReloadFailure_ReportedNotFatal(t *testing.T) {
	svc := newLiveDBHardeningTestService(t)

	reloader := &fakeConfigurableCaddyReloader{err: assertAnError}
	svc.SetCaddyReloader(reloader)

	record, err := svc.CreateBackupWithOptions(BackupOptions{Type: "manual"})
	require.NoError(t, err)

	result, err := svc.RestoreBackupSafe(record.Filename, "")
	require.NoError(t, err, "a caddy reload failure must never fail the overall restore")
	require.NotNil(t, result)
	assert.False(t, result.CaddyReloaded)
	assert.Equal(t, 1, reloader.calls)
}
