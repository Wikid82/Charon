// Package models defines the database schema and domain types.
package models

import (
	"time"
)

// DNSProvider represents a DNS provider configuration for ACME DNS-01 challenges.
// Credentials are stored encrypted at rest using AES-256-GCM.
type DNSProvider struct {
	ID           uint   `json:"-" gorm:"primaryKey"`
	UUID         string `json:"uuid" gorm:"uniqueIndex;size:36"`
	Name         string `json:"name" gorm:"index;not null;size:255"`
	ProviderType string `json:"provider_type" gorm:"index;not null;size:50"`
	Enabled      bool   `json:"enabled" gorm:"default:true;index"`
	IsDefault    bool   `json:"is_default" gorm:"default:false"`

	// Multi-credential mode (enables zone-specific credentials)
	UseMultiCredentials bool `json:"use_multi_credentials" gorm:"default:false"`

	// Relationship to zone-specific credentials
	Credentials []DNSProviderCredential `json:"credentials,omitempty" gorm:"foreignKey:DNSProviderID"`

	// Encrypted credentials (JSON blob, encrypted with AES-256-GCM)
	// Kept for backward compatibility when UseMultiCredentials=false
	CredentialsEncrypted string `json:"-" gorm:"type:text;column:credentials_encrypted"`

	// Encryption key version used for credentials (supports key rotation)
	KeyVersion int `json:"key_version" gorm:"default:1;index"`

	// Propagation settings
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
func (DNSProvider) TableName() string {
	return "dns_providers"
}
