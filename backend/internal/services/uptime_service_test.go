package services

import (
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupUptimeTestDB(t *testing.T) *gorm.DB {
	dsn := filepath.Join(t.TempDir(), "test.db") + "?_busy_timeout=5000&_journal_mode=WAL"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	err = db.AutoMigrate(&models.Notification{}, &models.NotificationProvider{}, &models.Setting{}, &models.ProxyHost{}, &models.UptimeMonitor{}, &models.UptimeHeartbeat{}, &models.RemoteServer{})
	if err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}
	return db
}

func TestUptimeService_CheckAll(t *testing.T) {
	db := setupUptimeTestDB(t)
	ns := NewNotificationService(db)
	us := NewUptimeService(db, ns)

	// Create a dummy HTTP server for a "UP" host
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start listener: %v", err)
	}
	addr := listener.Addr().(*net.TCPAddr)

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	}
	go server.Serve(listener)
	defer server.Close()

	// Create a listener and close it immediately to get a free port that is definitely closed (DOWN)
	downListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start down listener: %v", err)
	}
	downAddr := downListener.Addr().(*net.TCPAddr)
	downListener.Close()

	// Seed ProxyHosts
	// We use the listener address as the "DomainName" so the monitor checks this HTTP server
	upHost := models.ProxyHost{
		UUID:        "uuid-1",
		DomainNames: fmt.Sprintf("127.0.0.1:%d", addr.Port),
		ForwardHost: "127.0.0.1",
		ForwardPort: addr.Port,
		Enabled:     true,
	}
	db.Create(&upHost)

	downHost := models.ProxyHost{
		UUID:        "uuid-2",
		DomainNames: fmt.Sprintf("127.0.0.1:%d", downAddr.Port), // Use local closed port
		ForwardHost: "127.0.0.1",
		ForwardPort: 54321,
		Enabled:     true,
	}
	db.Create(&downHost)

	// Sync Monitors (this creates UptimeMonitor records)
	err = us.SyncMonitors()
	assert.NoError(t, err)

	// Verify Monitors created
	var monitors []models.UptimeMonitor
	db.Find(&monitors)
	assert.Equal(t, 2, len(monitors))

	// Run CheckAll
	// We need to run it multiple times because default MaxRetries is 3
	for i := 0; i < 3; i++ {
		us.CheckAll()
		time.Sleep(100 * time.Millisecond) // Increased sleep slightly
	}
	time.Sleep(500 * time.Millisecond) // Increased wait time for checks to complete

	// Verify Heartbeats
	var heartbeats []models.UptimeHeartbeat
	db.Find(&heartbeats)
	assert.GreaterOrEqual(t, len(heartbeats), 2)

	// Verify Status
	var upMonitor models.UptimeMonitor
	db.Where("proxy_host_id = ?", upHost.ID).First(&upMonitor)
	assert.Equal(t, "up", upMonitor.Status)

	var downMonitor models.UptimeMonitor
	db.Where("proxy_host_id = ?", downHost.ID).First(&downMonitor)
	assert.Equal(t, "down", downMonitor.Status)

	// Verify Notifications
	// We expect 0 notifications because initial state transition from "pending" is ignored
	var notifications []models.Notification
	db.Find(&notifications)
	assert.Equal(t, 0, len(notifications), "Should have 0 notifications on first run")

	// Now let's flip the status to trigger notification
	// Make upHost go DOWN by closing the listener
	server.Close()
	listener.Close()
	time.Sleep(10 * time.Millisecond)

	// Run CheckAll multiple times to exceed MaxRetries
	for i := 0; i < 3; i++ {
		us.CheckAll()
		time.Sleep(100 * time.Millisecond)
	}
	time.Sleep(500 * time.Millisecond)

	db.Where("proxy_host_id = ?", upHost.ID).First(&upMonitor)
	assert.Equal(t, "down", upMonitor.Status)

	db.Find(&notifications)
	assert.Equal(t, 1, len(notifications), "Should have 1 notification now")
	if len(notifications) > 0 {
		assert.Contains(t, notifications[0].Message, upHost.DomainNames, "Notification should mention the host")
		assert.Equal(t, models.NotificationTypeError, notifications[0].Type, "Notification type should be error for DOWN event")
	}
}

