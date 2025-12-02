package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

func setupUnitTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.UptimeMonitor{}, &models.UptimeHeartbeat{}, &models.UptimeHost{}))
	return db
}

func TestUpdateMonitorEnabled_Unit(t *testing.T) {
	db := setupUnitTestDB(t)
	svc := NewUptimeService(db, nil)

	monitor := models.UptimeMonitor{ID: uuid.New().String(), Name: "unit-test", URL: "http://example.com", Interval: 60, Enabled: true}
	require.NoError(t, db.Create(&monitor).Error)

	r, err := svc.UpdateMonitor(monitor.ID, map[string]interface{}{"enabled": false})
	require.NoError(t, err)
	require.False(t, r.Enabled)

	var m models.UptimeMonitor
	require.NoError(t, db.First(&m, "id = ?", monitor.ID).Error)
	require.False(t, m.Enabled)
}

func TestDeleteMonitorDeletesHeartbeats_Unit(t *testing.T) {
	db := setupUnitTestDB(t)
	svc := NewUptimeService(db, nil)

	monitor := models.UptimeMonitor{ID: uuid.New().String(), Name: "unit-delete", URL: "http://example.com", Interval: 60, Enabled: true}
	require.NoError(t, db.Create(&monitor).Error)

	hb := models.UptimeHeartbeat{MonitorID: monitor.ID, Status: "up", Latency: 10, CreatedAt: time.Now()}
	require.NoError(t, db.Create(&hb).Error)

	require.NoError(t, svc.DeleteMonitor(monitor.ID))

	var m models.UptimeMonitor
	require.Error(t, db.First(&m, "id = ?", monitor.ID).Error)

	var count int64
	db.Model(&models.UptimeHeartbeat{}).Where("monitor_id = ?", monitor.ID).Count(&count)
	require.Equal(t, int64(0), count)
}
