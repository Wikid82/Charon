package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// NotificationConfig stores configuration for security notifications.
type NotificationConfig struct {
	ID                  string `gorm:"primaryKey" json:"id"`
	Enabled             bool   `json:"enabled"`
	MinLogLevel         string `json:"min_log_level"` // error, warn, info, debug
	WebhookURL          string `json:"webhook_url"`
	// Blocker 2 Fix: API surface uses security_* field names per spec (internal fields remain notify_*)
	NotifyWAFBlocks     bool   `json:"security_waf_enabled"`
	NotifyACLDenies     bool   `json:"security_acl_enabled"`
	NotifyRateLimitHits bool   `json:"security_rate_limit_enabled"`
	EmailRecipients     string `json:"email_recipients"`

	// Legacy destination fields (compatibility, not stored in DB)
	DiscordWebhookURL    string `gorm:"-" json:"discord_webhook_url,omitempty"`
	SlackWebhookURL      string `gorm:"-" json:"slack_webhook_url,omitempty"`
	GotifyURL            string `gorm:"-" json:"gotify_url,omitempty"`
	GotifyToken          string `gorm:"-" json:"gotify_token,omitempty"`
	DestinationAmbiguous bool   `gorm:"-" json:"destination_ambiguous,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
