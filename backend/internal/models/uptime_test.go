package models_test

import (
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestUptimeMonitor_BeforeCreate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.UptimeMonitor{}))

	monitor := models.UptimeMonitor{
		Name: "Test",
	}
	err = db.Create(&monitor).Error
	require.NoError(t, err)

	assert.NotEmpty(t, monitor.ID)
	assert.Equal(t, "pending", monitor.Status)
}

func TestUptimeHeartbeat_FreshAutoMigrateIndexes(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.UptimeHeartbeat{}))

	var names []string
	require.NoError(t, db.Raw(
		"SELECT name FROM sqlite_master WHERE type='index' AND tbl_name='uptime_heartbeats'").
		Scan(&names).Error)

	assert.NotContains(t, names, "idx_uptime_heartbeats_monitor_id",
		"bare monitor_id index is redundant with the composites and must not be created")
	assert.Contains(t, names, "idx_heartbeat_lookup")
	assert.Contains(t, names, "idx_uptime_heartbeats_created_at")
}
