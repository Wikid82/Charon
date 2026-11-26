package services

import (
	"context"
	"testing"
	"time"

	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/testutil"
)

// openApplyHistoryTestDB returns a migrated database with three apply results: two refusals with message text and
// one success.
func openApplyHistoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(testutil.MemoryDSN(t)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.CaddyConfig{}, &models.Setting{}))
	when := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	require.NoError(t, db.Create(&[]models.CaddyConfig{
		{ConfigHash: "hash-a", AppliedAt: when, Success: false, ErrorMsg: "refused text one"},
		{ConfigHash: "hash-b", AppliedAt: when.Add(time.Minute), Success: false, ErrorMsg: "refused text two"},
		{ConfigHash: "hash-c", AppliedAt: when.Add(2 * time.Minute), Success: true},
	}).Error)
	return db
}

// TestClearStoredApplyErrorTextClearsTheTextOnce covers the one-time clearing: the text goes, the rows, times,
// outcomes and hashes stay, the marker is written, and a second run changes nothing and logs nothing.
func TestClearStoredApplyErrorTextClearsTheTextOnce(t *testing.T) {
	db := openApplyHistoryTestDB(t)
	hook := logtest.NewLocal(logger.Log().Logger)
	t.Cleanup(hook.Reset)

	cleared, err := ClearStoredApplyErrorText(context.Background(), db)

	require.NoError(t, err)
	assert.Equal(t, int64(2), cleared)
	var rows []models.CaddyConfig
	require.NoError(t, db.Order("id").Find(&rows).Error)
	require.Len(t, rows, 3)
	for _, row := range rows {
		assert.Empty(t, row.ErrorMsg)
		assert.NotEmpty(t, row.ConfigHash)
	}
	assert.False(t, rows[0].Success)
	assert.True(t, rows[2].Success)
	assert.Equal(t, time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC), rows[0].AppliedAt.UTC())

	var marker models.Setting
	require.NoError(t, db.Where("key = ?", CaddyApplyErrorTextMarker).First(&marker).Error)
	assert.Equal(t, "migration", marker.Category)
	require.Len(t, hook.AllEntries(), 1, "one Info line for the first run")
	assert.Equal(t, int64(2), hook.LastEntry().Data["count"])

	hook.Reset()
	cleared, err = ClearStoredApplyErrorText(context.Background(), db)
	require.NoError(t, err)
	assert.Zero(t, cleared)
	assert.Empty(t, hook.AllEntries(), "the second run logs nothing")
}

// TestClearStoredApplyErrorTextWithoutRowsWritesTheMarker covers a database that never had a refusal.
func TestClearStoredApplyErrorTextWithoutRowsWritesTheMarker(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(testutil.MemoryDSN(t)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.CaddyConfig{}, &models.Setting{}))

	cleared, err := ClearStoredApplyErrorText(context.Background(), db)

	require.NoError(t, err)
	assert.Zero(t, cleared)
	var count int64
	require.NoError(t, db.Model(&models.Setting{}).Where("key = ?", CaddyApplyErrorTextMarker).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

// TestClearStoredApplyErrorTextReportsAMissingTable returns the error when the tables are absent.
func TestClearStoredApplyErrorTextReportsAMissingTable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(testutil.MemoryDSN(t)), &gorm.Config{})
	require.NoError(t, err)

	_, err = ClearStoredApplyErrorText(context.Background(), db)

	require.Error(t, err)
}
