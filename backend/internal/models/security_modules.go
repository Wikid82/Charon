package models

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// Settings keys that switch a security module on or off. A recognised value ("true" or "false", any case, with
// surrounding whitespace ignored) wins over the default SecurityConfig row. Any other value is ignored.
const (
	SecurityWAFEnabledSettingKey       = "security.waf.enabled"
	SecurityRateLimitEnabledSettingKey = "security.rate_limit.enabled"
	SecurityACLEnabledSettingKey       = "security.acl.enabled"
)

// ModuleRowDecision reads a module's state from the default SecurityConfig row. set is false when the row has no
// opinion (for example an empty column), so the static configuration decides.
type ModuleRowDecision func(cfg SecurityConfig) (on, set bool)

// ModuleResolution is the state of one security module and where it came from.
type ModuleResolution struct {
	// Enabled is the resolved state.
	Enabled bool
	// FromSetting is true when a recognised settings key decided Enabled.
	FromSetting bool
}

// StaticWAFEnabled reports whether a static WAF mode switches the WAF on. An empty or "disabled" mode is off.
func StaticWAFEnabled(mode string) bool {
	return mode != "" && mode != "disabled"
}

// DefaultSecurityConfigOrZero returns the default row, or the zero value when there is no row or no database. A
// query error other than a missing row is returned with the zero value, so callers can log it and carry on.
func DefaultSecurityConfigOrZero(db *gorm.DB) (SecurityConfig, error) {
	if db == nil {
		return SecurityConfig{}, nil
	}
	cfg, err := FindDefaultSecurityConfig(db)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return SecurityConfig{}, nil
		}
		return SecurityConfig{}, err
	}
	return cfg, nil
}

// ResolveModule is the single decision for a security module, shared by the proxy config, the status endpoint and
// the API middleware. A recognised settings key wins, then the default row (passed in by the caller, the zero value
// when there is none, so an empty row has no opinion), then the static configuration. Callers apply the Cerberus-off
// gate themselves.
//
// A query error on the settings key is returned together with the decision the remaining sources give, so callers
// can log it and carry on. A nil db skips the settings key.
func ResolveModule(db *gorm.DB, key string, row SecurityConfig, rowOn ModuleRowDecision, static bool) (ModuleResolution, error) {
	value, present, keyErr := readTrimmedSetting(db, key)
	if keyErr == nil && present {
		if on, recognised := parseModuleSetting(value); recognised {
			return ModuleResolution{Enabled: on, FromSetting: true}, nil
		}
	}

	if rowOn != nil {
		if on, set := rowOn(row); set {
			return ModuleResolution{Enabled: on}, wrapModuleError(key, keyErr)
		}
	}
	return ModuleResolution{Enabled: static}, wrapModuleError(key, keyErr)
}

// parseModuleSetting interprets a trimmed settings value. Only "true" and "false" (case-insensitive) are recognised.
func parseModuleSetting(value string) (on, recognised bool) {
	switch {
	case strings.EqualFold(value, "true"):
		return true, true
	case strings.EqualFold(value, "false"):
		return false, true
	default:
		return false, false
	}
}

// wrapModuleError adds the setting key to a resolution error. A nil error stays nil.
func wrapModuleError(key string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("resolve %s: %w", key, err)
}

// wafRowDecision: an empty WAF mode has no opinion; any other mode is on unless it is "disabled".
func wafRowDecision(cfg SecurityConfig) (on, set bool) {
	if cfg.WAFMode == "" {
		return false, false
	}
	return !strings.EqualFold(cfg.WAFMode, "disabled"), true
}

// rateLimitRowDecision: the rate-limit mode wins when set; otherwise the legacy enable flag decides.
func rateLimitRowDecision(cfg SecurityConfig) (on, set bool) {
	if cfg.RateLimitMode != "" {
		return strings.EqualFold(cfg.RateLimitMode, "enabled"), true
	}
	if cfg.RateLimitEnable {
		return true, true
	}
	return false, false
}

// ResolveWAF resolves the WAF state from the WAF settings key, the default row and the static WAF decision.
func ResolveWAF(db *gorm.DB, row SecurityConfig, static bool) (ModuleResolution, error) {
	return ResolveModule(db, SecurityWAFEnabledSettingKey, row, wafRowDecision, static)
}

// ResolveRateLimit resolves the rate-limit state from its settings key, the default row and the static decision.
func ResolveRateLimit(db *gorm.DB, row SecurityConfig, static bool) (ModuleResolution, error) {
	return ResolveModule(db, SecurityRateLimitEnabledSettingKey, row, rateLimitRowDecision, static)
}

// ResolveACL resolves the ACL state. ACL has no row column, so only the settings key and the static decision apply.
func ResolveACL(db *gorm.DB, static bool) (ModuleResolution, error) {
	return ResolveModule(db, SecurityACLEnabledSettingKey, SecurityConfig{}, nil, static)
}

// WAFEnabled returns the resolved WAF state; see ResolveWAF.
func WAFEnabled(db *gorm.DB, row SecurityConfig, static bool) (bool, error) {
	res, err := ResolveWAF(db, row, static)
	return res.Enabled, err
}

// RateLimitEnabled returns the resolved rate-limit state; see ResolveRateLimit.
func RateLimitEnabled(db *gorm.DB, row SecurityConfig, static bool) (bool, error) {
	res, err := ResolveRateLimit(db, row, static)
	return res.Enabled, err
}

// ACLEnabled returns the resolved ACL state; see ResolveACL.
func ACLEnabled(db *gorm.DB, static bool) (bool, error) {
	res, err := ResolveACL(db, static)
	return res.Enabled, err
}
