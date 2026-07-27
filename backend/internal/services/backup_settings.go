package services

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

// Sentinel errors for the typed /api/v1/backups/settings endpoint (spec
// §3.3.2), mapped by the handler to specific HTTP status/error_code pairs.
var (
	ErrInvalidCronSchedule        = fmt.Errorf("invalid cron schedule")
	ErrInvalidRetentionCount      = fmt.Errorf("retention count must be >= 1")
	ErrEncryptionKeyMissingForOps = fmt.Errorf("%w: CHARON_ENCRYPTION_KEY is required for this operation", ErrEncryptionKeyMissing)
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

	// defaultScheduleCron is used until/unless a persisted
	// backup.schedule_cron setting is found (spec §3.4.4).
	defaultScheduleCron = "0 3 * * *"

	// defaultRemoteRetentionCount mirrors DefaultBackupRetention for the
	// per-target remote retention default (spec §3.4.4).
	defaultRemoteRetentionCount = DefaultBackupRetention
)

// readBackupSettingString returns the raw string Value of a backup.* (or
// any) Setting key. The bool return reports whether the row exists at all,
// so callers can distinguish "not set" from "set to the empty string".
func readBackupSettingString(db *gorm.DB, key string) (string, bool) {
	if db == nil {
		return "", false
	}
	var setting models.Setting
	if err := db.Where("key = ?", key).First(&setting).Error; err != nil {
		return "", false
	}
	return setting.Value, true
}

