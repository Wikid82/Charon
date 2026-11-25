package services

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/crypto/cryptotest"
	"github.com/Wikid82/charon/backend/internal/models"
)

// seedWebDAVTarget inserts a WebDAV target row with the given raw config JSON,
// bypassing validation so the test can model a target saved before the HTTPS rule.
func seedWebDAVTarget(t *testing.T, db *gorm.DB, name, configJSON, secrets string) *models.RemoteStorageTarget {
	t.Helper()
	target := &models.RemoteStorageTarget{
		UUID:             uuid.NewString(),
		Name:             name,
		Type:             "webdav",
		Enabled:          true,
		ConfigJSON:       configJSON,
		SecretsEncrypted: secrets,
		LastTestStatus:   "never",
	}
	require.NoError(t, db.Create(target).Error)
	return target
}

func TestValidateRemoteTargetConfig_WebDAV_HTTPWithoutFlagRefused(t *testing.T) {
	err := validateRemoteTargetConfig("webdav", RemoteTargetConfig{
		WebDAV: &WebDAVConfig{URL: "http://203.0.113.10/dav/"},
	})
	require.ErrorIs(t, err, ErrWebDAVHTTPSRequired)
	assert.Equal(t, "Use an https:// address, or turn on \"Allow unencrypted HTTP\" for this destination.", err.Error())
}

func TestValidateRemoteTargetConfig_WebDAV_HTTPWithFlagAccepted(t *testing.T) {
	err := validateRemoteTargetConfig("webdav", RemoteTargetConfig{
		WebDAV: &WebDAVConfig{URL: "http://203.0.113.10/dav/", AllowInsecureHTTP: true},
	})
	require.NoError(t, err)
}

func TestValidateRemoteTargetConfig_WebDAV_OtherSchemeRefused(t *testing.T) {
	err := validateRemoteTargetConfig("webdav", RemoteTargetConfig{
		WebDAV: &WebDAVConfig{URL: "ftp://203.0.113.10/dav/", AllowInsecureHTTP: true},
	})
	require.ErrorIs(t, err, ErrWebDAVScheme)
	assert.Equal(t, "WebDAV addresses must start with https:// or http://.", err.Error())
}

func TestValidateRemoteTargetConfig_WebDAV_UpperCaseSchemeNormalised(t *testing.T) {
	err := validateRemoteTargetConfig("webdav", RemoteTargetConfig{
		WebDAV: &WebDAVConfig{URL: "HTTPS://203.0.113.10/dav/"},
	})
	require.NoError(t, err)
}

