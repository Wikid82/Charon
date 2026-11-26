package routes

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/testutil"
)

// TestClearApplyErrorTextClearsAndRecordsTheMarker runs the start-up fix on a migrated database with one stored
// refusal and expects the text gone and the marker written.
func TestClearApplyErrorTextClearsAndRecordsTheMarker(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(testutil.MemoryDSN(t)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.CaddyConfig{}, &models.Setting{}))
	require.NoError(t, db.Create(&models.CaddyConfig{ConfigHash: "hash", ErrorMsg: "stored refusal text"}).Error)

	require.NoError(t, clearApplyErrorText(context.Background(), db))

	var row models.CaddyConfig
	require.NoError(t, db.First(&row).Error)
	assert.Empty(t, row.ErrorMsg)
	var marked int64
	require.NoError(t, db.Model(&models.Setting{}).Where("key = ?", "migration.caddy_apply_error_text_v1").Count(&marked).Error)
	assert.Equal(t, int64(1), marked)
}

// TestClearApplyErrorTextReportsAFailure returns the error when the tables are missing, so the caller can log it.
func TestClearApplyErrorTextReportsAFailure(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(testutil.MemoryDSN(t)), &gorm.Config{})
	require.NoError(t, err)

	require.Error(t, clearApplyErrorText(context.Background(), db))
}
