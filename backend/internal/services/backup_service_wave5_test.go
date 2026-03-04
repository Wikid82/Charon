package services

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBackupServiceWave5_Rehydrate_FallbackWhenRestorePathMissing(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o700))
	restoredDBPath := filepath.Join(dataDir, "charon.db")
	createSQLiteTestDB(t, restoredDBPath)

	activeDB, err := gorm.Open(sqlite.Open(filepath.Join(tmpDir, "active.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, activeDB.Exec(`CREATE TABLE healthcheck (id INTEGER PRIMARY KEY, value TEXT)`).Error)

	svc := &BackupService{
		DataDir:       dataDir,
		DatabaseName:  "charon.db",
		restoreDBPath: filepath.Join(tmpDir, "missing-restore.sqlite"),
	}
	require.NoError(t, svc.RehydrateLiveDatabase(activeDB))
}

func TestBackupServiceWave5_Rehydrate_DisableForeignKeysError(t *testing.T) {
	activeDB, dataDir, restoreDBPath := setupRehydrateDBPair(t)

	registerBackupRawErrorHook(t, activeDB, "wave5-disable-fk", func(tx *gorm.DB) bool {
		return backupSQLContains(tx, "pragma foreign_keys = off")
	})

	svc := &BackupService{DataDir: dataDir, DatabaseName: "charon.db", restoreDBPath: restoreDBPath}
	err := svc.RehydrateLiveDatabase(activeDB)
	require.Error(t, err)
	require.Contains(t, err.Error(), "disable foreign keys")
}

func TestBackupServiceWave5_Rehydrate_ClearTableError(t *testing.T) {
	activeDB, dataDir, restoreDBPath := setupRehydrateDBPair(t)

	registerBackupRawErrorHook(t, activeDB, "wave5-clear-users", func(tx *gorm.DB) bool {
		return backupSQLContains(tx, "delete from \"users\"")
	})

	svc := &BackupService{DataDir: dataDir, DatabaseName: "charon.db", restoreDBPath: restoreDBPath}
	err := svc.RehydrateLiveDatabase(activeDB)
	require.Error(t, err)
	require.Contains(t, err.Error(), "clear table users")
}
