package models_test

import (
	"encoding/json"
	"testing"
	"time"

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

// TestRemoteStorageTarget_Type_AcceptsGoogleDriveWithoutTruncation proves the
// Type column was widened (size:10 -> size:20, spec §3.4) so "google_drive"
// (12 chars) round-trips through a real Create/reload without truncation —
// the exact regression the S3/SFTP plan's original size:10 hint would have
// caused on any backend that enforces VARCHAR(n).
func TestRemoteStorageTarget_Type_AcceptsGoogleDriveWithoutTruncation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.RemoteStorageTarget{}))

	target := models.RemoteStorageTarget{Name: "Drive", Type: "google_drive"}
	require.NoError(t, db.Create(&target).Error)

	var reloaded models.RemoteStorageTarget
	require.NoError(t, db.First(&reloaded, target.ID).Error)
	assert.Equal(t, "google_drive", reloaded.Type)
}

// TestRemoteStorageTarget_OAuthFields_SerializeWhenSet proves oauth_status
// and oauth_connected_at (spec §3.4) round-trip through JSON when populated,
// and are omitted entirely (omitempty) for non-OAuth types like s3/sftp so
// existing wire format stays byte-identical (Issue #32 Phase 2 "zero
// behavior change to s3/sftp" goal).
func TestRemoteStorageTarget_OAuthFields_SerializeWhenSet(t *testing.T) {
	connectedAt := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	target := models.RemoteStorageTarget{
		UUID:             "target-uuid",
		Name:             "Dropbox",
		Type:             "dropbox",
		OAuthStatus:      "connected",
		OAuthConnectedAt: &connectedAt,
	}

	raw, err := json.Marshal(target)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))

	assert.Equal(t, "connected", decoded["oauth_status"])
	assert.Contains(t, decoded, "oauth_connected_at")
}

// TestRemoteStorageTarget_OAuthFields_OmittedWhenZero proves the s3/sftp
// byte-identical-wire-format guarantee: a target that never touches OAuth
// fields must not have oauth_status/oauth_connected_at keys in its JSON at
// all (omitempty), not just null/empty-string values.
func TestRemoteStorageTarget_OAuthFields_OmittedWhenZero(t *testing.T) {
	target := models.RemoteStorageTarget{UUID: "target-uuid", Name: "Home NAS", Type: "sftp"}

	raw, err := json.Marshal(target)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))

	assert.NotContains(t, decoded, "oauth_status")
	assert.NotContains(t, decoded, "oauth_connected_at")
}
