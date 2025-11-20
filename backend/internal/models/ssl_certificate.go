package models

import (
	"time"
)

// SSLCertificate represents TLS certificates managed by CPM+.
// Can be Let's Encrypt auto-generated or custom uploaded certs.
type SSLCertificate struct {
	ID          uint       `json:"id" gorm:"primaryKey"`
	UUID        string     `json:"uuid" gorm:"uniqueIndex"`
	Name        string     `json:"name"`
	Provider    string     `json:"provider"`                     // "letsencrypt", "custom", "self-signed"
	Domains     string     `json:"domains"`                      // comma-separated list of domains
	Certificate string     `json:"certificate" gorm:"type:text"` // PEM-encoded certificate
	PrivateKey  string     `json:"private_key" gorm:"type:text"` // PEM-encoded private key
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	AutoRenew   bool       `json:"auto_renew" gorm:"default:false"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
