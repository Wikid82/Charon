package models

import (
	"time"
)

// SecurityDecision stores a decision/action taken by CrowdSec/WAF/RateLimit or manual
// override so it can be audited and surfaced in the UI.
type SecurityDecision struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UUID      string    `json:"uuid" gorm:"uniqueIndex"`
	Source    string    `json:"source"` // e.g., crowdsec, waf, ratelimit, manual
	Action    string    `json:"action"` // allow, block, challenge
	IP        string    `json:"ip"`
	Host      string    `json:"host"` // optional
	RuleID    string    `json:"rule_id"`
	Details   string    `json:"details" gorm:"type:text"`
	CreatedAt time.Time `json:"created_at"`
}
