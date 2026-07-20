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

// remoteTargetConfigOuter is the subset of the API-facing
// services.RemoteTargetConfig this package needs to unmarshal in order to
// reach the webdav/dropbox/google_drive nested sub-config out of
// target.ConfigJSON (spec §3.2/§3.5). It intentionally mirrors only the
// three new nested fields — the s3/sftp cases below keep parsing their own
// flat config structs directly, unchanged.
type remoteTargetConfigOuter struct {
	WebDAV      *WebDAVConfig      `json:"webdav,omitempty"`
	Dropbox     *DropboxConfig     `json:"dropbox,omitempty"`
	GoogleDrive *GoogleDriveConfig `json:"google_drive,omitempty"`
}

// New constructs the Uploader implementation for target.Type ("s3", "sftp",
// "webdav", "dropbox", or "google_drive"), parsing target.ConfigJSON into
// the type-specific config struct and combining it with the
// already-decrypted secrets map (spec §3.7, §3.5 Commit 3). tokenSaver is
// used only by the two OAuth providers (dropbox/google_drive) to persist a
// transparently-refreshed token back to encrypted storage; s3/sftp/webdav
// ignore it entirely — passing nil from tests/those callers is fine.
func New(target *models.RemoteStorageTarget, secrets map[string]string, tokenSaver TokenSaver) (Uploader, error) {
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
	case "webdav":
		var outer remoteTargetConfigOuter
		if target.ConfigJSON != "" {
			if err := json.Unmarshal([]byte(target.ConfigJSON), &outer); err != nil {
				return nil, fmt.Errorf("remotestorage: parse webdav config: %w", err)
			}
		}
		if outer.WebDAV == nil {
			return nil, fmt.Errorf("remotestorage: webdav config is required")
		}
		return newWebDAVUploader(*outer.WebDAV, WebDAVSecrets{
			Password:    secrets["password"],
			BearerToken: secrets["bearer_token"],
		})
	case "dropbox":
		var outer remoteTargetConfigOuter
		if target.ConfigJSON != "" {
			if err := json.Unmarshal([]byte(target.ConfigJSON), &outer); err != nil {
				return nil, fmt.Errorf("remotestorage: parse dropbox config: %w", err)
			}
		}
		if outer.Dropbox == nil {
			return nil, fmt.Errorf("remotestorage: dropbox config is required")
		}
		return newDropboxUploader(*outer.Dropbox, secretsFromMap(secrets), tokenSaver)
	case "google_drive":
		var outer remoteTargetConfigOuter
		if target.ConfigJSON != "" {
			if err := json.Unmarshal([]byte(target.ConfigJSON), &outer); err != nil {
				return nil, fmt.Errorf("remotestorage: parse google_drive config: %w", err)
			}
		}
		if outer.GoogleDrive == nil {
			return nil, fmt.Errorf("remotestorage: google_drive config is required")
		}
		return newGoogleDriveUploader(*outer.GoogleDrive, secretsFromMap(secrets), tokenSaver)
	default:
		return nil, fmt.Errorf("remotestorage: unknown remote storage target type %q", target.Type)
	}
}

// secretsFromMap adapts the plain map[string]string secrets bag (the shape
// BackupRemoteService.uploaderFor already decrypts into) into a
// RemoteTargetSecrets struct for the OAuth providers, which need more than
// one or two named fields (spec §3.5 Commit 3).
func secretsFromMap(secrets map[string]string) RemoteTargetSecrets {
	return RemoteTargetSecrets{
		OAuthClientSecret: secrets["oauth_client_secret"],
		OAuthAccessToken:  secrets["oauth_access_token"],
		OAuthRefreshToken: secrets["oauth_refresh_token"],
		OAuthExpiresAt:    secrets["oauth_expires_at"],
	}
}
