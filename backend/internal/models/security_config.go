package models

import (
    "time"
)

// SecurityConfig represents global Cerberus/CrowdSec/WAF/RateLimit settings
// used by the server and propagated into the generated Caddy config.
type SecurityConfig struct {
    ID              uint      `json:"id" gorm:"primaryKey"`
    UUID            string    `json:"uuid" gorm:"uniqueIndex"`
    Name            string    `json:"name" gorm:"index"`
    Enabled         bool      `json:"enabled"`
    AdminWhitelist  string    `json:"admin_whitelist" gorm:"type:text"` // JSON array or comma-separated CIDRs
    BreakGlassHash  string    `json:"-" gorm:"column:break_glass_hash"`
    CrowdSecMode    string    `json:"crowdsec_mode"` // "disabled" or "local"
    CrowdSecAPIURL  string    `json:"crowdsec_api_url" gorm:"type:text"`
    WAFMode         string    `json:"waf_mode"`      // "disabled", "monitor", "block"
    WAFRulesSource  string    `json:"waf_rules_source" gorm:"type:text"` // URL or name of ruleset
    WAFLearning     bool      `json:"waf_learning"`
    RateLimitEnable bool      `json:"rate_limit_enable"`
    RateLimitBurst  int       `json:"rate_limit_burst"`
    RateLimitRequests int     `json:"rate_limit_requests"`
    RateLimitWindowSec int    `json:"rate_limit_window_sec"`
    CreatedAt       time.Time `json:"created_at"`
    UpdatedAt       time.Time `json:"updated_at"`
}
