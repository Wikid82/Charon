package services

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

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

// Allowed ranges for the numeric settings. Zero for a rate-limit number means not set. The ceilings are above the UI's
// own limits (1000 requests, 100 burst, 3600 s window) so an API or script user is not boxed in.
const (
	MaxWAFParanoiaLevel    = 4
	MaxRateLimitRequests   = 100000
	MaxRateLimitBurst      = 10000
	MaxRateLimitWindowSec  = 86400
	minWAFParanoiaLevel    = 1
	minRateLimitNumberSize = 0
)

// ErrInvalidRateLimitMode is returned for a rate_limit_mode that is not exactly "", "disabled" or "enabled".
var ErrInvalidRateLimitMode = errors.New(`rate_limit_mode must be "", "disabled" or "enabled"`)

// ErrInvalidRateLimitBypass is returned for a rate_limit_bypass_list with an entry that is not an IP address or CIDR.
var ErrInvalidRateLimitBypass = errors.New("rate_limit_bypass_list must be a comma-separated list of IP addresses or CIDR ranges")

// ErrInvalidCrowdSecAPIURL is returned for a crowdsec_api_url that is neither empty nor an absolute http or https URL.
var ErrInvalidCrowdSecAPIURL = errors.New("crowdsec_api_url must be empty or an absolute http or https URL with a host")

// validRateLimitModes are the stored rate-limit modes. The proxy compares the mode with ==, so only the exact lowercase
// values are accepted.
var validRateLimitModes = map[string]bool{"": true, "disabled": true, "enabled": true}

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

// checkValues refuses a value outside its allowed range or shape, inside the write transaction and before any column is
// written. A field equal to its stored value is not checked (the echo rule), so a save that posts back the row it read,
// or an unrelated save, never fails on a value that was already stored. stored is the row before the save, or the model
// defaults for a missing row.
func (u SecurityConfigUpdate) checkValues(stored models.SecurityConfig) error {
	if u.WAFParanoiaLevel != nil {
		if err := checkIntRange("waf_paranoia_level", *u.WAFParanoiaLevel, stored.WAFParanoiaLevel, minWAFParanoiaLevel, MaxWAFParanoiaLevel); err != nil {
			return err
		}
	}
	if u.RateLimitRequests != nil {
		if err := checkIntRange("rate_limit_requests", *u.RateLimitRequests, stored.RateLimitRequests, minRateLimitNumberSize, MaxRateLimitRequests); err != nil {
			return err
		}
	}
	if u.RateLimitBurst != nil {
		if err := checkIntRange("rate_limit_burst", *u.RateLimitBurst, stored.RateLimitBurst, minRateLimitNumberSize, MaxRateLimitBurst); err != nil {
			return err
		}
	}
	if u.RateLimitWindowSec != nil {
		if err := checkIntRange("rate_limit_window_sec", *u.RateLimitWindowSec, stored.RateLimitWindowSec, minRateLimitNumberSize, MaxRateLimitWindowSec); err != nil {
			return err
		}
	}
	// The mode is checked whenever it is present, even when rate_limit_enable is also sent.
	if u.RateLimitMode != nil && *u.RateLimitMode != stored.RateLimitMode && !validRateLimitModes[*u.RateLimitMode] {
		return &InvalidConfigError{Err: ErrInvalidRateLimitMode}
	}
	if u.RateLimitBypassList != nil && *u.RateLimitBypassList != stored.RateLimitBypassList {
		if err := validateAddressList(*u.RateLimitBypassList, ErrInvalidRateLimitBypass); err != nil {
			return &InvalidConfigError{Err: err}
		}
	}
	if u.CrowdSecAPIURL != nil {
		value := *u.CrowdSecAPIURL
		if value != stored.CrowdSecAPIURL && value != "" && !isHTTPURL(value) {
			return &InvalidConfigError{Err: ErrInvalidCrowdSecAPIURL}
		}
	}
	return nil
}

