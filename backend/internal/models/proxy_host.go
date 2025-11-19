package models

import (
	"time"
)

// ProxyHost represents a reverse proxy configuration for a single domain.
type ProxyHost struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	UUID         string    `json:"uuid" gorm:"uniqueIndex"`
	Name         string    `json:"name"`
	Domain       string    `json:"domain" gorm:"uniqueIndex"`
	TargetScheme string    `json:"target_scheme"` // http/https
	TargetHost   string    `json:"target_host"`
	TargetPort   int       `json:"target_port"`
	EnableTLS    bool      `json:"enable_tls" gorm:"default:false"`
	EnableWS     bool      `json:"enable_websockets" gorm:"default:false"`
	Enabled      bool      `json:"enabled" gorm:"default:true"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
