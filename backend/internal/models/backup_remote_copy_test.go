package models_test

import (
	"encoding/json"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackupRemoteCopy_TableName(t *testing.T) {
	assert.Equal(t, "backup_remote_copies", models.BackupRemoteCopy{}.TableName())
}

// TestBackupRemoteCopy_JSONTags asserts the handler-populated TargetUUID/
// TargetName fields (gorm:"-", not persisted) still serialize, while the
// internal foreign keys and preloaded relation never leak.
func TestBackupRemoteCopy_JSONTags(t *testing.T) {
	copyRow := models.BackupRemoteCopy{
		ID:             1,
		BackupRecordID: 2,
		RemoteTargetID: 3,
		TargetUUID:     "target-uuid",
		TargetName:     "Home NAS",
		RemoteKey:      "backups/backup_x.zip",
		Status:         "uploaded",
	}

	raw, err := json.Marshal(copyRow)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))

	assert.NotContains(t, decoded, "ID")
	assert.NotContains(t, decoded, "BackupRecordID")
	assert.NotContains(t, decoded, "RemoteTargetID")
	assert.NotContains(t, decoded, "RemoteTarget")

	assert.Equal(t, "target-uuid", decoded["target_uuid"])
	assert.Equal(t, "Home NAS", decoded["target_name"])
	assert.Equal(t, "backups/backup_x.zip", decoded["remote_key"])
	assert.Equal(t, "uploaded", decoded["status"])
	assert.NotContains(t, decoded, "error_message")
	assert.NotContains(t, decoded, "uploaded_at")
}
