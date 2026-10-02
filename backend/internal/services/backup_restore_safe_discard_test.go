package services

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// useScopedTempDir points the process temp directory at a fresh directory so a
// test can assert on exactly which restore snapshots the service leaves in it.
func useScopedTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	return dir
}

// assertRestoreSnapshotDiscarded fails when the service still tracks a staged
// restore snapshot or any extracted snapshot file remains in tmpDir.
func assertRestoreSnapshotDiscarded(t *testing.T, svc *BackupService, tmpDir string) {
	t.Helper()
	assert.Empty(t, svc.restoreDBPath, "staged restore snapshot must no longer be tracked")
	leftovers, err := filepath.Glob(filepath.Join(tmpDir, "charon-restore-db-*"))
	require.NoError(t, err)
	assert.Empty(t, leftovers, "extracted restore snapshot must be removed")
}

func TestRestoreBackupSafe_Success_DiscardsStagedSnapshot(t *testing.T) {
	tmpDir := useScopedTempDir(t)
	svc, _ := newLiveDBRestoreErrorTestService(t)

	record, err := svc.CreateBackupWithOptions(BackupOptions{Type: "manual"})
	require.NoError(t, err)

	result, err := svc.RestoreBackupSafe(record.Filename, "")
	require.NoError(t, err)
	require.True(t, result.LiveRehydrateApplied)

	assertRestoreSnapshotDiscarded(t, svc, tmpDir)
}

func TestRestoreBackupSafe_PreRestoreBackupFailure_DiscardsStagedSnapshot(t *testing.T) {
	tmpDir := useScopedTempDir(t)
	svc := newHardeningTestService(t)

	record, err := svc.CreateBackupWithOptions(BackupOptions{Type: "manual"})
	require.NoError(t, err)
	require.NoError(t, os.Remove(filepath.Join(svc.DataDir, svc.DatabaseName)))

	_, err = svc.RestoreBackupSafe(record.Filename, "")
	require.ErrorContains(t, err, "create pre-restore safety backup")

	assertRestoreSnapshotDiscarded(t, svc, tmpDir)
}

func TestRestoreBackupSafe_ApplyFailure_DiscardsStagedSnapshot(t *testing.T) {
	tmpDir := useScopedTempDir(t)
	svc := newHardeningTestService(t)

	dbContent, err := os.ReadFile(filepath.Join(svc.DataDir, svc.DatabaseName))
	require.NoError(t, err)

	filename := "zip_slip_legacy.zip"
	out, err := os.Create(filepath.Join(svc.BackupDir, filename)) // #nosec G304 -- test-controlled path
	require.NoError(t, err)
	w := zip.NewWriter(out)
	dbWriter, err := w.Create("charon.db")
	require.NoError(t, err)
	_, err = dbWriter.Write(dbContent)
	require.NoError(t, err)
	evil, err := w.Create("caddy/../../evil.txt")
	require.NoError(t, err)
	_, err = evil.Write([]byte("escaped"))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	require.NoError(t, out.Close())

	_, err = svc.RestoreBackupSafe(filename, "")
	require.ErrorContains(t, err, "restore failed, previous state restored")

	assertRestoreSnapshotDiscarded(t, svc, tmpDir)
}

// The durable pending-restore file is a copy, so discarding the staged
// snapshot must leave it intact for the next boot.
func TestRestoreBackupSafe_PendingFileWritten_DiscardsSnapshotButKeepsPendingCopy(t *testing.T) {
	tmpDir := useScopedTempDir(t)
	svc, db := newLiveDBRestoreErrorTestService(t)

	record, err := svc.CreateBackupWithOptions(BackupOptions{Type: "manual"})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	result, err := svc.RestoreBackupSafe(record.Filename, "")
	require.NoError(t, err)
	require.True(t, result.DatabaseSwapPending)

	assertRestoreSnapshotDiscarded(t, svc, tmpDir)
	info, statErr := os.Stat(filepath.Join(svc.DataDir, svc.DatabaseName+".pending-restore"))
	require.NoError(t, statErr, "pending-restore file must survive the snapshot discard")
	assert.Positive(t, info.Size())
}

func TestRestoreBackupSafe_Unrecoverable_DiscardsStagedSnapshot(t *testing.T) {
	tmpDir := useScopedTempDir(t)
	svc, db := newLiveDBRestoreErrorTestService(t)

	record, err := svc.CreateBackupWithOptions(BackupOptions{Type: "manual"})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	require.NoError(t, os.MkdirAll(filepath.Join(svc.DataDir, svc.DatabaseName+".pending-restore"), 0o700))

	_, err = svc.RestoreBackupSafe(record.Filename, "")
	require.ErrorIs(t, err, ErrRestoreUnrecoverable)

	assertRestoreSnapshotDiscarded(t, svc, tmpDir)
}

func TestDiscardRestoreSnapshot_RemovesSnapshotAndSidecars(t *testing.T) {
	svc := newHardeningTestService(t)
	base := filepath.Join(t.TempDir(), "snapshot.sqlite")
	for _, p := range []string{base, base + "-wal", base + "-shm"} {
		require.NoError(t, os.WriteFile(p, []byte("x"), 0o600))
	}
	svc.restoreDBPath = base

	svc.discardRestoreSnapshot()

	assert.Empty(t, svc.restoreDBPath)
	for _, p := range []string{base, base + "-wal", base + "-shm"} {
		assert.NoFileExists(t, p)
	}
}

func TestDiscardRestoreSnapshot_EmptyFieldIsNoop(t *testing.T) {
	svc := newHardeningTestService(t)
	svc.restoreDBPath = ""
	svc.discardRestoreSnapshot()
	assert.Empty(t, svc.restoreDBPath)
}

func TestDiscardRestoreSnapshot_AlreadyRemovedFileStillClearsField(t *testing.T) {
	svc := newHardeningTestService(t)
	svc.restoreDBPath = filepath.Join(t.TempDir(), "gone.sqlite")
	svc.discardRestoreSnapshot()
	assert.Empty(t, svc.restoreDBPath)
}
