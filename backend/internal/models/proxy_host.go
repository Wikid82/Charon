package models

import (
	"time"
)

// ProxyHost is the foundational entity representing a proxied upstream service.
type ProxyHost struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	UUID         string    `json:"uuid" gorm:"uniqueIndex"`
	Name         string    `json:"name"`
	Domain       string    `json:"domain" gorm:"index"`
	TargetScheme string    `json:"target_scheme"`
	TargetHost   string    `json:"target_host"`
	TargetPort   int       `json:"target_port"`
	EnableTLS    bool      `json:"enable_tls"`
	EnableWS     bool      `json:"enable_websockets"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
