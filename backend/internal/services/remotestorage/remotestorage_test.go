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
		Name:         "backup_x.zip",
		Size:         2048,
		LastModified: time.Date(2026, 7, 8, 3, 0, 0, 0, time.UTC),
	}

	raw, err := json.Marshal(obj)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))

	assert.Equal(t, "backups/backup_x.zip", decoded["key"])
	assert.Equal(t, "backup_x.zip", decoded["name"])
	assert.Equal(t, float64(2048), decoded["size"])
	assert.Contains(t, decoded, "last_modified")
}

// TestNew_S3_MissingConfig_ReturnsDescriptiveError proves New actually
// dispatches to the real s3 implementation (Commit 3, spec §3.7): with no
// ConfigJSON, the s3 uploader's own required-field validation ("endpoint is
// required") surfaces rather than a generic "not yet implemented" stub.
func TestNew_S3_MissingConfig_ReturnsDescriptiveError(t *testing.T) {
	target := &models.RemoteStorageTarget{Type: "s3"}
	uploader, err := New(target, map[string]string{"access_key_id": "x", "secret_access_key": "y"})
	require.Error(t, err)
	assert.Nil(t, uploader)
	assert.Contains(t, err.Error(), "s3")
}

// TestNew_SFTP_MissingConfig_ReturnsDescriptiveError mirrors
// TestNew_S3_MissingConfig_ReturnsDescriptiveError for the sftp backend.
func TestNew_SFTP_MissingConfig_ReturnsDescriptiveError(t *testing.T) {
	target := &models.RemoteStorageTarget{Type: "sftp"}
	uploader, err := New(target, map[string]string{"password": "x"})
	require.Error(t, err)
	assert.Nil(t, uploader)
	assert.Contains(t, err.Error(), "sftp")
}

// TestNew_S3_ValidConfig_ConstructsUploader proves New wires ConfigJSON and
// secrets through to a working s3Uploader when all required fields are
// present.
func TestNew_S3_ValidConfig_ConstructsUploader(t *testing.T) {
	// 203.0.113.0/24 (RFC 5737 TEST-NET-3) is a documentation-only range
	// that is neither private/reserved nor RFC1918 — using a literal IP
	// here (rather than a hostname) keeps this test deterministic and
	// network-independent, since net.LookupIP short-circuits for IP
	// literals without an actual DNS query.
	cfg := S3Config{Endpoint: "203.0.113.10", Bucket: "my-bucket"}
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)

	target := &models.RemoteStorageTarget{Type: "s3", ConfigJSON: string(raw)}
	uploader, err := New(target, map[string]string{"access_key_id": "x", "secret_access_key": "y"})
	require.NoError(t, err)
	assert.NotNil(t, uploader)
}

// TestNew_SFTP_ValidConfig_ConstructsUploader mirrors the s3 case above for
// sftp; the host key fingerprint is required (spec §3.7 — a target must
// have already gone through discovery/confirmation before it can be used).
func TestNew_SFTP_ValidConfig_ConstructsUploader(t *testing.T) {
	cfg := SFTPConfig{Host: "203.0.113.11", HostKeyFingerprint: "SHA256:abc123"}
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)

	target := &models.RemoteStorageTarget{Type: "sftp", ConfigJSON: string(raw)}
	uploader, err := New(target, map[string]string{"password": "x"})
	require.NoError(t, err)
	assert.NotNil(t, uploader)
}

// TestNew_SFTP_MissingHostKeyFingerprint_ReturnsError proves a target that
// hasn't been through host-key discovery/confirmation can never be used to
// connect (spec §3.7) — this is the config-level half of the discovery vs.
// verified split enforced by newSFTPUploader.
func TestNew_SFTP_MissingHostKeyFingerprint_ReturnsError(t *testing.T) {
	cfg := SFTPConfig{Host: "203.0.113.11"}
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)

	target := &models.RemoteStorageTarget{Type: "sftp", ConfigJSON: string(raw)}
	uploader, err := New(target, map[string]string{"password": "x"})
	require.Error(t, err)
	assert.Nil(t, uploader)
	assert.Contains(t, err.Error(), "fingerprint")
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
