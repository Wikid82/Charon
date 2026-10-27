package models

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/util"
)

// maxLogValueRunes bounds a caller-influenced value written to a module log line.
const maxLogValueRunes = 128

// The module switches are written in one place. A switch is a settings key (the proxy and the status read it first)
// and, for the WAF and the rate limit, a mirror in the default SecurityConfig row. Every function here takes the
// caller's transaction: on the single-connection pool a query issued through the outer handle inside a transaction
// waits for the connection the transaction holds.

const (
	// CerberusFeatureSettingKey and CerberusLegacySettingKey are the framework switches a module enable turns on.
	CerberusFeatureSettingKey = "feature.cerberus.enabled"
	CerberusLegacySettingKey  = "security.cerberus.enabled"
)

// securityModuleKeys are the settings keys that switch a module on or off.
var securityModuleKeys = []string{SecurityWAFEnabledSettingKey, SecurityRateLimitEnabledSettingKey, SecurityACLEnabledSettingKey}

// IsSecurityModuleKey reports whether key is one of the module switch keys.
func IsSecurityModuleKey(key string) bool {
	for _, k := range securityModuleKeys {
		if k == key {
			return true
		}
	}
	return false
}

// ParseModuleSettingValue reads a module switch value the way the resolver does: surrounding whitespace is ignored and
// "true" or "false" in any case is recognised. recognised is false for any other value.
func ParseModuleSettingValue(value string) (on, recognised bool) {
	return parseModuleSetting(strings.TrimSpace(value))
}

// SetModuleState switches a security module on or off. The settings key is written first. A switch on also turns the
// security framework on and mirrors the module into the default row: the WAF moves a missing or disabled mode to block
// (a monitor mode stays), the rate limit is marked enabled. A switch off never touches the row.
//
// source names the caller in the log line (the actor of the request).
func SetModuleState(tx *gorm.DB, module string, on bool, source string) error {
	return SetModuleStateOpt(tx, module, on, source, true)
}

// SetModuleStateOpt is SetModuleState with the framework enable optional. ApplyUpdate passes false when the same request
// carries enabled:false, so a module key written by a config save does not switch the framework back on.
func SetModuleStateOpt(tx *gorm.DB, module string, on bool, source string, enableFramework bool) error {
	if !IsSecurityModuleKey(module) {
		return fmt.Errorf("set module state: %q is not a security module key", module)
	}
	if err := writeModuleKey(tx, module, on, source); err != nil {
		return err
	}
	if !on {
		return nil
	}
	if enableFramework {
		if err := EnableSecurityFramework(tx); err != nil {
			return err
		}
	}
	return mirrorModuleRow(tx, module)
}

// logValue returns a caller-influenced string (the actor or a settings key) in a form safe to write to the log:
// control characters and line breaks are replaced and the value is bounded, so a supplied value cannot forge or
// split a log line.
func logValue(s string) string {
	s = util.SanitizeForLog(s)
	if runes := []rune(s); len(runes) > maxLogValueRunes {
		return string(runes[:maxLogValueRunes])
	}
	return s
}

// ClearModuleKey removes a module key, so the resolver falls back to the default row and then the static configuration.
func ClearModuleKey(tx *gorm.DB, module, source string) error {
	if !IsSecurityModuleKey(module) {
		return fmt.Errorf("clear module key: %q is not a security module key", module)
	}
	if err := tx.Where("key = ?", module).Delete(&Setting{}).Error; err != nil {
		return fmt.Errorf("delete setting %s: %w", module, err)
	}
	logger.Log().WithFields(map[string]any{"setting_key": logValue(module), "value": "", "actor": logValue(source)}).Info("security module switch key removed")
	return nil
}

// writeModuleKey stores a module key as "true" or "false" with the security category.
func writeModuleKey(tx *gorm.DB, key string, on bool, source string) error {
	value := "false"
	if on {
		value = "true"
	}
	setting := Setting{Key: key, Value: value, Category: "security", Type: "bool"}
	if err := tx.Where(Setting{Key: key}).Assign(setting).FirstOrCreate(&setting).Error; err != nil {
		return fmt.Errorf("save setting %s: %w", key, err)
	}
	logger.Log().WithFields(map[string]any{"setting_key": logValue(key), "value": value, "actor": logValue(source)}).Info("security module switch written")
	return nil
}

// mirrorModuleRow copies an enabled module into the default row. A missing row has nothing to mirror.
func mirrorModuleRow(tx *gorm.DB, module string) error {
	switch module {
	case SecurityWAFEnabledSettingKey:
		// Only a missing or disabled mode moves to block; an explicit monitor mode is kept.
		err := tx.Model(&SecurityConfig{}).
			Where("name = ? AND LOWER(waf_mode) IN ?", DefaultSecurityConfigName, []string{"", "disabled"}).
			Update("waf_mode", "block").Error
		if err != nil {
			return fmt.Errorf("mirror waf mode: %w", err)
		}
	case SecurityRateLimitEnabledSettingKey:
		return mirrorRateLimitRow(tx)
	}
	return nil
}

