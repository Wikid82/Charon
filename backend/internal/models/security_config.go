package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// The standard rate limit values. Turning the rate limit on writes them into the default row for each threshold the row
// does not set, so the stored numbers are the ones that apply. The in-process API limiter uses the same budget.
const (
	StandardRateLimitRequests  = 100
	StandardRateLimitWindowSec = 60
	StandardRateLimitBurst     = 20
)

// SecurityConfig represents global Cerberus/CrowdSec/WAF/RateLimit settings
// used by the server and propagated into the generated Caddy config.
type SecurityConfig struct {
	ID                  uint      `json:"-" gorm:"primaryKey"`
	UUID                string    `json:"uuid" gorm:"uniqueIndex"`
	Name                string    `json:"name" gorm:"index"`
	Enabled             bool      `json:"enabled" gorm:"index"`
	AdminWhitelist      string    `json:"admin_whitelist" gorm:"type:text"` // JSON array or comma-separated CIDRs
	BreakGlassHash      string    `json:"-" gorm:"column:break_glass_hash"`
	CrowdSecMode        string    `json:"crowdsec_mode"` // "disabled" or "local"
	CrowdSecAPIURL      string    `json:"crowdsec_api_url" gorm:"type:text"`
	WAFMode             string    `json:"waf_mode"`                          // "disabled", "monitor", "block"
	WAFRulesSource      string    `json:"waf_rules_source" gorm:"type:text"` // URL or name of ruleset
	WAFLearning         bool      `json:"waf_learning"`
	WAFParanoiaLevel    int       `json:"waf_paranoia_level" gorm:"default:1"` // 1-4, OWASP CRS paranoia level
	WAFExclusions       string    `json:"waf_exclusions" gorm:"type:text"`     // JSON array of rule exclusions
	RateLimitMode       string    `json:"rate_limit_mode"`                     // "disabled", "enabled"
	RateLimitEnable     bool      `json:"rate_limit_enable"`
	RateLimitBurst      int       `json:"rate_limit_burst"`
	RateLimitRequests   int       `json:"rate_limit_requests"`
	RateLimitWindowSec  int       `json:"rate_limit_window_sec"`
	RateLimitBypassList string    `json:"rate_limit_bypass_list" gorm:"type:text"` // Comma-separated CIDRs
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// BeforeCreate generates a server-side UUID when the caller did not supply one.
// Every creator of a SecurityConfig row goes through this hook, so no row can be
// inserted with an empty UUID and collide on the unique UUID index.
func (c *SecurityConfig) BeforeCreate(_ *gorm.DB) error {
	if c.UUID == "" {
		c.UUID = uuid.NewString()
	}
	return nil
}
