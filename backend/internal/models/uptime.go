package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UptimeMonitor struct {
	ID             string     `gorm:"primaryKey" json:"id"`
	ProxyHostID    *uint      `json:"proxy_host_id" gorm:"index"`                         // Optional link to proxy host
	ProxyHost      *ProxyHost `json:"proxy_host,omitempty" gorm:"foreignKey:ProxyHostID"` // Relationship for automatic loading
	RemoteServerID *uint      `json:"remote_server_id" gorm:"index"`                      // Optional link to remote server
	UptimeHostID   *string    `json:"uptime_host_id" gorm:"index"`                        // Link to parent host for grouping
	Name           string     `json:"name" gorm:"index"`
	Type           string     `json:"type"` // http, tcp, ping
	URL            string     `json:"url"`
	UpstreamHost   string     `json:"upstream_host" gorm:"index"` // The actual backend host/IP (for grouping)
	Interval       int        `json:"interval"`                   // seconds
	Enabled        bool       `json:"enabled" gorm:"index"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`

	// Current Status (Cached)
	Status           string    `json:"status" gorm:"index"` // up, down, maintenance, pending
	LastCheck        time.Time `json:"last_check"`
	Latency          int64     `json:"latency"` // ms
	FailureCount     int       `json:"failure_count"`
	LastStatusChange time.Time `json:"last_status_change"`
	MaxRetries       int       `json:"max_retries" gorm:"default:3"`

	// Notification tracking
	LastNotifiedDown time.Time `json:"last_notified_down"` // Prevent duplicate notifications
	NotifiedInBatch  bool      `json:"notified_in_batch"`  // Was this included in a batch notification?
}

type UptimeHeartbeat struct {
	ID        uint      `gorm:"primaryKey" json:"-"`
	MonitorID string    `json:"monitor_id" gorm:"index;index:idx_heartbeat_lookup,priority:1"`
	Status    string    `json:"status" gorm:"index:idx_heartbeat_lookup,priority:2"`
	Latency   int64     `json:"latency"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at" gorm:"index;index:idx_heartbeat_lookup,priority:3"`
}

func (m *UptimeMonitor) BeforeCreate(tx *gorm.DB) (err error) {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	if m.Status == "" {
		m.Status = "pending"
	}
	return
}