// mirrorRateLimitRow marks the rate limit enabled in the default row and fills each threshold the row does not set
// (zero or negative) with its standard value. A threshold the row already holds is kept, so an admin value supplied in
// the same request wins over the fill. The fill is written in the caller's transaction, so it is visible in the row
// the admin sees and it is restored with the row when the apply fails.
func mirrorRateLimitRow(tx *gorm.DB) error {
	row, err := ReadModuleRow(tx, DefaultSecurityConfigName)
	if err != nil {
		return fmt.Errorf("mirror rate limit: %w", err)
	}
	if row == nil {
		return nil
	}

	updates := map[string]any{"rate_limit_mode": "enabled", "rate_limit_enable": true}
	filled := map[string]int{}
	if row.RateLimitRequests <= 0 {
		updates["rate_limit_requests"] = StandardRateLimitRequests
		filled["rate_limit_requests"] = StandardRateLimitRequests
	}
	if row.RateLimitWindowSec <= 0 {
		updates["rate_limit_window_sec"] = StandardRateLimitWindowSec
		filled["rate_limit_window_sec"] = StandardRateLimitWindowSec
	}
	if row.RateLimitBurst <= 0 {
		updates["rate_limit_burst"] = StandardRateLimitBurst
		filled["rate_limit_burst"] = StandardRateLimitBurst
	}

	if err := tx.Model(&SecurityConfig{}).Where("name = ?", DefaultSecurityConfigName).Updates(updates).Error; err != nil {
		return fmt.Errorf("mirror rate limit: %w", err)
	}
	if len(filled) > 0 {
		logger.Log().WithField("filled", filled).Info("rate limit enabled: unset thresholds set to the standard values")
	}
	return nil
}

// EnableSecurityFramework turns the global security switch and Cerberus on. It is the one path shared by the module
// enables, the settings save and a successful CrowdSec start. A missing default row is seeded first.
func EnableSecurityFramework(tx *gorm.DB) error {
	if _, err := FindDefaultSecurityConfig(tx); err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("enable default security config: %w", err)
		}
		if _, seedErr := SeedDefaultSecurityConfig(tx); seedErr != nil {
			return fmt.Errorf("seed default security config: %w", seedErr)
		}
	}
	if err := tx.Model(&SecurityConfig{}).Where("name = ?", DefaultSecurityConfigName).Update("enabled", true).Error; err != nil {
		return fmt.Errorf("enable default security config: %w", err)
	}
	for _, key := range []string{CerberusFeatureSettingKey, CerberusLegacySettingKey} {
		setting := Setting{Key: key, Value: "true", Category: "security", Type: "bool"}
		if key == CerberusFeatureSettingKey {
			setting.Category = "feature"
		}
		if err := tx.Where(Setting{Key: key}).Assign(setting).FirstOrCreate(&setting).Error; err != nil {
			return fmt.Errorf("save setting %s: %w", key, err)
		}
	}
	return nil
}

// ReconcileWAFDetectionMode is the start-up step that gives a WAF that is on with no stored mode the monitor mode.
// When the WAF key is on and the default row holds an empty or disabled mode, the row moves to monitor, so the WAF
// runs in detection mode. Blocking stays an explicit admin step. A key that is off or absent is left alone, and a
// second run changes nothing. It returns the number of rows changed and logs one Info line when there were any.
func ReconcileWAFDetectionMode(db *gorm.DB) (int64, error) {
	var changed int64
	err := db.Transaction(func(tx *gorm.DB) error {
		row, err := DefaultSecurityConfigOrZero(tx)
		if err != nil {
			return fmt.Errorf("read default security config: %w", err)
		}
		res, err := ResolveWAF(tx, row, false)
		if err != nil {
			return fmt.Errorf("resolve waf key: %w", err)
		}
		if !res.FromSetting || !res.Enabled {
			return nil
		}
		result := tx.Model(&SecurityConfig{}).
			Where("name = ? AND LOWER(waf_mode) IN ?", DefaultSecurityConfigName, []string{"", "disabled"}).
			Update("waf_mode", "monitor")
		if result.Error != nil {
			return fmt.Errorf("reconcile waf mode: %w", result.Error)
		}
		changed = result.RowsAffected
		return nil
	})
	if err != nil {
		return 0, err
	}
	if changed > 0 {
		logger.Log().WithField("rows", changed).Info("WAF switch is on with no WAF mode set: default row set to monitor")
	}
	return changed, nil
}