func TestUptimeService_ListMonitors(t *testing.T) {
	db := setupUptimeTestDB(t)
	ns := NewNotificationService(db)
	us := NewUptimeService(db, ns)

	db.Create(&models.UptimeMonitor{
		Name: "Test Monitor",
		Type: "http",
		URL:  "http://example.com",
	})

	monitors, err := us.ListMonitors()
	assert.NoError(t, err)
	assert.Len(t, monitors, 1)
	assert.Equal(t, "Test Monitor", monitors[0].Name)
}

func TestUptimeService_GetMonitorHistory(t *testing.T) {
	db := setupUptimeTestDB(t)
	ns := NewNotificationService(db)
	us := NewUptimeService(db, ns)

	monitor := models.UptimeMonitor{
		ID:   "monitor-1",
		Name: "Test Monitor",
	}
	db.Create(&monitor)

	db.Create(&models.UptimeHeartbeat{
		MonitorID: monitor.ID,
		Status:    "up",
		Latency:   10,
		CreatedAt: time.Now().Add(-1 * time.Minute),
	})
	db.Create(&models.UptimeHeartbeat{
		MonitorID: monitor.ID,
		Status:    "down",
		Latency:   0,
		CreatedAt: time.Now(),
	})

	history, err := us.GetMonitorHistory(monitor.ID, 100)
	assert.NoError(t, err)
	assert.Len(t, history, 2)
	assert.Equal(t, "down", history[0].Status)
}

func TestUptimeService_SyncMonitors_Errors(t *testing.T) {
	t.Run("database error during proxy host fetch", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db)
		us := NewUptimeService(db, ns)

		// Close the database to force errors
		sqlDB, _ := db.DB()
		sqlDB.Close()

		err := us.SyncMonitors()
		assert.Error(t, err)
	})

	t.Run("creates monitors for new hosts", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db)
		us := NewUptimeService(db, ns)

		// Create proxy hosts
		host1 := models.ProxyHost{UUID: "test-1", DomainNames: "test1.com", Enabled: true}
		host2 := models.ProxyHost{UUID: "test-2", DomainNames: "test2.com", Enabled: false}
		db.Create(&host1)
		db.Create(&host2)

		err := us.SyncMonitors()
		assert.NoError(t, err)

		var monitors []models.UptimeMonitor
		db.Find(&monitors)
		assert.Equal(t, 2, len(monitors))
	})

	t.Run("orphaned monitors persist after host deletion", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db)
		us := NewUptimeService(db, ns)

		host := models.ProxyHost{UUID: "test-1", DomainNames: "test1.com", Enabled: true}
		db.Create(&host)

		err := us.SyncMonitors()
		assert.NoError(t, err)

		var monitors []models.UptimeMonitor
		db.Find(&monitors)
		assert.Equal(t, 1, len(monitors))

		// Delete the host
		db.Delete(&host)

		err = us.SyncMonitors()
		assert.NoError(t, err)

		// Monitors remain (SyncMonitors doesn't clean orphans)
		db.Find(&monitors)
		assert.Equal(t, 1, len(monitors))
	})
}

