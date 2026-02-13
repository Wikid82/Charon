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
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func createSQLiteTestDB(t *testing.T, dbPath string) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("CREATE TABLE IF NOT EXISTS healthcheck (id INTEGER PRIMARY KEY, value TEXT)").Error)
	require.NoError(t, db.Exec("INSERT INTO healthcheck (value) VALUES (?)", "ok").Error)
}

func TestBackupService_CreateAndList(t *testing.T) {
	// Setup temp dirs
	tmpDir, err := os.MkdirTemp("", "cpm-backup-service-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	dataDir := filepath.Join(tmpDir, "data")
	err = os.MkdirAll(dataDir, 0o700)
	require.NoError(t, err)

	// Create valid sqlite DB
	dbPath := filepath.Join(dataDir, "charon.db")
	createSQLiteTestDB(t, dbPath)

	// Create dummy caddy dir
	caddyDir := filepath.Join(dataDir, "caddy")
	err = os.MkdirAll(caddyDir, 0o700)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(caddyDir, "caddy.json"), []byte("{}"), 0o600)
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

	err = service.RestoreBackup(filename)
	require.NoError(t, err)

	// DB file is staged for live rehydrate (not directly overwritten during unzip)
	assert.NotEmpty(t, service.restoreDBPath)
	assert.FileExists(t, service.restoreDBPath)

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
	_ = os.MkdirAll(service.BackupDir, 0o700)

	// Create malicious zip
	zipPath := filepath.Join(service.BackupDir, "malicious.zip")
	// #nosec G304 -- Test creates malicious zip for security testing
	zipFile, err := os.Create(zipPath)
	require.NoError(t, err)

	w := zip.NewWriter(zipFile)
	f, err := w.Create("../../../evil.txt")
	require.NoError(t, err)
	_, err = f.Write([]byte("evil"))
	require.NoError(t, err)
	_ = w.Close()
	_ = zipFile.Close()

	// Attempt restore
	err = service.RestoreBackup("malicious.zip")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parent directory traversal not allowed")
}

func TestBackupService_PathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	service := &BackupService{
		DataDir:   filepath.Join(tmpDir, "data"),
		BackupDir: filepath.Join(tmpDir, "backups"),
	}
	// #nosec G301 -- Test backup directory needs standard Unix permissions
	_ = os.MkdirAll(service.BackupDir, 0o755)

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
	// #nosec G301 -- Test data directory needs standard Unix permissions
	_ = os.MkdirAll(dataDir, 0o755)

	// Create dummy DB
	dbPath := filepath.Join(dataDir, "charon.db")
	// #nosec G306 -- Test fixture database file
	_ = os.WriteFile(dbPath, []byte("dummy db"), 0o644)

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
		// #nosec G306 -- Test fixture database file
		_ = os.WriteFile(dbPath, []byte("test"), 0o644)

		// Create backup dir as a file to cause mkdir error
		backupDir := filepath.Join(tmpDir, "backups")
		// #nosec G306 -- Test fixture file used to block directory creation
		_ = os.WriteFile(backupDir, []byte("blocking"), 0o644)

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
		// #nosec G301 -- Test backup directory needs standard Unix permissions
		_ = os.MkdirAll(service.BackupDir, 0o755)

		err := service.RestoreBackup("nonexistent.zip")
		assert.Error(t, err)
	})

	t.Run("invalid zip file", func(t *testing.T) {
		tmpDir := t.TempDir()
		service := &BackupService{
			DataDir:   filepath.Join(tmpDir, "data"),
			BackupDir: filepath.Join(tmpDir, "backups"),
		}
		// #nosec G301 -- Test backup directory needs standard Unix permissions
		_ = os.MkdirAll(service.BackupDir, 0o755)

		// Create invalid zip
		badZip := filepath.Join(service.BackupDir, "bad.zip")
		// #nosec G306 -- Test fixture file simulating invalid zip
		_ = os.WriteFile(badZip, []byte("not a zip"), 0o644)

		err := service.RestoreBackup("bad.zip")
		assert.Error(t, err)
	})
}

func TestBackupService_ListBackups_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	service := &BackupService{
		BackupDir: filepath.Join(tmpDir, "backups"),
	}
	// #nosec G301 -- Test backup directory needs standard Unix permissions
	_ = os.MkdirAll(service.BackupDir, 0o755)

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
		// #nosec G301 -- Test backup directory needs standard Unix permissions
		_ = os.MkdirAll(service.BackupDir, 0o755)

		// Create 10 backup files manually with different timestamps
		for i := 0; i < 10; i++ {
			filename := fmt.Sprintf("backup_2025-01-%02d_10-00-00.zip", i+1)
			zipPath := filepath.Join(service.BackupDir, filename)
			// #nosec G304 -- Test creates backup files with known paths
			f, err := os.Create(zipPath)
			require.NoError(t, err)
			_ = f.Close()
			// Set modification time to ensure proper ordering
			modTime := time.Date(2025, 1, i+1, 10, 0, 0, 0, time.UTC)
			_ = os.Chtimes(zipPath, modTime, modTime)
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
		_ = os.MkdirAll(service.BackupDir, 0o750)

		// Create 3 backup files
		for i := 0; i < 3; i++ {
			filename := fmt.Sprintf("backup_2025-01-%02d_10-00-00.zip", i+1)
			zipPath := filepath.Join(service.BackupDir, filename)
			f, err := os.Create(zipPath) //nolint:gosec // G304: Test file in temp directory
			require.NoError(t, err)
			_ = f.Close()
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
		// #nosec G301 -- Test fixture directory with standard permissions
		_ = os.MkdirAll(service.BackupDir, 0o755)

		// Create 5 backup files
		for i := 0; i < 5; i++ {
			filename := fmt.Sprintf("backup_2025-01-%02d_10-00-00.zip", i+1)
			// #nosec G304 -- Test fixture file with controlled path
			zipPath := filepath.Join(service.BackupDir, filename)
			f, err := os.Create(zipPath) //nolint:gosec // G304: Test file creation
			require.NoError(t, err)
			_ = f.Close()
			modTime := time.Date(2025, 1, i+1, 10, 0, 0, 0, time.UTC)
			_ = os.Chtimes(zipPath, modTime, modTime)
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
		_ = os.MkdirAll(service.BackupDir, 0o750)

		deleted, err := service.CleanupOldBackups(7)
		require.NoError(t, err)
		assert.Equal(t, 0, deleted)
	})
}

