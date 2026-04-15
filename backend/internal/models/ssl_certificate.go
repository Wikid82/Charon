package models

import (
	"time"
)

// SSLCertificate represents TLS certificates managed by Charon.
// Can be Let's Encrypt auto-generated or custom uploaded certs.
type SSLCertificate struct {
	ID                  uint       `json:"-" gorm:"primaryKey"`
	UUID                string     `json:"uuid" gorm:"uniqueIndex"`
	Name                string     `json:"name" gorm:"index"`
	Provider            string     `json:"provider" gorm:"index"`
	Domains             string     `json:"domains" gorm:"index"`
	CommonName          string     `json:"common_name"`
	Certificate         string     `json:"-" gorm:"type:text"`
	CertificateChain    string     `json:"-" gorm:"type:text"`
	PrivateKeyEncrypted string     `json:"-" gorm:"column:private_key_enc;type:text"`
	PrivateKey          string     `json:"-" gorm:"-"`
	KeyVersion          int        `json:"-" gorm:"default:1"`
	Fingerprint         string     `json:"fingerprint"`
	SerialNumber        string     `json:"serial_number"`
	IssuerOrg           string     `json:"issuer_org"`
	KeyType             string     `json:"key_type"`
	ExpiresAt           *time.Time `json:"expires_at,omitempty" gorm:"index"`
	NotBefore           *time.Time `json:"not_before,omitempty"`
	AutoRenew           bool       `json:"auto_renew" gorm:"default:false"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}