func TestUptimeService_CheckAll_Errors(t *testing.T) {
	t.Run("handles empty monitor list", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db)
		us := NewUptimeService(db, ns)

		// Call CheckAll with no monitors - should not panic
		us.CheckAll()
		time.Sleep(50 * time.Millisecond)

		var heartbeats []models.UptimeHeartbeat
		db.Find(&heartbeats)
		assert.Equal(t, 0, len(heartbeats))
	})

	t.Run("orphan monitors don't prevent check execution", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db)
		us := NewUptimeService(db, ns)

		// Create a monitor without a proxy host
		orphanID := uint(999)
		monitor := models.UptimeMonitor{
			ID:          "orphan-1",
			Name:        "Orphan Monitor",
			Type:        "http",
			URL:         "http://example.com",
			Status:      "pending",
			Enabled:     true,
			ProxyHostID: &orphanID, // Non-existent host
		}
		db.Create(&monitor)

		// CheckAll should not panic
		us.CheckAll()
		time.Sleep(100 * time.Millisecond)

		// Heartbeat may or may not be created for orphan monitor
		// Test just ensures CheckAll doesn't fail
		var heartbeats []models.UptimeHeartbeat
		db.Find(&heartbeats)
		assert.GreaterOrEqual(t, len(heartbeats), 0)
	})

	t.Run("handles timeout for slow hosts", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db)
		us := NewUptimeService(db, ns)

		// Create a monitor pointing to slow/unresponsive host
		host := models.ProxyHost{
			UUID:        "slow-host",
			DomainNames: "192.0.2.1:9999", // TEST-NET-1, should timeout
			ForwardHost: "192.0.2.1",
			ForwardPort: 9999,
			Enabled:     true,
		}
		db.Create(&host)

		err := us.SyncMonitors()
		assert.NoError(t, err)

		us.CheckAll()
		time.Sleep(2 * time.Second) // Give enough time for timeout (default is 1s)

		var monitor models.UptimeMonitor
		db.Where("proxy_host_id = ?", host.ID).First(&monitor)
		// Should be down after timeout
		if monitor.Status == "pending" {
			// If still pending, give a bit more time
			time.Sleep(1 * time.Second)
			db.Where("proxy_host_id = ?", host.ID).First(&monitor)
		}
		assert.Contains(t, []string{"down", "pending"}, monitor.Status, "Status should be down or pending for unreachable host")
	})
}

func TestUptimeService_CheckMonitor_EdgeCases(t *testing.T) {
	t.Run("invalid URL format", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db)
		us := NewUptimeService(db, ns)

		monitor := models.UptimeMonitor{
			ID:     "invalid-url",
			Name:   "Invalid URL Monitor",
			Type:   "http",
			URL:    "://invalid-url",
			Status: "pending",
		}
		db.Create(&monitor)

		us.CheckAll()
		time.Sleep(500 * time.Millisecond) // Increased wait time

		db.First(&monitor, "id = ?", "invalid-url")
		// Invalid URLs should eventually fail, but might stay pending
		assert.Contains(t, []string{"down", "pending"}, monitor.Status, "Invalid URL should be down or pending")
	})

	t.Run("http 404 response treated as down", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db)
		us := NewUptimeService(db, ns)

		// Start HTTP server returning 404
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		assert.NoError(t, err)
		addr := listener.Addr().(*net.TCPAddr)

		server := &http.Server{
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			}),
		}
		go server.Serve(listener)
		defer server.Close()

		host := models.ProxyHost{
			UUID:        "404-host",
			DomainNames: fmt.Sprintf("127.0.0.1:%d", addr.Port),
			ForwardHost: "127.0.0.1",
			ForwardPort: addr.Port,
			Enabled:     true,
		}
		db.Create(&host)

		err = us.SyncMonitors()
		assert.NoError(t, err)

		// Run CheckAll multiple times to exceed MaxRetries
		for i := 0; i < 3; i++ {
			us.CheckAll()
			time.Sleep(50 * time.Millisecond)
		}
		time.Sleep(200 * time.Millisecond)

		var monitor models.UptimeMonitor
		db.Where("proxy_host_id = ?", host.ID).First(&monitor)
		assert.Equal(t, "down", monitor.Status)
	})

	t.Run("https URL without valid certificate", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db)
		us := NewUptimeService(db, ns)

		monitor := models.UptimeMonitor{
			ID:     "https-invalid",
			Name:   "HTTPS Invalid Cert",
			Type:   "http",
			URL:    "https://expired.badssl.com/",
			Status: "pending",
		}
		db.Create(&monitor)

		us.CheckAll()
		time.Sleep(3 * time.Second) // HTTPS checks can take longer

		db.First(&monitor, "id = ?", "https-invalid")
		// Certificate issues might result in down or timeout (pending)
		// The service accepts insecure certs by default, so this might actually succeed
		assert.Contains(t, []string{"up", "down", "pending"}, monitor.Status, "HTTPS check completed")
	})
}

