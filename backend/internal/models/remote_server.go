package models

import (
	"time"
)

// RemoteServer represents a known backend server that can be selected
// when creating proxy hosts, eliminating manual IP/port entry.
type RemoteServer struct {
	ID          uint       `json:"id" gorm:"primaryKey"`
	UUID        string     `json:"uuid" gorm:"uniqueIndex"`
	Name        string     `json:"name" gorm:"index"`
	Provider    string     `json:"provider"` // e.g., "docker", "vm", "cloud", "manual"
	Host        string     `json:"host"`     // IP address or hostname
	Port        int        `json:"port"`
	Scheme      string     `json:"scheme"` // http/https
	Tags        string     `json:"tags"`   // comma-separated tags for filtering
	Description string     `json:"description"`
	Enabled     bool       `json:"enabled" gorm:"default:true"`
	LastChecked *time.Time `json:"last_checked,omitempty"`
	Reachable   bool       `json:"reachable" gorm:"default:false"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
