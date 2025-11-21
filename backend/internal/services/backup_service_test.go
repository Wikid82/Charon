package services

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackupService_CreateAndList(t *testing.T) {
	// Setup temp dirs
	tmpDir, err := os.MkdirTemp("", "cpm-backup-service-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dataDir := filepath.Join(tmpDir, "data")
	err = os.MkdirAll(dataDir, 0755)
	require.NoError(t, err)

	// Create dummy DB
	dbPath := filepath.Join(dataDir, "cpm.db")
	err = os.WriteFile(dbPath, []byte("dummy db"), 0644)
	require.NoError(t, err)

	// Create dummy caddy dir
	caddyDir := filepath.Join(dataDir, "caddy")
	err = os.MkdirAll(caddyDir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(caddyDir, "caddy.json"), []byte("{}"), 0644)
	require.NoError(t, err)

	cfg := &config.Config{DatabasePath: dbPath}
	service := NewBackupService(cfg)

	// Test Create
	filename, err := service.CreateBackup()
	require.NoError(t, err)
	assert.NotEmpty(t, filename)
	assert.FileExists(t, filepath.Join(service.BackupDir, filename))

	// Test List
	backups, err := service.ListBackups()
	require.NoError(t, err)
	assert.Len(t, backups, 1)
	assert.Equal(t, filename, backups[0].Filename)
	assert.True(t, backups[0].Size > 0)

	// Test GetBackupPath
	path := service.GetBackupPath(filename)
	assert.Equal(t, filepath.Join(service.BackupDir, filename), path)

	// Test Restore
	// Modify DB to verify restore
	err = os.WriteFile(dbPath, []byte("modified db"), 0644)
	require.NoError(t, err)

	err = service.RestoreBackup(filename)
	require.NoError(t, err)

	// Verify DB content restored
	content, err := os.ReadFile(dbPath)
	require.NoError(t, err)
	assert.Equal(t, "dummy db", string(content))

	// Test Delete
	err = service.DeleteBackup(filename)
	require.NoError(t, err)
	assert.NoFileExists(t, filepath.Join(service.BackupDir, filename))

	// Test Delete Non-existent
	err = service.DeleteBackup("non-existent.zip")
	assert.Error(t, err)
}

func TestBackupService_Restore_ZipSlip(t *testing.T) {
	// Setup temp dirs
	tmpDir := t.TempDir()
	service := &BackupService{
		DataDir:   filepath.Join(tmpDir, "data"),
		BackupDir: filepath.Join(tmpDir, "backups"),
	}
	os.MkdirAll(service.BackupDir, 0755)

	// Create malicious zip
	zipPath := filepath.Join(service.BackupDir, "malicious.zip")
	zipFile, err := os.Create(zipPath)
	require.NoError(t, err)

	w := zip.NewWriter(zipFile)
	f, err := w.Create("../../../evil.txt")
	require.NoError(t, err)
	_, err = f.Write([]byte("evil"))
	require.NoError(t, err)
	w.Close()
	zipFile.Close()

	// Attempt restore
	err = service.RestoreBackup("malicious.zip")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "illegal file path")
}

func TestBackupService_PathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	service := &BackupService{
		DataDir:   filepath.Join(tmpDir, "data"),
		BackupDir: filepath.Join(tmpDir, "backups"),
	}
	os.MkdirAll(service.BackupDir, 0755)

	// Test GetBackupPath with traversal
	// Should resolve to .../backups/passwd, NOT .../etc/passwd
	path := service.GetBackupPath("../../etc/passwd")
	expected := filepath.Join(service.BackupDir, "passwd")
	assert.Equal(t, expected, path)

	// Test DeleteBackup with traversal
	// Should try to delete .../backups/passwd, fail because it doesn't exist
	err := service.DeleteBackup("../../etc/passwd")
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}
