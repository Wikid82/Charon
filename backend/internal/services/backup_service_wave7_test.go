package services

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeLargeZipEntry(t *testing.T, writer *zip.Writer, name string, sizeBytes int64) {
	t.Helper()
	entry, err := writer.Create(name)
	require.NoError(t, err)

	chunk := make([]byte, 1024*1024)
	remaining := sizeBytes
	for remaining > 0 {
		toWrite := int64(len(chunk))
		if remaining < toWrite {
			toWrite = remaining
		}
		_, err := entry.Write(chunk[:toWrite])
		require.NoError(t, err)
		remaining -= toWrite
	}
}

func TestBackupServiceWave7_CreateBackup_SnapshotFailureForNonSQLiteDB(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	require.NoError(t, os.MkdirAll(backupDir, 0o700))

	dbPath := filepath.Join(tmpDir, "charon.db")
	require.NoError(t, os.WriteFile(dbPath, []byte("not-a-sqlite-db"), 0o600))

	svc := &BackupService{
		DataDir:      tmpDir,
		BackupDir:    backupDir,
		DatabaseName: "charon.db",
	}

	_, err := svc.CreateBackup()
	require.Error(t, err)
	// Deliberate behavior change from this plan's §2.5/§3.9 pre-flight
	// integrity check (docs/plans/current_spec.md, Async Backup/Restore
	// Jobs): createBackupLockedWithProgress now runs a dedicated-connection
	// PRAGMA quick_check BEFORE attempting the VACUUM INTO snapshot this
	// test used to reach and fail inside — a non-SQLite file like this one
	// is caught immediately by that pre-flight check (ErrDatabaseCorrupted)
	// instead of failing deeper inside createSQLiteSnapshot's VACUUM INTO
	// call. This is the intended, faster, more clearly-classified failure
	// mode §2.5/§3.9 exists to produce; the old assertion tested the
	// superseded behavior.
	require.ErrorIs(t, err, ErrDatabaseCorrupted)
}

func TestBackupServiceWave7_ExtractDatabaseFromBackup_DBEntryOverLimit(t *testing.T) {
	// See TestBackupService_UnzipWithSkip_RejectsExcessiveUncompressedSize:
	// the flat legacy per-entry cap is now 2GiB (spec §3.9), so temporarily
	// lower it to prove the enforcement path still fires without
	// allocating a multi-gigabyte fixture.
	originalCap := legacyPerEntryDecompressionCap
	legacyPerEntryDecompressionCap = 100 * 1024 * 1024
	t.Cleanup(func() { legacyPerEntryDecompressionCap = originalCap })

	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "db-over-limit.zip")

	zipFile, err := os.Create(zipPath) // #nosec G304 -- path is derived from t.TempDir()
	require.NoError(t, err)
	writer := zip.NewWriter(zipFile)

	writeLargeZipEntry(t, writer, "charon.db", int64(101*1024*1024))

	require.NoError(t, writer.Close())
	require.NoError(t, zipFile.Close())

	svc := &BackupService{DatabaseName: "charon.db"}
	_, err = svc.extractDatabaseFromBackup(zipPath)
	require.Error(t, err)
	require.Contains(t, err.Error(), "extract database entry from backup archive")
	require.Contains(t, err.Error(), "decompression limit")
}

func TestBackupServiceWave7_ExtractDatabaseFromBackup_WALEntryOverLimit(t *testing.T) {
	originalCap := legacyPerEntryDecompressionCap
	legacyPerEntryDecompressionCap = 100 * 1024 * 1024
	t.Cleanup(func() { legacyPerEntryDecompressionCap = originalCap })

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "charon.db")
	createSQLiteTestDB(t, dbPath)

	zipPath := filepath.Join(tmpDir, "wal-over-limit.zip")
	zipFile, err := os.Create(zipPath) // #nosec G304 -- path is derived from t.TempDir()
	require.NoError(t, err)
	writer := zip.NewWriter(zipFile)

	dbBytes, err := os.ReadFile(dbPath) // #nosec G304 -- path is derived from t.TempDir()
	require.NoError(t, err)
	dbEntry, err := writer.Create("charon.db")
	require.NoError(t, err)
	_, err = dbEntry.Write(dbBytes)
	require.NoError(t, err)

	writeLargeZipEntry(t, writer, "charon.db-wal", int64(101*1024*1024))

	require.NoError(t, writer.Close())
	require.NoError(t, zipFile.Close())

	svc := &BackupService{DatabaseName: "charon.db"}
	_, err = svc.extractDatabaseFromBackup(zipPath)
	require.Error(t, err)
	require.Contains(t, err.Error(), "extract wal entry from backup archive")
	require.Contains(t, err.Error(), "decompression limit")
}
