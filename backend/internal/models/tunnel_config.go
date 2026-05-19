package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TunnelProviderType identifies the external tunnel service backend.
type TunnelProviderType string

const (
	ProviderCloudflare TunnelProviderType = "cloudflare"
	ProviderTailscale  TunnelProviderType = "tailscale"
	ProviderZeroTier   TunnelProviderType = "zerotier"
	ProviderNetBird    TunnelProviderType = "netbird"
)

// TunnelConfig stores configuration and AES-256-GCM encrypted credentials
// for a managed external tunnel service.
type TunnelConfig struct {
	ID       uint               `json:"-" gorm:"primaryKey"`
	UUID     string             `json:"uuid" gorm:"uniqueIndex;not null"`
	Name     string             `json:"name" gorm:"not null;index"`
	Provider TunnelProviderType `json:"provider" gorm:"not null;index"`

	// Never serialized to JSON responses.
	EncryptedCredentials string `json:"-" gorm:"type:text;not null"`

	// Provider-specific JSON config (e.g., account_id, tunnel_name, controller_url).
	Configuration string `json:"configuration" gorm:"type:text"`

	IsActive  bool      `json:"is_active" gorm:"default:false;index"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (tc *TunnelConfig) BeforeCreate(tx *gorm.DB) (err error) {
	if tc.UUID == "" {
		tc.UUID = uuid.New().String()
	}
	return
}
