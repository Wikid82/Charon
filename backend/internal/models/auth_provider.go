package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AuthProvider represents an external OAuth/OIDC provider configuration
type AuthProvider struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	UUID      string    `gorm:"uniqueIndex;not null" json:"uuid"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Provider configuration
	Name    string `gorm:"uniqueIndex;not null" json:"name"` // e.g., "Google", "GitHub"
	Type    string `gorm:"not null" json:"type"`             // "google", "github", "oidc", "saml"
	Enabled bool   `gorm:"default:true" json:"enabled"`

	// OAuth/OIDC credentials
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"-"` // Never expose in JSON

	// OIDC specific
	IssuerURL   string `json:"issuer_url,omitempty"`    // For generic OIDC providers
	AuthURL     string `json:"auth_url,omitempty"`      // Optional override
	TokenURL    string `json:"token_url,omitempty"`     // Optional override
	UserInfoURL string `json:"user_info_url,omitempty"` // Optional override

	// Scopes and mappings
	Scopes      string `json:"scopes"`       // Comma-separated (e.g., "openid,profile,email")
	RoleMapping string `json:"role_mapping"` // JSON mapping from provider claims to roles

	// UI customization
	IconURL     string `json:"icon_url,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
}

// BeforeCreate generates UUID for new auth providers
func (p *AuthProvider) BeforeCreate(tx *gorm.DB) error {
	if p.UUID == "" {
		p.UUID = uuid.New().String()
	}
	return nil
}