// checkIntRange refuses value when it is outside [minValue, maxValue] and differs from the stored value. The message
// names the field and its range.
func checkIntRange(field string, value, stored, minValue, maxValue int) error {
	if value == stored || (value >= minValue && value <= maxValue) {
		return nil
	}
	return &InvalidConfigError{Err: fmt.Errorf("%s must be between %d and %d", field, minValue, maxValue)}
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
		// The enable flag mirrors the mode, so clearing the mode can never leave the flag on.
		row.RateLimitMode = *u.RateLimitMode
		row.RateLimitEnable = strings.EqualFold(*u.RateLimitMode, "enabled")
		columns = append(columns, "rate_limit_mode", "rate_limit_enable")
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
// It validates the update first, so an invalid update never reaches the database. The write runs in a write transaction
// (see applyUpdateLocked). A missing row is created from the model defaults. An existing row is updated in the present
// columns only, so columns the request does not carry keep their stored values.
func (s *SecurityService) ApplyUpdate(actor, name string, u SecurityConfigUpdate) (*models.SecurityConfig, models.ModuleStateSnapshot, error) {
	if err := u.validate(); err != nil {
		return nil, models.ModuleStateSnapshot{}, err
	}
	return s.ApplyUpdateFunc(actor, name, func(models.SecurityConfig) (SecurityConfigUpdate, error) {
		return u, nil
	})
}

// ApplyUpdateFunc is ApplyUpdate for a save that depends on the stored row. The row is read inside the write lock, and
// build derives the update from it, so the check and the write see the same row. A build error aborts the transaction
// before any write and is returned as it is.
func (s *SecurityService) ApplyUpdateFunc(actor, name string, build func(stored models.SecurityConfig) (SecurityConfigUpdate, error)) (*models.SecurityConfig, models.ModuleStateSnapshot, error) {
	var stored *models.SecurityConfig
	var snapshot models.ModuleStateSnapshot
	err := WithWriteLock(s.db, &models.SecurityConfig{}, func(tx *gorm.DB) error {
		var err error
		stored, snapshot, err = applyUpdateLocked(tx, actor, name, build)
		return err
	})
	if err != nil {
		return nil, models.ModuleStateSnapshot{}, err
	}
	return stored, snapshot, nil
}

// ApplyUpdateTx applies u to the named row inside the caller's transaction. The caller holds the write lock and has
// captured its own module state snapshot, so no snapshot is returned here.
func (s *SecurityService) ApplyUpdateTx(tx *gorm.DB, actor, name string, u SecurityConfigUpdate) (*models.SecurityConfig, error) {
	if err := u.validate(); err != nil {
		return nil, err
	}
	stored, _, err := applyUpdateLocked(tx, actor, name, func(models.SecurityConfig) (SecurityConfigUpdate, error) {
		return u, nil
	})
	return stored, err
}

// applyUpdateLocked is the body of every config save. It runs on tx, which must hold the write lock: every query goes
// through tx, because the connection pool holds a single connection. build receives the stored row (the model defaults
// when none exists) and returns the update; the update is validated, checked against that row and applied. For the
// default row the module keys follow the save (applyModuleKeys). It returns the stored row and the pre-image snapshot.
func applyUpdateLocked(tx *gorm.DB, actor, name string, build func(stored models.SecurityConfig) (SecurityConfigUpdate, error)) (*models.SecurityConfig, models.ModuleStateSnapshot, error) {
	var row models.SecurityConfig
	before := newSecurityConfigRow(name)
	exists := true
	findErr := tx.Where("name = ?", name).First(&row).Error
	switch {
	case errors.Is(findErr, gorm.ErrRecordNotFound):
		exists = false
		row = before
	case findErr != nil:
		return nil, models.ModuleStateSnapshot{}, fmt.Errorf("find security config %q: %w", name, findErr)
	default:
		before = row
	}

	// The pre-image is taken from the row read above, before any column is applied.
	var current *models.SecurityConfig
	if exists {
		current = &before
	}
	snapshot, err := models.CaptureModuleState(tx, name, current)
	if err != nil {
		return nil, models.ModuleStateSnapshot{}, fmt.Errorf("capture security state %q: %w", name, err)
	}

	u, err := build(before)
	if err != nil {
		return nil, models.ModuleStateSnapshot{}, err
	}
	if err := u.validate(); err != nil {
		return nil, models.ModuleStateSnapshot{}, err
	}
	if err := u.checkValues(before); err != nil {
		return nil, models.ModuleStateSnapshot{}, err
	}
	if err := u.checkRulesSource(tx, before); err != nil {
		return nil, models.ModuleStateSnapshot{}, err
	}

	if !exists {
		u.applyTo(&row)
		if err := tx.Create(&row).Error; err != nil {
			return nil, models.ModuleStateSnapshot{}, fmt.Errorf("create security config %q: %w", name, err)
		}
	} else if columns := u.applyTo(&row); len(columns) > 0 {
		// An empty column list would make GORM write every non-zero field of row, so it is skipped.
		if err := tx.Model(&row).Select(columns).Updates(&row).Error; err != nil {
			return nil, models.ModuleStateSnapshot{}, fmt.Errorf("update security config %q: %w", name, err)
		}
	}

	if isDefaultRowName(name) {
		if err := u.applyModuleKeys(tx, before, actor); err != nil {
			return nil, models.ModuleStateSnapshot{}, fmt.Errorf("apply security module keys: %w", err)
		}
	}

	if err := snapshot.Seal(tx); err != nil {
		return nil, models.ModuleStateSnapshot{}, fmt.Errorf("seal security state %q: %w", name, err)
	}

	var stored models.SecurityConfig
	if err := tx.Where("name = ?", name).First(&stored).Error; err != nil {
		return nil, models.ModuleStateSnapshot{}, fmt.Errorf("read security config %q: %w", name, err)
	}
	return &stored, snapshot, nil
}

// ErrWAFExclusionExists is returned when a WAF exclusion with the same rule_id and target is already stored.
var ErrWAFExclusionExists = errors.New("waf exclusion already exists")

// ErrWAFExclusionNotFound is returned when no stored WAF exclusion matches the rule_id and target.
var ErrWAFExclusionNotFound = errors.New("waf exclusion not found")
