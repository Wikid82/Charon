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
	err = db.AutoMigrate(&models.Notification{}, &models.NotificationProvider{}, &models.Setting{}, &models.ProxyHost{}, &models.UptimeMonitor{}, &models.UptimeHeartbeat{})
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
		DomainNames: "down.example.com", // This won't resolve or connect
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
	us.CheckAll()
	time.Sleep(200 * time.Millisecond) // Increased wait time for HTTP check

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

	us.CheckAll()
	time.Sleep(200 * time.Millisecond)

	db.Where("proxy_host_id = ?", upHost.ID).First(&upMonitor)
	assert.Equal(t, "down", upMonitor.Status)

	db.Find(&notifications)
	assert.Equal(t, 1, len(notifications), "Should have 1 notification now")
	if len(notifications) > 0 {
		assert.Contains(t, notifications[0].Message, upHost.DomainNames, "Notification should mention the host")
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
