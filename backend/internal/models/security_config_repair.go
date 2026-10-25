package models

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/logger"
)

// legacyDefaultSecurityConfigName is the name the singleton row was created under before
// the lookup helper existed (#1727). Readers ignore rows with this name, so the repair below
// renames or removes them.
const legacyDefaultSecurityConfigName = "Default Security Config"

// RepairDefaultSecurityConfigName moves rows written under the legacy name back onto the
// singleton "default" row that the proxy and every reader use. It runs in one transaction and
// is idempotent: with no legacy rows it changes nothing.
//
//   - No "default" row: the lowest-id legacy row becomes "default". A literal or empty UUID is
//     replaced with a fresh one. Every other legacy row is deleted.
//   - Both exist: the "default" row keeps its columns, except crowd_sec_mode, which follows the
//     security.crowdsec.enabled setting when that setting is present, and otherwise the
//     lowest-id legacy row. All legacy rows are deleted.
func RepairDefaultSecurityConfigName(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var legacy []SecurityConfig
		if err := tx.Where("name = ?", legacyDefaultSecurityConfigName).Order("id ASC").Find(&legacy).Error; err != nil {
			return fmt.Errorf("find legacy default security config rows: %w", err)
		}
		if len(legacy) == 0 {
			return nil
		}

		var defaultCount int64
		if err := tx.Model(&SecurityConfig{}).Where("name = ?", DefaultSecurityConfigName).Count(&defaultCount).Error; err != nil {
			return fmt.Errorf("count default security config rows: %w", err)
		}

		if defaultCount == 0 {
			return renameLegacyDefaultSecurityConfig(tx, legacy)
		}
		return mergeLegacyDefaultSecurityConfig(tx, legacy)
	})
}

// renameLegacyDefaultSecurityConfig promotes the lowest-id legacy row to "default" and deletes the rest.
func renameLegacyDefaultSecurityConfig(tx *gorm.DB, legacy []SecurityConfig) error {
	keep := legacy[0]
	updates := map[string]any{"name": DefaultSecurityConfigName}
	if keep.UUID == "" || keep.UUID == DefaultSecurityConfigName {
		updates["uuid"] = uuid.NewString()
	}
	if err := tx.Model(&SecurityConfig{}).Where("id = ?", keep.ID).Updates(updates).Error; err != nil {
		return fmt.Errorf("rename legacy default security config: %w", err)
	}
	if err := deleteLegacySecurityConfigRows(tx); err != nil {
		return err
	}
	logger.Log().WithFields(map[string]any{
		"kept_id":    keep.ID,
		"removed":    len(legacy) - 1,
		"renamed_to": DefaultSecurityConfigName,
	}).Info("security config repair: renamed legacy default security config row")
	return nil
}

// mergeLegacyDefaultSecurityConfig keeps the "default" row, syncs its crowd_sec_mode and deletes the legacy rows.
func mergeLegacyDefaultSecurityConfig(tx *gorm.DB, legacy []SecurityConfig) error {
	var defaultRow SecurityConfig
	if err := tx.Where("name = ?", DefaultSecurityConfigName).Order("id ASC").First(&defaultRow).Error; err != nil {
		return fmt.Errorf("load default security config row: %w", err)
	}

	mode, found, err := crowdSecModeFromSetting(tx)
	if err != nil {
		return err
	}
	if !found {
		mode = legacy[0].CrowdSecMode
	}

	if err := tx.Model(&SecurityConfig{}).Where("id = ?", defaultRow.ID).Update("crowd_sec_mode", mode).Error; err != nil {
		return fmt.Errorf("sync default security config crowd_sec_mode: %w", err)
	}
	if err := deleteLegacySecurityConfigRows(tx); err != nil {
		return err
	}
	logger.Log().WithFields(map[string]any{
		"kept_id":        defaultRow.ID,
		"removed":        len(legacy),
		"crowd_sec_mode": mode,
	}).Info("security config repair: removed legacy default security config rows")
	return nil
}

// crowdSecModeFromSetting maps the security.crowdsec.enabled setting to a crowd_sec_mode value.
// found is false when the settings table or the key is missing, or the value is empty.
func crowdSecModeFromSetting(tx *gorm.DB) (mode string, found bool, err error) {
	value, found, err := readTrimmedSetting(tx, CrowdSecEnabledSettingKey)
	if err != nil || !found {
		return "", false, err
	}
	if strings.EqualFold(value, "true") {
		return "local", true, nil
	}
	return "disabled", true, nil
}

func deleteLegacySecurityConfigRows(tx *gorm.DB) error {
	if err := tx.Where("name = ?", legacyDefaultSecurityConfigName).Delete(&SecurityConfig{}).Error; err != nil {
		return fmt.Errorf("delete legacy default security config rows: %w", err)
	}
	return nil
}