// ModuleStateSnapshot is the pre-image of the module switches and of the default row columns a module writer
// changes. Restore puts them back when the proxy apply fails after the writes committed.
type ModuleStateSnapshot struct {
	name      string
	keys      []settingImage
	rowExists bool
	row       securityConfigColumns
}

// settingImage is one settings key before a write.
type settingImage struct {
	key     string
	exists  bool
	setting Setting
}

// securityConfigColumns are the default-row columns a module writer or a config save can change.
type securityConfigColumns struct {
	Enabled             bool
	WAFMode             string
	WAFRulesSource      string
	AdminWhitelist      string
	RateLimitMode       string
	RateLimitEnable     bool
	RateLimitRequests   int
	RateLimitWindowSec  int
	RateLimitBurst      int
	RateLimitBypassList string
}

// ReadModuleRow returns the row named name, or nil when it does not exist. It runs on tx like every writer helper.
func ReadModuleRow(tx *gorm.DB, name string) (*SecurityConfig, error) {
	var row SecurityConfig
	err := tx.Where("name = ?", name).First(&row).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("read security config %q: %w", name, err)
	}
	return &row, nil
}

// CaptureModuleState records the pre-image of the module and framework keys (plus any extra keys) and of the row
// named name. row is the row as the caller just read it, nil when it does not exist; the caller reads it so the
// capture adds no query on the row table. It must run inside the write transaction it guards.
func CaptureModuleState(tx *gorm.DB, name string, row *SecurityConfig, extraKeys ...string) (ModuleStateSnapshot, error) {
	snap := ModuleStateSnapshot{name: name}

	keys := append(append([]string{}, securityModuleKeys...), CerberusFeatureSettingKey, CerberusLegacySettingKey)
	seen := make(map[string]bool, len(keys)+len(extraKeys))
	for _, key := range append(keys, extraKeys...) {
		if seen[key] {
			continue
		}
		seen[key] = true

		var existing Setting
		result := tx.Where("key = ?", key).Limit(1).Find(&existing)
		if result.Error != nil {
			return ModuleStateSnapshot{}, fmt.Errorf("snapshot setting %s: %w", key, result.Error)
		}
		snap.keys = append(snap.keys, settingImage{key: key, exists: result.RowsAffected > 0, setting: existing})
	}

	if row == nil {
		return snap, nil
	}
	snap.rowExists = true
	snap.row = securityConfigColumns{
		Enabled:             row.Enabled,
		WAFMode:             row.WAFMode,
		WAFRulesSource:      row.WAFRulesSource,
		AdminWhitelist:      row.AdminWhitelist,
		RateLimitMode:       row.RateLimitMode,
		RateLimitEnable:     row.RateLimitEnable,
		RateLimitRequests:   row.RateLimitRequests,
		RateLimitWindowSec:  row.RateLimitWindowSec,
		RateLimitBurst:      row.RateLimitBurst,
		RateLimitBypassList: row.RateLimitBypassList,
	}
	return snap, nil
}

// RestoreSettings writes the pre-image of the settings keys back. A key that did not exist is removed again. It is
// independent of RestoreRow, so a failure in one does not undo the other.
func (s ModuleStateSnapshot) RestoreSettings(db *gorm.DB) error {
	for _, img := range s.keys {
		if !img.exists {
			if err := db.Where("key = ?", img.key).Delete(&Setting{}).Error; err != nil {
				return fmt.Errorf("restore setting %s: %w", img.key, err)
			}
			continue
		}
		restore := img.setting
		if err := db.Save(&restore).Error; err != nil {
			return fmt.Errorf("restore setting %s: %w", img.key, err)
		}
	}
	return nil
}

// RestoreRow writes the pre-image of the default row columns back. A row that did not exist is deleted again.
func (s ModuleStateSnapshot) RestoreRow(db *gorm.DB) error {
	if !s.rowExists {
		if err := db.Where("name = ?", s.name).Delete(&SecurityConfig{}).Error; err != nil {
			return fmt.Errorf("restore security config %q: %w", s.name, err)
		}
		return nil
	}
	columns := map[string]any{
		"enabled":                s.row.Enabled,
		"waf_mode":               s.row.WAFMode,
		"waf_rules_source":       s.row.WAFRulesSource,
		"admin_whitelist":        s.row.AdminWhitelist,
		"rate_limit_mode":        s.row.RateLimitMode,
		"rate_limit_enable":      s.row.RateLimitEnable,
		"rate_limit_requests":    s.row.RateLimitRequests,
		"rate_limit_window_sec":  s.row.RateLimitWindowSec,
		"rate_limit_burst":       s.row.RateLimitBurst,
		"rate_limit_bypass_list": s.row.RateLimitBypassList,
	}
	if err := db.Model(&SecurityConfig{}).Where("name = ?", s.name).Updates(columns).Error; err != nil {
		return fmt.Errorf("restore security config %q: %w", s.name, err)
	}
	return nil
}
