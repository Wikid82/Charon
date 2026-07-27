package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RemoteStorageTarget stores a configured off-host backup destination
// (S3-compatible, SFTP, WebDAV, Dropbox, or Google Drive). It mirrors the
// DNSProviderCredential secret
// pattern: non-secret config lives in ConfigJSON, secrets are AES-256-GCM
// encrypted (crypto.EncryptionService) into SecretsEncrypted, and neither
// field is ever serialized to JSON — handlers decode ConfigJSON into a typed
// response object and expose only a "secrets_set" boolean derived from
// whether SecretsEncrypted is non-empty.
type RemoteStorageTarget struct {
	ID   uint   `json:"-" gorm:"primaryKey"`
	UUID string `json:"uuid" gorm:"uniqueIndex;size:36"`

	Name string `json:"name" gorm:"not null;size:255"`
	// Type: s3|sftp|webdav|dropbox|google_drive. Widened from size:10 to
	// size:20 (Issue #32 Phase 2) — "google_drive" is 12 characters and the
	// original size:10 hint silently truncated it on any backend that
	// enforces VARCHAR(n) (SQLite itself ignores the hint, but the model
	// must stop lying about the constraint for portability/correctness).
	Type string `json:"type" gorm:"index;not null;size:20"`

	Enabled bool `json:"enabled" gorm:"default:true;index"`

	// ConfigJSON holds non-secret configuration (endpoint/bucket/host/port/...).
	ConfigJSON string `json:"-" gorm:"type:text"`

	// SecretsEncrypted holds the AES-256-GCM encrypted JSON blob of credentials
	// (access keys, passwords, private keys). Never exposed via JSON.
	SecretsEncrypted string `json:"-" gorm:"type:text"`

	// KeyVersion supports encryption key rotation (crypto.RotationService).
	KeyVersion int `json:"key_version" gorm:"default:1"`

	LastTestAt *time.Time `json:"last_test_at,omitempty"`
	// LastTestStatus: ok|failed|never.
	LastTestStatus string `json:"last_test_status" gorm:"size:20"`
	LastError      string `json:"last_error,omitempty" gorm:"type:text"`

	// OAuthStatus: ""|not_connected|connected|revoked — "" for non-OAuth
	// types (s3/sftp/webdav). Promoted to a plaintext column (rather than
	// living only inside SecretsEncrypted) so the list-view status badge
	// can read it without a decrypt round-trip per row — mirrors the
	// existing LastTestStatus/LastTestAt pattern on this same model, for
	// the same reason (Issue #32 Phase 2, spec §3.4).
	OAuthStatus string `json:"oauth_status,omitempty" gorm:"size:20"`
	// OAuthConnectedAt is set when OAuthStatus transitions to "connected".
	OAuthConnectedAt *time.Time `json:"oauth_connected_at,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName specifies the database table name.
func (RemoteStorageTarget) TableName() string { return "remote_storage_targets" }

// BeforeCreate generates a server-side UUID when one was not already supplied
// (pattern: notification_provider.go:48).
func (r *RemoteStorageTarget) BeforeCreate(_ *gorm.DB) (err error) {
	if r.UUID == "" {
		r.UUID = uuid.New().String()
	}
	return nil
}
