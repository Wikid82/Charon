package models

import (
	"time"
)

// Setting stores global application configuration as key-value pairs.
// Used for system-wide preferences, feature flags, and runtime config.
type Setting struct {
	ID        uint      `json:"-" gorm:"primaryKey"`
	Key       string    `json:"key" gorm:"uniqueIndex"`
	Value     string    `json:"value" gorm:"type:text"`
	Type      string    `json:"type" gorm:"index"`     // "string", "int", "bool", "json"
	Category  string    `json:"category" gorm:"index"` // "general", "security", "caddy", "smtp", etc.
	UpdatedAt time.Time `json:"updated_at"`
}
