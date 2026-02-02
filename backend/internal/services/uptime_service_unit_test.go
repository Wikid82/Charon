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

	// Close database connection when test completes
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func TestExtractPort(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"http url default", "http://example.com", "80"},
		{"https url default", "https://example.com", "443"},
		{"http with port", "http://example.com:8080", "8080"},
		{"https with port", "https://example.com:8443", "8443"},
		{"host:port", "example.com:3000", "3000"},
		{"plain host", "example.com", ""},
		{"localhost with port", "localhost:5000", "5000"},
		{"ip with port", "192.168.1.1:9090", "9090"},
		{"ipv6 with port", "[::1]:8080", "8080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractPort(tt.input)
			require.Equal(t, tt.expected, got)
		})
	}
}

func TestUpdateMonitorEnabled_Unit(t *testing.T) {
	db := setupUnitTestDB(t)
	svc := NewUptimeService(db, nil)

	monitor := models.UptimeMonitor{ID: uuid.New().String(), Name: "unit-test", URL: "http://example.com", Interval: 60, Enabled: true}
	require.NoError(t, db.Create(&monitor).Error)

	r, err := svc.UpdateMonitor(monitor.ID, map[string]any{"enabled": false})
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

// TestCheckMonitor_PublicAPI tests the public CheckMonitor wrapper
func TestCheckMonitor_PublicAPI(t *testing.T) {
	db := setupUnitTestDB(t)
	svc := NewUptimeService(db, nil)

	monitor := models.UptimeMonitor{
		ID:       uuid.New().String(),
		Name:     "test-public-check",
		URL:      "https://httpbin.org/status/200",
		Type:     "https",
		Interval: 60,
		Enabled:  true,
	}
	require.NoError(t, db.Create(&monitor).Error)

	// Call the public API (doesn't return error, just executes)
	svc.CheckMonitor(monitor)

	// Verify heartbeat was created
	var count int64
	db.Model(&models.UptimeHeartbeat{}).Where("monitor_id = ?", monitor.ID).Count(&count)
	require.Greater(t, count, int64(0))
}

// TestCheckMonitor_InvalidURL tests checking with invalid URL
func TestCheckMonitor_InvalidURL(t *testing.T) {
	db := setupUnitTestDB(t)
	svc := NewUptimeService(db, nil)

	monitor := models.UptimeMonitor{
		ID:       uuid.New().String(),
		Name:     "test-invalid-url",
		URL:      "http://invalid-domain-that-does-not-exist-12345.com",
		Type:     "http",
		Interval: 60,
		Enabled:  true,
	}
	require.NoError(t, db.Create(&monitor).Error)

	// This should create a "down" heartbeat
	svc.checkMonitor(monitor)

	// Verify heartbeat was created with "down" status
	var hb models.UptimeHeartbeat
	err := db.Where("monitor_id = ?", monitor.ID).Order("created_at desc").First(&hb).Error
	require.NoError(t, err)
	require.Equal(t, "down", hb.Status)
	require.NotEmpty(t, hb.Message)
}

// TestCheckMonitor_TCPSuccess tests TCP monitor success
func TestCheckMonitor_TCPSuccess(t *testing.T) {
	db := setupUnitTestDB(t)
	svc := NewUptimeService(db, nil)

	// Use a known accessible TCP port (Google DNS)
	monitor := models.UptimeMonitor{
		ID:       uuid.New().String(),
		Name:     "test-tcp-success",
		URL:      "8.8.8.8:53",
		Type:     "tcp",
		Interval: 60,
		Enabled:  true,
	}
	require.NoError(t, db.Create(&monitor).Error)

	svc.checkMonitor(monitor)

	// Verify heartbeat was created with "up" status
	var hb models.UptimeHeartbeat
	err := db.Where("monitor_id = ?", monitor.ID).Order("created_at desc").First(&hb).Error
	require.NoError(t, err)
	require.Equal(t, "up", hb.Status)
}

// TestCheckMonitor_TCPFailure tests TCP monitor failure
func TestCheckMonitor_TCPFailure(t *testing.T) {
	db := setupUnitTestDB(t)
	svc := NewUptimeService(db, nil)

	monitor := models.UptimeMonitor{
		ID:       uuid.New().String(),
		Name:     "test-tcp-failure",
		URL:      "192.0.2.1:9999", // TEST-NET-1, should timeout
		Type:     "tcp",
		Interval: 60,
		Enabled:  true,
	}
	require.NoError(t, db.Create(&monitor).Error)

	svc.checkMonitor(monitor)

	// Verify heartbeat was created with "down" status
	var hb models.UptimeHeartbeat
	err := db.Where("monitor_id = ?", monitor.ID).Order("created_at desc").First(&hb).Error
	require.NoError(t, err)
	require.Equal(t, "down", hb.Status)
	require.NotEmpty(t, hb.Message)
}

// TestCheckMonitor_UnknownType tests unknown monitor type
func TestCheckMonitor_UnknownType(t *testing.T) {
	db := setupUnitTestDB(t)
	svc := NewUptimeService(db, nil)

	monitor := models.UptimeMonitor{
		ID:       uuid.New().String(),
		Name:     "test-unknown-type",
		URL:      "http://example.com",
		Type:     "unknown-type",
		Interval: 60,
		Enabled:  true,
	}
	require.NoError(t, db.Create(&monitor).Error)

	svc.checkMonitor(monitor)

	// Verify heartbeat was created with "down" status
	var hb models.UptimeHeartbeat
	err := db.Where("monitor_id = ?", monitor.ID).Order("created_at desc").First(&hb).Error
	require.NoError(t, err)
	require.Equal(t, "down", hb.Status)
	require.Equal(t, "Unknown monitor type", hb.Message)
}

// TestDeleteMonitor_NonExistent tests deleting a non-existent monitor
func TestDeleteMonitor_NonExistent(t *testing.T) {
	db := setupUnitTestDB(t)
	svc := NewUptimeService(db, nil)

	// Try to delete non-existent monitor
	err := svc.DeleteMonitor("non-existent-id")
	require.Error(t, err)
}

// TestUpdateMonitor_NonExistent tests updating a non-existent monitor
func TestUpdateMonitor_NonExistent(t *testing.T) {
	db := setupUnitTestDB(t)
	svc := NewUptimeService(db, nil)

	// Try to update non-existent monitor
	_, err := svc.UpdateMonitor("non-existent-id", map[string]any{"enabled": false})
	require.Error(t, err)
}
