package models

import (
	"time"
)

// Location represents a custom path-based proxy configuration within a ProxyHost.
type Location struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	UUID          string    `json:"uuid" gorm:"uniqueIndex;not null"`
	ProxyHostID   uint      `json:"proxy_host_id" gorm:"not null;index"`
	Path          string    `json:"path" gorm:"not null"` // e.g., /api, /admin
	ForwardScheme string    `json:"forward_scheme" gorm:"default:http"`
	ForwardHost   string    `json:"forward_host" gorm:"not null"`
	ForwardPort   int       `json:"forward_port" gorm:"not null"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
