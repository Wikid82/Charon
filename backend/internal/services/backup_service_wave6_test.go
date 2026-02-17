package services

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBackupServiceWave6_ExtractDatabaseFromBackup_WithShmEntry(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "charon.db")
	createSQLiteTestDB(t, dbPath)

	zipPath := filepath.Join(tmpDir, "with-shm.zip")
	zipFile, err := os.Create(zipPath) // #nosec G304 -- path is derived from t.TempDir()
	require.NoError(t, err)
	writer := zip.NewWriter(zipFile)

	sourceDB, err := os.Open(dbPath) // #nosec G304 -- path is derived from t.TempDir()
	require.NoError(t, err)
	defer func() { _ = sourceDB.Close() }()

	dbEntry, err := writer.Create("charon.db")
	require.NoError(t, err)
	_, err = io.Copy(dbEntry, sourceDB)
	require.NoError(t, err)

	walEntry, err := writer.Create("charon.db-wal")
	require.NoError(t, err)
	_, err = walEntry.Write([]byte("invalid wal content"))
	require.NoError(t, err)

	shmEntry, err := writer.Create("charon.db-shm")
	require.NoError(t, err)
	_, err = shmEntry.Write([]byte("shm placeholder"))
	require.NoError(t, err)

	require.NoError(t, writer.Close())
	require.NoError(t, zipFile.Close())

	svc := &BackupService{DatabaseName: "charon.db"}
	restoredPath, err := svc.extractDatabaseFromBackup(zipPath)
	require.NoError(t, err)
	require.FileExists(t, restoredPath)
}
