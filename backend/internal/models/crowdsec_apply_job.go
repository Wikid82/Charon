package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Job states of a background hub preset install (#1615).
const (
	CrowdsecApplyJobQueued    = "queued"
	CrowdsecApplyJobRunning   = "running"
	CrowdsecApplyJobVerifying = "verifying"
	CrowdsecApplyJobSucceeded = "succeeded"
	CrowdsecApplyJobFailed    = "failed"
	CrowdsecApplyJobTimedOut  = "timed_out"
	CrowdsecApplyJobCancelled = "cancelled"
)

// Error codes stored in CrowdsecApplyJob.ErrorCode. The preset_* codes reuse the synchronous apply codes.
const (
	CrowdsecApplyErrInterrupted      = "interrupted"
	CrowdsecApplyErrTimeout          = "preset_apply_timeout"
	CrowdsecApplyErrCacheStale       = "preset_cache_stale"
	CrowdsecApplyErrHubUpdating      = "preset_hub_updating"
	CrowdsecApplyErrLocalModified    = "preset_local_modified"
	CrowdsecApplyErrCSCLIUnavailable = "cscli_unavailable"
)

// Limits on the free-text columns of CrowdsecApplyJob.
const (
	CrowdsecApplyMaxWarnings    = 50
	CrowdsecApplyMaxErrorLength = 500
)

// CrowdsecApplyJob is one background install of a CrowdSec hub preset. Rows are history as well as state:
// finished jobs are kept for a while so the UI can resume and show the last result.
type CrowdsecApplyJob struct {
	ID       string `json:"id" gorm:"primaryKey;size:36"`
	Slug     string `json:"slug" gorm:"size:255;index"`
	ItemType string `json:"item_type" gorm:"size:40"`
	CacheKey string `json:"cache_key" gorm:"size:255"`

	State string `json:"state" gorm:"size:20;index"`
	// Phase is a fixed code (preflight, snapshot, install, verify, reload, rollback) for translated UI text.
	Phase string `json:"phase" gorm:"size:20"`
	// ErrorCode is a machine-readable failure code; Error is a short message that never carries secrets.
	ErrorCode string `json:"error_code,omitempty" gorm:"size:60"`
	Error     string `json:"error,omitempty" gorm:"type:text"`
	// Warnings is a JSON array of strings, capped at CrowdsecApplyMaxWarnings entries.
	Warnings   string `json:"warnings,omitempty" gorm:"type:text"`
	BackupPath string `json:"backup_path,omitempty" gorm:"type:text"`

	ItemsInstalled  int  `json:"items_installed"`
	ItemsTotal      *int `json:"items_total"`
	CancelRequested bool `json:"cancel_requested"`

	CreatedAt  time.Time  `json:"created_at"`
	StartedAt  *time.Time `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// TableName specifies the database table name.
func (CrowdsecApplyJob) TableName() string { return "crowdsec_apply_jobs" }

// BeforeCreate generates the server-side UUID; clients never choose job ids.
func (j *CrowdsecApplyJob) BeforeCreate(_ *gorm.DB) error {
	if j.ID == "" {
		j.ID = uuid.New().String()
	}
	return nil
}

// IsActive reports whether the job has not finished.
func (j *CrowdsecApplyJob) IsActive() bool { return IsCrowdsecApplyJobActive(j.State) }

// IsCrowdsecApplyJobActive reports whether state is queued, running or verifying.
func IsCrowdsecApplyJobActive(state string) bool {
	switch state {
	case CrowdsecApplyJobQueued, CrowdsecApplyJobRunning, CrowdsecApplyJobVerifying:
		return true
	}
	return false
}
