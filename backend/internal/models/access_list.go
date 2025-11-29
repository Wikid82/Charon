package models

import (
	"time"
)

// AccessList defines IP-based or auth-based access control rules
// that can be applied to proxy hosts.
type AccessList struct {
	ID               uint      `json:"id" gorm:"primaryKey"`
	UUID             string    `json:"uuid" gorm:"uniqueIndex"`
	Name             string    `json:"name" gorm:"index"`
	Description      string    `json:"description"`
	Type             string    `json:"type"`                      // "whitelist", "blacklist", "geo_whitelist", "geo_blacklist"
	IPRules          string    `json:"ip_rules" gorm:"type:text"` // JSON array of IP/CIDR rules
	CountryCodes     string    `json:"country_codes"`             // Comma-separated ISO country codes (for geo types)
	LocalNetworkOnly bool      `json:"local_network_only"`        // RFC1918 private networks only
	Enabled          bool      `json:"enabled"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// AccessListRule represents a single IP or CIDR rule
type AccessListRule struct {
	CIDR        string `json:"cidr"`        // IP address or CIDR notation
	Description string `json:"description"` // Optional description
}
