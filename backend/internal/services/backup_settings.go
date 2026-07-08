package services

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Wikid82/charon/backend/internal/models"
	"gorm.io/gorm"
)

// Canonical Setting keys for backup configuration (category "backup").
// Read/written only through the typed /backups/settings API (Commit 3) and
// BackupService — see spec §3.4.4. The generic PUT /api/v1/settings endpoint
// must reject writes to these keys (Commit 3's UpdateSetting guard) so the
// cron-reschedule side effect stays owned by exactly one code path.
const (
	SettingKeyBackupScheduleEnabled      = "backup.schedule_enabled"
	SettingKeyBackupScheduleCron         = "backup.schedule_cron"
	SettingKeyBackupRetentionCount       = "backup.retention_count"
	SettingKeyBackupRemoteRetentionCount = "backup.remote_retention_count"
	SettingKeyBackupEncryptionEnabled    = "backup.encryption_enabled"
	// #nosec G101 -- this is a Setting *key name*, not a credential; the value it
	// points to is itself AES-256-GCM-encrypted before storage (spec §3.4.4).
	SettingKeyBackupEncryptionPassphraseEnc = "backup.encryption_passphrase_enc"

	// backupSettingsCategory is the models.Setting.Category value written for
	// every backup.* key, including keys produced by the legacy migration below.
	backupSettingsCategory = "backup"

	// Legacy dead-knob keys written by the pre-Issue-#32 frontend under
	// category "system" (frontend/src/pages/Backups.tsx). No backend code has
	// ever read them (verified in spec §3.4.4) — migrateLegacyBackupSettings
	// translates and deletes them on startup.
	legacySettingKeyBackupInterval  = "backup.interval"
	legacySettingKeyBackupRetention = "backup.retention"

	// defaultLegacyIntervalDays / defaultLegacyRetentionCount are the
	// fallbacks used when a legacy row's value fails to parse as a positive
	// integer, so a corrupt legacy row cannot block migration or produce a
	// nonsensical cron spec / retention count.
	defaultLegacyIntervalDays   = 1
	defaultLegacyRetentionCount = DefaultBackupRetention
)

// migrateLegacyBackupSettings translates the pre-Issue-#32 dead Settings
// knobs (backup.interval days, backup.retention count) into the canonical
// typed backup.* keys and deletes the legacy rows. It is idempotent: once the
// legacy rows are gone, subsequent calls are a no-op. A nil db (e.g. many
// existing unit tests construct BackupService without a real database) is
// tolerated as a no-op rather than an error, since this migration has no
// effect on in-memory-only service behavior.
func migrateLegacyBackupSettings(db *gorm.DB) error {
	if db == nil {
		return nil
	}

	var legacyRows []models.Setting
	if err := db.Where("key IN ?", []string{legacySettingKeyBackupInterval, legacySettingKeyBackupRetention}).
		Find(&legacyRows).Error; err != nil {
		return fmt.Errorf("query legacy backup settings: %w", err)
	}

	if len(legacyRows) == 0 {
		return nil
	}

	return db.Transaction(func(tx *gorm.DB) error {
		for _, row := range legacyRows {
			switch row.Key {
			case legacySettingKeyBackupInterval:
				days := defaultLegacyIntervalDays
				if parsed, err := strconv.Atoi(strings.TrimSpace(row.Value)); err == nil && parsed >= 1 {
					days = parsed
				}
				cronSpec := fmt.Sprintf("0 3 */%d * *", days)
				if err := upsertBackupSetting(tx, SettingKeyBackupScheduleCron, cronSpec, "string"); err != nil {
					return err
				}
			case legacySettingKeyBackupRetention:
				count := defaultLegacyRetentionCount
				if parsed, err := strconv.Atoi(strings.TrimSpace(row.Value)); err == nil && parsed >= 1 {
					count = parsed
				}
				if err := upsertBackupSetting(tx, SettingKeyBackupRetentionCount, strconv.Itoa(count), "int"); err != nil {
					return err
				}
			}

			if err := tx.Delete(&models.Setting{}, row.ID).Error; err != nil {
				return fmt.Errorf("delete legacy backup setting %s: %w", row.Key, err)
			}
		}
		return nil
	})
}

// upsertBackupSetting writes key/value/valueType under backupSettingsCategory,
// creating the row if absent or updating it in place if present.
func upsertBackupSetting(tx *gorm.DB, key, value, valueType string) error {
	var existing models.Setting
	err := tx.Where("key = ?", key).First(&existing).Error
	switch {
	case err == nil:
		existing.Value = value
		existing.Type = valueType
		existing.Category = backupSettingsCategory
		if saveErr := tx.Save(&existing).Error; saveErr != nil {
			return fmt.Errorf("update backup setting %s: %w", key, saveErr)
		}
		return nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		if createErr := tx.Create(&models.Setting{
			Key:      key,
			Value:    value,
			Type:     valueType,
			Category: backupSettingsCategory,
		}).Error; createErr != nil {
			return fmt.Errorf("create backup setting %s: %w", key, createErr)
		}
		return nil
	default:
		return fmt.Errorf("query backup setting %s: %w", key, err)
	}
}