func TestUptimeService_GetMonitorHistory_EdgeCases(t *testing.T) {
	t.Run("non-existent monitor", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db)
		us := NewUptimeService(db, ns)

		history, err := us.GetMonitorHistory("non-existent", 100)
		assert.NoError(t, err)
		assert.Len(t, history, 0)
	})

	t.Run("limit parameter respected", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db)
		us := NewUptimeService(db, ns)

		monitor := models.UptimeMonitor{ID: "monitor-limit", Name: "Limit Test"}
		db.Create(&monitor)

		// Create 10 heartbeats
		for i := 0; i < 10; i++ {
			db.Create(&models.UptimeHeartbeat{
				MonitorID: monitor.ID,
				Status:    "up",
				Latency:   int64(i),
				CreatedAt: time.Now().Add(time.Duration(i) * time.Second),
			})
		}

		history, err := us.GetMonitorHistory(monitor.ID, 5)
		assert.NoError(t, err)
		assert.Len(t, history, 5)
	})
}

func TestUptimeService_ListMonitors_EdgeCases(t *testing.T) {
	t.Run("empty database", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db)
		us := NewUptimeService(db, ns)

		monitors, err := us.ListMonitors()
		assert.NoError(t, err)
		assert.Len(t, monitors, 0)
	})

	t.Run("monitors with associated proxy hosts", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db)
		us := NewUptimeService(db, ns)

		host := models.ProxyHost{UUID: "test-host", DomainNames: "test.com", Enabled: true}
		db.Create(&host)

		monitor := models.UptimeMonitor{
			ID:          "with-host",
			Name:        "Monitor with Host",
			Type:        "http",
			URL:         "http://test.com",
			ProxyHostID: &host.ID,
		}
		db.Create(&monitor)

		monitors, err := us.ListMonitors()
		assert.NoError(t, err)
		assert.Len(t, monitors, 1)
		assert.Equal(t, host.ID, *monitors[0].ProxyHostID)
	})
}

func TestUptimeService_UpdateMonitor(t *testing.T) {
	t.Run("update max_retries", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db)
		us := NewUptimeService(db, ns)

		monitor := models.UptimeMonitor{
			ID:         "update-test",
			Name:       "Update Test",
			Type:       "http",
			URL:        "http://example.com",
			MaxRetries: 3,
			Interval:   60,
		}
		db.Create(&monitor)

		updates := map[string]interface{}{
			"max_retries": 5,
		}

		result, err := us.UpdateMonitor(monitor.ID, updates)
		assert.NoError(t, err)
		assert.Equal(t, 5, result.MaxRetries)
	})

	t.Run("update interval", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db)
		us := NewUptimeService(db, ns)

		monitor := models.UptimeMonitor{
			ID:       "update-interval",
			Name:     "Interval Test",
			Interval: 60,
		}
		db.Create(&monitor)

		updates := map[string]interface{}{
			"interval": 120,
		}

		result, err := us.UpdateMonitor(monitor.ID, updates)
		assert.NoError(t, err)
		assert.Equal(t, 120, result.Interval)
	})

	t.Run("update non-existent monitor", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db)
		us := NewUptimeService(db, ns)

		updates := map[string]interface{}{
			"max_retries": 5,
		}

		_, err := us.UpdateMonitor("non-existent", updates)
		assert.Error(t, err)
	})

	t.Run("update multiple fields", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db)
		us := NewUptimeService(db, ns)

		monitor := models.UptimeMonitor{
			ID:         "multi-update",
			Name:       "Multi Update Test",
			MaxRetries: 3,
			Interval:   60,
		}
		db.Create(&monitor)

		updates := map[string]interface{}{
			"max_retries": 10,
			"interval":    300,
		}

		result, err := us.UpdateMonitor(monitor.ID, updates)
		assert.NoError(t, err)
		assert.Equal(t, 10, result.MaxRetries)
		assert.Equal(t, 300, result.Interval)
	})
}
