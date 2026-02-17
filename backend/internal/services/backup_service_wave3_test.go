package services

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func openZipInTempDir(t *testing.T, tempDir, zipPath string) *os.File {
	t.Helper()

	absTempDir, err := filepath.Abs(tempDir)
	require.NoError(t, err)
	absZipPath, err := filepath.Abs(zipPath)
	require.NoError(t, err)

	relPath, err := filepath.Rel(absTempDir, absZipPath)
	require.NoError(t, err)
	require.False(t, relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)))

	// #nosec G304 -- absZipPath is constrained to test TempDir via Abs+Rel checks above.
	zipFile, err := os.OpenFile(absZipPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	require.NoError(t, err)

	return zipFile
}

func TestBackupService_UnzipWithSkip_SkipsDatabaseEntries(t *testing.T) {
	tmp := t.TempDir()
	destDir := filepath.Join(tmp, "data")
	require.NoError(t, os.MkdirAll(destDir, 0o700))

	zipPath := filepath.Join(tmp, "restore.zip")
	zipFile := openZipInTempDir(t, tmp, zipPath)

	writer := zip.NewWriter(zipFile)
	for name, content := range map[string]string{
		"charon.db":       "db",
		"charon.db-wal":   "wal",
		"charon.db-shm":   "shm",
		"caddy/config":    "cfg",
		"nested/file.txt": "hello",
	} {
		entry, createErr := writer.Create(name)
		require.NoError(t, createErr)
		_, writeErr := entry.Write([]byte(content))
		require.NoError(t, writeErr)
	}
	require.NoError(t, writer.Close())
	require.NoError(t, zipFile.Close())

	svc := &BackupService{DataDir: destDir, DatabaseName: "charon.db"}
	require.NoError(t, svc.unzipWithSkip(zipPath, destDir, map[string]struct{}{
		"charon.db":     {},
		"charon.db-wal": {},
		"charon.db-shm": {},
	}))

	_, err := os.Stat(filepath.Join(destDir, "charon.db"))
	require.Error(t, err)
	require.FileExists(t, filepath.Join(destDir, "caddy", "config"))
	require.FileExists(t, filepath.Join(destDir, "nested", "file.txt"))
}

func TestBackupService_ExtractDatabaseFromBackup_ExtractWalFailure(t *testing.T) {
	tmp := t.TempDir()

	zipPath := filepath.Join(tmp, "invalid-wal.zip")
	zipFile := openZipInTempDir(t, tmp, zipPath)
	writer := zip.NewWriter(zipFile)

	dbEntry, err := writer.Create("charon.db")
	require.NoError(t, err)
	_, err = dbEntry.Write([]byte("sqlite header placeholder"))
	require.NoError(t, err)

	walEntry, err := writer.Create("charon.db-wal")
	require.NoError(t, err)
	_, err = walEntry.Write([]byte("invalid wal content"))
	require.NoError(t, err)

	require.NoError(t, writer.Close())
	require.NoError(t, zipFile.Close())

	svc := &BackupService{DatabaseName: "charon.db"}
	_, err = svc.extractDatabaseFromBackup(zipPath)
	require.Error(t, err)
}
