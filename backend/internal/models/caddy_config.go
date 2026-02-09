package models

import (
	"time"
)

// CaddyConfig stores an audit trail of Caddy configuration changes.
type CaddyConfig struct {
	ID         uint      `json:"-" gorm:"primaryKey"`
	ConfigHash string    `json:"-" gorm:"index"`
	AppliedAt  time.Time `json:"applied_at"`
	Success    bool      `json:"success"`
	ErrorMsg   string    `json:"error_msg"`
}