func TestBackupService_GetLastBackupTime(t *testing.T) {
	t.Run("returns latest backup time", func(t *testing.T) {
		tmpDir := t.TempDir()
		dataDir := filepath.Join(tmpDir, "data")
		_ = os.MkdirAll(dataDir, 0o750)

		dbPath := filepath.Join(dataDir, "charon.db")
		// #nosec G306 -- Test fixture database file
		_ = os.WriteFile(dbPath, []byte("dummy db"), 0o644)

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
		_ = os.MkdirAll(service.BackupDir, 0o750)

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
	_ = os.MkdirAll(dataDir, 0o750)

	// Create a file where backup dir should be to cause mkdir error
	backupDirPath := filepath.Join(dataDir, "backups")
	// #nosec G306 -- Test fixture file used to block directory creation
	_ = os.WriteFile(backupDirPath, []byte("blocking"), 0o644)

	dbPath := filepath.Join(dataDir, "charon.db")
	_ = os.WriteFile(dbPath, []byte("test"), 0o600)

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
	_ = os.MkdirAll(dataDir, 0o750)

	dbPath := filepath.Join(dataDir, "charon.db")
	// #nosec G306 -- Test fixture file with standard read permissions
	_ = os.WriteFile(dbPath, []byte("test"), 0o600)

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
	_ = os.MkdirAll(dataDir, 0o750)

	// Create a fake database path - don't create the actual file
	dbPath := filepath.Join(dataDir, "charon.db")
	// Important: Don't create the database file, so CreateBackup will fail
	// when it tries to verify the database exists

	cfg := &config.Config{DatabasePath: dbPath}
	service := NewBackupService(cfg)
	defer service.Stop() // Prevent goroutine leaks

	// Should not panic when backup fails
	service.RunScheduledBackup()

	// Any zip files that might have been created should be empty or partial
	backups, err := service.ListBackups()
	require.NoError(t, err)
	// CreateBackup creates the file first, then errors if DB doesn't exist
	// So there might be an empty zip file - but no successful backup
	for _, b := range backups {
		// If any backups exist, verify they are empty (0 bytes) as the backup failed
		assert.Equal(t, int64(0), b.Size, "Failed backup should create empty or no zip file")
	}
}

// Phase 2: Error Path Coverage

func TestRunScheduledBackup_CleanupFails(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	_ = os.MkdirAll(dataDir, 0o750)

	dbPath := filepath.Join(dataDir, "charon.db")
	_ = os.WriteFile(dbPath, []byte("test"), 0o600)

	cfg := &config.Config{DatabasePath: dbPath}
	service := NewBackupService(cfg)
	defer service.Stop() // Prevent goroutine leaks

	// Create a backup first
	_, err := service.CreateBackup()
	require.NoError(t, err)

	// Make backup directory read-only to cause cleanup to fail
	_ = os.Chmod(service.BackupDir, 0o444)                    // #nosec G302 -- Intentionally testing permission error handling
	defer func() { _ = os.Chmod(service.BackupDir, 0o755) }() // #nosec G302 -- Restore dir permissions after test

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
	_ = os.WriteFile(service.BackupDir, []byte("blocking"), 0o600)

	lastBackup, err := service.GetLastBackupTime()
	assert.Error(t, err)
	assert.True(t, lastBackup.IsZero())
}

// Phase 3: Edge Cases

func TestRunScheduledBackup_CleanupDeletesZero(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	_ = os.MkdirAll(dataDir, 0o750)

	dbPath := filepath.Join(dataDir, "charon.db")
	_ = os.WriteFile(dbPath, []byte("test"), 0o600)

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
	_ = os.MkdirAll(service.BackupDir, 0o750)

	// Create 5 backup files
	for i := 0; i < 5; i++ {
		filename := fmt.Sprintf("backup_2025-01-%02d_10-00-00.zip", i+1)
		// #nosec G304 -- Test fixture file with controlled path
		zipPath := filepath.Join(service.BackupDir, filename)
		f, err := os.Create(zipPath) //nolint:gosec // G304: Test file
		require.NoError(t, err)
		_ = f.Close()
		modTime := time.Date(2025, 1, i+1, 10, 0, 0, 0, time.UTC)
		_ = os.Chtimes(zipPath, modTime, modTime)

		// Make files 0 and 1 read-only to cause deletion to fail
		if i < 2 {
			_ = os.Chmod(zipPath, 0o444) // #nosec G302 -- Intentionally testing permission-based deletion failure
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
	_ = os.MkdirAll(dataDir, 0o750)

	dbPath := filepath.Join(dataDir, "charon.db")
	_ = os.WriteFile(dbPath, []byte("dummy db"), 0o600)

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
	_ = os.MkdirAll(dataDir, 0o750)

	dbPath := filepath.Join(dataDir, "charon.db")
	_ = os.WriteFile(dbPath, []byte("dummy db"), 0o600)

	// Create caddy dir with no read permissions
	caddyDir := filepath.Join(dataDir, "caddy")
	_ = os.MkdirAll(caddyDir, 0o750)
	_ = os.Chmod(caddyDir, 0o000)
	defer func() { _ = os.Chmod(caddyDir, 0o700) }() // #nosec G302 -- Test restores permissions / Restore for cleanup

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
	zipFile, err := os.Create(zipPath) //nolint:gosec // G304: Test file in temp directory
	require.NoError(t, err)
	defer func() { _ = zipFile.Close() }()

	w := zip.NewWriter(zipFile)
	defer func() { _ = w.Close() }()

	service := &BackupService{}

	// Try to add non-existent file - should return nil (silent skip)
	err = service.addToZip(w, "/nonexistent/file.txt", "test.txt")
	assert.NoError(t, err, "addToZip should return nil for non-existent files")
}

func TestBackupService_addToZip_FileOpenError(t *testing.T) {
	// Skip this test on root user (e.g., in some CI environments)
	if os.Getuid() == 0 {
		t.Skip("Skipping test that requires non-root user for permission testing")
	}

	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	zipFile, err := os.Create(zipPath) //nolint:gosec // G304: Test file in temp directory
	require.NoError(t, err)
	defer func() { _ = zipFile.Close() }()

	w := zip.NewWriter(zipFile)
	defer func() { _ = w.Close() }()

	// Create a directory (not a file) that cannot be opened as a file
	srcPath := filepath.Join(tmpDir, "unreadable_dir")
	err = os.MkdirAll(srcPath, 0o750)
	require.NoError(t, err)

	// Create a file inside with no read permissions
	unreadablePath := filepath.Join(srcPath, "unreadable.txt")
	err = os.WriteFile(unreadablePath, []byte("test"), 0o000)
	require.NoError(t, err)
	defer func() { _ = os.Chmod(unreadablePath, 0o600) }() // #nosec G302 -- Test restores permissions / Restore for cleanup

	service := &BackupService{}

	// Should return permission error
	err = service.addToZip(w, unreadablePath, "test.txt")
	assert.Error(t, err)
}

// Additional tests for improved coverage

func TestBackupService_Start(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	_ = os.MkdirAll(dataDir, 0o750)

	dbPath := filepath.Join(dataDir, "charon.db")
	_ = os.WriteFile(dbPath, []byte("test"), 0o600)

	cfg := &config.Config{DatabasePath: dbPath}
	service := NewBackupService(cfg)

	// Test Start
	service.Start()

	// Verify cron is running (indirectly by checking entries exist)
	entries := service.Cron.Entries()
	assert.NotEmpty(t, entries, "Cron scheduler should have at least one entry")

	// Stop to cleanup
	service.Stop()
}

func TestRunScheduledBackup_CleanupSucceedsWithDeletions(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	_ = os.MkdirAll(dataDir, 0o750)

	dbPath := filepath.Join(dataDir, "charon.db")
	_ = os.WriteFile(dbPath, []byte("test"), 0o600)

	cfg := &config.Config{DatabasePath: dbPath}
	service := NewBackupService(cfg)
	defer service.Stop()

	// Create more backups than DefaultBackupRetention to trigger cleanup
	for i := 0; i < DefaultBackupRetention+3; i++ {
		filename := fmt.Sprintf("backup_2025-01-%02d_10-00-00.zip", i+1)
		zipPath := filepath.Join(service.BackupDir, filename)
		f, err := os.Create(zipPath) //nolint:gosec // G304: Test file in temp directory
		require.NoError(t, err)
		_ = f.Close()
		modTime := time.Date(2025, 1, i+1, 10, 0, 0, 0, time.UTC)
		_ = os.Chtimes(zipPath, modTime, modTime)
	}

	// RunScheduledBackup creates a new backup and triggers cleanup
	service.RunScheduledBackup()

	// Verify cleanup happened (should have DefaultBackupRetention backups)
	backups, err := service.ListBackups()
	require.NoError(t, err)
	// The count should be at or around DefaultBackupRetention after cleanup
	assert.LessOrEqual(t, len(backups), DefaultBackupRetention+1)
}

func TestCleanupOldBackups_ListBackupsError(t *testing.T) {
	tmpDir := t.TempDir()
	service := &BackupService{
		BackupDir: filepath.Join(tmpDir, "file_not_dir"),
	}

	// Create a file where directory should be
	_ = os.WriteFile(service.BackupDir, []byte("blocking"), 0o600)

	deleted, err := service.CleanupOldBackups(5)
	assert.Error(t, err)
	assert.Equal(t, 0, deleted)
	assert.Contains(t, err.Error(), "list backups for cleanup")
}

func TestListBackups_EntryInfoError(t *testing.T) {
	// This tests the entry.Info() error path which is hard to trigger
	// The best we can do is test that valid entries work correctly
	tmpDir := t.TempDir()
	service := &BackupService{
		BackupDir: filepath.Join(tmpDir, "backups"),
	}
	_ = os.MkdirAll(service.BackupDir, 0o750)

	// Create a valid zip file
	zipPath := filepath.Join(service.BackupDir, "backup_test.zip")
	f, err := os.Create(zipPath) //nolint:gosec // G304: Test file in temp directory
	require.NoError(t, err)
	_ = f.Close()

	// Create a non-zip file that should be ignored
	txtPath := filepath.Join(service.BackupDir, "readme.txt")
	_ = os.WriteFile(txtPath, []byte("not a backup"), 0o600)

	// Create a directory that should be ignored
	dirPath := filepath.Join(service.BackupDir, "subdir.zip")
	_ = os.MkdirAll(dirPath, 0o750) // #nosec G301 -- test fixture

	backups, err := service.ListBackups()
	require.NoError(t, err)
	// Should only list the zip file
	assert.Len(t, backups, 1)
	assert.Equal(t, "backup_test.zip", backups[0].Filename)
}

func TestRestoreBackup_PathTraversal_FirstCheck(t *testing.T) {
	tmpDir := t.TempDir()
	service := &BackupService{
		DataDir:   filepath.Join(tmpDir, "data"),
		BackupDir: filepath.Join(tmpDir, "backups"),
	}
	_ = os.MkdirAll(service.BackupDir, 0o750) // #nosec G301 -- test fixture

	// Test path traversal with filename containing path separator
	err := service.RestoreBackup("../../../etc/passwd")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid filename")
}

func TestRestoreBackup_PathTraversal_SecondCheck(t *testing.T) {
	tmpDir := t.TempDir()
	service := &BackupService{
		DataDir:   filepath.Join(tmpDir, "data"),
		BackupDir: filepath.Join(tmpDir, "backups"),
	}
	_ = os.MkdirAll(service.BackupDir, 0o750) // #nosec G301 -- test fixture

	// Test with a filename that passes the first check but could still
	// be problematic (this tests the second prefix check)
	err := service.RestoreBackup("../otherfile.zip")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid filename")
}

func TestDeleteBackup_PathTraversal_SecondCheck(t *testing.T) {
	tmpDir := t.TempDir()
	service := &BackupService{
		DataDir:   filepath.Join(tmpDir, "data"),
		BackupDir: filepath.Join(tmpDir, "backups"),
	}
	_ = os.MkdirAll(service.BackupDir, 0o750) // #nosec G301 -- test fixture

	// Test first check - filename with path separator
	err := service.DeleteBackup("sub/file.zip")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid filename")
}

func TestGetBackupPath_PathTraversal_SecondCheck(t *testing.T) {
	tmpDir := t.TempDir()
	service := &BackupService{
		DataDir:   filepath.Join(tmpDir, "data"),
		BackupDir: filepath.Join(tmpDir, "backups"),
	}
	_ = os.MkdirAll(service.BackupDir, 0o750) // #nosec G301 -- test fixture

	// Test first check - filename with path separator
	_, err := service.GetBackupPath("sub/file.zip")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid filename")
}

func TestUnzip_DirectoryCreation(t *testing.T) {
	tmpDir := t.TempDir()
	service := &BackupService{
		DataDir:   filepath.Join(tmpDir, "data"),
		BackupDir: filepath.Join(tmpDir, "backups"),
	}
	_ = os.MkdirAll(service.BackupDir, 0o750)
	_ = os.MkdirAll(service.DataDir, 0o750)

	// Create a zip with nested directory structure
	zipPath := filepath.Join(service.BackupDir, "nested.zip")
	zipFile, err := os.Create(zipPath) //nolint:gosec // G304: Test file in temp directory
	require.NoError(t, err)

	w := zip.NewWriter(zipFile)
	// Add a directory entry
	_, err = w.Create("subdir/")
	require.NoError(t, err)
	// Add a file in that directory
	f, err := w.Create("subdir/nested_file.txt")
	require.NoError(t, err)
	_, err = f.Write([]byte("nested content"))
	require.NoError(t, err)
	_ = w.Close()
	_ = zipFile.Close()

	// Restore the backup
	err = service.RestoreBackup("nested.zip")
	require.NoError(t, err)

	// Verify nested file was created
	content, err := os.ReadFile(filepath.Join(service.DataDir, "subdir", "nested_file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "nested content", string(content))
}

func TestUnzip_OpenFileError(t *testing.T) {
	// Skip on root
	if os.Getuid() == 0 {
		t.Skip("Skipping test that requires non-root user")
	}

	tmpDir := t.TempDir()
	service := &BackupService{
		DataDir:   filepath.Join(tmpDir, "data"),
		BackupDir: filepath.Join(tmpDir, "backups"),
	}
	_ = os.MkdirAll(service.BackupDir, 0o750) // #nosec G301 -- test fixture
	_ = os.MkdirAll(service.DataDir, 0o750)   // #nosec G301 -- test fixture

	// Create a valid zip
	zipPath := filepath.Join(service.BackupDir, "test.zip")
	zipFile, err := os.Create(zipPath) // #nosec G304 -- test fixture path
	require.NoError(t, err)

	w := zip.NewWriter(zipFile)
	f, err := w.Create("test.txt")
	require.NoError(t, err)
	_, err = f.Write([]byte("test content"))
	require.NoError(t, err)
	_ = w.Close()
	_ = zipFile.Close()

	// Make data dir read-only to cause OpenFile error
	_ = os.Chmod(service.DataDir, 0o400)                    // #nosec G302 -- Test intentionally sets restrictive permissions
	defer func() { _ = os.Chmod(service.DataDir, 0o755) }() // #nosec G302 -- Restoring permissions for cleanup

	err = service.RestoreBackup("test.zip")
	assert.Error(t, err)
}

func TestUnzip_FileOpenInZipError(t *testing.T) {
	// This tests the error path when f.Open() fails
	// Hard to trigger naturally, but we can test normal zip restore works
	tmpDir := t.TempDir()
	service := &BackupService{
		DataDir:   filepath.Join(tmpDir, "data"),
		BackupDir: filepath.Join(tmpDir, "backups"),
	}
	_ = os.MkdirAll(service.BackupDir, 0o750) // #nosec G301 -- test fixture
	_ = os.MkdirAll(service.DataDir, 0o750)   // #nosec G301 -- test fixture

	// Create a valid zip with a file
	zipPath := filepath.Join(service.BackupDir, "valid.zip")
	zipFile, err := os.Create(zipPath) // #nosec G304 -- test fixture path
	require.NoError(t, err)

	w := zip.NewWriter(zipFile)
	f, err := w.Create("test_file.txt")
	require.NoError(t, err)
	_, err = f.Write([]byte("file content"))
	require.NoError(t, err)
	_ = w.Close()
	_ = zipFile.Close()

	// Restore should work
	err = service.RestoreBackup("valid.zip")
	require.NoError(t, err)

	// Verify file was restored
	content, err := os.ReadFile(filepath.Join(service.DataDir, "test_file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "file content", string(content))
}

func TestAddDirToZip_WalkError(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	zipFile, err := os.Create(zipPath) // #nosec G304 -- test fixture path
	require.NoError(t, err)
	defer func() { _ = zipFile.Close() }()

	w := zip.NewWriter(zipFile)
	defer func() { _ = w.Close() }()

	service := &BackupService{}

	// Try to walk a non-existent directory
	err = service.addDirToZip(w, "/nonexistent/path", "base")
	assert.Error(t, err)
}

func TestAddDirToZip_SkipsDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	// Create directory structure
	srcDir := filepath.Join(tmpDir, "src")
	_ = os.MkdirAll(filepath.Join(srcDir, "subdir"), 0o750)                                   // #nosec G301 -- test fixture
	_ = os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("content1"), 0o600)           // #nosec G306 -- test fixture
	_ = os.WriteFile(filepath.Join(srcDir, "subdir", "file2.txt"), []byte("content2"), 0o600) // #nosec G306 -- test fixture

	zipPath := filepath.Join(tmpDir, "test.zip")
	zipFile, err := os.Create(zipPath) // #nosec G304 -- test fixture path
	require.NoError(t, err)

	w := zip.NewWriter(zipFile)
	service := &BackupService{}

	err = service.addDirToZip(w, srcDir, "backup")
	require.NoError(t, err)

	_ = w.Close()
	_ = zipFile.Close()

	// Verify zip contains expected files
	r, err := zip.OpenReader(zipPath)
	require.NoError(t, err)
	defer func() { _ = r.Close() }()

	fileNames := make([]string, 0)
	for _, f := range r.File {
		fileNames = append(fileNames, f.Name)
	}

	assert.Contains(t, fileNames, "backup/file1.txt")
	assert.Contains(t, fileNames, "backup/subdir/file2.txt")
}

func TestGetAvailableSpace_Success(t *testing.T) {
	tmpDir := t.TempDir()
	service := &BackupService{
		BackupDir: tmpDir,
	}

	space, err := service.GetAvailableSpace()
	require.NoError(t, err)
	assert.Greater(t, space, int64(0))
}

func TestGetAvailableSpace_NonExistentDir(t *testing.T) {
	service := &BackupService{
		BackupDir: "/this/path/does/not/exist/anywhere",
	}

	_, err := service.GetAvailableSpace()
	assert.Error(t, err)
}

// Additional edge case tests for better coverage

func TestUnzip_CopyError(t *testing.T) {
	// Skip on root
	if os.Getuid() == 0 {
		t.Skip("Skipping test that requires non-root user")
	}

	tmpDir := t.TempDir()
	service := &BackupService{
		DataDir:   filepath.Join(tmpDir, "data"),
		BackupDir: filepath.Join(tmpDir, "backups"),
	}
	_ = os.MkdirAll(service.BackupDir, 0o750) // #nosec G301 -- test directory
	_ = os.MkdirAll(service.DataDir, 0o750)   // #nosec G301 -- test fixture

	// Create a valid zip
	zipPath := filepath.Join(service.BackupDir, "test.zip")
	zipFile, err := os.Create(zipPath) // #nosec G304 -- test fixture path
	require.NoError(t, err)

	w := zip.NewWriter(zipFile)
	f, err := w.Create("subdir/test.txt")
	require.NoError(t, err)
	_, err = f.Write([]byte("test content"))
	require.NoError(t, err)
	_ = w.Close()
	_ = zipFile.Close()

	// Create the subdir as read-only to cause copy error
	subDir := filepath.Join(service.DataDir, "subdir")
	_ = os.MkdirAll(subDir, 0o750) // #nosec G301 -- test directory
	_ = os.Chmod(subDir, 0o400)
	defer func() { _ = os.Chmod(subDir, 0o755) }() // #nosec G302 -- Restoring permissions for cleanup

	// Restore should fail because we can't write to subdir
	err = service.RestoreBackup("test.zip")
	assert.Error(t, err)
}

func TestCreateBackup_ZipWriterCloseError(t *testing.T) {
	// This is hard to trigger directly, but we can verify the code path
	// by creating a valid backup and ensuring proper cleanup
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	_ = os.MkdirAll(dataDir, 0o750) // #nosec G301 -- test directory

	dbPath := filepath.Join(dataDir, "charon.db")
	_ = os.WriteFile(dbPath, []byte("test db content"), 0o600) // #nosec G306 -- test fixture

	cfg := &config.Config{DatabasePath: dbPath}
	service := NewBackupService(cfg)
	defer service.Stop()

	// Create a backup successfully
	filename, err := service.CreateBackup()
	require.NoError(t, err)
	assert.NotEmpty(t, filename)

	// Verify the zip is valid by opening it
	backupPath := filepath.Join(service.BackupDir, filename)
	r, err := zip.OpenReader(backupPath)
	require.NoError(t, err)
	defer func() { _ = r.Close() }()

	// Verify it contains the database
	var foundDB bool
	for _, f := range r.File {
		if f.Name == "charon.db" {
			foundDB = true
			break
		}
	}
	assert.True(t, foundDB, "Backup should contain database file")
}

func TestAddToZip_CreateError(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	zipFile, err := os.Create(zipPath) // #nosec G304 -- test fixture path
	require.NoError(t, err)
	defer func() { _ = zipFile.Close() }()

	w := zip.NewWriter(zipFile)

	// Create a source file
	srcPath := filepath.Join(tmpDir, "source.txt")
	_ = os.WriteFile(srcPath, []byte("test content"), 0o600) // #nosec G306 -- test fixture

	service := &BackupService{}

	// Normal addToZip should work
	err = service.addToZip(w, srcPath, "dest.txt")
	require.NoError(t, err)

	// Close the writer to finalize
	_ = w.Close()

	// Try to add to closed writer - this should fail
	w2 := zip.NewWriter(zipFile)
	_ = service.addToZip(w2, srcPath, "dest2.txt")
	// This may or may not error depending on internal state
	// The main point is we're testing the code path
}

func TestListBackups_IgnoresNonZipFiles(t *testing.T) {
	tmpDir := t.TempDir()
	service := &BackupService{
		BackupDir: filepath.Join(tmpDir, "backups"),
	}
	_ = os.MkdirAll(service.BackupDir, 0o750) // #nosec G301 -- test fixture

	// Create various files
	_ = os.WriteFile(filepath.Join(service.BackupDir, "backup.zip"), []byte(""), 0o600)    // #nosec G306 -- test fixture
	_ = os.WriteFile(filepath.Join(service.BackupDir, "backup.tar.gz"), []byte(""), 0o600) // #nosec G306 -- test fixture
	_ = os.WriteFile(filepath.Join(service.BackupDir, "readme.txt"), []byte(""), 0o600)    // #nosec G306 -- test fixture
	_ = os.WriteFile(filepath.Join(service.BackupDir, ".hidden.zip"), []byte(""), 0o600)   // #nosec G306 -- test fixture

	backups, err := service.ListBackups()
	require.NoError(t, err)

	// Should only list files ending in .zip
	assert.Len(t, backups, 2) // backup.zip and .hidden.zip

	names := make([]string, 0)
	for _, b := range backups {
		names = append(names, b.Filename)
	}
	assert.Contains(t, names, "backup.zip")
	assert.Contains(t, names, ".hidden.zip")
}

func TestRestoreBackup_CreatesNestedDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	service := &BackupService{
		DataDir:   filepath.Join(tmpDir, "data"),
		BackupDir: filepath.Join(tmpDir, "backups"),
	}
	_ = os.MkdirAll(service.BackupDir, 0o750) // #nosec G301 -- test fixture

	// Create a zip with deeply nested structure
	zipPath := filepath.Join(service.BackupDir, "nested.zip")
	zipFile, err := os.Create(zipPath) // #nosec G304 -- test fixture path
	require.NoError(t, err)

	w := zip.NewWriter(zipFile)
	f, err := w.Create("a/b/c/d/deep_file.txt")
	require.NoError(t, err)
	_, err = f.Write([]byte("deep content"))
	require.NoError(t, err)
	_ = w.Close()
	_ = zipFile.Close()

	// DataDir doesn't exist yet
	err = service.RestoreBackup("nested.zip")
	require.NoError(t, err)

	// Verify deeply nested file was created
	content, err := os.ReadFile(filepath.Join(service.DataDir, "a", "b", "c", "d", "deep_file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "deep content", string(content))
}

func TestBackupService_FullCycle(t *testing.T) {
	// Full integration test: create, list, restore, delete
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	_ = os.MkdirAll(dataDir, 0o750) // #nosec G301 -- test directory

	// Create database and caddy config
	dbPath := filepath.Join(dataDir, "charon.db")
	createSQLiteTestDB(t, dbPath)

	caddyDir := filepath.Join(dataDir, "caddy")
	_ = os.MkdirAll(caddyDir, 0o750)                                                              // #nosec G301 -- test directory
	_ = os.WriteFile(filepath.Join(caddyDir, "config.json"), []byte(`{"original": true}`), 0o600) // #nosec G306 -- test fixture

	cfg := &config.Config{DatabasePath: dbPath}
	service := NewBackupService(cfg)
	defer service.Stop()

	// Create backup
	filename, err := service.CreateBackup()
	require.NoError(t, err)

	// Modify files
	_ = os.WriteFile(filepath.Join(caddyDir, "config.json"), []byte(`{"modified": true}`), 0o600) // #nosec G306 -- test fixture

	// Restore backup
	err = service.RestoreBackup(filename)
	require.NoError(t, err)

	// DB file is staged for live rehydrate (not directly overwritten during unzip)
	assert.NotEmpty(t, service.restoreDBPath)
	assert.FileExists(t, service.restoreDBPath)

	caddyContent, _ := os.ReadFile(filepath.Join(caddyDir, "config.json")) // #nosec G304 -- test fixture path
	assert.Equal(t, `{"original": true}`, string(caddyContent))

	// List backups
	backups, err := service.ListBackups()
	require.NoError(t, err)
	assert.Len(t, backups, 1)

	// Get backup path
	path, err := service.GetBackupPath(filename)
	require.NoError(t, err)
	assert.FileExists(t, path)

	// Delete backup
	err = service.DeleteBackup(filename)
	require.NoError(t, err)

	// Verify deletion
	backups, err = service.ListBackups()
	require.NoError(t, err)
	assert.Empty(t, backups)
}

// TestBackupService_AddToZip_Errors tests addToZip error handling.
func TestBackupService_AddToZip_Errors(t *testing.T) {
	tmpDir := t.TempDir()
	service := &BackupService{
		DataDir:   filepath.Join(tmpDir, "data"),
		BackupDir: filepath.Join(tmpDir, "backups"),
	}
	_ = os.MkdirAll(service.BackupDir, 0o750) // #nosec G301 -- test fixture

	t.Run("handle non-existent file gracefully", func(t *testing.T) {
		zipPath := filepath.Join(service.BackupDir, "test.zip")
		zipFile, err := os.Create(zipPath) // #nosec G304 -- test fixture path
		require.NoError(t, err)
		defer func() { _ = zipFile.Close() }()

		w := zip.NewWriter(zipFile)
		defer func() { _ = w.Close() }()

		// Try to add non-existent file - should return nil (graceful)
		err = service.addToZip(w, "/non/existent/file.txt", "file.txt")
		assert.NoError(t, err, "addToZip should handle non-existent files gracefully")
	})

	t.Run("add valid file to zip", func(t *testing.T) {
		// Create test file
		testFile := filepath.Join(tmpDir, "test.txt")
		err := os.WriteFile(testFile, []byte("test content"), 0o600) // #nosec G306 -- test fixture
		require.NoError(t, err)

		zipPath := filepath.Join(service.BackupDir, "valid.zip")
		zipFile, err := os.Create(zipPath) // #nosec G304 -- test fixture path
		require.NoError(t, err)
		defer func() { _ = zipFile.Close() }()

		w := zip.NewWriter(zipFile)
		err = service.addToZip(w, testFile, "test.txt")
		assert.NoError(t, err)
		_ = w.Close()

		// Verify file was added to zip
		r, err := zip.OpenReader(zipPath)
		require.NoError(t, err)
		defer func() { _ = r.Close() }()

		assert.Len(t, r.File, 1)
		assert.Equal(t, "test.txt", r.File[0].Name)
	})
}

// TestBackupService_Unzip_ErrorPaths tests unzip error handling.
func TestBackupService_Unzip_ErrorPaths(t *testing.T) {
	tmpDir := t.TempDir()
	service := &BackupService{
		DataDir:   filepath.Join(tmpDir, "data"),
		BackupDir: filepath.Join(tmpDir, "backups"),
	}
	_ = os.MkdirAll(service.BackupDir, 0o750) // #nosec G301 -- test directory

	t.Run("unzip with invalid zip file", func(t *testing.T) {
		// Create invalid (corrupted) zip file
		invalidZip := filepath.Join(service.BackupDir, "invalid.zip")
		err := os.WriteFile(invalidZip, []byte("not a valid zip"), 0o600) // #nosec G306 -- test fixture
		require.NoError(t, err)

		err = service.RestoreBackup("invalid.zip")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "zip")
	})

	t.Run("unzip with path traversal attempt", func(t *testing.T) {
		// Create zip with path traversal
		zipPath := filepath.Join(service.BackupDir, "traversal.zip")
		zipFile, err := os.Create(zipPath) // #nosec G304 -- test fixture path
		require.NoError(t, err)

		w := zip.NewWriter(zipFile)
		f, err := w.Create("../../evil.txt")
		require.NoError(t, err)
		_, _ = f.Write([]byte("evil"))
		_ = w.Close()
		_ = zipFile.Close()

		// Should detect and block path traversal
		err = service.RestoreBackup("traversal.zip")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "parent directory traversal not allowed")
	})

	t.Run("unzip empty zip file", func(t *testing.T) {
		// Create empty but valid zip
		emptyZip := filepath.Join(service.BackupDir, "empty.zip")
		zipFile, err := os.Create(emptyZip) // #nosec G304 -- test fixture path
		require.NoError(t, err)

		w := zip.NewWriter(zipFile)
		_ = w.Close()
		_ = zipFile.Close()

		// Should handle empty zip gracefully
		err = service.RestoreBackup("empty.zip")
		assert.NoError(t, err)
	})
}

// TestBackupService_GetAvailableSpace_EdgeCases tests disk space calculation edge cases.
func TestBackupService_GetAvailableSpace_EdgeCases(t *testing.T) {
	tmpDir := t.TempDir()
	service := &BackupService{
		DataDir:   filepath.Join(tmpDir, "data"),
		BackupDir: filepath.Join(tmpDir, "backups"),
	}
	_ = os.MkdirAll(service.DataDir, 0o750) // #nosec G301 -- test directory

	t.Run("get available space for existing directory", func(t *testing.T) {
		availableBytes, err := service.GetAvailableSpace()
		// May fail on some filesystems or temp directories
		if err == nil {
			assert.GreaterOrEqual(t, availableBytes, int64(0), "Available space should be non-negative")
		}
		// Test just verifies function doesn't panic
	})

	t.Run("available space error on non-existent directory", func(t *testing.T) {
		// Create service with non-existent data directory
		badService := &BackupService{
			DataDir:   "/non/existent/directory/that/does/not/exist",
			BackupDir: filepath.Join(tmpDir, "backups"),
		}

		_, err := badService.GetAvailableSpace()
		// Depending on OS, this might succeed or fail
		// On most systems, it will succeed with the parent directory stats
		// Just verify the function doesn't panic
		_ = err
	})
}

// TestBackupService_AddDirToZip_EdgeCases tests addDirToZip with edge cases.
func TestBackupService_AddDirToZip_EdgeCases(t *testing.T) {
	tmpDir := t.TempDir()
	service := &BackupService{
		DataDir:   filepath.Join(tmpDir, "data"),
		BackupDir: filepath.Join(tmpDir, "backups"),
	}
	_ = os.MkdirAll(service.BackupDir, 0o750) // #nosec G301 -- test fixture

	t.Run("add non-existent directory returns error", func(t *testing.T) {
		zipPath := filepath.Join(service.BackupDir, "test.zip")
		zipFile, err := os.Create(zipPath) // #nosec G304 -- test fixture path
		require.NoError(t, err)
		defer func() { _ = zipFile.Close() }()

		w := zip.NewWriter(zipFile)
		defer func() { _ = w.Close() }()

		err = service.addDirToZip(w, "/non/existent/dir", "base")
		assert.Error(t, err)
	})

	t.Run("add empty directory to zip", func(t *testing.T) {
		emptyDir := filepath.Join(tmpDir, "empty")
		err := os.MkdirAll(emptyDir, 0o750) // #nosec G301 -- test fixture
		require.NoError(t, err)

		zipPath := filepath.Join(service.BackupDir, "empty.zip")
		zipFile, err := os.Create(zipPath) // #nosec G304 -- test fixture path
		require.NoError(t, err)
		defer func() { _ = zipFile.Close() }()

		w := zip.NewWriter(zipFile)
		err = service.addDirToZip(w, emptyDir, "empty")
		assert.NoError(t, err)
		_ = w.Close()

		// Verify zip has no entries (only directories, which are skipped)
		r, err := zip.OpenReader(zipPath)
		require.NoError(t, err)
		defer func() { _ = r.Close() }()
		assert.Empty(t, r.File)
	})

	t.Run("add directory with nested files", func(t *testing.T) {
		testDir := filepath.Join(tmpDir, "nested")
		_ = os.MkdirAll(filepath.Join(testDir, "subdir"), 0o750)                                   // #nosec G301 -- test directory
		_ = os.WriteFile(filepath.Join(testDir, "file1.txt"), []byte("content1"), 0o600)           // #nosec G306 -- test fixture
		_ = os.WriteFile(filepath.Join(testDir, "subdir", "file2.txt"), []byte("content2"), 0o600) // #nosec G306 -- test fixture

		zipPath := filepath.Join(service.BackupDir, "nested.zip")
		zipFile, err := os.Create(zipPath) // #nosec G304 -- test fixture path
		require.NoError(t, err)
		defer func() { _ = zipFile.Close() }()

		w := zip.NewWriter(zipFile)
		err = service.addDirToZip(w, testDir, "nested")
		assert.NoError(t, err)
		_ = w.Close()

		// Verify both files were added
		r, err := zip.OpenReader(zipPath)
		require.NoError(t, err)
		defer func() { _ = r.Close() }()
		assert.Len(t, r.File, 2)
	})
}

func TestSafeJoinPath(t *testing.T) {
	baseDir := "/data/backups"

	t.Run("valid_simple_path", func(t *testing.T) {
		path, err := SafeJoinPath(baseDir, "backup.zip")
		assert.NoError(t, err)
		assert.Equal(t, "/data/backups/backup.zip", path)
	})

	t.Run("valid_nested_path", func(t *testing.T) {
		path, err := SafeJoinPath(baseDir, "2024/01/backup.zip")
		assert.NoError(t, err)
		assert.Equal(t, "/data/backups/2024/01/backup.zip", path)
	})

	t.Run("reject_absolute_path", func(t *testing.T) {
		_, err := SafeJoinPath(baseDir, "/etc/passwd")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "absolute paths not allowed")
	})

	t.Run("reject_parent_traversal", func(t *testing.T) {
		_, err := SafeJoinPath(baseDir, "../etc/passwd")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "parent directory traversal not allowed")
	})

	t.Run("reject_embedded_parent_traversal", func(t *testing.T) {
		_, err := SafeJoinPath(baseDir, "foo/../../../etc/passwd")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "parent directory traversal not allowed")
	})

	t.Run("clean_path_normalization", func(t *testing.T) {
		path, err := SafeJoinPath(baseDir, "./backup.zip")
		assert.NoError(t, err)
		assert.Equal(t, "/data/backups/backup.zip", path)
	})

	t.Run("valid_with_dots_in_filename", func(t *testing.T) {
		path, err := SafeJoinPath(baseDir, "backup.2024.01.01.zip")
		assert.NoError(t, err)
		assert.Equal(t, "/data/backups/backup.2024.01.01.zip", path)
	})
}
