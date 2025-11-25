package models

import (
	"time"
)

// ForwardAuthConfig represents the global forward authentication configuration.
// This is stored as structured data to avoid multiple Setting entries.
type ForwardAuthConfig struct {
	ID                 uint      `json:"id" gorm:"primaryKey"`
	Provider           string    `json:"provider" gorm:"not null"` // "authelia", "authentik", "pomerium", "custom"
	Address            string    `json:"address" gorm:"not null"`  // e.g., "http://authelia:9091/api/verify"
	TrustForwardHeader bool      `json:"trust_forward_header" gorm:"default:true"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}
