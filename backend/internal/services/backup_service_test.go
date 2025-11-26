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
	path, err := service.GetBackupPath(filename)
	require.NoError(t, err)
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
	// Should return error
	_, err := service.GetBackupPath("../../etc/passwd")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid filename")

	// Test DeleteBackup with traversal
	// Should return error
	err = service.DeleteBackup("../../etc/passwd")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid filename")
}

func TestBackupService_RunScheduledBackup(t *testing.T) {
	// Setup temp dirs
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	os.MkdirAll(dataDir, 0755)

	// Create dummy DB
	dbPath := filepath.Join(dataDir, "cpm.db")
	os.WriteFile(dbPath, []byte("dummy db"), 0644)

	cfg := &config.Config{DatabasePath: dbPath}
	service := NewBackupService(cfg)

	// Run scheduled backup manually
	service.RunScheduledBackup()

	// Verify backup created
	backups, err := service.ListBackups()
	require.NoError(t, err)
	assert.Len(t, backups, 1)
}

func TestBackupService_CreateBackup_Errors(t *testing.T) {
	t.Run("missing database file", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfg := &config.Config{DatabasePath: filepath.Join(tmpDir, "nonexistent.db")}
		service := NewBackupService(cfg)

		_, err := service.CreateBackup()
		assert.Error(t, err)
	})

	t.Run("cannot create backup directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "cpm.db")
		os.WriteFile(dbPath, []byte("test"), 0644)

		// Create backup dir as a file to cause mkdir error
		backupDir := filepath.Join(tmpDir, "backups")
		os.WriteFile(backupDir, []byte("blocking"), 0644)

		service := &BackupService{
			DataDir:   tmpDir,
			BackupDir: backupDir,
		}

		_, err := service.CreateBackup()
		assert.Error(t, err)
	})
}

func TestBackupService_RestoreBackup_Errors(t *testing.T) {
	t.Run("non-existent backup", func(t *testing.T) {
		tmpDir := t.TempDir()
		service := &BackupService{
			DataDir:   filepath.Join(tmpDir, "data"),
			BackupDir: filepath.Join(tmpDir, "backups"),
		}
		os.MkdirAll(service.BackupDir, 0755)

		err := service.RestoreBackup("nonexistent.zip")
		assert.Error(t, err)
	})

	t.Run("invalid zip file", func(t *testing.T) {
		tmpDir := t.TempDir()
		service := &BackupService{
			DataDir:   filepath.Join(tmpDir, "data"),
			BackupDir: filepath.Join(tmpDir, "backups"),
		}
		os.MkdirAll(service.BackupDir, 0755)

		// Create invalid zip
		badZip := filepath.Join(service.BackupDir, "bad.zip")
		os.WriteFile(badZip, []byte("not a zip"), 0644)

		err := service.RestoreBackup("bad.zip")
		assert.Error(t, err)
	})
}

func TestBackupService_ListBackups_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	service := &BackupService{
		BackupDir: filepath.Join(tmpDir, "backups"),
	}
	os.MkdirAll(service.BackupDir, 0755)

	backups, err := service.ListBackups()
	require.NoError(t, err)
	assert.Empty(t, backups)
}

func TestBackupService_ListBackups_MissingDir(t *testing.T) {
	tmpDir := t.TempDir()
	service := &BackupService{
		BackupDir: filepath.Join(tmpDir, "nonexistent"),
	}

	backups, err := service.ListBackups()
	require.NoError(t, err)
	assert.Empty(t, backups)
}
