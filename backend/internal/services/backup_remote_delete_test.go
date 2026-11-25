package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

func openRemoteTargetDeleteDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := openReferenceEnforcedDB(t)
	require.NoError(t, db.AutoMigrate(&models.RemoteStorageTarget{}, &models.BackupRecord{}, &models.BackupRemoteCopy{}))
	return db
}

// TestBackupRemoteDelete_RemovesUploadHistoryOfTarget covers a target with upload history: the target and its copy
// rows are removed, and the other target and its copies stay.
func TestBackupRemoteDelete_RemovesUploadHistoryOfTarget(t *testing.T) {
	db := openRemoteTargetDeleteDB(t)
	removed := models.RemoteStorageTarget{UUID: uuid.NewString(), Name: "removed", Type: "webdav"}
	kept := models.RemoteStorageTarget{UUID: uuid.NewString(), Name: "kept", Type: "webdav"}
	require.NoError(t, db.Create(&removed).Error)
	require.NoError(t, db.Create(&kept).Error)
	record := models.BackupRecord{UUID: uuid.NewString(), Filename: "backup-1.zip"}
	require.NoError(t, db.Create(&record).Error)
	require.NoError(t, db.Create(&models.BackupRemoteCopy{BackupRecordID: record.ID, RemoteTargetID: removed.ID}).Error)
	require.NoError(t, db.Create(&models.BackupRemoteCopy{BackupRecordID: record.ID, RemoteTargetID: kept.ID}).Error)

	svc := NewBackupRemoteService(db, nil, t.TempDir())
	require.NoError(t, svc.Delete(removed.UUID))

	var targets []models.RemoteStorageTarget
	require.NoError(t, db.Find(&targets).Error)
	require.Len(t, targets, 1)
	assert.Equal(t, kept.ID, targets[0].ID)

	var copies []models.BackupRemoteCopy
	require.NoError(t, db.Find(&copies).Error)
	require.Len(t, copies, 1)
	assert.Equal(t, kept.ID, copies[0].RemoteTargetID)
}

// TestBackupRemoteDelete_UnknownUUIDIsNoOp keeps the existing behaviour for a UUID that matches nothing.
func TestBackupRemoteDelete_UnknownUUIDIsNoOp(t *testing.T) {
	db := openRemoteTargetDeleteDB(t)
	svc := NewBackupRemoteService(db, nil, t.TempDir())
	assert.NoError(t, svc.Delete(uuid.NewString()))
}
