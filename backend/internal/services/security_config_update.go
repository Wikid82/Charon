package services

import (
	"errors"
	"fmt"
	"net/url"

	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// SecurityConfigUpdate is the partial update accepted by the security config save endpoint.
// A nil field means the request did not carry that key (or sent null), so the stored value is kept.
// An explicit empty string, zero or false is applied. Keys mirror the JSON names of models.SecurityConfig.
// The name selects the row and is not a column. crowdsec_mode is validated but never stored; the CrowdSec
// lifecycle owns that column.
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

// ErrInvalidWAFMode is returned for a waf_mode outside the allowed set.
var ErrInvalidWAFMode = errors.New("waf_mode must be empty, disabled, monitor or block")

// ErrWAFRulesSourceUnknown is returned for a waf_rules_source that is neither a URL nor an existing rule set name.
var ErrWAFRulesSourceUnknown = errors.New("waf_rules_source names no rule set and is not a URL")

// InvalidConfigError marks a config save refused by validation before or during its writes. The message is the
// cause's message, and errors.Is and errors.As reach the cause.
type InvalidConfigError struct {
	Err error
}

// Error returns the cause's message.
func (e *InvalidConfigError) Error() string { return e.Err.Error() }

// Unwrap returns the cause.
func (e *InvalidConfigError) Unwrap() error { return e.Err }

// validWAFModes are the stored WAF modes. An empty mode leaves the WAF to the module key and the static mode.
var validWAFModes = map[string]bool{"": true, "disabled": true, "monitor": true, "block": true}

// validate refuses the update before anything is written. Only the values the request carries are checked.
func (u SecurityConfigUpdate) validate() error {
	if u.AdminWhitelist != nil {
		if err := ValidateAdminWhitelist(*u.AdminWhitelist); err != nil {
			return &InvalidConfigError{Err: err}
		}
	}
	if u.CrowdSecMode != nil {
		if err := ValidateCrowdSecMode(*u.CrowdSecMode); err != nil {
			return &InvalidConfigError{Err: err}
		}
	}
	if u.WAFMode != nil && !validWAFModes[*u.WAFMode] {
		return &InvalidConfigError{Err: ErrInvalidWAFMode}
	}
	return nil
}

// checkRulesSource validates waf_rules_source inside the write transaction. An empty value, a URL and the value already
// stored are accepted; any other value must name an existing rule set. The stored value is compared so an unrelated save
// never fails on a value that was valid when it was stored.
func (u SecurityConfigUpdate) checkRulesSource(tx *gorm.DB, stored models.SecurityConfig) error {
	if u.WAFRulesSource == nil {
		return nil
	}
	source := *u.WAFRulesSource
	if source == "" || source == stored.WAFRulesSource || isHTTPURL(source) {
		return nil
	}
	var matches int64
	if err := tx.Model(&models.SecurityRuleSet{}).Where("name = ?", source).Count(&matches).Error; err != nil {
		return fmt.Errorf("look up rule set %q: %w", source, err)
	}
	if matches == 0 {
		return &InvalidConfigError{Err: ErrWAFRulesSourceUnknown}
	}
	return nil
}

// isHTTPURL reports whether value is an absolute http or https URL with a host.
func isHTTPURL(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}
	return (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

// applyModuleKeys writes the module keys a save implies, for the default row only. A key is written only when the
// incoming mode or enable value differs from the value stored in the row before the save (the echo rule): a client that
// posts back the row it read changes no module. A key-on write also turns the framework on, unless the same request
// carries enabled:false.
func (u SecurityConfigUpdate) applyModuleKeys(tx *gorm.DB, stored models.SecurityConfig, actor string) error {
	enableFramework := u.Enabled == nil || *u.Enabled

	if u.WAFMode != nil && *u.WAFMode != stored.WAFMode {
		if err := writeWAFModuleKey(tx, *u.WAFMode, actor, enableFramework); err != nil {
			return err
		}
	}

	switch {
	case u.RateLimitEnable != nil:
		if *u.RateLimitEnable != stored.RateLimitEnable {
			return models.SetModuleStateOpt(tx, models.SecurityRateLimitEnabledSettingKey, *u.RateLimitEnable, actor, enableFramework)
		}
	case u.RateLimitMode != nil && *u.RateLimitMode != stored.RateLimitMode:
		return writeRateLimitModeKey(tx, *u.RateLimitMode, actor, enableFramework)
	}
	return nil
}

// isDefaultRowName reports whether name is the row the proxy and the module keys read.
func isDefaultRowName(name string) bool {
	return name == models.DefaultSecurityConfigName
}

// writeWAFModuleKey maps a stored WAF mode to the WAF key: block and monitor on, disabled off, empty removes the key.
func writeWAFModuleKey(tx *gorm.DB, mode, actor string, enableFramework bool) error {
	key := models.SecurityWAFEnabledSettingKey
	switch mode {
	case "block", "monitor":
		return models.SetModuleStateOpt(tx, key, true, actor, enableFramework)
	case "disabled":
		return models.SetModuleStateOpt(tx, key, false, actor, false)
	default:
		return models.ClearModuleKey(tx, key, actor)
	}
}

// writeRateLimitModeKey maps a mode-only rate-limit save to the rate-limit key: enabled on, any other non-empty mode
// off, empty removes the key.
func writeRateLimitModeKey(tx *gorm.DB, mode, actor string, enableFramework bool) error {
	key := models.SecurityRateLimitEnabledSettingKey
	switch mode {
	case "enabled":
		return models.SetModuleStateOpt(tx, key, true, actor, enableFramework)
	case "":
		return models.ClearModuleKey(tx, key, actor)
	default:
		return models.SetModuleStateOpt(tx, key, false, actor, false)
	}
}

// applyTo copies the fields present in u onto row and returns the database column of each field it wrote.
// The rate-limit mode rule (ResolveRateLimitMode) decides rate_limit_mode from the enable flag and the mode key:
// an enable flag always sets the mode, and a mode alone is stored as sent.
func (u SecurityConfigUpdate) applyTo(row *models.SecurityConfig) []string {
	var columns []string
	if u.Enabled != nil {
		row.Enabled = *u.Enabled
		columns = append(columns, "enabled")
	}
	if u.AdminWhitelist != nil {
		row.AdminWhitelist = *u.AdminWhitelist
		columns = append(columns, "admin_whitelist")
	}
	if u.CrowdSecAPIURL != nil {
		row.CrowdSecAPIURL = *u.CrowdSecAPIURL
		columns = append(columns, "crowd_sec_api_url") // GORM's column name for CrowdSecAPIURL
	}
	if u.WAFMode != nil {
		row.WAFMode = *u.WAFMode
		columns = append(columns, "waf_mode")
	}
	if u.WAFRulesSource != nil {
		row.WAFRulesSource = *u.WAFRulesSource
		columns = append(columns, "waf_rules_source")
	}
	if u.WAFLearning != nil {
		row.WAFLearning = *u.WAFLearning
		columns = append(columns, "waf_learning")
	}
	if u.WAFParanoiaLevel != nil {
		row.WAFParanoiaLevel = *u.WAFParanoiaLevel
		columns = append(columns, "waf_paranoia_level")
	}
	if u.WAFExclusions != nil {
		row.WAFExclusions = *u.WAFExclusions
		columns = append(columns, "waf_exclusions")
	}
	switch {
	case u.RateLimitEnable != nil:
		mode := ""
		if u.RateLimitMode != nil {
			mode = *u.RateLimitMode
		}
		row.RateLimitEnable = *u.RateLimitEnable
		row.RateLimitMode = ResolveRateLimitMode(*u.RateLimitEnable, mode)
		columns = append(columns, "rate_limit_enable", "rate_limit_mode")
	case u.RateLimitMode != nil:
		row.RateLimitMode = *u.RateLimitMode
		columns = append(columns, "rate_limit_mode")
	}
	if u.RateLimitBurst != nil {
		row.RateLimitBurst = *u.RateLimitBurst
		columns = append(columns, "rate_limit_burst")
	}
	if u.RateLimitRequests != nil {
		row.RateLimitRequests = *u.RateLimitRequests
		columns = append(columns, "rate_limit_requests")
	}
	if u.RateLimitWindowSec != nil {
		row.RateLimitWindowSec = *u.RateLimitWindowSec
		columns = append(columns, "rate_limit_window_sec")
	}
	if u.RateLimitBypassList != nil {
		row.RateLimitBypassList = *u.RateLimitBypassList
		columns = append(columns, "rate_limit_bypass_list")
	}
	return columns
}

// newSecurityConfigRow returns a row for name with the model defaults that a new row starts from.
func newSecurityConfigRow(name string) models.SecurityConfig {
	return models.SecurityConfig{
		Name:             name,
		RateLimitMode:    "disabled",
		WAFParanoiaLevel: 1,
	}
}

// ApplyUpdate applies only the fields present in u to the named security config row and returns the stored row, with
// the pre-image of the module state it may have changed (see models.ModuleStateSnapshot). The pre-image is what the
// caller restores when the proxy apply fails.
//
// It validates the update first, then runs inside a write transaction: every query goes through tx, because the
// connection pool holds a single connection. A missing row is created from the model defaults. An existing row is
// updated in the present columns only, so columns the request does not carry keep their stored values. An update
// with no present column issues no UPDATE. For the default row the module keys follow the save (applyModuleKeys).
func (s *SecurityService) ApplyUpdate(actor, name string, u SecurityConfigUpdate) (*models.SecurityConfig, models.ModuleStateSnapshot, error) {
	if err := u.validate(); err != nil {
		return nil, models.ModuleStateSnapshot{}, err
	}

	var stored models.SecurityConfig
	var snapshot models.ModuleStateSnapshot
	err := WithWriteLock(s.db, &models.SecurityConfig{}, func(tx *gorm.DB) error {
		var row models.SecurityConfig
		before := newSecurityConfigRow(name)
		exists := true
		findErr := tx.Where("name = ?", name).First(&row).Error
		switch {
		case errors.Is(findErr, gorm.ErrRecordNotFound):
			exists = false
			row = before
		case findErr != nil:
			return fmt.Errorf("find security config %q: %w", name, findErr)
		default:
			before = row
		}

		// The pre-image is taken from the row read above, before any column is applied.
		var current *models.SecurityConfig
		if exists {
			current = &before
		}
		var err error
		if snapshot, err = models.CaptureModuleState(tx, name, current); err != nil {
			return fmt.Errorf("capture security state %q: %w", name, err)
		}

		if err := u.checkRulesSource(tx, before); err != nil {
			return err
		}

		if !exists {
			u.applyTo(&row)
			if err := tx.Create(&row).Error; err != nil {
				return fmt.Errorf("create security config %q: %w", name, err)
			}
		} else if columns := u.applyTo(&row); len(columns) > 0 {
			// An empty column list would make GORM write every non-zero field of row, so it is skipped.
			if err := tx.Model(&row).Select(columns).Updates(&row).Error; err != nil {
				return fmt.Errorf("update security config %q: %w", name, err)
			}
		}

		if isDefaultRowName(name) {
			if err := u.applyModuleKeys(tx, before, actor); err != nil {
				return fmt.Errorf("apply security module keys: %w", err)
			}
		}

		if err := tx.Where("name = ?", name).First(&stored).Error; err != nil {
			return fmt.Errorf("read security config %q: %w", name, err)
		}
		return nil
	})
	if err != nil {
		return nil, models.ModuleStateSnapshot{}, err
	}
	return &stored, snapshot, nil
}
