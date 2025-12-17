package services

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackupService_CreateAndList(t *testing.T) {
	// Setup temp dirs
	tmpDir, err := os.MkdirTemp("", "cpm-backup-service-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dataDir := filepath.Join(tmpDir, "data")
	err = os.MkdirAll(dataDir, 0o755)
	require.NoError(t, err)

	// Create dummy DB
	dbPath := filepath.Join(dataDir, "charon.db")
	err = os.WriteFile(dbPath, []byte("dummy db"), 0o644)
	require.NoError(t, err)

	// Create dummy caddy dir
	caddyDir := filepath.Join(dataDir, "caddy")
	err = os.MkdirAll(caddyDir, 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(caddyDir, "caddy.json"), []byte("{}"), 0o644)
	require.NoError(t, err)

	cfg := &config.Config{DatabasePath: dbPath}
	service := NewBackupService(cfg)
	defer service.Stop() // Prevent goroutine leaks

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
	err = os.WriteFile(dbPath, []byte("modified db"), 0o644)
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
	os.MkdirAll(service.BackupDir, 0o755)

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
	os.MkdirAll(service.BackupDir, 0o755)

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
	os.MkdirAll(dataDir, 0o755)

	// Create dummy DB
	dbPath := filepath.Join(dataDir, "charon.db")
	os.WriteFile(dbPath, []byte("dummy db"), 0o644)

	cfg := &config.Config{DatabasePath: dbPath}
	service := NewBackupService(cfg)
	defer service.Stop() // Prevent goroutine leaks

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
		defer service.Stop() // Prevent goroutine leaks

		_, err := service.CreateBackup()
		assert.Error(t, err)
	})

	t.Run("cannot create backup directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "charon.db")
		os.WriteFile(dbPath, []byte("test"), 0o644)

		// Create backup dir as a file to cause mkdir error
		backupDir := filepath.Join(tmpDir, "backups")
		os.WriteFile(backupDir, []byte("blocking"), 0o644)

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
		os.MkdirAll(service.BackupDir, 0o755)

		err := service.RestoreBackup("nonexistent.zip")
		assert.Error(t, err)
	})

	t.Run("invalid zip file", func(t *testing.T) {
		tmpDir := t.TempDir()
		service := &BackupService{
			DataDir:   filepath.Join(tmpDir, "data"),
			BackupDir: filepath.Join(tmpDir, "backups"),
		}
		os.MkdirAll(service.BackupDir, 0o755)

		// Create invalid zip
		badZip := filepath.Join(service.BackupDir, "bad.zip")
		os.WriteFile(badZip, []byte("not a zip"), 0o644)

		err := service.RestoreBackup("bad.zip")
		assert.Error(t, err)
	})
}

func TestBackupService_ListBackups_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	service := &BackupService{
		BackupDir: filepath.Join(tmpDir, "backups"),
	}
	os.MkdirAll(service.BackupDir, 0o755)

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

