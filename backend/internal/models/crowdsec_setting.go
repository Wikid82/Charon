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
	for _, key := range []string{CrowdSecEnabledSettingKey, CrowdSecDeprecatedModeSettingKey} {
		value, present, readErr := readTrimmedSetting(db, key)
		if readErr != nil {
			return false, false, readErr
		}
		if !present {
			continue
		}
		if key == CrowdSecEnabledSettingKey {
			return strings.EqualFold(value, "true"), true, nil
		}
		return value == "local", true, nil
	}
	return false, false, nil
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
