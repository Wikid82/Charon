package models

import (
	"time"
)

// EmergencyToken stores metadata for database-backed emergency access tokens.
// Tokens are stored as bcrypt hashes for security.
type EmergencyToken struct {
	ID               uint       `json:"-" gorm:"primaryKey"`
	TokenHash        string     `json:"-" gorm:"type:text;not null"` // bcrypt hash, never exposed in JSON
	CreatedAt        time.Time  `json:"created_at"`
	ExpiresAt        *time.Time `json:"expires_at"`                                  // NULL = never expires
	ExpirationPolicy string     `json:"expiration_policy" gorm:"type:text;not null"` // "30_days", "60_days", "90_days", "custom", "never"
	CreatedByUserID  *uint      `json:"created_by_user_id"`                          // User who generated token (NULL for env var tokens)
	LastUsedAt       *time.Time `json:"last_used_at"`
	UseCount         int        `json:"use_count" gorm:"default:0"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// TableName specifies the table name for GORM
func (EmergencyToken) TableName() string {
	return "emergency_tokens"
}

// IsExpired checks if the token has expired
func (et *EmergencyToken) IsExpired() bool {
	if et.ExpiresAt == nil {
		return false // Never expires
	}
	return time.Now().After(*et.ExpiresAt)
}

// DaysUntilExpiration returns the number of days until expiration (negative if expired)
func (et *EmergencyToken) DaysUntilExpiration() int {
	if et.ExpiresAt == nil {
		return -1 // Special value for "never expires"
	}
	duration := time.Until(*et.ExpiresAt)
	return int(duration.Hours() / 24)
}
