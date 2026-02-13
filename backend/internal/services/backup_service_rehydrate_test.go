package services

import (
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
