// Package remotestorage defines the interface used to upload backup
// archives to off-host destinations (S3-compatible object storage, SFTP,
// WebDAV, and — landing in a later commit — Dropbox/Google Drive), and the
// concrete per-provider implementations (spec §3.7 Commit 3, §3.5 Issue #32
// Phase 2).
//
// Kept as its own package (rather than living directly in internal/services)
// so the uploader contract can be tested — and later faked — without pulling
// in GORM or the rest of the services package.
package remotestorage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
)

// Uploader is implemented by each remote storage backend (S3, SFTP).
type Uploader interface {
	// Upload copies the file at localPath to remoteKey on the remote target.
	Upload(ctx context.Context, localPath, remoteKey string) error
	// Delete removes remoteKey from the remote target.
	Delete(ctx context.Context, remoteKey string) error
	// List enumerates remote objects under prefix, for retention pruning.
	List(ctx context.Context, prefix string) ([]RemoteObject, error)
	// Test performs a cheap connectivity+auth+write probe.
	Test(ctx context.Context) error
}

// RemoteObject describes a single object/file present on a remote target.
//
// Key is the provider-native locator passed back into Delete: a path for
// S3/SFTP/WebDAV/Dropbox, or an opaque file ID for Google Drive (Commit 3).
// Never assume Key is human-readable or contains the filename.
//
// Name is always the human-readable backup filename (e.g.
// "backup_2026-07-13_03-00-00.zip"), independent of how Key addresses the
// object. Retention-candidate filtering in
// BackupRemoteService.pruneRemoteRetention MUST use Name, never Key, so the
// "backup_*.zip*" convention works identically across every provider
// including Google Drive's opaque IDs (spec §3.2, Issue #32 Phase 2).
type RemoteObject struct {
	Key          string    `json:"key"`
	Name         string    `json:"name"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
}

// New constructs the Uploader implementation for target.Type ("s3" or
// "sftp"), parsing target.ConfigJSON into the type-specific config struct
// and combining it with the already-decrypted secrets map (spec §3.7).
func New(target *models.RemoteStorageTarget, secrets map[string]string) (Uploader, error) {
	if target == nil {
		return nil, fmt.Errorf("remotestorage: target is required")
	}

	switch target.Type {
	case "s3":
		var cfg S3Config
		if target.ConfigJSON != "" {
			if err := json.Unmarshal([]byte(target.ConfigJSON), &cfg); err != nil {
				return nil, fmt.Errorf("remotestorage: parse s3 config: %w", err)
			}
		}
		return newS3Uploader(cfg, S3Secrets{
			AccessKeyID:     secrets["access_key_id"],
			SecretAccessKey: secrets["secret_access_key"],
		})
	case "sftp":
		var cfg SFTPConfig
		if target.ConfigJSON != "" {
			if err := json.Unmarshal([]byte(target.ConfigJSON), &cfg); err != nil {
				return nil, fmt.Errorf("remotestorage: parse sftp config: %w", err)
			}
		}
		return newSFTPUploader(cfg, SFTPSecrets{
			Password:      secrets["password"],
			PrivateKeyPEM: secrets["private_key_pem"],
			Passphrase:    secrets["passphrase"],
		})
	default:
		return nil, fmt.Errorf("remotestorage: unknown remote storage target type %q", target.Type)
	}
}
