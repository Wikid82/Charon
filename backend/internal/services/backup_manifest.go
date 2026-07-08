package services

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"time"
)

// BackupManifest describes the contents of a format-v2 backup archive. It is
// written as the LAST entry (manifest.json) in every v2 archive: each
// preceding entry's checksum is accumulated in a single streaming pass via
// newChecksumWriter, so the manifest cannot exist until every other entry has
// already been written (see spec §3.2). Wiring this into CreateBackup happens
// in Commit 3 — this commit only lands the types and the checksum-writer
// foundation, with no behavior change to archive creation.
type BackupManifest struct {
	FormatVersion int             `json:"format_version"` // 2
	CreatedAt     time.Time       `json:"created_at"`
	AppVersion    string          `json:"app_version"`
	BackupType    string          `json:"backup_type"` // manual|scheduled|pre_restore|uploaded
	DatabaseName  string          `json:"database_name"`
	Contents      []ManifestEntry `json:"contents"`
	// EncryptionKeyRequired is true when DB rows encrypted with
	// CHARON_ENCRYPTION_KEY exist (dns_provider_credentials, tunnel_configs,
	// remote_storage_targets, ...) — it powers a restore-time warning that the
	// target host needs the same key (spec §3.6).
	EncryptionKeyRequired bool `json:"encryption_key_required"`
}

// ManifestEntry describes a single file stored inside a backup archive.
type ManifestEntry struct {
	Path   string `json:"path"` // archive-relative path, forward slashes
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"` // hex-encoded
}

// checksumWriter streams every byte written to it into an underlying
// destination writer (typically an in-progress *zip.Writer entry) while
// simultaneously accumulating a running SHA-256 digest and byte count via
// io.MultiWriter(dest, sha256.New()). This lets CreateBackupWithOptions (Commit
// 3) compute a ManifestEntry for each archive member in the same single pass
// that writes it into the zip, with no re-read of already-written entries.
type checksumWriter struct {
	hash hash.Hash
	mw   io.Writer
	size int64
}

// newChecksumWriter wraps dest so every Write is hashed and counted as it
// streams through.
func newChecksumWriter(dest io.Writer) *checksumWriter {
	h := sha256.New()
	return &checksumWriter{
		hash: h,
		mw:   io.MultiWriter(dest, h),
	}
}

// Write implements io.Writer, forwarding bytes to the wrapped destination and
// hash simultaneously.
func (c *checksumWriter) Write(p []byte) (int, error) {
	n, err := c.mw.Write(p)
	c.size += int64(n)
	if err != nil {
		return n, fmt.Errorf("checksum writer: %w", err)
	}
	return n, nil
}

// Entry finalizes the running digest into a ManifestEntry for archivePath.
// Calling Entry does not reset the writer; it is intended to be called once,
// after all bytes for this archive member have been written.
func (c *checksumWriter) Entry(archivePath string) ManifestEntry {
	return ManifestEntry{
		Path:   archivePath,
		Size:   c.size,
		SHA256: hex.EncodeToString(c.hash.Sum(nil)),
	}
}
