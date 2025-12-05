package models

import (
	"time"

	"golang.org/x/crypto/bcrypt"
)

// PermissionMode determines how user access to proxy hosts is evaluated.
type PermissionMode string

const (
	// PermissionModeAllowAll grants access to all hosts except those in the exception list.
	PermissionModeAllowAll PermissionMode = "allow_all"
	// PermissionModeDenyAll denies access to all hosts except those in the exception list.
	PermissionModeDenyAll PermissionMode = "deny_all"
)

// User represents authenticated users with role-based access control.
// Supports local auth, SSO integration, and invite-based onboarding.
type User struct {
	ID                  uint       `json:"id" gorm:"primaryKey"`
	UUID                string     `json:"uuid" gorm:"uniqueIndex"`
	Email               string     `json:"email" gorm:"uniqueIndex"`
	APIKey              string     `json:"api_key" gorm:"uniqueIndex"` // For external API access
	PasswordHash        string     `json:"-"`                          // Never serialize password hash
	Name                string     `json:"name"`
	Role                string     `json:"role" gorm:"default:'user'"` // "admin", "user", "viewer"
	Enabled             bool       `json:"enabled" gorm:"default:true"`
	FailedLoginAttempts int        `json:"-" gorm:"default:0"`
	LockedUntil         *time.Time `json:"-"`
	LastLogin           *time.Time `json:"last_login,omitempty"`

	// Invite system fields
	InviteToken   string     `json:"-" gorm:"index"`          // Token sent via email for account setup
	InviteExpires *time.Time `json:"-"`                       // When the invite token expires
	InvitedAt     *time.Time `json:"invited_at,omitempty"`    // When the invite was sent
	InvitedBy     *uint      `json:"invited_by,omitempty"`    // ID of user who sent the invite
	InviteStatus  string     `json:"invite_status,omitempty"` // "pending", "accepted", "expired"

	// Permission system for forward auth / user gateway
	PermissionMode PermissionMode `json:"permission_mode" gorm:"default:'allow_all'"` // "allow_all" or "deny_all"
	PermittedHosts []ProxyHost    `json:"permitted_hosts,omitempty" gorm:"many2many:user_permitted_hosts;"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SetPassword hashes and sets the user's password.
func (u *User) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	return nil
}

// CheckPassword compares the provided password with the stored hash.
func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	return err == nil
}

// HasPendingInvite returns true if the user has a pending invite that hasn't expired.
func (u *User) HasPendingInvite() bool {
	if u.InviteToken == "" || u.InviteExpires == nil {
		return false
	}
	return u.InviteExpires.After(time.Now()) && u.InviteStatus == "pending"
}

// CanAccessHost determines if the user can access a given proxy host based on their permission mode.
// - allow_all mode: User can access everything EXCEPT hosts in PermittedHosts (blacklist)
// - deny_all mode: User can ONLY access hosts in PermittedHosts (whitelist)
func (u *User) CanAccessHost(hostID uint) bool {
	// Admins always have access
	if u.Role == "admin" {
		return true
	}

	// Check if host is in the permitted hosts list
	hostInList := false
	for _, h := range u.PermittedHosts {
		if h.ID == hostID {
			hostInList = true
			break
		}
	}

	switch u.PermissionMode {
	case PermissionModeAllowAll:
		// Allow all except those in the list (blacklist)
		return !hostInList
	case PermissionModeDenyAll:
		// Deny all except those in the list (whitelist)
		return hostInList
	default:
		// Default to allow_all behavior
		return !hostInList
	}
}

// UserPermittedHost is the join table for the many-to-many relationship.
// This is auto-created by GORM but defined here for clarity.
type UserPermittedHost struct {
	UserID      uint `gorm:"primaryKey"`
	ProxyHostID uint `gorm:"primaryKey"`
}
