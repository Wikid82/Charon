package models

import (
	"time"
)

// User represents authenticated users with role-based access control.
// Supports local auth, SSO integration planned for later phases.
type User struct {
	ID           uint       `json:"id" gorm:"primaryKey"`
	UUID         string     `json:"uuid" gorm:"uniqueIndex"`
	Email        string     `json:"email" gorm:"uniqueIndex"`
	PasswordHash string     `json:"-"` // Never serialize password hash
	Name         string     `json:"name"`
	Role         string     `json:"role" gorm:"default:'user'"` // "admin", "user", "viewer"
	Enabled      bool       `json:"enabled" gorm:"default:true"`
	LastLogin    *time.Time `json:"last_login,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}
