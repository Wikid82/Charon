package models

import (
	"time"
)

// ProxyHost represents a reverse proxy configuration.
type ProxyHost struct {
	ID               uint       `json:"id" gorm:"primaryKey"`
	UUID             string     `json:"uuid" gorm:"uniqueIndex;not null"`
	Name             string     `json:"name"`
	DomainNames      string     `json:"domain_names" gorm:"not null"` // Comma-separated list
	ForwardScheme    string     `json:"forward_scheme" gorm:"default:http"`
	ForwardHost      string     `json:"forward_host" gorm:"not null"`
	ForwardPort      int        `json:"forward_port" gorm:"not null"`
	SSLForced        bool       `json:"ssl_forced" gorm:"default:false"`
	HTTP2Support     bool       `json:"http2_support" gorm:"default:true"`
	HSTSEnabled      bool       `json:"hsts_enabled" gorm:"default:false"`
	HSTSSubdomains   bool       `json:"hsts_subdomains" gorm:"default:false"`
	BlockExploits    bool       `json:"block_exploits" gorm:"default:true"`
	WebsocketSupport bool       `json:"websocket_support" gorm:"default:false"`
	Enabled          bool       `json:"enabled" gorm:"default:true"`
	Locations        []Location `json:"locations" gorm:"foreignKey:ProxyHostID;constraint:OnDelete:CASCADE"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}
