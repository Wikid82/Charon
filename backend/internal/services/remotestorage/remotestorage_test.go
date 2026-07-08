package remotestorage

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRemoteObject_JSONTags locks in the wire schema consumed by retention
// pruning (spec §3.7) and, eventually, any remote-listing API surface.
func TestRemoteObject_JSONTags(t *testing.T) {
	obj := RemoteObject{
		Key:          "backups/backup_x.zip",
		Size:         2048,
		LastModified: time.Date(2026, 7, 8, 3, 0, 0, 0, time.UTC),
	}

	raw, err := json.Marshal(obj)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))

	assert.Equal(t, "backups/backup_x.zip", decoded["key"])
	assert.Equal(t, float64(2048), decoded["size"])
	assert.Contains(t, decoded, "last_modified")
}

// TestNew_S3_NotYetImplemented documents that the s3.go implementation lands
// in Commit 3 (spec §6) — the factory must still compile and return a
// descriptive error today so Commit 3 can build on this package without
// restructuring the call sites that will eventually consume a real Uploader.
func TestNew_S3_NotYetImplemented(t *testing.T) {
	target := &models.RemoteStorageTarget{Type: "s3"}
	uploader, err := New(target, map[string]string{"access_key_id": "x", "secret_access_key": "y"})
	require.Error(t, err)
	assert.Nil(t, uploader)
	assert.Contains(t, err.Error(), "s3")
}

// TestNew_SFTP_NotYetImplemented mirrors TestNew_S3_NotYetImplemented for the
// sftp backend.
func TestNew_SFTP_NotYetImplemented(t *testing.T) {
	target := &models.RemoteStorageTarget{Type: "sftp"}
	uploader, err := New(target, map[string]string{"password": "x"})
	require.Error(t, err)
	assert.Nil(t, uploader)
	assert.Contains(t, err.Error(), "sftp")
}

func TestNew_UnknownType_ReturnsError(t *testing.T) {
	target := &models.RemoteStorageTarget{Type: "ftp"}
	uploader, err := New(target, nil)
	require.Error(t, err)
	assert.Nil(t, uploader)
	assert.Contains(t, err.Error(), "ftp")
}

func TestNew_NilTarget_ReturnsError(t *testing.T) {
	uploader, err := New(nil, nil)
	require.Error(t, err)
	assert.Nil(t, uploader)
}

// TestUploaderInterface_Exists is a compile-time-flavored check: the package
// must export an Uploader interface type per spec §3.7 so Commit 3's s3.go /
// sftp.go implementations, and their fakes in handler tests, can reference it
// without restructuring this package.
func TestUploaderInterface_Exists(t *testing.T) {
	var uploader Uploader
	assert.Nil(t, uploader)
}
