package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BackupRecord persists metadata about a single created (or in-flight)
// backup archive, independent of the underlying file's presence on disk.
// Files found on disk without a matching record (pre-upgrade backups) are
// synthesized into legacy list entries by the handler layer; records whose
// file has disappeared are reconciled to Status "deleted" (see spec §3.10).
type BackupRecord struct {
	ID   uint   `json:"-" gorm:"primaryKey"`
	UUID string `json:"uuid" gorm:"uniqueIndex;size:36"`

	Filename string `json:"filename" gorm:"uniqueIndex;not null;size:255"`
	Size     int64  `json:"size"`
	SHA256   string `json:"sha256" gorm:"size:64"` // of the final on-disk file (post-encryption)

	// Type identifies why the backup was created: manual|scheduled|pre_restore|uploaded.
	Type string `json:"type" gorm:"index;size:20"`

	FormatVersion int    `json:"format_version" gorm:"default:2"`
	Encrypted     bool   `json:"encrypted" gorm:"default:false"`
	AppVersion    string `json:"app_version" gorm:"size:50"`

	// Status: completed|failed|deleted|restore_pending|restore_completed|restore_failed.
	Status       string `json:"status" gorm:"index;size:20"`
	ErrorMessage string `json:"error_message,omitempty" gorm:"type:text"`

	RemoteCopies []BackupRemoteCopy `json:"remote_copies,omitempty" gorm:"foreignKey:BackupRecordID"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName specifies the database table name.
func (BackupRecord) TableName() string { return "backup_records" }

// BeforeCreate generates a server-side UUID when one was not already supplied
// (pattern: notification_provider.go:48). Clients never send numeric IDs or
// UUIDs — see CLAUDE.md's IDs convention.
func (b *BackupRecord) BeforeCreate(_ *gorm.DB) (err error) {
	if b.UUID == "" {
		b.UUID = uuid.New().String()
	}
	return nil
}
