package models

import "time"

// BackupRemoteCopy tracks the upload status of one BackupRecord to one
// RemoteStorageTarget. TargetUUID/TargetName are populated by the handler
// layer from the joined RemoteStorageTarget (gorm:"-" — not persisted) so
// API responses never need to leak the target's numeric primary key.
type BackupRemoteCopy struct {
	ID             uint                 `json:"-" gorm:"primaryKey"`
	BackupRecordID uint                 `json:"-" gorm:"index;not null"`
	RemoteTargetID uint                 `json:"-" gorm:"index;not null"`
	RemoteTarget   *RemoteStorageTarget `json:"-" gorm:"foreignKey:RemoteTargetID"`

	TargetUUID string `json:"target_uuid" gorm:"-"`
	TargetName string `json:"target_name" gorm:"-"`

	RemoteKey string `json:"remote_key" gorm:"size:512"` // object key / remote path

	// Status: pending|uploading|uploaded|failed|pruned.
	Status       string `json:"status" gorm:"index;size:20"`
	ErrorMessage string `json:"error_message,omitempty" gorm:"type:text"`

	UploadedAt *time.Time `json:"uploaded_at,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName specifies the database table name.
func (BackupRemoteCopy) TableName() string { return "backup_remote_copies" }
