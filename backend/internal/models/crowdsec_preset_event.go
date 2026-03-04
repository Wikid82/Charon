package models

import "time"

// CrowdsecPresetEvent captures audit trail for preset pull/apply events.
type CrowdsecPresetEvent struct {
	ID         uint      `gorm:"primarykey" json:"-"`
	Slug       string    `json:"slug"`
	Action     string    `json:"action"`
	Status     string    `json:"status"`
	CacheKey   string    `json:"cache_key"`
	BackupPath string    `json:"backup_path"`
	Error      string    `json:"error,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
