// Package models defines the database schema and domain types.
package models

import (
	"time"
)

// DNSProviderCredential represents a zone-specific credential set for a DNS provider.
// This allows different credentials to be used for different domains/zones within the same provider.
type DNSProviderCredential struct {
	ID            uint         `json:"-" gorm:"primaryKey"`
	UUID          string       `json:"uuid" gorm:"uniqueIndex;size:36"`
	DNSProviderID uint         `json:"dns_provider_id" gorm:"index;not null"`
	DNSProvider   *DNSProvider `json:"dns_provider,omitempty" gorm:"foreignKey:DNSProviderID"`

	// Credential metadata
	Label      string `json:"label" gorm:"not null;size:255"`
	ZoneFilter string `json:"zone_filter" gorm:"type:text"` // Comma-separated list of domains (e.g., "example.com,*.example.org")
	Enabled    bool   `json:"enabled" gorm:"default:true;index"`

	// Encrypted credentials (JSON blob, encrypted with AES-256-GCM)
	CredentialsEncrypted string `json:"-" gorm:"type:text;not null"`

	// Encryption key version used for credentials (supports key rotation)
	KeyVersion int `json:"key_version" gorm:"default:1;index"`

	// Propagation settings (overrides provider defaults if non-zero)
	PropagationTimeout int `json:"propagation_timeout" gorm:"default:120"` // seconds
	PollingInterval    int `json:"polling_interval" gorm:"default:5"`      // seconds

	// Usage tracking
	LastUsedAt   *time.Time `json:"last_used_at,omitempty"`
	SuccessCount int        `json:"success_count" gorm:"default:0"`
	FailureCount int        `json:"failure_count" gorm:"default:0"`
	LastError    string     `json:"last_error,omitempty" gorm:"type:text"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName specifies the database table name.
func (DNSProviderCredential) TableName() string {
	return "dns_provider_credentials"
}
