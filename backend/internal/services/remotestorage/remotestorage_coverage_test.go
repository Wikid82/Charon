package remotestorage

import (
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNew_S3_MalformedConfigJSON_ReturnsParseError proves New wraps a
// json.Unmarshal failure on target.ConfigJSON with a descriptive
// "parse s3 config" error rather than a generic/blank one.
func TestNew_S3_MalformedConfigJSON_ReturnsParseError(t *testing.T) {
	target := &models.RemoteStorageTarget{Type: "s3", ConfigJSON: "not valid json"}
	uploader, err := New(target, nil, nil)
	require.Error(t, err)
	assert.Nil(t, uploader)
	assert.Contains(t, err.Error(), "parse s3 config")
}

// TestNew_SFTP_MalformedConfigJSON_ReturnsParseError mirrors the s3 case
// for the sftp branch.
func TestNew_SFTP_MalformedConfigJSON_ReturnsParseError(t *testing.T) {
	target := &models.RemoteStorageTarget{Type: "sftp", ConfigJSON: "not valid json"}
	uploader, err := New(target, nil, nil)
	require.Error(t, err)
	assert.Nil(t, uploader)
	assert.Contains(t, err.Error(), "parse sftp config")
}

// TestNewSFTPUploader_SSRFRejected proves newSFTPUploader's own SSRF guard
// (distinct from newS3Uploader's, already covered) rejects a loopback/
// metadata host.
func TestNewSFTPUploader_SSRFRejected(t *testing.T) {
	_, err := newSFTPUploader(SFTPConfig{Host: "169.254.169.254", HostKeyFingerprint: "SHA256:abc"}, SFTPSecrets{Password: "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SSRF validation")
}
