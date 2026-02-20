package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type NotificationProvider struct {
	ID             string     `gorm:"primaryKey" json:"id"`
	Name           string     `json:"name" gorm:"index"`
	Type           string     `json:"type" gorm:"index"`                         // discord, slack, gotify, telegram, generic, webhook
	URL            string     `json:"url"`                                       // The shoutrrr URL or webhook URL
	Token          string     `json:"-"`                                         // Auth token for providers (e.g., Gotify) - never exposed in API
	Engine         string     `json:"engine,omitempty" gorm:"index"`             // legacy_shoutrrr | notify_v1
	Config         string     `json:"config"`                                    // JSON payload template for custom webhooks
	ServiceConfig  string     `json:"service_config,omitempty" gorm:"type:text"` // JSON blob for typed service config
	LegacyURL      string     `json:"legacy_url,omitempty"`                      // Preserved original URL during migration
	Template       string     `json:"template" gorm:"default:minimal"`           // minimal|detailed|custom
	MigrationState string     `json:"migration_state,omitempty" gorm:"index"`    // pending | migrated | failed
	MigrationError string     `json:"migration_error,omitempty" gorm:"type:text"`
	LastMigratedAt *time.Time `json:"last_migrated_at,omitempty"`
	Enabled        bool       `json:"enabled" gorm:"index"`

	// Notification Preferences
	NotifyProxyHosts    bool `json:"notify_proxy_hosts" gorm:"default:true"`
	NotifyRemoteServers bool `json:"notify_remote_servers" gorm:"default:true"`
	NotifyDomains       bool `json:"notify_domains" gorm:"default:true"`
	NotifyCerts         bool `json:"notify_certs" gorm:"default:true"`
	NotifyUptime        bool `json:"notify_uptime" gorm:"default:true"`

	// Security Event Notifications (Provider-based)
	NotifySecurityWAFBlocks     bool `json:"notify_security_waf_blocks" gorm:"default:false"`
	NotifySecurityACLDenies     bool `json:"notify_security_acl_denies" gorm:"default:false"`
	NotifySecurityRateLimitHits bool `json:"notify_security_rate_limit_hits" gorm:"default:false"`

	// Managed Legacy Provider Marker
	ManagedLegacySecurity bool `json:"managed_legacy_security" gorm:"index;default:false"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (n *NotificationProvider) BeforeCreate(tx *gorm.DB) (err error) {
	if n.ID == "" {
		n.ID = uuid.New().String()
	}
	// Set defaults if not explicitly set (though gorm default tag handles DB side)
	// We can't easily distinguish between false and unset for bools here without pointers,
	// but for new creations via API, we can assume the frontend sends what it wants.
	// If we wanted to force defaults in Go:
	// n.NotifyProxyHosts = true ...
	if strings.TrimSpace(n.Template) == "" {
		if strings.TrimSpace(n.Config) != "" {
			n.Template = "custom"
		} else {
			n.Template = "minimal"
		}
	}
	return
}
