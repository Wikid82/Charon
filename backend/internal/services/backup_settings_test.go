package services

import (
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestBackupSettingKeys_Constants locks in the canonical Setting keys from
// spec §3.4.4 — the typed /backups/settings API (Commit 3) and BackupService
// read/write these exact strings, category "backup".
func TestBackupSettingKeys_Constants(t *testing.T) {
	assert.Equal(t, "backup.schedule_enabled", SettingKeyBackupScheduleEnabled)
	assert.Equal(t, "backup.schedule_cron", SettingKeyBackupScheduleCron)
	assert.Equal(t, "backup.retention_count", SettingKeyBackupRetentionCount)
	assert.Equal(t, "backup.remote_retention_count", SettingKeyBackupRemoteRetentionCount)
	assert.Equal(t, "backup.encryption_enabled", SettingKeyBackupEncryptionEnabled)
	assert.Equal(t, "backup.encryption_passphrase_enc", SettingKeyBackupEncryptionPassphraseEnc)
}

func newTestSettingsDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Setting{}))
	return db
}

func TestMigrateLegacyBackupSettings_NilDB_NoOp(t *testing.T) {
	assert.NoError(t, migrateLegacyBackupSettings(nil))
}

func TestMigrateLegacyBackupSettings_NoLegacyRows_NoOp(t *testing.T) {
	db := newTestSettingsDB(t)
	require.NoError(t, migrateLegacyBackupSettings(db))

	var count int64
	require.NoError(t, db.Model(&models.Setting{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestMigrateLegacyBackupSettings_TranslatesIntervalAndRetention(t *testing.T) {
	db := newTestSettingsDB(t)
	require.NoError(t, db.Create(&models.Setting{Key: "backup.interval", Value: "3", Type: "int", Category: "system"}).Error)
	require.NoError(t, db.Create(&models.Setting{Key: "backup.retention", Value: "14", Type: "int", Category: "system"}).Error)

	require.NoError(t, migrateLegacyBackupSettings(db))

	// Legacy rows must be gone.
	var legacyCount int64
	require.NoError(t, db.Model(&models.Setting{}).
		Where("key IN ?", []string{"backup.interval", "backup.retention"}).
		Count(&legacyCount).Error)
	assert.Zero(t, legacyCount)

	var cronSetting models.Setting
	require.NoError(t, db.Where("key = ?", SettingKeyBackupScheduleCron).First(&cronSetting).Error)
	assert.Equal(t, "0 3 */3 * *", cronSetting.Value)
	assert.Equal(t, "backup", cronSetting.Category)

	var retentionSetting models.Setting
	require.NoError(t, db.Where("key = ?", SettingKeyBackupRetentionCount).First(&retentionSetting).Error)
	assert.Equal(t, "14", retentionSetting.Value)
	assert.Equal(t, "backup", retentionSetting.Category)
}

func TestMigrateLegacyBackupSettings_InvalidValues_FallBackToDefaults(t *testing.T) {
	db := newTestSettingsDB(t)
	require.NoError(t, db.Create(&models.Setting{Key: "backup.interval", Value: "not-a-number", Type: "int", Category: "system"}).Error)
	require.NoError(t, db.Create(&models.Setting{Key: "backup.retention", Value: "-5", Type: "int", Category: "system"}).Error)

	require.NoError(t, migrateLegacyBackupSettings(db))

	var cronSetting models.Setting
	require.NoError(t, db.Where("key = ?", SettingKeyBackupScheduleCron).First(&cronSetting).Error)
	assert.Equal(t, "0 3 */1 * *", cronSetting.Value)

	var retentionSetting models.Setting
	require.NoError(t, db.Where("key = ?", SettingKeyBackupRetentionCount).First(&retentionSetting).Error)
	assert.Equal(t, "7", retentionSetting.Value)
}

func TestMigrateLegacyBackupSettings_Idempotent(t *testing.T) {
	db := newTestSettingsDB(t)
	require.NoError(t, db.Create(&models.Setting{Key: "backup.interval", Value: "1", Type: "int", Category: "system"}).Error)

	require.NoError(t, migrateLegacyBackupSettings(db))
	require.NoError(t, migrateLegacyBackupSettings(db)) // second call must be a no-op, not error

	var count int64
	require.NoError(t, db.Model(&models.Setting{}).Where("key = ?", SettingKeyBackupScheduleCron).Count(&count).Error)
	assert.EqualValues(t, 1, count)
}

func TestMigrateLegacyBackupSettings_OnlyRetentionPresent(t *testing.T) {
	db := newTestSettingsDB(t)
	require.NoError(t, db.Create(&models.Setting{Key: "backup.retention", Value: "5", Type: "int", Category: "system"}).Error)

	require.NoError(t, migrateLegacyBackupSettings(db))

	var retentionSetting models.Setting
	require.NoError(t, db.Where("key = ?", SettingKeyBackupRetentionCount).First(&retentionSetting).Error)
	assert.Equal(t, "5", retentionSetting.Value)

	// schedule_cron must not have been created — only retention was migrated.
	var cronCount int64
	require.NoError(t, db.Model(&models.Setting{}).Where("key = ?", SettingKeyBackupScheduleCron).Count(&cronCount).Error)
	assert.Zero(t, cronCount)
}
