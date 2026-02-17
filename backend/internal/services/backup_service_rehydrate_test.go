package services

import (
	"archive/zip"
	"fmt"
	"io"
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

func TestCreateSQLiteSnapshot_InvalidDBPath(t *testing.T) {
	badPath := filepath.Join(t.TempDir(), "missing-parent", "missing.db")
	_, _, err := createSQLiteSnapshot(badPath)
	require.Error(t, err)
}

func TestCheckpointSQLiteDatabase_InvalidDBPath(t *testing.T) {
	badPath := filepath.Join(t.TempDir(), "missing-parent", "missing.db")
	err := checkpointSQLiteDatabase(badPath)
	require.Error(t, err)
}

func TestBackupService_RehydrateLiveDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o700))

	dbPath := filepath.Join(dataDir, "charon.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("PRAGMA journal_mode=WAL").Error)
	require.NoError(t, db.Exec("PRAGMA wal_autocheckpoint=0").Error)
	require.NoError(t, db.AutoMigrate(&models.User{}))

	seedUser := models.User{
		UUID:    uuid.NewString(),
		Email:   "restore-user@example.com",
		Name:    "Restore User",
		Role:    "user",
		Enabled: true,
		APIKey:  uuid.NewString(),
	}
	require.NoError(t, db.Create(&seedUser).Error)

	svc := NewBackupService(&config.Config{DatabasePath: dbPath})
	defer svc.Stop()

	backupFile, err := svc.CreateBackup()
	require.NoError(t, err)

	require.NoError(t, db.Where("1 = 1").Delete(&models.User{}).Error)
	var countAfterDelete int64
	require.NoError(t, db.Model(&models.User{}).Count(&countAfterDelete).Error)
	require.Equal(t, int64(0), countAfterDelete)

	require.NoError(t, svc.RestoreBackup(backupFile))
	require.NoError(t, svc.RehydrateLiveDatabase(db))

	var restoredUsers []models.User
	require.NoError(t, db.Find(&restoredUsers).Error)
	require.Len(t, restoredUsers, 1)
	assert.Equal(t, "restore-user@example.com", restoredUsers[0].Email)
}

func TestBackupService_RehydrateLiveDatabase_FromBackupWithWAL(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o700))

	dbPath := filepath.Join(dataDir, "charon.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("PRAGMA journal_mode=WAL").Error)
	require.NoError(t, db.Exec("PRAGMA wal_autocheckpoint=0").Error)
	require.NoError(t, db.AutoMigrate(&models.User{}))

	seedUser := models.User{
		UUID:    uuid.NewString(),
		Email:   "restore-from-wal@example.com",
		Name:    "Restore From WAL",
		Role:    "user",
		Enabled: true,
		APIKey:  uuid.NewString(),
	}
	require.NoError(t, db.Create(&seedUser).Error)

	walPath := dbPath + "-wal"
	_, err = os.Stat(walPath)
	require.NoError(t, err)

	svc := NewBackupService(&config.Config{DatabasePath: dbPath})
	defer svc.Stop()

	backupName := "backup_with_wal.zip"
	backupPath := filepath.Join(svc.BackupDir, backupName)
	backupFile, err := os.Create(backupPath) // #nosec G304 -- backupPath is built from service BackupDir and fixed test filename
	require.NoError(t, err)
	zipWriter := zip.NewWriter(backupFile)

	addFileToZip := func(sourcePath, zipEntryName string) {
		sourceFile, openErr := os.Open(sourcePath) // #nosec G304 -- sourcePath is provided by test with controlled db/wal paths under TempDir
		require.NoError(t, openErr)
		defer func() {
			_ = sourceFile.Close()
		}()

		zipEntry, createErr := zipWriter.Create(zipEntryName)
		require.NoError(t, createErr)
		_, copyErr := io.Copy(zipEntry, sourceFile)
		require.NoError(t, copyErr)
	}

	addFileToZip(dbPath, svc.DatabaseName)
	addFileToZip(walPath, svc.DatabaseName+"-wal")
	require.NoError(t, zipWriter.Close())
	require.NoError(t, backupFile.Close())

	require.NoError(t, db.Where("1 = 1").Delete(&models.User{}).Error)
	require.NoError(t, svc.RestoreBackup(backupName))
	require.NoError(t, svc.RehydrateLiveDatabase(db))

	var restoredUsers []models.User
	require.NoError(t, db.Find(&restoredUsers).Error)
	require.Len(t, restoredUsers, 1)
	assert.Equal(t, "restore-from-wal@example.com", restoredUsers[0].Email)
}

