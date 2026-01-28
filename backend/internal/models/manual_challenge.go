// Package models defines the database schema and domain types.
package models

import (
	"time"
)

// ChallengeStatus represents the state of a manual DNS challenge.
type ChallengeStatus string

const (
	// ChallengeStatusCreated indicates the challenge has been created but not yet processed.
	ChallengeStatusCreated ChallengeStatus = "created"
	// ChallengeStatusPending indicates the challenge is waiting for DNS propagation.
	ChallengeStatusPending ChallengeStatus = "pending"
	// ChallengeStatusVerifying indicates DNS record was found, verification in progress.
	ChallengeStatusVerifying ChallengeStatus = "verifying"
	// ChallengeStatusVerified indicates the challenge was successfully verified.
	ChallengeStatusVerified ChallengeStatus = "verified"
	// ChallengeStatusExpired indicates the challenge timed out.
	ChallengeStatusExpired ChallengeStatus = "expired"
	// ChallengeStatusFailed indicates the challenge failed.
	ChallengeStatusFailed ChallengeStatus = "failed"
)

// ManualChallenge represents a manual DNS challenge for ACME DNS-01 validation.
// Users manually create the required TXT record at their DNS provider.
type ManualChallenge struct {
	// ID is the primary key (UUIDv4, cryptographically random).
	ID string `json:"id" gorm:"primaryKey;size:36"`

	// ProviderID is the foreign key to the DNS provider.
	ProviderID uint `json:"provider_id" gorm:"index;not null"`

	// UserID is the foreign key for ownership validation.
	UserID uint `json:"user_id" gorm:"index;not null"`

	// FQDN is the fully qualified domain name for the TXT record.
	// Example: "_acme-challenge.example.com"
	FQDN string `json:"fqdn" gorm:"index;not null;size:255"`

	// Token is the ACME challenge token (for identification), never exposed in JSON.
	Token string `json:"-" gorm:"size:255"`

	// Value is the TXT record value that must be created.
	Value string `json:"value" gorm:"not null;size:255"`

	// Status is the current state of the challenge.
	Status ChallengeStatus `json:"status" gorm:"index;not null;size:20;default:'created'"`

	// ErrorMessage stores any error message if the challenge failed.
	ErrorMessage string `json:"error_message,omitempty" gorm:"type:text"`

	// DNSPropagated indicates if the DNS record has been detected.
	DNSPropagated bool `json:"dns_propagated" gorm:"default:false"`

	// CreatedAt is when the challenge was created.
	CreatedAt time.Time `json:"created_at"`

	// ExpiresAt is when the challenge will expire.
	ExpiresAt time.Time `json:"expires_at" gorm:"index"`

	// LastCheckAt is when DNS was last checked for propagation.
	LastCheckAt *time.Time `json:"last_check_at,omitempty"`

	// VerifiedAt is when the challenge was successfully verified.
	VerifiedAt *time.Time `json:"verified_at,omitempty"`
}

// TableName specifies the database table name.
func (ManualChallenge) TableName() string {
	return "manual_challenges"
}

// IsTerminal returns true if the challenge is in a terminal state.
func (c *ManualChallenge) IsTerminal() bool {
	return c.Status == ChallengeStatusVerified ||
		c.Status == ChallengeStatusExpired ||
		c.Status == ChallengeStatusFailed
}

// IsActive returns true if the challenge is in an active state.
func (c *ManualChallenge) IsActive() bool {
	return c.Status == ChallengeStatusCreated ||
		c.Status == ChallengeStatusPending ||
		c.Status == ChallengeStatusVerifying
}

// TimeRemaining returns the duration until the challenge expires.
func (c *ManualChallenge) TimeRemaining() time.Duration {
	remaining := time.Until(c.ExpiresAt)
	if remaining < 0 {
		return 0
	}
	return remaining
}
