package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BackupJob tracks a single async create-backup or restore-backup operation
// (this plan's §3.1/§3.2.3 — NS_BINDING_ABORTED remediation; the external
// Issue #32 Phase 2 spec this codebase's other backup comments cite tops out
// at §3.10 today, e.g. backup_service.go:151, so this new work is cited
// against this plan's own section numbers, not a fabricated §3.11 of that
// spec). Decoupled from BackupRecord (no FK) because a create job has no
// BackupRecord until it succeeds, and a restore job never produces a new
// BackupRecord at all.
type BackupJob struct {
	ID   uint   `json:"-" gorm:"primaryKey"`
	UUID string `json:"uuid" gorm:"uniqueIndex;size:36"`

	// Type: "create" | "restore".
	Type string `json:"type" gorm:"index;size:20"`
	// Status: "pending" | "running" | "completed" | "failed".
	Status string `json:"status" gorm:"index;size:20"`
	// Stage is a coarse, human-readable progress label updated at each
	// pipeline checkpoint (§3.3.2). Optional/best-effort — never blocks job
	// completion if an update fails to persist.
	Stage string `json:"stage,omitempty" gorm:"size:40"`

	// Filename: create -> the archive filename once known; restore -> the
	// source archive filename being restored (known at job start).
	Filename string `json:"filename,omitempty" gorm:"size:255"`
	// ResultUUID: create -> the persisted BackupRecord.UUID on success.
	ResultUUID string `json:"result_uuid,omitempty" gorm:"size:36"`
	// ResultJSON: restore -> the serialized *services.RestoreResult on
	// success (unmarshaled by the handler when building the poll response).
	// Unused for create (result is fully described by Filename+ResultUUID).
	// Deliberately excluded from JSON — it's an internal serialization
	// detail the handler decodes into the typed response shape, never
	// exposed raw (spec §3.2.3).
	ResultJSON string `json:"-" gorm:"type:text"`

	ErrorMessage string `json:"error_message,omitempty" gorm:"type:text"`
	// ErrorCode mirrors the existing error_code values already used by
	// respondCreateError/respondRestoreError (backup_insufficient_space,
	// backup_passphrase_invalid, backup_validation_failed,
	// backup_restore_unrecoverable, ...) so the frontend's existing
	// error-code-driven UI copy keeps working unchanged.
	ErrorCode string `json:"error_code,omitempty" gorm:"size:60"`

	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// TableName specifies the database table name.
func (BackupJob) TableName() string { return "backup_jobs" }

// BeforeCreate generates a server-side UUID when one was not already
// supplied (mirrors models.BackupRecord.BeforeCreate, backup_record.go:47).
// Clients never send numeric IDs or UUIDs — see CLAUDE.md's IDs convention.
func (b *BackupJob) BeforeCreate(_ *gorm.DB) (err error) {
	if b.UUID == "" {
		b.UUID = uuid.New().String()
	}
	return nil
}
