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

func TestBackupJob_TableName(t *testing.T) {
	assert.Equal(t, "backup_jobs", models.BackupJob{}.TableName())
}

func TestBackupJob_BeforeCreate_GeneratesUUID(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.BackupJob{}))

	job := models.BackupJob{
		Type:   "create",
		Status: "pending",
	}
	require.NoError(t, db.Create(&job).Error)
	assert.NotEmpty(t, job.UUID)

	// A caller-supplied UUID must be preserved, never overwritten (mirrors
	// models.BackupRecord.BeforeCreate).
	preset := models.BackupJob{
		UUID:   "22222222-2222-2222-2222-222222222222",
		Type:   "restore",
		Status: "pending",
	}
	require.NoError(t, db.Create(&preset).Error)
	assert.Equal(t, "22222222-2222-2222-2222-222222222222", preset.UUID)
}

// TestBackupJob_JSONTags locks in the exact snake_case wire format the
// frontend depends on (spec §3.1/§3.2.3). ID and ResultJSON are
// deliberately excluded from JSON — ResultJSON is an internal-only
// serialization detail unmarshaled by the handler (spec §3.5), never
// exposed raw.
func TestBackupJob_JSONTags(t *testing.T) {
	job := models.BackupJob{
		ID:           1,
		UUID:         "uuid-1",
		Type:         "create",
		Status:       "running",
		Stage:        "archiving_files",
		Filename:     "backup_x.zip",
		ResultUUID:   "result-uuid",
		ResultJSON:   `{"message":"should not leak"}`,
		ErrorMessage: "",
		ErrorCode:    "",
	}

	raw, err := json.Marshal(job)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))

	assert.NotContains(t, decoded, "ID")
	assert.NotContains(t, decoded, "ResultJSON")
	assert.NotContains(t, decoded, "result_json")
	for _, key := range []string{
		"uuid", "type", "status", "stage", "filename", "result_uuid",
		"created_at", "updated_at",
	} {
		assert.Contains(t, decoded, key)
	}
	// error_message/error_code are omitempty and should be absent when blank.
	assert.NotContains(t, decoded, "error_message")
	assert.NotContains(t, decoded, "error_code")
}

func TestBackupJob_UniqueUUIDIndex(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.BackupJob{}))

	job1 := models.BackupJob{UUID: "dup-uuid", Type: "create", Status: "pending"}
	require.NoError(t, db.Create(&job1).Error)

	job2 := models.BackupJob{UUID: "dup-uuid", Type: "create", Status: "pending"}
	require.Error(t, db.Create(&job2).Error)
}
