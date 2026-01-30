package models

import (
	"time"
)

// SecurityAudit records admin actions or important changes related to security.
type SecurityAudit struct {
	ID            uint      `json:"-" gorm:"primaryKey"`
	UUID          string    `json:"uuid" gorm:"uniqueIndex"`
	Actor         string    `json:"actor" gorm:"index"`
	Action        string    `json:"action"`
	EventCategory string    `json:"event_category" gorm:"index"`
	ResourceID    *uint     `json:"resource_id,omitempty"`
	ResourceUUID  string    `json:"resource_uuid,omitempty" gorm:"index"`
	Details       string    `json:"details" gorm:"type:text"`
	IPAddress     string    `json:"ip_address,omitempty"`
	UserAgent     string    `json:"user_agent,omitempty"`
	CreatedAt     time.Time `json:"created_at" gorm:"index"`
}
