package models_test

import (
	"encoding/json"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBackupRecord_TableName(t *testing.T) {
	assert.Equal(t, "backup_records", models.BackupRecord{}.TableName())
}

func TestBackupRecord_BeforeCreate_GeneratesUUID(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.BackupRecord{}, &models.BackupRemoteCopy{}, &models.RemoteStorageTarget{}))

	record := models.BackupRecord{
		Filename: "backup_2026-07-08_03-00-00.zip",
		Type:     "manual",
		Status:   "completed",
	}
	require.NoError(t, db.Create(&record).Error)
	assert.NotEmpty(t, record.UUID)

	// A caller-supplied UUID must be preserved, never overwritten.
	preset := models.BackupRecord{
		UUID:     "11111111-1111-1111-1111-111111111111",
		Filename: "backup_2026-07-08_04-00-00.zip",
	}
	require.NoError(t, db.Create(&preset).Error)
	assert.Equal(t, "11111111-1111-1111-1111-111111111111", preset.UUID)
}

// TestBackupRecord_JSONTags locks in the exact snake_case wire format the
// frontend depends on (CLAUDE.md: JSON tags must match what the frontend
// expects). ID is deliberately excluded from JSON.
func TestBackupRecord_JSONTags(t *testing.T) {
	record := models.BackupRecord{
		ID:            1,
		UUID:          "uuid-1",
		Filename:      "backup_x.zip",
		Size:          1024,
		SHA256:        "abc123",
		Type:          "manual",
		FormatVersion: 2,
		Encrypted:     true,
		AppVersion:    "1.42.0",
		Status:        "completed",
	}

	raw, err := json.Marshal(record)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))

	assert.NotContains(t, decoded, "ID")
	for _, key := range []string{
		"uuid", "filename", "size", "sha256", "type",
		"format_version", "encrypted", "app_version", "status",
	} {
		assert.Contains(t, decoded, key)
	}
	// error_message is omitempty and should be absent when blank.
	assert.NotContains(t, decoded, "error_message")
}

func TestBackupRecord_RemoteCopiesForeignKey(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.BackupRecord{}, &models.BackupRemoteCopy{}, &models.RemoteStorageTarget{}))

	record := models.BackupRecord{Filename: "backup_with_copies.zip", Type: "scheduled", Status: "completed"}
	require.NoError(t, db.Create(&record).Error)

	target := models.RemoteStorageTarget{Name: "NAS", Type: "sftp"}
	require.NoError(t, db.Create(&target).Error)

	copy1 := models.BackupRemoteCopy{
		BackupRecordID: record.ID,
		RemoteTargetID: target.ID,
		RemoteKey:      "backups/backup_with_copies.zip",
		Status:         "uploaded",
	}
	require.NoError(t, db.Create(&copy1).Error)

	var loaded models.BackupRecord
	require.NoError(t, db.Preload("RemoteCopies").First(&loaded, record.ID).Error)
	require.Len(t, loaded.RemoteCopies, 1)
	assert.Equal(t, "backups/backup_with_copies.zip", loaded.RemoteCopies[0].RemoteKey)
}
