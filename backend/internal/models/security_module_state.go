package models

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

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

// IsCerberusSwitchKey reports whether key is one of the two Cerberus switch keys. Both are written together by
// SetCerberusSwitch.
func IsCerberusSwitchKey(key string) bool {
	return key == CerberusFeatureSettingKey || key == CerberusLegacySettingKey
}

// SetCerberusSwitch writes the Cerberus switch on both keys as exactly "true" or "false". The feature key is in the
// feature category and the legacy mirror in the security category. Both are written in the caller's transaction, so the
// two keys never disagree.
func SetCerberusSwitch(tx *gorm.DB, on bool) error {
	value := "false"
	if on {
		value = "true"
	}
	for _, key := range []string{CerberusFeatureSettingKey, CerberusLegacySettingKey} {
		category := "security"
		if key == CerberusFeatureSettingKey {
			category = "feature"
		}
		setting := Setting{Key: key, Value: value, Category: category, Type: "bool"}
		if err := tx.Where(Setting{Key: key}).Assign(setting).FirstOrCreate(&setting).Error; err != nil {
			return fmt.Errorf("save setting %s: %w", key, err)
		}
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
	return SetCerberusSwitch(tx, true)
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

// ModuleStateSnapshot is the pre-image of the module switches and of the default row columns a module writer changes,
// plus the after-image Seal records once the writer's writes are in place. A restore is a compare-and-restore: it puts
// a key or column the writer touched back only while it still holds the value the writer left. A concurrent write made
// after the writer committed is therefore kept, and a skipped restore is logged.
//
// Accepted limits, which the compare cannot see:
//  1. A concurrent write that stores the same value as the writer's after-image is indistinguishable from the writer's
//     own write, so the restore overwrites it.
//  2. RestoreSettings and RestoreRow run in separate write-locked transactions, so a concurrent write can land between
//     them. The outcome is mixed: one part is restored and the other keeps the concurrent write.
//  3. A concurrent admin write's own proxy apply may already have applied the failed writer's values to the proxy. The
//     restore does not re-apply them; the next apply corrects the proxy.
//  4. A row the writer created is deleted when every imaged column is still unchanged, even if a non-imaged column was
//     changed by a concurrent write. That concurrent write is lost with the row.
//
// A restore that puts back every column the writer touched also puts back the row's update time. When a concurrent
// write kept any column, the update time is the restore time, because that row was written again.
type ModuleStateSnapshot struct {
	name      string
	keys      []settingImage
	rowBefore map[string]any // nil when the row did not exist before the write
	// rowUpdatedAt is the row's update time before the write. It is not compared, only written back by a full restore.
	rowUpdatedAt time.Time
	sealed       bool
	rowAfter     map[string]any // nil when the row is absent after the write
	// rowTouched lists the row columns the writer changed, in column-name order. A created row lists every column.
	rowTouched []string
	rowCreated bool
}

// errNotSealed is returned by a restore when Seal has not recorded the after-image. Without it the restore cannot tell
// what the writer changed, so it refuses rather than restoring everything.
var errNotSealed = errors.New("module state snapshot is not sealed")

// settingImage is one settings key: its pre-image, and after Seal its after-image and whether the writer changed it.
type settingImage struct {
	key     string
	before  settingState
	setting Setting // the stored record before the write, written back on restore
	after   settingState
	touched bool
}

// settingState is what a settings key holds for the compare: whether it exists, its value and its category. The ID and
// the update time are not compared, because a rewrite of the same value changes them.
type settingState struct {
	exists   bool
	value    string
	category string
}

// settingStateOf returns the compared state of a settings record read from the table.
func settingStateOf(setting Setting, exists bool) settingState {
	if !exists {
		return settingState{}
	}
	return settingState{exists: true, value: setting.Value, category: setting.Category}
}

// readSetting returns the stored record of key and whether it exists.
func readSetting(tx *gorm.DB, key string) (Setting, bool, error) {
	var setting Setting
	result := tx.Where("key = ?", key).Limit(1).Find(&setting)
	if result.Error != nil {
		return Setting{}, false, result.Error
	}
	return setting, result.RowsAffected > 0, nil
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

// rowColumns returns the default-row columns a module writer, a CrowdSec start or a config save can change, keyed by
// column name. Every column applyTo can write is listed (see ImagedRowColumns). A missing row has no columns and
// returns nil.
func rowColumns(row *SecurityConfig) map[string]any {
	if row == nil {
		return nil
	}
	return map[string]any{
		"enabled":                row.Enabled,
		"crowd_sec_mode":         row.CrowdSecMode,
		"crowd_sec_api_url":      row.CrowdSecAPIURL,
		"admin_whitelist":        row.AdminWhitelist,
		"waf_mode":               row.WAFMode,
		"waf_rules_source":       row.WAFRulesSource,
		"waf_learning":           row.WAFLearning,
		"waf_paranoia_level":     row.WAFParanoiaLevel,
		"waf_exclusions":         row.WAFExclusions,
		"rate_limit_mode":        row.RateLimitMode,
		"rate_limit_enable":      row.RateLimitEnable,
		"rate_limit_requests":    row.RateLimitRequests,
		"rate_limit_window_sec":  row.RateLimitWindowSec,
		"rate_limit_burst":       row.RateLimitBurst,
		"rate_limit_bypass_list": row.RateLimitBypassList,
	}
}

// ImagedRowColumns returns the database column names that rowColumns images, in sorted order. A parity test in the
// services package checks that every column a config save can write is in this list.
func ImagedRowColumns() []string {
	return sortedColumns(rowColumns(&SecurityConfig{}))
}

// sortedColumns returns the column names of a row image in a stable order.
func sortedColumns(columns map[string]any) []string {
	names := make([]string, 0, len(columns))
	for name := range columns {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// warnRestoreSkipped logs a restore that was skipped because a concurrent write changed the value after the writer left
// it. target names the settings key or the row column.
func warnRestoreSkipped(target string) {
	logger.Log().WithField("target", logValue(target)).Warn("security state restore skipped: a concurrent write changed this value")
}

// CaptureModuleState records the pre-image of the module and framework keys (plus any extra keys) and of the row named
// name. row is the row as the caller just read it, nil when it does not exist; the caller reads it so the capture adds
// no query on the row table. It must run inside the write transaction it guards, before the writer's writes. Seal
// records the after-image once the writes are in place.
func CaptureModuleState(tx *gorm.DB, name string, row *SecurityConfig, extraKeys ...string) (ModuleStateSnapshot, error) {
	snap := ModuleStateSnapshot{name: name, rowBefore: rowColumns(row)}
	if row != nil {
		snap.rowUpdatedAt = row.UpdatedAt
	}

	keys := append(append([]string{}, securityModuleKeys...), CerberusFeatureSettingKey, CerberusLegacySettingKey)
	seen := make(map[string]bool, len(keys)+len(extraKeys))
	for _, key := range append(keys, extraKeys...) {
		if seen[key] {
			continue
		}
		seen[key] = true

		setting, exists, err := readSetting(tx, key)
		if err != nil {
			return ModuleStateSnapshot{}, fmt.Errorf("snapshot setting %s: %w", key, err)
		}
		snap.keys = append(snap.keys, settingImage{key: key, before: settingStateOf(setting, exists), setting: setting})
	}
	return snap, nil
}

// Seal records the after-image: the values the writer left, read inside the writer's transaction after its writes. Each
// imaged key and column whose value differs from the pre-image is marked touched; a restore only touches those. A row
// the writer created lists every column as touched. Seal issues one read per imaged key and one read of the row. It
// fails when the row the writer left in place is gone, because there is then nothing a restore can match against.
func (s *ModuleStateSnapshot) Seal(tx *gorm.DB) error {
	for i := range s.keys {
		img := &s.keys[i]
		setting, exists, err := readSetting(tx, img.key)
		if err != nil {
			return fmt.Errorf("seal setting %s: %w", img.key, err)
		}
		img.after = settingStateOf(setting, exists)
		img.touched = img.after != img.before
	}

	row, err := ReadModuleRow(tx, s.name)
	if err != nil {
		return fmt.Errorf("seal security config %q: %w", s.name, err)
	}
	if row == nil && s.rowBefore != nil {
		return fmt.Errorf("seal security config %q: row removed during the write", s.name)
	}
	s.rowAfter = rowColumns(row)
	s.rowTouched = nil
	s.rowCreated = row != nil && s.rowBefore == nil
	for _, column := range sortedColumns(s.rowAfter) {
		if s.rowCreated || s.rowBefore[column] != s.rowAfter[column] {
			s.rowTouched = append(s.rowTouched, column)
		}
	}
	s.sealed = true
	return nil
}

// RestoreSettings writes the pre-image of each touched settings key back, but only while the key still holds the value
// the writer left. A key that did not exist before is removed again. A key a concurrent write changed is kept and a
// warning is logged. It is independent of RestoreRow, so a failure in one does not undo the other.
func (s ModuleStateSnapshot) RestoreSettings(db *gorm.DB) error {
	if !s.sealed {
		return errNotSealed
	}
	for _, img := range s.keys {
		if !img.touched {
			continue
		}
		current, exists, err := readSetting(db, img.key)
		if err != nil {
			return fmt.Errorf("restore setting %s: %w", img.key, err)
		}
		if settingStateOf(current, exists) != img.after {
			warnRestoreSkipped(img.key)
			continue
		}
		if !img.before.exists {
			if err := db.Where("key = ?", img.key).Delete(&Setting{}).Error; err != nil {
				return fmt.Errorf("restore setting %s: %w", img.key, err)
			}
			continue
		}
		// Every stored column is written explicitly: a struct Assign skips zero values, so a blank value or an empty type
		// or category would keep the writer's value. UpdateColumns leaves updated_at to the pre-image value, so the
		// restore time is not stamped over it.
		restore := img.setting
		res := db.Model(&Setting{}).Where("key = ?", img.key).UpdateColumns(map[string]any{
			"value":      restore.Value,
			"category":   restore.Category,
			"type":       restore.Type,
			"updated_at": restore.UpdatedAt,
		})
		if res.Error != nil {
			return fmt.Errorf("restore setting %s: %w", img.key, res.Error)
		}
		if res.RowsAffected > 0 {
			continue
		}
		// The writer removed the key, so the pre-image record is inserted again under a fresh ID.
		restore.ID = 0
		if err := db.Create(&restore).Error; err != nil {
			return fmt.Errorf("restore setting %s: %w", img.key, err)
		}
	}
	return nil
}

// RestoreRow writes the pre-image of each touched default row column back, but only while the column still holds the
// value the writer left. A column a concurrent write changed is kept and a warning is logged. A row the writer created
// is deleted again only when every column it set is still unchanged; otherwise it is kept.
func (s ModuleStateSnapshot) RestoreRow(db *gorm.DB) error {
	if !s.sealed {
		return errNotSealed
	}
	if len(s.rowTouched) == 0 {
		return nil
	}
	current, err := ReadModuleRow(db, s.name)
	if err != nil {
		return fmt.Errorf("restore security config %q: %w", s.name, err)
	}
	if current == nil && s.rowCreated {
		return nil
	}

	currentColumns := rowColumns(current)
	restore := make(map[string]any, len(s.rowTouched))
	kept := 0
	for _, column := range s.rowTouched {
		if current == nil || currentColumns[column] != s.rowAfter[column] {
			warnRestoreSkipped(s.name + "." + column)
			kept++
			continue
		}
		restore[column] = s.rowBefore[column]
	}

	if s.rowCreated {
		if kept > 0 {
			return nil
		}
		if err := db.Where("name = ?", s.name).Delete(&SecurityConfig{}).Error; err != nil {
			return fmt.Errorf("restore security config %q: %w", s.name, err)
		}
		return nil
	}
	if len(restore) == 0 {
		return nil
	}
	if kept > 0 {
		// A kept column means a concurrent write touched this row, so the update time is the restore time.
		if err := db.Model(&SecurityConfig{}).Where("name = ?", s.name).Updates(restore).Error; err != nil {
			return fmt.Errorf("restore security config %q: %w", s.name, err)
		}
		return nil
	}
	// Every touched column is back to the pre-image, so the row is back to the pre-image: the update time too. UpdateColumns
	// keeps GORM from stamping the restore time.
	restore["updated_at"] = s.rowUpdatedAt
	if err := db.Model(&SecurityConfig{}).Where("name = ?", s.name).UpdateColumns(restore).Error; err != nil {
		return fmt.Errorf("restore security config %q: %w", s.name, err)
	}
	return nil
}
