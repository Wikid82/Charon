package models

import (
	"fmt"
	"strconv"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Wikid82/charon/backend/internal/logger"
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
	if db == nil {
		return "", false, nil
	}

	var setting Setting
	if queryErr := db.Where("key = ?", key).Limit(1).Find(&setting).Error; queryErr != nil {
		// The table check runs only on failure: the happy path stays one query per key on hot status and config reads.
		if !db.Migrator().HasTable(&Setting{}) {
			return "", false, nil
		}
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
	rowEnabled := CrowdSecRowEnabled(cfg)
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

// CrowdSecRowEnabled is the CrowdSec decision of the default SecurityConfig row alone: on only for mode "local".
func CrowdSecRowEnabled(cfg SecurityConfig) bool {
	return cfg.CrowdSecMode == "local"
}

// Decision sources logged by RetireCrowdSecModeSetting. Only the label is logged, never a setting value.
const (
	retireSourceCarriedOver    = "carried_over"
	retireSourceEnabledWins    = "enabled_key_wins"
	retireSourceBlankModeValue = "mode_blank"
)

// RetireCrowdSecModeSetting retires the deprecated mode setting. When the mode row exists and the enabled setting has
// no value (absent or blank), the decision the mode row produces is written to the enabled setting, so the enabled
// setting alone gives the same CrowdSec decision afterwards. The mode row is then deleted. A non-blank enabled value
// is never overwritten. The step and its writes run in one transaction, so a failure leaves both rows as they were.
//
// It is idempotent and runs on every start: with no mode row it changes nothing and returns retired=false. retired
// is true only when a mode row was deleted. A missing settings table counts as no mode row.
func RetireCrowdSecModeSetting(db *gorm.DB) (retired bool, err error) {
	if db == nil {
		return false, nil
	}

	source := ""
	err = db.Transaction(func(tx *gorm.DB) error {
		modeValue, modeExists, readErr := readSettingRow(tx, CrowdSecDeprecatedModeSettingKey)
		if readErr != nil {
			return readErr
		}
		if !modeExists {
			return nil
		}

		mode := strings.TrimSpace(modeValue)
		enabledValue, _, enabledErr := readSettingRow(tx, CrowdSecEnabledSettingKey)
		if enabledErr != nil {
			return enabledErr
		}

		source = retireSourceEnabledWins
		switch {
		case mode == "":
			source = retireSourceBlankModeValue
		case strings.TrimSpace(enabledValue) == "":
			if writeErr := upsertCrowdSecEnabledSetting(tx, strconv.FormatBool(mode == "local")); writeErr != nil {
				return writeErr
			}
			source = retireSourceCarriedOver
		}

		if deleteErr := tx.Where("key = ?", CrowdSecDeprecatedModeSettingKey).Delete(&Setting{}).Error; deleteErr != nil {
			return fmt.Errorf("delete %s setting: %w", CrowdSecDeprecatedModeSettingKey, deleteErr)
		}
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("retire CrowdSec mode setting: %w", err)
	}
	if source == "" {
		return false, nil
	}

	logger.Log().WithFields(map[string]any{
		"retired": true,
		"source":  source,
	}).Info("CrowdSec mode setting retired")
	return true, nil
}

// readSettingRow returns the stored value of key and whether a row exists, without trimming or interpreting it. A
// missing settings table reads as no row, so the step is safe before the schema exists.
func readSettingRow(tx *gorm.DB, key string) (value string, exists bool, err error) {
	var setting Setting
	res := tx.Where("key = ?", key).Limit(1).Find(&setting)
	if res.Error != nil {
		if !tx.Migrator().HasTable(&Setting{}) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("read %s setting: %w", key, res.Error)
	}
	return setting.Value, res.RowsAffected > 0, nil
}

// upsertCrowdSecEnabledSetting writes the enabled setting, creating the row or replacing its value.
func upsertCrowdSecEnabledSetting(tx *gorm.DB, value string) error {
	setting := Setting{Key: CrowdSecEnabledSettingKey, Value: value, Category: "security", Type: "bool"}
	err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "type", "category", "updated_at"}),
	}).Create(&setting).Error
	if err != nil {
		return fmt.Errorf("write %s setting: %w", CrowdSecEnabledSettingKey, err)
	}
	return nil
}

// CrowdSecEnabled is the single CrowdSec decision used by the proxy config, the status endpoint and boot
// reconciliation. It applies the settings override first and falls back to the default row (mode "local").
// On a query error it returns the row decision together with the error, so callers can log it and carry on.
func CrowdSecEnabled(db *gorm.DB, cfg SecurityConfig) (bool, error) {
	rowEnabled := CrowdSecRowEnabled(cfg)
	enabled, found, err := CrowdSecSettingOverride(db)
	if err != nil {
		return rowEnabled, fmt.Errorf("resolve CrowdSec setting: %w", err)
	}
	if found {
		return enabled, nil
	}
	return rowEnabled, nil
}
