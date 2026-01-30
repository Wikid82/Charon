package models

import (
	"time"
)

// SecurityDecision stores a decision/action taken by CrowdSec/WAF/RateLimit or manual
// override so it can be audited and surfaced in the UI.
type SecurityDecision struct {
	ID        uint      `json:"-" gorm:"primaryKey"`
	UUID      string    `json:"uuid" gorm:"uniqueIndex"`
	Source    string    `json:"source" gorm:"index"` // e.g., crowdsec, waf, ratelimit, manual
	Action    string    `json:"action" gorm:"index"` // allow, block, challenge
	IP        string    `json:"ip" gorm:"index"`
	Host      string    `json:"host" gorm:"index"` // optional
	RuleID    string    `json:"rule_id" gorm:"index"`
	Details   string    `json:"details" gorm:"type:text"`
	CreatedAt time.Time `json:"created_at" gorm:"index"`
}