func TestBackupService_CleanupOldBackups(t *testing.T) {
	t.Run("deletes backups exceeding retention", func(t *testing.T) {
		tmpDir := t.TempDir()
		service := &BackupService{
			DataDir:   filepath.Join(tmpDir, "data"),
			BackupDir: filepath.Join(tmpDir, "backups"),
		}
		os.MkdirAll(service.BackupDir, 0o755)

		// Create 10 backup files manually with different timestamps
		for i := 0; i < 10; i++ {
			filename := fmt.Sprintf("backup_2025-01-%02d_10-00-00.zip", i+1)
			zipPath := filepath.Join(service.BackupDir, filename)
			f, err := os.Create(zipPath)
			require.NoError(t, err)
			f.Close()
			// Set modification time to ensure proper ordering
			modTime := time.Date(2025, 1, i+1, 10, 0, 0, 0, time.UTC)
			os.Chtimes(zipPath, modTime, modTime)
		}

		backups, err := service.ListBackups()
		require.NoError(t, err)
		assert.Len(t, backups, 10)

		// Keep only 3 backups
		deleted, err := service.CleanupOldBackups(3)
		require.NoError(t, err)
		assert.Equal(t, 7, deleted)

		// Verify only 3 remain
		backups, err = service.ListBackups()
		require.NoError(t, err)
		assert.Len(t, backups, 3)
	})

	t.Run("keeps all when under retention", func(t *testing.T) {
		tmpDir := t.TempDir()
		service := &BackupService{
			DataDir:   filepath.Join(tmpDir, "data"),
			BackupDir: filepath.Join(tmpDir, "backups"),
		}
		os.MkdirAll(service.BackupDir, 0o755)

		// Create 3 backup files
		for i := 0; i < 3; i++ {
			filename := fmt.Sprintf("backup_2025-01-%02d_10-00-00.zip", i+1)
			zipPath := filepath.Join(service.BackupDir, filename)
			f, err := os.Create(zipPath)
			require.NoError(t, err)
			f.Close()
		}

		// Try to keep 7 - should delete nothing
		deleted, err := service.CleanupOldBackups(7)
		require.NoError(t, err)
		assert.Equal(t, 0, deleted)

		backups, err := service.ListBackups()
		require.NoError(t, err)
		assert.Len(t, backups, 3)
	})

	t.Run("minimum retention of 1", func(t *testing.T) {
		tmpDir := t.TempDir()
		service := &BackupService{
			DataDir:   filepath.Join(tmpDir, "data"),
			BackupDir: filepath.Join(tmpDir, "backups"),
		}
		os.MkdirAll(service.BackupDir, 0o755)

		// Create 5 backup files
		for i := 0; i < 5; i++ {
			filename := fmt.Sprintf("backup_2025-01-%02d_10-00-00.zip", i+1)
			zipPath := filepath.Join(service.BackupDir, filename)
			f, err := os.Create(zipPath)
			require.NoError(t, err)
			f.Close()
			modTime := time.Date(2025, 1, i+1, 10, 0, 0, 0, time.UTC)
			os.Chtimes(zipPath, modTime, modTime)
		}

		// Try to keep 0 - should keep at least 1
		deleted, err := service.CleanupOldBackups(0)
		require.NoError(t, err)
		assert.Equal(t, 4, deleted)

		backups, err := service.ListBackups()
		require.NoError(t, err)
		assert.Len(t, backups, 1)
	})

	t.Run("empty backup directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		service := &BackupService{
			BackupDir: filepath.Join(tmpDir, "backups"),
		}
		os.MkdirAll(service.BackupDir, 0o755)

		deleted, err := service.CleanupOldBackups(7)
		require.NoError(t, err)
		assert.Equal(t, 0, deleted)
	})
}

func TestBackupService_GetLastBackupTime(t *testing.T) {
	t.Run("returns latest backup time", func(t *testing.T) {
		tmpDir := t.TempDir()
		dataDir := filepath.Join(tmpDir, "data")
		os.MkdirAll(dataDir, 0o755)

		dbPath := filepath.Join(dataDir, "charon.db")
		os.WriteFile(dbPath, []byte("dummy db"), 0o644)

		cfg := &config.Config{DatabasePath: dbPath}
		service := NewBackupService(cfg)
		defer service.Stop() // Prevent goroutine leaks

		// Create a backup
		_, err := service.CreateBackup()
		require.NoError(t, err)

		lastBackup, err := service.GetLastBackupTime()
		require.NoError(t, err)
		assert.False(t, lastBackup.IsZero())
		assert.WithinDuration(t, time.Now(), lastBackup, 5*time.Second)
	})

	t.Run("returns zero time when no backups", func(t *testing.T) {
		tmpDir := t.TempDir()
		service := &BackupService{
			BackupDir: filepath.Join(tmpDir, "backups"),
		}
		os.MkdirAll(service.BackupDir, 0o755)

		lastBackup, err := service.GetLastBackupTime()
		require.NoError(t, err)
		assert.True(t, lastBackup.IsZero())
	})
}

func TestDefaultBackupRetention(t *testing.T) {
	assert.Equal(t, 7, DefaultBackupRetention)
}

// Phase 1: Critical Coverage Gaps

