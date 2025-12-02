package models

import (
	"time"
)

// SecurityRuleSet stores metadata about WAF/CrowdSec rule sets that the server can download and apply.
type SecurityRuleSet struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	UUID        string    `json:"uuid" gorm:"uniqueIndex"`
	Name        string    `json:"name" gorm:"index"`
	SourceURL   string    `json:"source_url" gorm:"type:text"`
	Mode        string    `json:"mode"` // optional e.g., 'owasp', 'custom'
	LastUpdated time.Time `json:"last_updated"`
	Content     string    `json:"content" gorm:"type:text"`
}
