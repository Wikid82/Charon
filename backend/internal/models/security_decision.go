package models

import (
	"time"
)

// SecurityDecision stores a decision/action taken by CrowdSec/WAF/RateLimit or manual
// override so it can be audited and surfaced in the UI.
type SecurityDecision struct {
	ID        uint      `json:"-" gorm:"primaryKey"`
	UUID      string    `json:"uuid" gorm:"uniqueIndex"`
	Source    string    `json:"source" gorm:"index;compositeIndex:idx_sd_source_created;compositeIndex:idx_sd_source_scenario_created;compositeIndex:idx_sd_source_ip_created"` // e.g., crowdsec, waf, ratelimit, manual
	Action    string    `json:"action" gorm:"index"`                                                                                                                            // allow, block, challenge
	IP        string    `json:"ip" gorm:"index;compositeIndex:idx_sd_source_ip_created"`
	Host      string    `json:"host" gorm:"index"` // optional
	RuleID    string    `json:"rule_id" gorm:"index"`
	Details   string    `json:"details" gorm:"type:text"`
	CreatedAt time.Time `json:"created_at" gorm:"index;compositeIndex:idx_sd_source_created,sort:desc;compositeIndex:idx_sd_source_scenario_created,sort:desc;compositeIndex:idx_sd_source_ip_created,sort:desc"`

	// Dashboard enrichment fields (Issue #26, PR-1)
	Scenario  string    `json:"scenario" gorm:"index;compositeIndex:idx_sd_source_scenario_created"`
	Country   string    `json:"country" gorm:"index;size:2"`
	ExpiresAt time.Time `json:"expires_at" gorm:"index"`
}
