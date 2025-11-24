package models

import (
	"time"
)

// AccessList defines IP-based or auth-based access control rules
// that can be applied to proxy hosts.
type AccessList struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	UUID        string    `json:"uuid" gorm:"uniqueIndex"`
	Name        string    `json:"name" gorm:"index"`
	Description string    `json:"description"`
	Type        string    `json:"type"`                   // "allow", "deny", "basic_auth", "forward_auth"
	Rules       string    `json:"rules" gorm:"type:text"` // JSON array of rule definitions
	Enabled     bool      `json:"enabled" gorm:"default:true"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