// readBackupSettingBool returns the parsed bool Value of key, or fallback
// when the row is absent or fails to parse as a bool.
func readBackupSettingBool(db *gorm.DB, key string, fallback bool) bool {
	raw, ok := readBackupSettingString(db, key)
	if !ok {
		return fallback
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return parsed
}

// readBackupSettingInt returns the parsed int Value of key, or fallback
// when the row is absent or fails to parse as an int.
func readBackupSettingInt(db *gorm.DB, key string, fallback int) int {
	raw, ok := readBackupSettingString(db, key)
	if !ok {
		return fallback
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return parsed
}

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

// BackupSettings is the typed facade over the backup.* Setting rows (spec
// §3.3.2 GET /api/v1/backups/settings).
type BackupSettings struct {
	ScheduleEnabled         bool   `json:"schedule_enabled"`
	ScheduleCron            string `json:"schedule_cron"`
	RetentionCount          int    `json:"retention_count"`
	RemoteRetentionCount    int    `json:"remote_retention_count"`
	EncryptionEnabled       bool   `json:"encryption_enabled"`
	EncryptionPassphraseSet bool   `json:"encryption_passphrase_set"`
}

// GetSettings reads the current backup.* settings (falling back to their
// documented defaults for any missing row).
func (s *BackupService) GetSettings() BackupSettings {
	scheduleCron := defaultScheduleCron
	if persisted, ok := readBackupSettingString(s.db, SettingKeyBackupScheduleCron); ok && strings.TrimSpace(persisted) != "" {
		scheduleCron = persisted
	}

	passphrase, hasPassphrase := readBackupSettingString(s.db, SettingKeyBackupEncryptionPassphraseEnc)

	return BackupSettings{
		ScheduleEnabled:         readBackupSettingBool(s.db, SettingKeyBackupScheduleEnabled, true),
		ScheduleCron:            scheduleCron,
		RetentionCount:          readBackupSettingInt(s.db, SettingKeyBackupRetentionCount, DefaultBackupRetention),
		RemoteRetentionCount:    readBackupSettingInt(s.db, SettingKeyBackupRemoteRetentionCount, defaultRemoteRetentionCount),
		EncryptionEnabled:       readBackupSettingBool(s.db, SettingKeyBackupEncryptionEnabled, false),
		EncryptionPassphraseSet: hasPassphrase && strings.TrimSpace(passphrase) != "",
	}
}

// BackupSettingsUpdate is a partial update to the backup.* settings (spec
// §3.3.2 PUT /api/v1/backups/settings) — every field is optional; a nil
// pointer means "leave this setting unchanged". EncryptionPassphrase, when
// non-nil and non-empty, is encrypted with crypto.EncryptionService before
// storage; an empty string clears the stored passphrase.
type BackupSettingsUpdate struct {
	ScheduleEnabled      *bool
	ScheduleCron         *string
	RetentionCount       *int
	RemoteRetentionCount *int
	EncryptionEnabled    *bool
	EncryptionPassphrase *string
}

// UpdateSettings validates and persists a partial settings update, then
// reschedules the cron job if the schedule was touched (spec §3.4.4).
func (s *BackupService) UpdateSettings(update BackupSettingsUpdate) error {
	if s.db == nil {
		return fmt.Errorf("backup settings require a database connection")
	}

	if update.ScheduleCron != nil {
		if _, err := cron.ParseStandard(*update.ScheduleCron); err != nil {
			return fmt.Errorf("%w: %s", ErrInvalidCronSchedule, err.Error())
		}
	}
	if update.RetentionCount != nil && *update.RetentionCount < 1 {
		return ErrInvalidRetentionCount
	}
	if update.RemoteRetentionCount != nil && *update.RemoteRetentionCount < 1 {
		return ErrInvalidRetentionCount
	}
	if update.EncryptionEnabled != nil && *update.EncryptionEnabled && s.encryption == nil {
		existing, hasExisting := readBackupSettingString(s.db, SettingKeyBackupEncryptionPassphraseEnc)
		settingNow := update.EncryptionPassphrase != nil && *update.EncryptionPassphrase != ""
		if (!hasExisting || existing == "") && !settingNow {
			return ErrEncryptionKeyMissingForOps
		}
	}
	if update.EncryptionPassphrase != nil && *update.EncryptionPassphrase != "" && s.encryption == nil {
		return ErrEncryptionKeyMissingForOps
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		if update.ScheduleEnabled != nil {
			if err := upsertBackupSetting(tx, SettingKeyBackupScheduleEnabled, strconv.FormatBool(*update.ScheduleEnabled), "bool"); err != nil {
				return err
			}
		}
		if update.ScheduleCron != nil {
			if err := upsertBackupSetting(tx, SettingKeyBackupScheduleCron, *update.ScheduleCron, "string"); err != nil {
				return err
			}
		}
		if update.RetentionCount != nil {
			if err := upsertBackupSetting(tx, SettingKeyBackupRetentionCount, strconv.Itoa(*update.RetentionCount), "int"); err != nil {
				return err
			}
		}
		if update.RemoteRetentionCount != nil {
			if err := upsertBackupSetting(tx, SettingKeyBackupRemoteRetentionCount, strconv.Itoa(*update.RemoteRetentionCount), "int"); err != nil {
				return err
			}
		}
		if update.EncryptionEnabled != nil {
			if err := upsertBackupSetting(tx, SettingKeyBackupEncryptionEnabled, strconv.FormatBool(*update.EncryptionEnabled), "bool"); err != nil {
				return err
			}
		}
		if update.EncryptionPassphrase != nil {
			value := ""
			if *update.EncryptionPassphrase != "" {
				encrypted, encErr := s.encryption.Encrypt([]byte(*update.EncryptionPassphrase))
				if encErr != nil {
					return fmt.Errorf("encrypt backup passphrase: %w", encErr)
				}
				value = encrypted
			}
			if err := upsertBackupSetting(tx, SettingKeyBackupEncryptionPassphraseEnc, value, "string"); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	if update.ScheduleEnabled != nil || update.ScheduleCron != nil {
		effective := s.GetSettings()
		if effective.ScheduleEnabled {
			if rescheduleErr := s.Reschedule(effective.ScheduleCron); rescheduleErr != nil {
				return fmt.Errorf("reschedule backup cron: %w", rescheduleErr)
			}
		} else {
			s.mu.Lock()
			if s.scheduleEntry != 0 {
				s.Cron.Remove(s.scheduleEntry)
				s.scheduleEntry = 0
			}
			s.mu.Unlock()
		}
	}

	return nil
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
