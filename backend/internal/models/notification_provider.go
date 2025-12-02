package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type NotificationProvider struct {
	ID       string `gorm:"primaryKey" json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`                            // discord, slack, gotify, telegram, generic, webhook
	URL      string `json:"url"`                             // The shoutrrr URL or webhook URL
	Config   string `json:"config"`                          // JSON payload template for custom webhooks
	Template string `json:"template" gorm:"default:minimal"` // minimal|detailed|custom
	Enabled  bool   `json:"enabled"`

	// Notification Preferences
	NotifyProxyHosts    bool `json:"notify_proxy_hosts" gorm:"default:true"`
	NotifyRemoteServers bool `json:"notify_remote_servers" gorm:"default:true"`
	NotifyDomains       bool `json:"notify_domains" gorm:"default:true"`
	NotifyCerts         bool `json:"notify_certs" gorm:"default:true"`
	NotifyUptime        bool `json:"notify_uptime" gorm:"default:true"`

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
