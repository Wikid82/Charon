package services

import (
	"errors"

	"github.com/Wikid82/charon/backend/internal/models"
)

// SecurityConfigUpdate is the partial update accepted by the security config save endpoint.
// A nil field means the request did not carry that key (or sent null), so the stored value is kept.
// Keys mirror the JSON names of models.SecurityConfig.
type SecurityConfigUpdate struct {
	Name                *string `json:"name"`
	Enabled             *bool   `json:"enabled"`
	AdminWhitelist      *string `json:"admin_whitelist"`
	CrowdSecMode        *string `json:"crowdsec_mode"`
	CrowdSecAPIURL      *string `json:"crowdsec_api_url"`
	WAFMode             *string `json:"waf_mode"`
	WAFRulesSource      *string `json:"waf_rules_source"`
	WAFLearning         *bool   `json:"waf_learning"`
	WAFParanoiaLevel    *int    `json:"waf_paranoia_level"`
	WAFExclusions       *string `json:"waf_exclusions"`
	RateLimitMode       *string `json:"rate_limit_mode"`
	RateLimitEnable     *bool   `json:"rate_limit_enable"`
	RateLimitBurst      *int    `json:"rate_limit_burst"`
	RateLimitRequests   *int    `json:"rate_limit_requests"`
	RateLimitWindowSec  *int    `json:"rate_limit_window_sec"`
	RateLimitBypassList *string `json:"rate_limit_bypass_list"`
}

// errSecurityConfigUpdateNotImplemented is returned until the field-by-field write is in place.
var errSecurityConfigUpdateNotImplemented = errors.New("security config field update is not implemented")

// ApplyUpdate applies only the fields present in u to the named security config row.
// Compile-only placeholder: it never reads or writes the database.
func (s *SecurityService) ApplyUpdate(_ string, _ SecurityConfigUpdate) (*models.SecurityConfig, error) {
	return nil, errSecurityConfigUpdateNotImplemented
}
