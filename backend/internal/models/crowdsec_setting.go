package models

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

const (
	// CrowdSecEnabledSettingKey is the setting that switches CrowdSec on or off. It takes precedence over the
	// deprecated mode key and over the SecurityConfig row.
	CrowdSecEnabledSettingKey = "security.crowdsec.enabled"
	// CrowdSecDeprecatedModeSettingKey is the older mode setting. It only counts when the enabled key is absent or empty.
	CrowdSecDeprecatedModeSettingKey = "security.crowdsec.mode"
)

// readTrimmedSetting returns the whitespace-trimmed value of key. found is false when the settings table does not
// exist yet, the key has no row, or the trimmed value is empty, so a whitespace-only value counts as absent.
// Every CrowdSec setting read goes through this helper so all readers parse a value the same way.
func readTrimmedSetting(db *gorm.DB, key string) (value string, found bool, err error) {
	if db == nil || !db.Migrator().HasTable(&Setting{}) {
		return "", false, nil
	}

	var setting Setting
	if queryErr := db.Where("key = ?", key).Limit(1).Find(&setting).Error; queryErr != nil {
		return "", false, fmt.Errorf("read %s setting: %w", key, queryErr)
	}
	value = strings.TrimSpace(setting.Value)
	return value, value != "", nil
}

// CrowdSecSettingOverride returns the CrowdSec decision stored in the settings table.
// The enabled key wins; the deprecated mode key counts only when the enabled key is absent or empty.
// An empty or whitespace-only value counts as absent. found is false when neither key decides, including when the
// settings table does not exist yet. A query error is returned and found is false.
func CrowdSecSettingOverride(db *gorm.DB) (enabled, found bool, err error) {
	return crowdSecOverride(db, []string{CrowdSecEnabledSettingKey, CrowdSecDeprecatedModeSettingKey})
}

// CrowdSecDecisionWithoutEnabledKey returns the CrowdSec decision as if the enabled key were absent: the deprecated
// mode key decides when it has a value, otherwise the default row does (mode "local"). cfg is the default row, or
// the zero value when no row exists. On a query error it returns the row decision together with the error.
// The settings API uses it to resolve an empty enabled value without writing it first.
func CrowdSecDecisionWithoutEnabledKey(db *gorm.DB, cfg SecurityConfig) (bool, error) {
	rowEnabled := cfg.CrowdSecMode == "local"
	enabled, found, err := crowdSecOverride(db, []string{CrowdSecDeprecatedModeSettingKey})
	if err != nil {
		return rowEnabled, fmt.Errorf("resolve CrowdSec mode setting: %w", err)
	}
	if found {
		return enabled, nil
	}
	return rowEnabled, nil
}

// crowdSecOverride returns the decision of the first key in keys that has a value. Keys are read in order, and an
// empty or whitespace-only value counts as absent. found is false when no key has a value.
func crowdSecOverride(db *gorm.DB, keys []string) (enabled, found bool, err error) {
	for _, key := range keys {
		value, present, readErr := readTrimmedSetting(db, key)
		if readErr != nil {
			return false, false, readErr
		}
		if !present {
			continue
		}
		return crowdSecSettingDecision(key, value), true, nil
	}
	return false, false, nil
}

// crowdSecSettingDecision interprets one stored CrowdSec setting value: the enabled key is on only for "true"
// (case-insensitive), and the deprecated mode key is on only for "local".
func crowdSecSettingDecision(key, value string) bool {
	if key == CrowdSecEnabledSettingKey {
		return strings.EqualFold(value, "true")
	}
	return value == "local"
}

// CrowdSecEnabled is the single CrowdSec decision used by the proxy config, the status endpoint and boot
// reconciliation. It applies the settings override first and falls back to the default row (mode "local").
// On a query error it returns the row decision together with the error, so callers can log it and carry on.
func CrowdSecEnabled(db *gorm.DB, cfg SecurityConfig) (bool, error) {
	rowEnabled := cfg.CrowdSecMode == "local"
	enabled, found, err := CrowdSecSettingOverride(db)
	if err != nil {
		return rowEnabled, fmt.Errorf("resolve CrowdSec setting: %w", err)
	}
	if found {
		return enabled, nil
	}
	return rowEnabled, nil
}
