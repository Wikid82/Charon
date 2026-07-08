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

func TestRemoteStorageTarget_TableName(t *testing.T) {
	assert.Equal(t, "remote_storage_targets", models.RemoteStorageTarget{}.TableName())
}

func TestRemoteStorageTarget_BeforeCreate_GeneratesUUID(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.RemoteStorageTarget{}))

	target := models.RemoteStorageTarget{Name: "Home NAS", Type: "sftp"}
	require.NoError(t, db.Create(&target).Error)
	assert.NotEmpty(t, target.UUID)
}

// TestRemoteStorageTarget_SecretsNeverSerialized locks in the DNSProviderCredential
// secret pattern (spec §3.4.3): ConfigJSON and SecretsEncrypted must never appear
// in a JSON-encoded response, only a "secrets_set"-style boolean computed by the
// handler layer from whether SecretsEncrypted is populated.
func TestRemoteStorageTarget_SecretsNeverSerialized(t *testing.T) {
	target := models.RemoteStorageTarget{
		ID:               1,
		UUID:             "target-uuid",
		Name:             "Home NAS",
		Type:             "sftp",
		Enabled:          true,
		ConfigJSON:       `{"host":"nas.lan"}`,
		SecretsEncrypted: "ciphertext-blob",
		KeyVersion:       1,
		LastTestStatus:   "ok",
	}

	raw, err := json.Marshal(target)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))

	assert.NotContains(t, decoded, "ID")
	assert.NotContains(t, decoded, "ConfigJSON")
	assert.NotContains(t, decoded, "SecretsEncrypted")
	assert.NotContains(t, decoded, "config")
	assert.NotContains(t, decoded, "secrets_encrypted")

	assert.Equal(t, "target-uuid", decoded["uuid"])
	assert.Equal(t, "Home NAS", decoded["name"])
	assert.Equal(t, "sftp", decoded["type"])
	assert.Equal(t, true, decoded["enabled"])
	assert.Equal(t, "ok", decoded["last_test_status"])
}
