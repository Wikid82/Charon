// Package models defines the database schema and domain types.
package models

import (
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// UserRole represents an authenticated user's privilege tier.
type UserRole string

const (
	// RoleAdmin has full access to all Charon features and management.
	RoleAdmin UserRole = "admin"
	// RoleUser can access the Charon management UI with restricted permissions.
	RoleUser UserRole = "user"
	// RolePassthrough can only authenticate for forward-auth proxy access.
	RolePassthrough UserRole = "passthrough"
)

// IsValid returns true when the role is one of the recognised privilege tiers.
func (r UserRole) IsValid() bool {
	switch r {
	case RoleAdmin, RoleUser, RolePassthrough:
		return true
	}
	return false
}

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
	ID                  uint       `json:"-" gorm:"primaryKey"`
	UUID                string     `json:"uuid" gorm:"uniqueIndex"`
	Email               string     `json:"email" gorm:"uniqueIndex"`
	APIKey              string     `json:"-" gorm:"uniqueIndex"` // For external API access, never exposed in JSON
	PasswordHash        string     `json:"-"`                    // Never serialize password hash
	Name                string     `json:"name"`
	Role                UserRole   `json:"role" gorm:"default:'user'"`
	Enabled             bool       `json:"enabled" gorm:"default:true"`
	FailedLoginAttempts int        `json:"-" gorm:"default:0"`
	LockedUntil         *time.Time `json:"-"`
	LastLogin           *time.Time `json:"last_login,omitempty"`
	SessionVersion      uint       `json:"-" gorm:"default:0"`

	// Changelog / "What's New" tracking
	LastSeenVersion string `json:"last_seen_version" gorm:"default:''"`
	ChangelogOptOut bool   `json:"changelog_opt_out" gorm:"default:false"`

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

// Invite status values stored in User.InviteStatus.
const (
	InviteStatusPending  = "pending"
	InviteStatusAccepted = "accepted"
	InviteStatusExpired  = "expired"
)

// HashPassword returns the bcrypt hash of password at the default cost.
func HashPassword(password string) (string, error) {
	return HashPasswordCost(password, bcrypt.DefaultCost)
}

// HashPasswordCost returns the bcrypt hash of password at the given cost.
func HashPasswordCost(password string, cost int) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", fmt.Errorf("bcrypt password: %w", err)
	}
	return string(hash), nil
}

// SetPassword hashes and sets the user's password.
func (u *User) SetPassword(password string) error {
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	u.PasswordHash = hash
	return nil
}

// CheckPassword compares the provided password with the stored hash.
func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	return err == nil
}

// InviteState is where a user's invite stands at a given moment.
type InviteState int

const (
	// InviteNone means there is no invite token.
	InviteNone InviteState = iota
	// InviteExpired means a token is present but its expiry is unset or has passed.
	InviteExpired
	// InviteClosed means the invite was accepted, is no longer pending, or the account already has a password.
	InviteClosed
	// InviteOpen means the invite can be accepted.
	InviteOpen
)

// InviteStateAt classifies the invite at now. Accept, validate and the pending check share this rule.
// An account with a password is never an open invite, whatever its enabled flag says. The order keeps
// today's responses: a passed expiry is reported first, and only a pending invite with no expiry at
// all fails closed as expired.
func (u *User) InviteStateAt(now time.Time) InviteState {
	if u.InviteToken == "" {
		return InviteNone
	}
	if u.InviteExpires != nil && !u.InviteExpires.After(now) {
		return InviteExpired
	}
	if u.InviteStatus != InviteStatusPending || u.PasswordHash != "" {
		return InviteClosed
	}
	if u.InviteExpires == nil {
		return InviteExpired
	}
	return InviteOpen
}

// HasPendingInvite reports whether the invite can be accepted now.
func (u *User) HasPendingInvite() bool {
	return u.InviteStateAt(time.Now()) == InviteOpen
}

// CanAccessHost determines if the user can access a given proxy host based on their permission mode.
// - allow_all mode: User can access everything EXCEPT hosts in PermittedHosts (blacklist)
// - deny_all mode: User can ONLY access hosts in PermittedHosts (whitelist)
func (u *User) CanAccessHost(hostID uint) bool {
	// Admins always have access
	if u.Role == RoleAdmin {
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