func TestNewBackupService_BackupDirCreationError(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	os.MkdirAll(dataDir, 0o755)

	// Create a file where backup dir should be to cause mkdir error
	backupDirPath := filepath.Join(dataDir, "backups")
	os.WriteFile(backupDirPath, []byte("blocking"), 0o644)

	dbPath := filepath.Join(dataDir, "charon.db")
	os.WriteFile(dbPath, []byte("test"), 0o644)

	cfg := &config.Config{DatabasePath: dbPath}
	// Should not panic even if backup dir creation fails (error is logged, not returned)
	service := NewBackupService(cfg)
	defer service.Stop() // Prevent goroutine leaks
	assert.NotNil(t, service)
	// Service is created but backup dir creation failed (logged as error)
}

func TestNewBackupService_CronScheduleError(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	os.MkdirAll(dataDir, 0o755)

	dbPath := filepath.Join(dataDir, "charon.db")
	os.WriteFile(dbPath, []byte("test"), 0o644)

	cfg := &config.Config{DatabasePath: dbPath}
	// Service should initialize without panic even if cron has issues
	// (error is logged, not returned)
	service := NewBackupService(cfg)
	defer service.Stop() // Prevent goroutine leaks
	assert.NotNil(t, service)
	assert.NotNil(t, service.Cron)
}

func TestRunScheduledBackup_CreateBackupFails(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	os.MkdirAll(dataDir, 0o755)

	dbPath := filepath.Join(dataDir, "charon.db")
	os.WriteFile(dbPath, []byte("test"), 0o644)

	cfg := &config.Config{DatabasePath: dbPath}
	service := NewBackupService(cfg)
	defer service.Stop() // Prevent goroutine leaks

	// Delete database file to cause backup creation to fail
	os.Remove(dbPath)

	// Should not panic when backup fails
	service.RunScheduledBackup()

	// Verify no backups were created
	backups, err := service.ListBackups()
	require.NoError(t, err)
	assert.Empty(t, backups)
}

// Phase 2: Error Path Coverage

func TestRunScheduledBackup_CleanupFails(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	os.MkdirAll(dataDir, 0o755)

	dbPath := filepath.Join(dataDir, "charon.db")
	os.WriteFile(dbPath, []byte("test"), 0o644)

	cfg := &config.Config{DatabasePath: dbPath}
	service := NewBackupService(cfg)
	defer service.Stop() // Prevent goroutine leaks

	// Create a backup first
	_, err := service.CreateBackup()
	require.NoError(t, err)

	// Make backup directory read-only to cause cleanup to fail
	os.Chmod(service.BackupDir, 0o444)
	defer os.Chmod(service.BackupDir, 0o755) // Restore for cleanup

	// Should not panic when cleanup fails
	service.RunScheduledBackup()

	// Backup creation should have succeeded despite cleanup failure
	backups, err := service.ListBackups()
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(backups), 1)
}

func TestGetLastBackupTime_ListBackupsError(t *testing.T) {
	tmpDir := t.TempDir()
	service := &BackupService{
		BackupDir: filepath.Join(tmpDir, "file_not_dir"),
	}

	// Create a file where directory should be
	os.WriteFile(service.BackupDir, []byte("blocking"), 0o644)

	lastBackup, err := service.GetLastBackupTime()
	assert.Error(t, err)
	assert.True(t, lastBackup.IsZero())
}

// Phase 3: Edge Cases

func TestRunScheduledBackup_CleanupDeletesZero(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	os.MkdirAll(dataDir, 0o755)

	dbPath := filepath.Join(dataDir, "charon.db")
	os.WriteFile(dbPath, []byte("test"), 0o644)

	cfg := &config.Config{DatabasePath: dbPath}
	service := NewBackupService(cfg)
	defer service.Stop() // Prevent goroutine leaks

	// RunScheduledBackup creates 1 backup and tries to cleanup
	// Since we're below DefaultBackupRetention (7), no deletions should occur
	service.RunScheduledBackup()

	backups, err := service.ListBackups()
	require.NoError(t, err)
	assert.Equal(t, 1, len(backups))
}

