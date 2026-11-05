package models_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
)

// captureRestoreWarnings routes the module logger to a buffer for the test and returns the buffer.
func captureRestoreWarnings(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	logger.Init(false, &buf)
	t.Cleanup(func() { logger.Init(false, io.Discard) })
	return &buf
}

// countQueriesOn counts the queries run against table from now on. The count is read through the returned pointer.
func countQueriesOn(db *gorm.DB, table string) *int {
	count := new(int)
	_ = db.Callback().Query().Before("gorm:query").Register("test:count_"+table, func(tx *gorm.DB) {
		if tx.Statement.Table == table {
			*count++
		}
	})
	return count
}

// captureWafWriterChange captures the default row and the WAF switch, then switches the WAF on as a writer would. The
// writer's changes are sealed by the caller.
func captureWafWriterChange(t *testing.T, db *gorm.DB, row models.SecurityConfig) models.ModuleStateSnapshot {
	t.Helper()
	snap, err := models.CaptureModuleState(db, models.DefaultSecurityConfigName, &row)
	require.NoError(t, err)
	require.NoError(t, models.SetModuleState(db, models.SecurityWAFEnabledSettingKey, true, "writer"))
	return snap
}

func TestSeal_ReadsEachImagedKeyOnceAndTheRowOnce(t *testing.T) {
	db := newModuleTestDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, WAFMode: "block"}).Error)
	row := readDefaultRow(t, db)
	snap, err := models.CaptureModuleState(db, models.DefaultSecurityConfigName, &row, "security.extra.key")
	require.NoError(t, err)

	settingReads := countQueriesOn(db, "settings")
	rowReads := countQueriesOn(db, "security_configs")
	require.NoError(t, snap.Seal(db))

	// Three module switches, the two Cerberus keys and the extra key are imaged: six keys, one read each.
	assert.Equal(t, 6, *settingReads)
	assert.Equal(t, 1, *rowReads)
}

func TestSeal_ErrorsWhenAReadFails(t *testing.T) {
	t.Run("settings read fails", func(t *testing.T) {
		db := newModuleTestDB(t)
		snap, err := models.CaptureModuleState(db, models.DefaultSecurityConfigName, nil)
		require.NoError(t, err)
		failQueriesOn(db, "settings")
		assert.Error(t, snap.Seal(db))
	})
	t.Run("row read fails", func(t *testing.T) {
		db := newModuleTestDB(t)
		snap, err := models.CaptureModuleState(db, models.DefaultSecurityConfigName, nil)
		require.NoError(t, err)
		failQueriesOn(db, "security_configs")
		assert.Error(t, snap.Seal(db))
	})
	t.Run("row vanished during the write", func(t *testing.T) {
		db := newModuleTestDB(t)
		require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName}).Error)
		row := readDefaultRow(t, db)
		snap, err := models.CaptureModuleState(db, models.DefaultSecurityConfigName, &row)
		require.NoError(t, err)
		require.NoError(t, db.Where("name = ?", models.DefaultSecurityConfigName).Delete(&models.SecurityConfig{}).Error)
		assert.Error(t, snap.Seal(db))
	})
}

func TestRestore_RefusesAnUnsealedSnapshot(t *testing.T) {
	db := newModuleTestDB(t)
	snap, err := models.CaptureModuleState(db, models.DefaultSecurityConfigName, nil)
	require.NoError(t, err)
	assert.Error(t, snap.RestoreSettings(db))
	assert.Error(t, snap.RestoreRow(db))
}

func TestRestore_KeepsConcurrentWriteToUntouchedKeysAndColumns(t *testing.T) {
	db := newModuleTestDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, WAFMode: "disabled", AdminWhitelist: "192.0.2.0/24"}).Error)
	snap := captureWafWriterChange(t, db, readDefaultRow(t, db))
	require.NoError(t, snap.Seal(db))

	// A concurrent admin write lands on the ACL switch and the admin whitelist column, which the writer did not touch.
	setModuleSetting(t, db, models.SecurityACLEnabledSettingKey, "true")
	require.NoError(t, db.Model(&models.SecurityConfig{}).Where("name = ?", models.DefaultSecurityConfigName).Update("admin_whitelist", "203.0.113.0/24").Error)

	require.NoError(t, snap.RestoreSettings(db))
	require.NoError(t, snap.RestoreRow(db))

	acl, found := readSettingValue(t, db, models.SecurityACLEnabledSettingKey)
	assert.True(t, found)
	assert.Equal(t, "true", acl, "an untouched key keeps the concurrent value")
	restored := readDefaultRow(t, db)
	assert.Equal(t, "203.0.113.0/24", restored.AdminWhitelist, "an untouched column keeps the concurrent value")
	assert.Equal(t, "disabled", restored.WAFMode, "a touched column is restored")
	assert.False(t, restored.Enabled, "a touched column is restored")
	_, walFound := readSettingValue(t, db, models.SecurityWAFEnabledSettingKey)
	assert.False(t, walFound, "a touched key that did not exist is removed again")
}

