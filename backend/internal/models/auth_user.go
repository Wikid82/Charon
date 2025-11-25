package models

import (
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// AuthUser represents a local user for the built-in SSO system
type AuthUser struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	UUID      string    `gorm:"uniqueIndex;not null" json:"uuid"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// User identification
	Username string `gorm:"uniqueIndex;not null" json:"username"`
	Email    string `gorm:"uniqueIndex;not null" json:"email"`
	Name     string `json:"name"` // Full name for display

	// Authentication
	PasswordHash string `gorm:"not null" json:"-"` // Never expose in JSON
	Enabled      bool   `gorm:"default:true" json:"enabled"`

	// Additional emails for linking identities (comma-separated)
	AdditionalEmails string `json:"additional_emails"`

	// Authorization
	Roles string `json:"roles"` // Comma-separated roles (e.g., "admin,user")

	// MFA
	MFAEnabled bool   `json:"mfa_enabled"`
	MFASecret  string `json:"-"` // TOTP secret, never expose

	// Metadata
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

// BeforeCreate generates UUID for new auth users
func (u *AuthUser) BeforeCreate(tx *gorm.DB) error {
	if u.UUID == "" {
		u.UUID = uuid.New().String()
	}
	return nil
}

// SetPassword hashes and sets the user's password
func (u *AuthUser) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	return nil
}

// CheckPassword verifies a password against the stored hash
func (u *AuthUser) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	return err == nil
}

// HasRole checks if the user has a specific role
func (u *AuthUser) HasRole(role string) bool {
	if u.Roles == "" {
		return false
	}
	// Simple contains check for comma-separated roles
	for _, r := range splitRoles(u.Roles) {
		if r == role {
			return true
		}
	}
	return false
}

// splitRoles splits comma-separated roles string
func splitRoles(roles string) []string {
	if roles == "" {
		return []string{}
	}
	var result []string
	for i := 0; i < len(roles); {
		start := i
		for i < len(roles) && roles[i] != ',' {
			i++
		}
		if start < i {
			role := roles[start:i]
			// Trim spaces
			for len(role) > 0 && role[0] == ' ' {
				role = role[1:]
			}
			for len(role) > 0 && role[len(role)-1] == ' ' {
				role = role[:len(role)-1]
			}
			if role != "" {
				result = append(result, role)
			}
		}
		i++ // Skip comma
	}
	return result
}
