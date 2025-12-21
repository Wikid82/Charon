package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// NotificationConfig stores configuration for security notifications.
type NotificationConfig struct {
	ID              string    `gorm:"primaryKey" json:"id"`
	Enabled         bool      `json:"enabled"`
	MinLogLevel     string    `json:"min_log_level"` // error, warn, info, debug
	WebhookURL      string    `json:"webhook_url"`
	NotifyWAFBlocks bool      `json:"notify_waf_blocks"`
	NotifyACLDenies bool      `json:"notify_acl_denies"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// BeforeCreate sets the ID if not already set.
func (nc *NotificationConfig) BeforeCreate(tx *gorm.DB) error {
	if nc.ID == "" {
		nc.ID = uuid.New().String()
	}
	return nil
}

// SecurityEvent represents a security event for notification dispatch.
type SecurityEvent struct {
	EventType string         `json:"event_type"` // waf_block, acl_deny, etc.
	Severity  string         `json:"severity"`   // error, warn, info
	Message   string         `json:"message"`
	ClientIP  string         `json:"client_ip"`
	Path      string         `json:"path"`
	Timestamp time.Time      `json:"timestamp"`
	Metadata  map[string]any `json:"metadata"`
}