func TestRestore_KeepsConcurrentWriteToTouchedKeysAndLogsEachSkip(t *testing.T) {
	db := newModuleTestDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, WAFMode: "disabled"}).Error)
	snap := captureWafWriterChange(t, db, readDefaultRow(t, db))
	require.NoError(t, snap.Seal(db))

	// A concurrent write changes the WAF switch and the WAF mode column the writer touched.
	require.NoError(t, db.Model(&models.Setting{}).Where("key = ?", models.SecurityWAFEnabledSettingKey).Update("value", "false").Error)
	require.NoError(t, db.Model(&models.SecurityConfig{}).Where("name = ?", models.DefaultSecurityConfigName).Update("waf_mode", "monitor").Error)

	logs := captureRestoreWarnings(t)
	require.NoError(t, snap.RestoreSettings(db))
	require.NoError(t, snap.RestoreRow(db))

	value, found := readSettingValue(t, db, models.SecurityWAFEnabledSettingKey)
	assert.True(t, found)
	assert.Equal(t, "false", value, "a touched key changed by a concurrent write is kept")
	assert.Equal(t, "monitor", readDefaultRow(t, db).WAFMode, "a touched column changed by a concurrent write is kept")
	assert.Contains(t, logs.String(), "restore skipped", "each skipped restore logs one warning")
	assert.Contains(t, logs.String(), models.SecurityWAFEnabledSettingKey)
	assert.Contains(t, logs.String(), "waf_mode")
}

func TestRestore_MixedOutcomeAcrossTheTwoTransactions(t *testing.T) {
	db := newModuleTestDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, WAFMode: "disabled"}).Error)
	snap := captureWafWriterChange(t, db, readDefaultRow(t, db))
	require.NoError(t, snap.Seal(db))

	require.NoError(t, snap.RestoreSettings(db))
	// A concurrent admin write lands between the settings transaction and the row transaction, on a touched column.
	require.NoError(t, db.Model(&models.SecurityConfig{}).Where("name = ?", models.DefaultSecurityConfigName).Update("waf_mode", "monitor").Error)
	require.NoError(t, snap.RestoreRow(db))

	// The documented mixed result: the settings are back to the prior state, the concurrent column is kept.
	_, found := readSettingValue(t, db, models.SecurityWAFEnabledSettingKey)
	assert.False(t, found, "the settings transaction restored the key")
	restored := readDefaultRow(t, db)
	assert.Equal(t, "monitor", restored.WAFMode, "the row transaction kept the concurrent column")
	assert.False(t, restored.Enabled, "the row transaction restored its untouched-by-concurrency column")
}

func TestRestore_KeepsARowCreatedByTheWriterThatAConcurrentWriteChanged(t *testing.T) {
	db := newModuleTestDB(t)
	snap, err := models.CaptureModuleState(db, models.DefaultSecurityConfigName, nil)
	require.NoError(t, err)
	require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, AdminWhitelist: "192.0.2.0/24"}).Error)
	require.NoError(t, snap.Seal(db))

	require.NoError(t, db.Model(&models.SecurityConfig{}).Where("name = ?", models.DefaultSecurityConfigName).Update("admin_whitelist", "203.0.113.0/24").Error)
	logs := captureRestoreWarnings(t)
	require.NoError(t, snap.RestoreRow(db))

	restored := readDefaultRow(t, db)
	assert.Equal(t, "203.0.113.0/24", restored.AdminWhitelist, "a row a concurrent write changed is kept")
	assert.Contains(t, logs.String(), "restore skipped")
}

func TestRestore_DeletesARowCreatedByTheWriterWhenItIsUnchanged(t *testing.T) {
	db := newModuleTestDB(t)
	snap, err := models.CaptureModuleState(db, models.DefaultSecurityConfigName, nil)
	require.NoError(t, err)
	require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, WAFMode: "block"}).Error)
	require.NoError(t, snap.Seal(db))

	require.NoError(t, snap.RestoreRow(db))
	_, err = models.FindDefaultSecurityConfig(db)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestRestore_NoTouchedChangesIssueNoWrites(t *testing.T) {
	db := newModuleTestDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, WAFMode: "block"}).Error)
	row := readDefaultRow(t, db)
	snap, err := models.CaptureModuleState(db, models.DefaultSecurityConfigName, &row)
	require.NoError(t, err)
	require.NoError(t, snap.Seal(db))

	// Nothing changed between capture and seal, so a restore leaves a concurrent write to the same row alone.
	require.NoError(t, db.Model(&models.SecurityConfig{}).Where("name = ?", models.DefaultSecurityConfigName).Update("waf_mode", "monitor").Error)
	require.NoError(t, snap.RestoreRow(db))
	assert.Equal(t, "monitor", readDefaultRow(t, db).WAFMode)
}