func TestBackupRemoteService_Create_WebDAVHTTPWithoutFlag_StoresNothing(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, cryptotest.SingleKey(newRemoteServiceCoverageEncryption(t)), t.TempDir())

	_, err := svc.Create("Plain", "webdav", true,
		RemoteTargetConfig{WebDAV: &WebDAVConfig{URL: "http://203.0.113.10/dav/"}},
		RemoteTargetSecrets{Password: "pw"})
	require.ErrorIs(t, err, ErrWebDAVHTTPSRequired)

	var count int64
	require.NoError(t, db.Model(&models.RemoteStorageTarget{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestBackupRemoteService_Update_WebDAVConfigKeepsFlagRule(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, cryptotest.SingleKey(newRemoteServiceCoverageEncryption(t)), t.TempDir())
	target, err := svc.Create("Dav", "webdav", true,
		RemoteTargetConfig{WebDAV: &WebDAVConfig{URL: "https://203.0.113.10/dav/"}},
		RemoteTargetSecrets{Password: "pw"})
	require.NoError(t, err)

	// Keeping the same http URL and flag set is accepted.
	_, err = svc.Update(target.UUID, nil, nil,
		&RemoteTargetConfig{WebDAV: &WebDAVConfig{URL: "http://203.0.113.10/dav/", AllowInsecureHTTP: true}}, nil)
	require.NoError(t, err)

	// Switching to http without the flag is refused and the stored config stays unchanged.
	_, err = svc.Update(target.UUID, nil, nil,
		&RemoteTargetConfig{WebDAV: &WebDAVConfig{URL: "http://203.0.113.10/dav/"}}, nil)
	require.ErrorIs(t, err, ErrWebDAVHTTPSRequired)
	stored, err := svc.Get(target.UUID)
	require.NoError(t, err)
	assert.Contains(t, stored.ConfigJSON, `"allow_insecure_http":true`)
}

func TestMarkExistingInsecureWebDAV_SetsFlagKeepsUnknownKeysAndSecrets(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, cryptotest.SingleKey(newRemoteServiceCoverageEncryption(t)), t.TempDir())
	legacy := `{"webdav":{"url":"http://203.0.113.10/dav/","username":"ann"},"future_key":"kept"}`
	target := seedWebDAVTarget(t, db, "legacy", legacy, "cipher-blob")

	var before models.RemoteStorageTarget
	require.NoError(t, db.Where("uuid = ?", target.UUID).First(&before).Error)
	// Make updated_at observable: move it to a known past instant.
	past := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	require.NoError(t, db.Model(&models.RemoteStorageTarget{}).Where("uuid = ?", target.UUID).
		UpdateColumn("updated_at", past).Error)

	count, err := svc.MarkExistingInsecureWebDAV(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	var after models.RemoteStorageTarget
	require.NoError(t, db.Where("uuid = ?", target.UUID).First(&after).Error)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(after.ConfigJSON), &decoded))
	dav, ok := decoded["webdav"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, dav["allow_insecure_http"])
	assert.Equal(t, "http://203.0.113.10/dav/", dav["url"])
	assert.Equal(t, "ann", dav["username"])
	assert.Equal(t, "kept", decoded["future_key"], "unknown config keys must survive the backfill")
	assert.Equal(t, "cipher-blob", after.SecretsEncrypted, "secrets must not be touched")
	assert.True(t, after.UpdatedAt.Equal(past), "updated_at must not change")
}

func TestMarkExistingInsecureWebDAV_Idempotent(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, cryptotest.SingleKey(newRemoteServiceCoverageEncryption(t)), t.TempDir())
	seedWebDAVTarget(t, db, "legacy", `{"webdav":{"url":"http://203.0.113.10/dav/"}}`, "")

	first, err := svc.MarkExistingInsecureWebDAV(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, first)

	second, err := svc.MarkExistingInsecureWebDAV(context.Background())
	require.NoError(t, err)
	assert.Zero(t, second, "a target already flagged must not be rewritten")
}

func TestMarkExistingInsecureWebDAV_IgnoresHTTPSAndOtherTypes(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, cryptotest.SingleKey(newRemoteServiceCoverageEncryption(t)), t.TempDir())
	seedWebDAVTarget(t, db, "secure", `{"webdav":{"url":"https://203.0.113.10/dav/"}}`, "")
	seedWebDAVTarget(t, db, "broken", `not json`, "")
	s3 := &models.RemoteStorageTarget{UUID: uuid.NewString(), Name: "s3", Type: "s3",
		ConfigJSON: `{"endpoint":"http://203.0.113.10"}`, LastTestStatus: "never"}
	require.NoError(t, db.Create(s3).Error)

	count, err := svc.MarkExistingInsecureWebDAV(context.Background())
	require.NoError(t, err)
	assert.Zero(t, count)

	var stored models.RemoteStorageTarget
	require.NoError(t, db.Where("uuid = ?", s3.UUID).First(&stored).Error)
	assert.Equal(t, s3.ConfigJSON, stored.ConfigJSON)
}

func TestBackupRemoteService_Rehydrate_BackfillsFlag(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, cryptotest.SingleKey(newRemoteServiceCoverageEncryption(t)), t.TempDir())
	target := seedWebDAVTarget(t, db, "restored", `{"webdav":{"url":"http://203.0.113.10/dav/"}}`, "")

	require.NoError(t, svc.Rehydrate(context.Background()))

	var stored models.RemoteStorageTarget
	require.NoError(t, db.Where("uuid = ?", target.UUID).First(&stored).Error)
	assert.Contains(t, stored.ConfigJSON, `"allow_insecure_http":true`)
}

func TestMarkExistingInsecureWebDAV_ReturnsDatabaseError(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, cryptotest.SingleKey(newRemoteServiceCoverageEncryption(t)), t.TempDir())
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	_, err = svc.MarkExistingInsecureWebDAV(context.Background())
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrWebDAVHTTPSRequired))
}
