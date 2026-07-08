package services

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBackupManifest_JSONTags locks in the exact wire schema from spec §3.2 —
// this is the contract every future manifest reader/writer (Commit 3) and the
// frontend depend on.
func TestBackupManifest_JSONTags(t *testing.T) {
	manifest := BackupManifest{
		FormatVersion: 2,
		CreatedAt:     time.Date(2026, 7, 8, 3, 0, 0, 0, time.UTC),
		AppVersion:    "1.42.0",
		BackupType:    "scheduled",
		DatabaseName:  "charon.db",
		Contents: []ManifestEntry{
			{Path: "charon.db", Size: 4096, SHA256: "deadbeef"},
		},
		EncryptionKeyRequired: true,
	}

	raw, err := json.Marshal(manifest)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))

	assert.Equal(t, float64(2), decoded["format_version"])
	assert.Equal(t, "1.42.0", decoded["app_version"])
	assert.Equal(t, "scheduled", decoded["backup_type"])
	assert.Equal(t, "charon.db", decoded["database_name"])
	assert.Equal(t, true, decoded["encryption_key_required"])
	assert.Contains(t, decoded, "created_at")
	assert.Contains(t, decoded, "contents")

	contents, ok := decoded["contents"].([]any)
	require.True(t, ok)
	require.Len(t, contents, 1)
	entry, ok := contents[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "charon.db", entry["path"])
	assert.Equal(t, float64(4096), entry["size"])
	assert.Equal(t, "deadbeef", entry["sha256"])
}

func TestBackupManifest_RoundTrip(t *testing.T) {
	original := BackupManifest{
		FormatVersion: 2,
		CreatedAt:     time.Now().UTC().Truncate(time.Second),
		AppVersion:    "1.0.0",
		BackupType:    "manual",
		DatabaseName:  "charon.db",
		Contents: []ManifestEntry{
			{Path: "caddy/certs/example.crt", Size: 1234, SHA256: "abcd"},
		},
	}

	raw, err := json.Marshal(original)
	require.NoError(t, err)

	var roundTripped BackupManifest
	require.NoError(t, json.Unmarshal(raw, &roundTripped))
	assert.Equal(t, original, roundTripped)
}

// TestChecksumWriter_ComputesSHA256AndSize verifies the streaming
// io.MultiWriter(dest, sha256.New()) pattern: every byte written to the
// checksumWriter must reach dest unmodified, and the final Entry() must
// report the same size/SHA-256 as an independent sha256.Sum256 over the
// written bytes — proving no re-read of dest is required to compute the
// manifest checksum (spec §3.2).
func TestChecksumWriter_ComputesSHA256AndSize(t *testing.T) {
	data := []byte("the quick brown fox jumps over the lazy dog, repeated for good measure")

	var dest bytes.Buffer
	cw := newChecksumWriter(&dest)

	// Write in multiple chunks to exercise incremental hashing.
	mid := len(data) / 2
	n1, err := cw.Write(data[:mid])
	require.NoError(t, err)
	assert.Equal(t, mid, n1)

	n2, err := cw.Write(data[mid:])
	require.NoError(t, err)
	assert.Equal(t, len(data)-mid, n2)

	entry := cw.Entry("charon.db")

	assert.Equal(t, "charon.db", entry.Path)
	assert.Equal(t, int64(len(data)), entry.Size)

	wantSum := sha256.Sum256(data)
	assert.Equal(t, hex.EncodeToString(wantSum[:]), entry.SHA256)

	// dest must have received exactly the written bytes, unmodified.
	assert.Equal(t, data, dest.Bytes())
}

// TestChecksumWriter_EmptyWrite verifies the zero-byte case produces the
// well-known SHA-256 of the empty string rather than a zero-valued/invalid
// digest.
func TestChecksumWriter_EmptyWrite(t *testing.T) {
	var dest bytes.Buffer
	cw := newChecksumWriter(&dest)

	entry := cw.Entry("empty.txt")
	assert.Equal(t, int64(0), entry.Size)
	assert.Equal(t, hex.EncodeToString(sha256.New().Sum(nil)), entry.SHA256)
}

type erroringWriter struct{}

func (erroringWriter) Write([]byte) (int, error) {
	return 0, errors.New("disk full")
}

// TestChecksumWriter_PropagatesWriteError ensures underlying write failures
// (e.g. the zip writer hitting a full disk mid-entry) surface to the caller
// wrapped with context, per CLAUDE.md's fmt.Errorf error-wrapping convention.
func TestChecksumWriter_PropagatesWriteError(t *testing.T) {
	cw := newChecksumWriter(erroringWriter{})

	_, err := cw.Write([]byte("data"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "checksum writer")
}