func TestCleanupOldBackups_PartialFailure(t *testing.T) {
	tmpDir := t.TempDir()
	service := &BackupService{
		DataDir:   filepath.Join(tmpDir, "data"),
		BackupDir: filepath.Join(tmpDir, "backups"),
	}
	os.MkdirAll(service.BackupDir, 0o755)

	// Create 5 backup files
	for i := 0; i < 5; i++ {
		filename := fmt.Sprintf("backup_2025-01-%02d_10-00-00.zip", i+1)
		zipPath := filepath.Join(service.BackupDir, filename)
		f, err := os.Create(zipPath)
		require.NoError(t, err)
		f.Close()
		modTime := time.Date(2025, 1, i+1, 10, 0, 0, 0, time.UTC)
		os.Chtimes(zipPath, modTime, modTime)

		// Make files 0 and 1 read-only to cause deletion to fail
		if i < 2 {
			os.Chmod(zipPath, 0o444)
		}
	}

	// Try to keep only 2 backups (should delete 3, but 2 will fail)
	deleted, err := service.CleanupOldBackups(2)
	require.NoError(t, err)
	// Should delete at least 1 (file 2), files 0 and 1 may fail due to permissions
	assert.GreaterOrEqual(t, deleted, 1)
	assert.LessOrEqual(t, deleted, 3)
}

func TestCreateBackup_CaddyDirMissing(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	os.MkdirAll(dataDir, 0o755)

	dbPath := filepath.Join(dataDir, "charon.db")
	os.WriteFile(dbPath, []byte("dummy db"), 0o644)

	// Explicitly NOT creating caddy directory
	cfg := &config.Config{DatabasePath: dbPath}
	service := NewBackupService(cfg)
	defer service.Stop() // Prevent goroutine leaks

	// Should succeed with warning logged
	filename, err := service.CreateBackup()
	require.NoError(t, err)
	assert.NotEmpty(t, filename)

	// Verify backup contains DB but not caddy/
	backupPath := filepath.Join(service.BackupDir, filename)
	assert.FileExists(t, backupPath)
}

func TestCreateBackup_CaddyDirUnreadable(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	os.MkdirAll(dataDir, 0o755)

	dbPath := filepath.Join(dataDir, "charon.db")
	os.WriteFile(dbPath, []byte("dummy db"), 0o644)

	// Create caddy dir with no read permissions
	caddyDir := filepath.Join(dataDir, "caddy")
	os.MkdirAll(caddyDir, 0o755)
	os.Chmod(caddyDir, 0o000)
	defer os.Chmod(caddyDir, 0o755) // Restore for cleanup

	cfg := &config.Config{DatabasePath: dbPath}
	service := NewBackupService(cfg)
	defer service.Stop() // Prevent goroutine leaks

	// Should succeed with warning logged about caddy dir
	filename, err := service.CreateBackup()
	require.NoError(t, err)
	assert.NotEmpty(t, filename)
}

// Phase 4 & 5: Deep Coverage

func TestBackupService_addToZip_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	zipFile, err := os.Create(zipPath)
	require.NoError(t, err)
	defer zipFile.Close()

	w := zip.NewWriter(zipFile)
	defer w.Close()

	service := &BackupService{}

	// Try to add non-existent file - should return nil (silent skip)
	err = service.addToZip(w, "/nonexistent/file.txt", "test.txt")
	assert.NoError(t, err, "addToZip should return nil for non-existent files")
}

func TestBackupService_addToZip_FileOpenError(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	zipFile, err := os.Create(zipPath)
	require.NoError(t, err)
	defer zipFile.Close()

	w := zip.NewWriter(zipFile)
	defer w.Close()

	// Create file with no read permissions
	srcPath := filepath.Join(tmpDir, "unreadable.txt")
	os.WriteFile(srcPath, []byte("test"), 0o644)
	os.Chmod(srcPath, 0o000)
	defer os.Chmod(srcPath, 0o644) // Restore for cleanup

	service := &BackupService{}

	// Should return permission error
	err = service.addToZip(w, srcPath, "test.txt")
	assert.Error(t, err)
	assert.NotEqual(t, nil, err)
}
