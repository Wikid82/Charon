package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UptimeHost represents a unique upstream host/IP that may have multiple services.
// This enables host-level health checks to avoid notification storms when a whole server goes down.
type UptimeHost struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	Host      string    `json:"host" gorm:"uniqueIndex;not null"` // IP address or hostname
	Name      string    `json:"name"`                             // Friendly name (auto-generated or from first service)
	Status    string    `json:"status"`                           // up, down, pending
	LastCheck time.Time `json:"last_check"`
	Latency   int64     `json:"latency"` // ms for ping/TCP check

	// Notification tracking
	LastNotifiedDown     time.Time `json:"last_notified_down"`     // When we last sent DOWN notification
	LastNotifiedUp       time.Time `json:"last_notified_up"`       // When we last sent UP notification
	NotifiedServiceCount int       `json:"notified_service_count"` // Number of services in last notification
	LastStatusChange     time.Time `json:"last_status_change"`     // When status last changed

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (h *UptimeHost) BeforeCreate(tx *gorm.DB) (err error) {
	if h.ID == "" {
		h.ID = uuid.New().String()
	}
	if h.Status == "" {
		h.Status = "pending"
	}
	return
}

// UptimeNotificationEvent tracks notification batches to prevent duplicates
type UptimeNotificationEvent struct {
	ID         string    `gorm:"primaryKey" json:"id"`
	HostID     string    `json:"host_id" gorm:"index"`
	EventType  string    `json:"event_type"`  // down, up, partial_recovery
	MonitorIDs string    `json:"monitor_ids"` // JSON array of monitor IDs included in this notification
	Message    string    `json:"message"`
	SentAt     time.Time `json:"sent_at"`
	CreatedAt  time.Time `json:"created_at"`
}

func (e *UptimeNotificationEvent) BeforeCreate(tx *gorm.DB) (err error) {
	if e.ID == "" {
		e.ID = uuid.New().String()
	}
	return
}