func TestBackupService_ExtractDatabaseFromBackup_WALCheckpointFailure(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "with-invalid-wal.zip")

	zipFile, err := os.Create(zipPath) //nolint:gosec
	require.NoError(t, err)
	writer := zip.NewWriter(zipFile)

	dbEntry, err := writer.Create("charon.db")
	require.NoError(t, err)
	_, err = dbEntry.Write([]byte("not-a-valid-sqlite-db"))
	require.NoError(t, err)

	walEntry, err := writer.Create("charon.db-wal")
	require.NoError(t, err)
	_, err = walEntry.Write([]byte("not-a-valid-wal"))
	require.NoError(t, err)

	require.NoError(t, writer.Close())
	require.NoError(t, zipFile.Close())

	svc := &BackupService{DatabaseName: "charon.db"}
	_, err = svc.extractDatabaseFromBackup(zipPath)
	require.Error(t, err)
	require.Contains(t, err.Error(), "checkpoint extracted sqlite wal")
}

func TestBackupService_RehydrateLiveDatabase_InvalidRestoreDB(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o700))

	activeDBPath := filepath.Join(dataDir, "charon.db")
	activeDB, err := gorm.Open(sqlite.Open(activeDBPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, activeDB.Exec("CREATE TABLE IF NOT EXISTS healthcheck (id INTEGER PRIMARY KEY, value TEXT)").Error)

	invalidRestorePath := filepath.Join(tmpDir, "invalid-restore.sqlite")
	require.NoError(t, os.WriteFile(invalidRestorePath, []byte("invalid sqlite content"), 0o600))

	svc := &BackupService{
		DataDir:       dataDir,
		DatabaseName:  "charon.db",
		restoreDBPath: invalidRestorePath,
	}

	err = svc.RehydrateLiveDatabase(activeDB)
	require.Error(t, err)
	require.Contains(t, err.Error(), "attach restored database")
}

func TestBackupService_RehydrateLiveDatabase_InvalidTableIdentifier(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o700))

	activeDBPath := filepath.Join(dataDir, "charon.db")
	activeDB, err := gorm.Open(sqlite.Open(activeDBPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, activeDB.Exec("CREATE TABLE \"bad-name\" (id INTEGER PRIMARY KEY, value TEXT)").Error)

	restoreDBPath := filepath.Join(tmpDir, "restore.sqlite")
	restoreDB, err := gorm.Open(sqlite.Open(restoreDBPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, restoreDB.Exec("CREATE TABLE \"bad-name\" (id INTEGER PRIMARY KEY, value TEXT)").Error)
	require.NoError(t, restoreDB.Exec("INSERT INTO \"bad-name\" (value) VALUES (?)", "ok").Error)

	svc := &BackupService{
		DataDir:       dataDir,
		DatabaseName:  "charon.db",
		restoreDBPath: restoreDBPath,
	}

	err = svc.RehydrateLiveDatabase(activeDB)
	require.Error(t, err)
	require.Contains(t, err.Error(), "quote table identifier")
}

func TestBackupService_CreateSQLiteSnapshot_TempDirInvalid(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "charon.db")
	createSQLiteTestDB(t, dbPath)

	originalTmp := os.Getenv("TMPDIR")
	t.Setenv("TMPDIR", filepath.Join(tmpDir, "nonexistent-tmp"))
	defer func() {
		_ = os.Setenv("TMPDIR", originalTmp)
	}()

	_, _, err := createSQLiteSnapshot(dbPath)
	require.Error(t, err)
	require.Contains(t, err.Error(), "create sqlite snapshot file")
}

func TestBackupService_RunScheduledBackup_CreateBackupAndCleanupHooks(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o700))

	cfg := &config.Config{DatabasePath: filepath.Join(dataDir, "charon.db")}
	service := NewBackupService(cfg)
	defer service.Stop()

	createCalls := 0
	cleanupCalls := 0
	service.createBackup = func() (string, error) {
		createCalls++
		return fmt.Sprintf("backup-%d.zip", createCalls), nil
	}
	service.cleanupOld = func(keep int) (int, error) {
		cleanupCalls++
		return 1, nil
	}

	service.RunScheduledBackup()
	require.Equal(t, 1, createCalls)
	require.Equal(t, 1, cleanupCalls)
}
