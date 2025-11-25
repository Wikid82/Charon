package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AuthPolicy represents an access control policy for proxy hosts
type AuthPolicy struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	UUID      string    `gorm:"uniqueIndex;not null" json:"uuid"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Policy identification
	Name        string `gorm:"uniqueIndex;not null" json:"name"`
	Description string `json:"description"`
	Enabled     bool   `gorm:"default:true" json:"enabled"`

	// Access rules
	AllowedRoles   string `json:"allowed_roles"`   // Comma-separated roles (e.g., "admin,user")
	AllowedUsers   string `json:"allowed_users"`   // Comma-separated usernames or emails
	AllowedDomains string `json:"allowed_domains"` // Comma-separated email domains (e.g., "@example.com")

	// Policy settings
	RequireMFA     bool `json:"require_mfa"`
	SessionTimeout int  `json:"session_timeout"` // In seconds, 0 = use default
}

// BeforeCreate generates UUID for new auth policies
func (p *AuthPolicy) BeforeCreate(tx *gorm.DB) error {
	if p.UUID == "" {
		p.UUID = uuid.New().String()
	}
	return nil
}

// IsPublic returns true if this policy allows public access (no restrictions)
func (p *AuthPolicy) IsPublic() bool {
	return p.AllowedRoles == "" && p.AllowedUsers == "" && p.AllowedDomains == ""
}
