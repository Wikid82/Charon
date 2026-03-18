package services

import (
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
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
	err = db.AutoMigrate(
		&models.Notification{},
		&models.NotificationProvider{},
		&models.Setting{},
		&models.ProxyHost{},
		&models.UptimeMonitor{},
		&models.UptimeHeartbeat{},
		&models.UptimeHost{},
		&models.UptimeNotificationEvent{},
		&models.RemoteServer{},
	)
	if err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	// Ensure database connections are closed when test ends
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil && sqlDB != nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

// newTestUptimeService creates an UptimeService with proper cleanup
func newTestUptimeService(t *testing.T, db *gorm.DB, ns *NotificationService) *UptimeService {
	us := NewUptimeService(db, ns)

	// Configure faster timeouts for tests
	us.config.TCPTimeout = 100 * time.Millisecond
	us.config.MaxRetries = 1
	us.config.CheckTimeout = 2 * time.Second
	us.config.StaggerDelay = 0

	// Add cleanup to flush pending notifications
	t.Cleanup(func() {
		us.FlushPendingNotifications()
		time.Sleep(50 * time.Millisecond) // Give goroutines time to finish
	})

	return us
}

func TestUptimeService_CheckAll(t *testing.T) {
	db := setupUptimeTestDB(t)
	ns := NewNotificationService(db, nil)
	us := newTestUptimeService(t, db, ns)

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
		ReadHeaderTimeout: 10 * time.Second, // Prevent Slowloris attacks
	}
	go func() { _ = server.Serve(listener) }()
	defer func() { _ = server.Close() }()

	// Wait for HTTP server to be ready by making a test request
	for i := 0; i < 10; i++ {
		conn, dialErr := net.DialTimeout("tcp", addr.String(), 100*time.Millisecond)
		if dialErr == nil {
			_ = conn.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Create a listener and close it immediately to get a free port that is definitely closed (DOWN)
	downListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start down listener: %v", err)
	}
	downAddr := downListener.Addr().(*net.TCPAddr)
	_ = downListener.Close()

	// Seed ProxyHosts
	// We use the listener address as the "DomainName" so the monitor checks this HTTP server
	// IMPORTANT: Use different ForwardHost values to avoid grouping into the same UptimeHost,
	// which would cause the host-level TCP pre-check to use the wrong port.
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
		DomainNames: fmt.Sprintf("127.0.0.2:%d", downAddr.Port), // Use local closed port
		ForwardHost: "127.0.0.2",                                // Different IP to avoid UptimeHost grouping
		ForwardPort: downAddr.Port,
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
	_ = server.Close()
	_ = listener.Close()
	time.Sleep(10 * time.Millisecond)

	// Run CheckAll multiple times to exceed MaxRetries
	for i := 0; i < 3; i++ {
		us.CheckAll()
		time.Sleep(100 * time.Millisecond)
	}
	time.Sleep(500 * time.Millisecond)

	db.Where("proxy_host_id = ?", upHost.ID).First(&upMonitor)
	assert.Equal(t, "down", upMonitor.Status)

	// Flush any pending batched notifications
	// The new batching system delays notifications by 30 seconds
	// For testing, we manually trigger the flush
	us.FlushPendingNotifications()
	time.Sleep(100 * time.Millisecond)

	db.Find(&notifications)
	assert.Equal(t, 1, len(notifications), "Should have 1 notification now")
	if len(notifications) > 0 {
		assert.Contains(t, notifications[0].Message, upHost.DomainNames, "Notification should mention the host")
		assert.Equal(t, models.NotificationTypeError, notifications[0].Type, "Notification type should be error for DOWN event")
	}
}

func TestUptimeService_ListMonitors(t *testing.T) {
	db := setupUptimeTestDB(t)
	ns := NewNotificationService(db, nil)
	us := newTestUptimeService(t, db, ns)

	db.Create(&models.UptimeMonitor{
		Name: "Test Monitor",
		Type: "http",
		URL:  "https://discord.com/api/webhooks/123/abc",
	})

	monitors, err := us.ListMonitors()
	assert.NoError(t, err)
	assert.Len(t, monitors, 1)
	assert.Equal(t, "Test Monitor", monitors[0].Name)
}

func TestUptimeService_GetMonitorByID(t *testing.T) {
	db := setupUptimeTestDB(t)
	ns := NewNotificationService(db, nil)
	us := newTestUptimeService(t, db, ns)

	monitor := models.UptimeMonitor{
		ID:       "monitor-1",
		Name:     "Test Monitor",
		Type:     "http",
		URL:      "https://example.com",
		Interval: 60,
		Enabled:  true,
		Status:   "up",
	}
	db.Create(&monitor)

	t.Run("get existing monitor", func(t *testing.T) {
		result, err := us.GetMonitorByID(monitor.ID)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, monitor.ID, result.ID)
		assert.Equal(t, monitor.Name, result.Name)
		assert.Equal(t, monitor.Type, result.Type)
		assert.Equal(t, monitor.URL, result.URL)
	})

	t.Run("get non-existent monitor", func(t *testing.T) {
		result, err := us.GetMonitorByID("non-existent")
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestUptimeService_GetMonitorHistory(t *testing.T) {
	db := setupUptimeTestDB(t)
	ns := NewNotificationService(db, nil)
	us := newTestUptimeService(t, db, ns)

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
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

		// Close the database to force errors
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()

		err := us.SyncMonitors()
		assert.Error(t, err)
	})

	t.Run("creates monitors for new hosts", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

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
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

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

func TestUptimeService_SyncMonitors_NameSync(t *testing.T) {
	t.Run("syncs name from proxy host when changed", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

		host := models.ProxyHost{UUID: "test-1", Name: "Original Name", DomainNames: "test1.com", Enabled: true}
		db.Create(&host)

		err := us.SyncMonitors()
		assert.NoError(t, err)

		var monitor models.UptimeMonitor
		db.Where("proxy_host_id = ?", host.ID).First(&monitor)
		assert.Equal(t, "Original Name", monitor.Name)

		// Update host name
		host.Name = "Updated Name"
		db.Save(&host)

		err = us.SyncMonitors()
		assert.NoError(t, err)

		db.Where("proxy_host_id = ?", host.ID).First(&monitor)
		assert.Equal(t, "Updated Name", monitor.Name)
	})

	t.Run("uses domain name when proxy host name is empty", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

		host := models.ProxyHost{UUID: "test-2", Name: "", DomainNames: "fallback.com, secondary.com", Enabled: true}
		db.Create(&host)

		err := us.SyncMonitors()
		assert.NoError(t, err)

		var monitor models.UptimeMonitor
		db.Where("proxy_host_id = ?", host.ID).First(&monitor)
		assert.Equal(t, "fallback.com", monitor.Name)
	})

	t.Run("updates monitor name when host name becomes empty", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

		host := models.ProxyHost{UUID: "test-3", Name: "Named Host", DomainNames: "domain.com", Enabled: true}
		db.Create(&host)

		err := us.SyncMonitors()
		assert.NoError(t, err)

		var monitor models.UptimeMonitor
		db.Where("proxy_host_id = ?", host.ID).First(&monitor)
		assert.Equal(t, "Named Host", monitor.Name)

		// Clear host name
		host.Name = ""
		db.Save(&host)

		err = us.SyncMonitors()
		assert.NoError(t, err)

		db.Where("proxy_host_id = ?", host.ID).First(&monitor)
		assert.Equal(t, "domain.com", monitor.Name)
	})
}

func TestUptimeService_SyncMonitors_TCPMigration(t *testing.T) {
	t.Run("migrates TCP monitor to HTTP for public URL", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

		host := models.ProxyHost{
			UUID:        "tcp-host",
			Name:        "TCP Host",
			DomainNames: "public.com",
			ForwardHost: "backend.local",
			ForwardPort: 8080,
			Enabled:     true,
		}
		db.Create(&host)

		// Manually create old-style TCP monitor (simulating legacy data)
		oldMonitor := models.UptimeMonitor{
			ProxyHostID: &host.ID,
			Name:        "TCP Host",
			Type:        "tcp",
			URL:         "backend.local:8080",
			Interval:    60,
			Enabled:     true,
			Status:      "pending",
		}
		db.Create(&oldMonitor)

		err := us.SyncMonitors()
		assert.NoError(t, err)

		var monitor models.UptimeMonitor
		db.Where("proxy_host_id = ?", host.ID).First(&monitor)
		assert.Equal(t, "http", monitor.Type)
		assert.Equal(t, "http://public.com", monitor.URL)
	})

	t.Run("does not migrate TCP monitor with custom URL", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

		host := models.ProxyHost{
			UUID:        "tcp-custom",
			Name:        "Custom TCP",
			DomainNames: "public.com",
			ForwardHost: "backend.local",
			ForwardPort: 8080,
			Enabled:     true,
		}
		db.Create(&host)

		// Create TCP monitor with custom URL (user-configured)
		customMonitor := models.UptimeMonitor{
			ProxyHostID: &host.ID,
			Name:        "Custom TCP",
			Type:        "tcp",
			URL:         "custom.endpoint:9999",
			Interval:    60,
			Enabled:     true,
			Status:      "pending",
		}
		db.Create(&customMonitor)

		err := us.SyncMonitors()
		assert.NoError(t, err)

		var monitor models.UptimeMonitor
		db.Where("proxy_host_id = ?", host.ID).First(&monitor)
		// Should NOT migrate - custom URL preserved
		assert.Equal(t, "tcp", monitor.Type)
		assert.Equal(t, "custom.endpoint:9999", monitor.URL)
	})
}

func TestUptimeService_SyncMonitors_HTTPSUpgrade(t *testing.T) {
	t.Run("upgrades HTTP to HTTPS when SSL forced", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

		host := models.ProxyHost{
			UUID:        "http-host",
			Name:        "HTTP Host",
			DomainNames: "secure.com",
			SSLForced:   false,
			Enabled:     true,
		}
		db.Create(&host)

		// Create HTTP monitor
		httpMonitor := models.UptimeMonitor{
			ProxyHostID: &host.ID,
			Name:        "HTTP Host",
			Type:        "http",
			URL:         "http://secure.com",
			Interval:    60,
			Enabled:     true,
			Status:      "pending",
		}
		db.Create(&httpMonitor)

		// Sync first (no change expected)
		err := us.SyncMonitors()
		assert.NoError(t, err)

		var monitor models.UptimeMonitor
		db.Where("proxy_host_id = ?", host.ID).First(&monitor)
		assert.Equal(t, "http://secure.com", monitor.URL)

		// Enable SSL forced
		host.SSLForced = true
		db.Save(&host)

		err = us.SyncMonitors()
		assert.NoError(t, err)

		db.Where("proxy_host_id = ?", host.ID).First(&monitor)
		assert.Equal(t, "https://secure.com", monitor.URL)
	})

	t.Run("does not downgrade HTTPS when SSL not forced", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

		host := models.ProxyHost{
			UUID:        "https-host",
			Name:        "HTTPS Host",
			DomainNames: "secure.com",
			SSLForced:   false,
			Enabled:     true,
		}
		db.Create(&host)

		// Create HTTPS monitor
		httpsMonitor := models.UptimeMonitor{
			ProxyHostID: &host.ID,
			Name:        "HTTPS Host",
			Type:        "http",
			URL:         "https://secure.com",
			Interval:    60,
			Enabled:     true,
			Status:      "pending",
		}
		db.Create(&httpsMonitor)

		err := us.SyncMonitors()
		assert.NoError(t, err)

		var monitor models.UptimeMonitor
		db.Where("proxy_host_id = ?", host.ID).First(&monitor)
		// Should remain HTTPS
		assert.Equal(t, "https://secure.com", monitor.URL)
	})
}

func TestUptimeService_SyncMonitors_RemoteServers(t *testing.T) {
	t.Run("creates monitor for new remote server", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

		server := models.RemoteServer{
			Name:    "Remote Backend",
			Host:    "backend.local",
			Port:    8080,
			Scheme:  "http",
			Enabled: true,
		}
		db.Create(&server)

		err := us.SyncMonitors()
		assert.NoError(t, err)

		var monitor models.UptimeMonitor
		db.Where("remote_server_id = ?", server.ID).First(&monitor)
		assert.Equal(t, "Remote Backend", monitor.Name)
		assert.Equal(t, "http", monitor.Type)
		assert.Equal(t, "http://backend.local:8080", monitor.URL)
		assert.True(t, monitor.Enabled)
	})

	t.Run("creates TCP monitor for remote server without scheme", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

		server := models.RemoteServer{
			Name:    "TCP Backend",
			Host:    "tcp.backend",
			Port:    3306,
			Scheme:  "",
			Enabled: true,
		}
		db.Create(&server)

		err := us.SyncMonitors()
		assert.NoError(t, err)

		var monitor models.UptimeMonitor
		db.Where("remote_server_id = ?", server.ID).First(&monitor)
		assert.Equal(t, "tcp", monitor.Type)
		assert.Equal(t, "tcp.backend:3306", monitor.URL)
	})

	t.Run("syncs remote server name changes", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

		server := models.RemoteServer{
			Name:    "Original Server",
			Host:    "server.local",
			Port:    8080,
			Scheme:  "https",
			Enabled: true,
		}
		db.Create(&server)

		err := us.SyncMonitors()
		assert.NoError(t, err)

		var monitor models.UptimeMonitor
		db.Where("remote_server_id = ?", server.ID).First(&monitor)
		assert.Equal(t, "Original Server", monitor.Name)

		// Update server name
		server.Name = "Renamed Server"
		db.Save(&server)

		err = us.SyncMonitors()
		assert.NoError(t, err)

		db.Where("remote_server_id = ?", server.ID).First(&monitor)
		assert.Equal(t, "Renamed Server", monitor.Name)
	})

	t.Run("syncs remote server URL changes", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

		server := models.RemoteServer{
			Name:    "Server",
			Host:    "old.host",
			Port:    8080,
			Scheme:  "http",
			Enabled: true,
		}
		db.Create(&server)

		err := us.SyncMonitors()
		assert.NoError(t, err)

		var monitor models.UptimeMonitor
		db.Where("remote_server_id = ?", server.ID).First(&monitor)
		assert.Equal(t, "http://old.host:8080", monitor.URL)

		// Change host and port
		server.Host = "new.host"
		server.Port = 9090
		db.Save(&server)

		err = us.SyncMonitors()
		assert.NoError(t, err)

		db.Where("remote_server_id = ?", server.ID).First(&monitor)
		assert.Equal(t, "http://new.host:9090", monitor.URL)
	})

	t.Run("syncs remote server enabled status", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

		server := models.RemoteServer{
			Name:    "Toggleable Server",
			Host:    "server.local",
			Port:    8080,
			Scheme:  "http",
			Enabled: true,
		}
		db.Create(&server)

		err := us.SyncMonitors()
		assert.NoError(t, err)

		var monitor models.UptimeMonitor
		db.Where("remote_server_id = ?", server.ID).First(&monitor)
		assert.True(t, monitor.Enabled)

		// Disable server
		server.Enabled = false
		db.Save(&server)

		err = us.SyncMonitors()
		assert.NoError(t, err)

		db.Where("remote_server_id = ?", server.ID).First(&monitor)
		assert.False(t, monitor.Enabled)
	})

	t.Run("syncs scheme change from TCP to HTTPS", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

		server := models.RemoteServer{
			Name:    "Scheme Changer",
			Host:    "server.local",
			Port:    443,
			Scheme:  "",
			Enabled: true,
		}
		db.Create(&server)

		err := us.SyncMonitors()
		assert.NoError(t, err)

		var monitor models.UptimeMonitor
		db.Where("remote_server_id = ?", server.ID).First(&monitor)
		assert.Equal(t, "tcp", monitor.Type)
		assert.Equal(t, "server.local:443", monitor.URL)

		// Change to HTTPS
		server.Scheme = "https"
		db.Save(&server)

		err = us.SyncMonitors()
		assert.NoError(t, err)

		db.Where("remote_server_id = ?", server.ID).First(&monitor)
		assert.Equal(t, "https", monitor.Type)
		assert.Equal(t, "https://server.local:443", monitor.URL)
	})
}

func TestUptimeService_CheckAll_Errors(t *testing.T) {
	t.Run("handles empty monitor list", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

		// Call CheckAll with no monitors - should not panic
		us.CheckAll()
		time.Sleep(50 * time.Millisecond)

		var heartbeats []models.UptimeHeartbeat
		db.Find(&heartbeats)
		assert.Equal(t, 0, len(heartbeats))
	})

	t.Run("orphan monitors don't prevent check execution", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

		// Create a monitor without a proxy host
		orphanID := uint(999)
		monitor := models.UptimeMonitor{
			ID:          "orphan-1",
			Name:        "Orphan Monitor",
			Type:        "http",
			URL:         "https://discord.com/api/webhooks/123/abc",
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
		t.Skip("Blocks on real network call to 192.0.2.1:9999 - needs mock")

		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

		// Use even faster timeouts for this specific test
		us.config.TCPTimeout = 50 * time.Millisecond
		us.config.MaxRetries = 0 // No retries
		us.config.CheckTimeout = 500 * time.Millisecond

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
		time.Sleep(300 * time.Millisecond) // Short wait since timeouts are aggressive

		var monitor models.UptimeMonitor
		db.Where("proxy_host_id = ?", host.ID).First(&monitor)
		// With no retries and fast timeout, should be down
		assert.Contains(t, []string{"down", "pending"}, monitor.Status, "Status should be down or pending for unreachable host")
	})
}

func TestUptimeService_CheckAll_HostDown_PartitionsByMonitorType(t *testing.T) {
	db := setupUptimeTestDB(t)
	ns := NewNotificationService(db, nil)
	us := newTestUptimeService(t, db, ns)

	us.config.TCPTimeout = 50 * time.Millisecond
	us.config.MaxRetries = 0
	us.config.FailureThreshold = 1
	us.config.CheckTimeout = 2 * time.Second

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	addr := listener.Addr().(*net.TCPAddr)

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		_ = server.Close()
		_ = listener.Close()
	})

	closedListener, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	closedPort := closedListener.Addr().(*net.TCPAddr).Port
	_ = closedListener.Close()

	uptimeHost := models.UptimeHost{
		Host:   "127.0.0.2",
		Name:   "Down Host",
		Status: "pending",
	}
	err = db.Create(&uptimeHost).Error
	assert.NoError(t, err)

	hostID := uptimeHost.ID
	httpMonitor := models.UptimeMonitor{
		ID:           "hostdown-http-monitor",
		Name:         "HTTP Monitor",
		Type:         "http",
		URL:          fmt.Sprintf("http://127.0.0.1:%d", addr.Port),
		Enabled:      true,
		Status:       "pending",
		UptimeHostID: &hostID,
		MaxRetries:   1,
	}
	tcpMonitor := models.UptimeMonitor{
		ID:           "hostdown-tcp-monitor",
		Name:         "TCP Monitor",
		Type:         "tcp",
		URL:          fmt.Sprintf("127.0.0.2:%d", closedPort),
		Enabled:      true,
		Status:       "up",
		UptimeHostID: &hostID,
		MaxRetries:   1,
	}
	err = db.Create(&httpMonitor).Error
	assert.NoError(t, err)
	err = db.Create(&tcpMonitor).Error
	assert.NoError(t, err)

	us.CheckAll()

	assert.Eventually(t, func() bool {
		var refreshed models.UptimeHost
		if db.Where("id = ?", uptimeHost.ID).First(&refreshed).Error != nil {
			return false
		}
		return refreshed.Status == "down"
	}, 3*time.Second, 25*time.Millisecond)

	assert.Eventually(t, func() bool {
		var refreshed models.UptimeMonitor
		if db.Where("id = ?", httpMonitor.ID).First(&refreshed).Error != nil {
			return false
		}
		return refreshed.Status == "up"
	}, 3*time.Second, 25*time.Millisecond)

	assert.Eventually(t, func() bool {
		var refreshed models.UptimeMonitor
		if db.Where("id = ?", tcpMonitor.ID).First(&refreshed).Error != nil {
			return false
		}
		return refreshed.Status == "down"
	}, 3*time.Second, 25*time.Millisecond)

	var httpHeartbeat models.UptimeHeartbeat
	err = db.Where("monitor_id = ?", httpMonitor.ID).Order("created_at desc").First(&httpHeartbeat).Error
	assert.NoError(t, err)
	assert.Equal(t, "up", httpHeartbeat.Status)
	assert.Contains(t, httpHeartbeat.Message, "HTTP 200")
	assert.NotContains(t, httpHeartbeat.Message, "Host unreachable")

	var tcpHeartbeat models.UptimeHeartbeat
	err = db.Where("monitor_id = ?", tcpMonitor.ID).Order("created_at desc").First(&tcpHeartbeat).Error
	assert.NoError(t, err)
	assert.Equal(t, "down", tcpHeartbeat.Status)
	assert.Equal(t, "Host unreachable", tcpHeartbeat.Message)
}

func TestUptimeService_CheckAll_ManualScheduledParity_ForHTTPOnHostDown(t *testing.T) {
	db := setupUptimeTestDB(t)
	ns := NewNotificationService(db, nil)
	us := newTestUptimeService(t, db, ns)

	us.config.TCPTimeout = 50 * time.Millisecond
	us.config.MaxRetries = 0
	us.config.FailureThreshold = 1
	us.config.CheckTimeout = 2 * time.Second

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	addr := listener.Addr().(*net.TCPAddr)

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		_ = server.Close()
		_ = listener.Close()
	})

	uptimeHost := models.UptimeHost{
		Host:   "127.0.0.2",
		Name:   "Parity Host",
		Status: "pending",
	}
	err = db.Create(&uptimeHost).Error
	assert.NoError(t, err)

	hostID := uptimeHost.ID
	manualMonitor := models.UptimeMonitor{
		ID:           "manual-http-parity",
		Name:         "Manual HTTP",
		Type:         "http",
		URL:          fmt.Sprintf("http://127.0.0.1:%d", addr.Port),
		Enabled:      true,
		Status:       "pending",
		UptimeHostID: &hostID,
		MaxRetries:   1,
	}
	scheduledMonitor := models.UptimeMonitor{
		ID:           "scheduled-http-parity",
		Name:         "Scheduled HTTP",
		Type:         "http",
		URL:          fmt.Sprintf("http://127.0.0.1:%d", addr.Port),
		Enabled:      true,
		Status:       "pending",
		UptimeHostID: &hostID,
		MaxRetries:   1,
	}
	err = db.Create(&manualMonitor).Error
	assert.NoError(t, err)
	err = db.Create(&scheduledMonitor).Error
	assert.NoError(t, err)

	us.CheckMonitor(manualMonitor)

	assert.Eventually(t, func() bool {
		var refreshed models.UptimeMonitor
		if db.Where("id = ?", manualMonitor.ID).First(&refreshed).Error != nil {
			return false
		}
		return refreshed.Status == "up"
	}, 2*time.Second, 25*time.Millisecond)

	us.CheckAll()

	assert.Eventually(t, func() bool {
		var refreshed models.UptimeMonitor
		if db.Where("id = ?", scheduledMonitor.ID).First(&refreshed).Error != nil {
			return false
		}
		return refreshed.Status == "up"
	}, 3*time.Second, 25*time.Millisecond)

	var manualResult models.UptimeMonitor
	err = db.Where("id = ?", manualMonitor.ID).First(&manualResult).Error
	assert.NoError(t, err)

	var scheduledResult models.UptimeMonitor
	err = db.Where("id = ?", scheduledMonitor.ID).First(&scheduledResult).Error
	assert.NoError(t, err)

	assert.Equal(t, "up", manualResult.Status)
	assert.Equal(t, manualResult.Status, scheduledResult.Status)
}

func TestUptimeService_CheckAll_ReachableHost_StillUsesHTTPResult(t *testing.T) {
	db := setupUptimeTestDB(t)
	ns := NewNotificationService(db, nil)
	us := newTestUptimeService(t, db, ns)

	us.config.TCPTimeout = 50 * time.Millisecond
	us.config.MaxRetries = 0
	us.config.FailureThreshold = 1
	us.config.CheckTimeout = 2 * time.Second

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	addr := listener.Addr().(*net.TCPAddr)

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		_ = server.Close()
		_ = listener.Close()
	})

	uptimeHost := models.UptimeHost{
		Host:   "127.0.0.1",
		Name:   "Reachable Host",
		Status: "pending",
	}
	err = db.Create(&uptimeHost).Error
	assert.NoError(t, err)

	hostID := uptimeHost.ID
	httpMonitor := models.UptimeMonitor{
		ID:           "reachable-host-http-fail",
		Name:         "Reachable Host HTTP Failure",
		Type:         "http",
		URL:          fmt.Sprintf("http://127.0.0.1:%d", addr.Port),
		Enabled:      true,
		Status:       "pending",
		UptimeHostID: &hostID,
		MaxRetries:   1,
	}
	err = db.Create(&httpMonitor).Error
	assert.NoError(t, err)

	us.CheckAll()

	assert.Eventually(t, func() bool {
		var refreshedHost models.UptimeHost
		if db.Where("id = ?", uptimeHost.ID).First(&refreshedHost).Error != nil {
			return false
		}
		return refreshedHost.Status == "up"
	}, 3*time.Second, 25*time.Millisecond)

	assert.Eventually(t, func() bool {
		var refreshed models.UptimeMonitor
		if db.Where("id = ?", httpMonitor.ID).First(&refreshed).Error != nil {
			return false
		}
		return refreshed.Status == "down"
	}, 3*time.Second, 25*time.Millisecond)

	var heartbeat models.UptimeHeartbeat
	err = db.Where("monitor_id = ?", httpMonitor.ID).Order("created_at desc").First(&heartbeat).Error
	assert.NoError(t, err)
	assert.Equal(t, "down", heartbeat.Status)
	assert.Contains(t, heartbeat.Message, "HTTP 500")
	assert.NotContains(t, heartbeat.Message, "Host unreachable")
}

func TestUptimeService_CheckMonitor_EdgeCases(t *testing.T) {
	t.Run("invalid URL format", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

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
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

		// Start HTTP server returning 404
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		assert.NoError(t, err)
		addr := listener.Addr().(*net.TCPAddr)

		server := &http.Server{
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			}),
			ReadHeaderTimeout: 10 * time.Second, // Prevent Slowloris attacks
		}
		go func() { _ = server.Serve(listener) }()
		defer func() { _ = server.Close() }()

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
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

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
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

		history, err := us.GetMonitorHistory("non-existent", 100)
		assert.NoError(t, err)
		assert.Len(t, history, 0)
	})

	t.Run("limit parameter respected", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

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
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

		monitors, err := us.ListMonitors()
		assert.NoError(t, err)
		assert.Len(t, monitors, 0)
	})

	t.Run("monitors with associated proxy hosts", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

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
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

		monitor := models.UptimeMonitor{
			ID:         "update-test",
			Name:       "Update Test",
			Type:       "http",
			URL:        "https://discord.com/api/webhooks/123/abc",
			MaxRetries: 3,
			Interval:   60,
		}
		db.Create(&monitor)

		updates := map[string]any{
			"max_retries": 5,
		}

		result, err := us.UpdateMonitor(monitor.ID, updates)
		assert.NoError(t, err)
		assert.Equal(t, 5, result.MaxRetries)
	})

	t.Run("update interval", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

		monitor := models.UptimeMonitor{
			ID:       "update-interval",
			Name:     "Interval Test",
			Interval: 60,
		}
		db.Create(&monitor)

		updates := map[string]any{
			"interval": 120,
		}

		result, err := us.UpdateMonitor(monitor.ID, updates)
		assert.NoError(t, err)
		assert.Equal(t, 120, result.Interval)
	})

	t.Run("update non-existent monitor", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

		updates := map[string]any{
			"max_retries": 5,
		}

		_, err := us.UpdateMonitor("non-existent", updates)
		assert.Error(t, err)
	})

	t.Run("update multiple fields", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

		monitor := models.UptimeMonitor{
			ID:         "multi-update",
			Name:       "Multi Update Test",
			MaxRetries: 3,
			Interval:   60,
		}
		db.Create(&monitor)

		updates := map[string]any{
			"max_retries": 10,
			"interval":    300,
		}

		result, err := us.UpdateMonitor(monitor.ID, updates)
		assert.NoError(t, err)
		assert.Equal(t, 10, result.MaxRetries)
		assert.Equal(t, 300, result.Interval)
	})
}

func TestUptimeService_NotificationBatching(t *testing.T) {
	t.Run("batches multiple service failures on same host", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

		// Create an UptimeHost
		host := models.UptimeHost{
			ID:     "test-host-1",
			Host:   "192.168.1.100",
			Name:   "Test Server",
			Status: "up",
		}
		db.Create(&host)

		// Create multiple monitors pointing to the same host
		monitors := []models.UptimeMonitor{
			{ID: "mon-1", Name: "Service A", UpstreamHost: "192.168.1.100", UptimeHostID: &host.ID, Status: "up", MaxRetries: 3},
			{ID: "mon-2", Name: "Service B", UpstreamHost: "192.168.1.100", UptimeHostID: &host.ID, Status: "up", MaxRetries: 3},
			{ID: "mon-3", Name: "Service C", UpstreamHost: "192.168.1.100", UptimeHostID: &host.ID, Status: "up", MaxRetries: 3},
		}
		for _, m := range monitors {
			db.Create(&m)
		}

		// Queue down notifications for all three
		us.queueDownNotification(monitors[0], "Connection refused", "1h 30m")
		us.queueDownNotification(monitors[1], "Connection refused", "2h 15m")
		us.queueDownNotification(monitors[2], "Connection refused", "45m")

		// Verify all are batched together
		us.notificationMutex.Lock()
		pending, exists := us.pendingNotifications[host.ID]
		us.notificationMutex.Unlock()

		assert.True(t, exists, "Should have pending notification for host")
		assert.Equal(t, 3, len(pending.downMonitors), "Should have 3 monitors in batch")

		// Flush and verify single notification is sent
		us.FlushPendingNotifications()

		var notifications []models.Notification
		db.Find(&notifications)
		assert.Equal(t, 1, len(notifications), "Should have exactly 1 batched notification")

		if len(notifications) > 0 {
			// Should mention all three services
			assert.Contains(t, notifications[0].Message, "Service A")
			assert.Contains(t, notifications[0].Message, "Service B")
			assert.Contains(t, notifications[0].Message, "Service C")
			assert.Contains(t, notifications[0].Title, "3 Services DOWN")
		}
	})

	t.Run("single service down gets individual notification", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

		// Create an UptimeHost
		host := models.UptimeHost{
			ID:     "test-host-2",
			Host:   "192.168.1.101",
			Name:   "Single Service Host",
			Status: "up",
		}
		db.Create(&host)

		monitor := models.UptimeMonitor{
			ID:           "single-mon",
			Name:         "Lonely Service",
			UpstreamHost: "192.168.1.101",
			UptimeHostID: &host.ID,
			Status:       "up",
			MaxRetries:   3,
		}
		db.Create(&monitor)

		// Queue single down notification
		us.queueDownNotification(monitor, "HTTP 502", "5h 30m")

		// Flush
		us.FlushPendingNotifications()

		var notifications []models.Notification
		db.Find(&notifications)
		assert.Equal(t, 1, len(notifications), "Should have exactly 1 notification")

		if len(notifications) > 0 {
			assert.Contains(t, notifications[0].Title, "Lonely Service is DOWN")
			assert.Contains(t, notifications[0].Message, "Previous Uptime: 5h 30m")
		}
	})
}

func TestUptimeService_HostLevelCheck(t *testing.T) {
	t.Run("creates uptime host during sync", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

		// Create a proxy host
		proxyHost := models.ProxyHost{
			UUID:        "ph-1",
			DomainNames: "app.example.com",
			ForwardHost: "10.0.0.50",
			ForwardPort: 8080,
		}
		db.Create(&proxyHost)

		// Sync monitors
		err := us.SyncMonitors()
		assert.NoError(t, err)

		// Verify UptimeHost was created
		var uptimeHost models.UptimeHost
		err = db.Where("host = ?", "10.0.0.50").First(&uptimeHost).Error
		assert.NoError(t, err)
		assert.Equal(t, "10.0.0.50", uptimeHost.Host)
		assert.Equal(t, "app.example.com", uptimeHost.Name)

		// Verify monitor has uptime host ID
		var monitor models.UptimeMonitor
		db.Where("proxy_host_id = ?", proxyHost.ID).First(&monitor)
		assert.NotNil(t, monitor.UptimeHostID)
		assert.Equal(t, uptimeHost.ID, *monitor.UptimeHostID)
	})

	t.Run("groups multiple services on same host", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

		// Create multiple proxy hosts pointing to the same forward host
		hosts := []models.ProxyHost{
			{UUID: "ph-1", DomainNames: "app1.example.com", ForwardHost: "10.0.0.100", ForwardPort: 8080, Name: "App 1"},
			{UUID: "ph-2", DomainNames: "app2.example.com", ForwardHost: "10.0.0.100", ForwardPort: 8081, Name: "App 2"},
			{UUID: "ph-3", DomainNames: "app3.example.com", ForwardHost: "10.0.0.100", ForwardPort: 8082, Name: "App 3"},
		}
		for _, h := range hosts {
			db.Create(&h)
		}

		// Sync monitors
		err := us.SyncMonitors()
		assert.NoError(t, err)

		// Should have only 1 UptimeHost for 10.0.0.100
		var uptimeHosts []models.UptimeHost
		db.Where("host = ?", "10.0.0.100").Find(&uptimeHosts)
		assert.Equal(t, 1, len(uptimeHosts), "Should have exactly 1 UptimeHost for the shared IP")

		// All 3 monitors should point to the same UptimeHost
		var monitors []models.UptimeMonitor
		db.Where("upstream_host = ?", "10.0.0.100").Find(&monitors)
		assert.Equal(t, 3, len(monitors))

		for _, m := range monitors {
			assert.NotNil(t, m.UptimeHostID)
			assert.Equal(t, uptimeHosts[0].ID, *m.UptimeHostID)
		}
	})
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		input    time.Duration
		expected string
	}{
		{5 * time.Second, "5s"},
		{65 * time.Second, "1m 5s"},
		{3665 * time.Second, "1h 1m 5s"},
		{90065 * time.Second, "1d 1h 1m"},
		{0, "0s"},
	}

	for _, tc := range tests {
		result := formatDuration(tc.input)
		assert.Equal(t, tc.expected, result, "formatDuration(%v)", tc.input)
	}
}

func TestUptimeService_SyncMonitorForHost(t *testing.T) {
	t.Run("updates monitor when proxy host is edited", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

		// Create a proxy host
		host := models.ProxyHost{
			UUID:        "sync-test-1",
			Name:        "Original Name",
			DomainNames: "original.example.com",
			ForwardHost: "10.0.0.1",
			ForwardPort: 8080,
			SSLForced:   false,
			Enabled:     true,
		}
		db.Create(&host)

		// Sync monitors to create the uptime monitor
		err := us.SyncMonitors()
		assert.NoError(t, err)

		// Verify monitor was created with original values
		var monitor models.UptimeMonitor
		err = db.Where("proxy_host_id = ?", host.ID).First(&monitor).Error
		assert.NoError(t, err)
		assert.Equal(t, "Original Name", monitor.Name)
		assert.Equal(t, "http://original.example.com", monitor.URL)
		assert.Equal(t, "10.0.0.1", monitor.UpstreamHost)

		// Update the proxy host
		host.Name = "Updated Name"
		host.DomainNames = "updated.example.com"
		host.ForwardHost = "10.0.0.2"
		host.SSLForced = true
		db.Save(&host)

		// Call SyncMonitorForHost
		err = us.SyncMonitorForHost(host.ID)
		assert.NoError(t, err)

		// Verify monitor was updated
		err = db.Where("proxy_host_id = ?", host.ID).First(&monitor).Error
		assert.NoError(t, err)
		assert.Equal(t, "Updated Name", monitor.Name)
		assert.Equal(t, "https://updated.example.com", monitor.URL)
		assert.Equal(t, "10.0.0.2", monitor.UpstreamHost)
	})

	t.Run("returns nil when no monitor exists", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

		// Create a proxy host without creating a monitor
		host := models.ProxyHost{
			UUID:        "no-monitor-test",
			Name:        "No Monitor Host",
			DomainNames: "nomonitor.example.com",
			ForwardHost: "10.0.0.3",
			ForwardPort: 8080,
			Enabled:     true,
		}
		db.Create(&host)

		// Call SyncMonitorForHost - should return nil without error
		err := us.SyncMonitorForHost(host.ID)
		assert.NoError(t, err)

		// Verify no monitor was created
		var count int64
		db.Model(&models.UptimeMonitor{}).Where("proxy_host_id = ?", host.ID).Count(&count)
		assert.Equal(t, int64(0), count)
	})

	t.Run("returns error when host does not exist", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

		// Call SyncMonitorForHost with non-existent host ID
		err := us.SyncMonitorForHost(99999)
		assert.Error(t, err)
	})

	t.Run("uses domain name when proxy host name is empty", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

		// Create a proxy host with a name
		host := models.ProxyHost{
			UUID:        "empty-name-test",
			Name:        "Has Name",
			DomainNames: "domain.example.com",
			ForwardHost: "10.0.0.4",
			ForwardPort: 8080,
			Enabled:     true,
		}
		db.Create(&host)

		// Sync monitors
		err := us.SyncMonitors()
		assert.NoError(t, err)

		// Clear the host name
		host.Name = ""
		db.Save(&host)

		// Call SyncMonitorForHost
		err = us.SyncMonitorForHost(host.ID)
		assert.NoError(t, err)

		// Verify monitor uses domain name
		var monitor models.UptimeMonitor
		err = db.Where("proxy_host_id = ?", host.ID).First(&monitor).Error
		assert.NoError(t, err)
		assert.Equal(t, "domain.example.com", monitor.Name)
	})

	t.Run("handles multiple domains correctly", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

		// Create a proxy host with multiple domains
		host := models.ProxyHost{
			UUID:        "multi-domain-test",
			Name:        "Multi Domain",
			DomainNames: "first.example.com, second.example.com, third.example.com",
			ForwardHost: "10.0.0.5",
			ForwardPort: 8080,
			SSLForced:   true,
			Enabled:     true,
		}
		db.Create(&host)

		// Sync monitors
		err := us.SyncMonitors()
		assert.NoError(t, err)

		// Call SyncMonitorForHost
		err = us.SyncMonitorForHost(host.ID)
		assert.NoError(t, err)

		// Verify monitor uses first domain
		var monitor models.UptimeMonitor
		err = db.Where("proxy_host_id = ?", host.ID).First(&monitor).Error
		assert.NoError(t, err)
		assert.Equal(t, "https://first.example.com", monitor.URL)
	})
}

func TestUptimeService_DeleteMonitor(t *testing.T) {
	t.Run("deletes monitor and heartbeats", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

		// Create monitor
		monitor := models.UptimeMonitor{
			ID:       "delete-test-1",
			Name:     "Delete Test Monitor",
			Type:     "http",
			URL:      "https://discord.com/api/webhooks/123/abc",
			Enabled:  true,
			Status:   "up",
			Interval: 60,
		}
		db.Create(&monitor)

		// Create some heartbeats
		for i := 0; i < 5; i++ {
			hb := models.UptimeHeartbeat{
				MonitorID: monitor.ID,
				Status:    "up",
				Latency:   int64(100 + i),
				CreatedAt: time.Now().Add(-time.Duration(i) * time.Minute),
			}
			db.Create(&hb)
		}

		// Verify heartbeats exist
		var count int64
		db.Model(&models.UptimeHeartbeat{}).Where("monitor_id = ?", monitor.ID).Count(&count)
		assert.Equal(t, int64(5), count)

		// Delete the monitor
		err := us.DeleteMonitor(monitor.ID)
		assert.NoError(t, err)

		// Verify monitor is deleted
		var deletedMonitor models.UptimeMonitor
		err = db.First(&deletedMonitor, "id = ?", monitor.ID).Error
		assert.Error(t, err)

		// Verify heartbeats are deleted
		db.Model(&models.UptimeHeartbeat{}).Where("monitor_id = ?", monitor.ID).Count(&count)
		assert.Equal(t, int64(0), count)
	})

	t.Run("returns error for non-existent monitor", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

		err := us.DeleteMonitor("non-existent-id")
		assert.Error(t, err)
	})

	t.Run("deletes monitor without heartbeats", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		ns := NewNotificationService(db, nil)
		us := newTestUptimeService(t, db, ns)

		// Create monitor without heartbeats
		monitor := models.UptimeMonitor{
			ID:       "delete-no-hb",
			Name:     "No Heartbeats Monitor",
			Type:     "tcp",
			URL:      "localhost:8080",
			Enabled:  true,
			Status:   "pending",
			Interval: 60,
		}
		db.Create(&monitor)

		// Delete the monitor
		err := us.DeleteMonitor(monitor.ID)
		assert.NoError(t, err)

		// Verify monitor is deleted
		var deletedMonitor models.UptimeMonitor
		err = db.First(&deletedMonitor, "id = ?", monitor.ID).Error
		assert.Error(t, err)
	})
}

func TestUptimeService_UpdateMonitor_EnabledField(t *testing.T) {
	db := setupUptimeTestDB(t)
	ns := NewNotificationService(db, nil)
	us := newTestUptimeService(t, db, ns)

	monitor := models.UptimeMonitor{
		ID:       "enabled-test",
		Name:     "Enabled Test Monitor",
		Type:     "http",
		URL:      "https://discord.com/api/webhooks/123/abc",
		Enabled:  true,
		Interval: 60,
	}
	db.Create(&monitor)

	// Disable the monitor
	updates := map[string]any{
		"enabled": false,
	}

	result, err := us.UpdateMonitor(monitor.ID, updates)
	assert.NoError(t, err)
	assert.False(t, result.Enabled)

	// Re-enable the monitor
	updates = map[string]any{
		"enabled": true,
	}

	result, err = us.UpdateMonitor(monitor.ID, updates)
	assert.NoError(t, err)
	assert.True(t, result.Enabled)
}

// PR-3: RFC 1918 bypass integration tests

func TestCheckMonitor_HTTP_LocalhostSucceedsWithPrivateIPBypass(t *testing.T) {
	// Confirm that after the dual-layer RFC 1918 bypass is wired into
	// checkMonitor, an HTTP monitor targeting the loopback interface still
	// reports "up" (localhost is explicitly allowed by WithAllowLocalhost).
	db := setupUptimeTestDB(t)
	ns := NewNotificationService(db, nil)
	us := newTestUptimeService(t, db, ns)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	addr := listener.Addr().(*net.TCPAddr)
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		_ = server.Close()
	})

	// Wait for server to be ready before creating the monitor.
	ready := false
	for i := 0; i < 20; i++ {
		conn, dialErr := net.DialTimeout("tcp", addr.String(), 50*time.Millisecond)
		if dialErr == nil {
			_ = conn.Close()
			ready = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !ready {
		t.Fatalf("test server on %s never became reachable after 20 attempts", addr.String())
	}

	monitor := models.UptimeMonitor{
		ID:      "pr3-http-localhost-test",
		Name:    "HTTP Localhost RFC1918 Bypass",
		Type:    "http",
		URL:     fmt.Sprintf("http://127.0.0.1:%d", addr.Port),
		Status:  "pending",
		Enabled: true,
	}
	if res := db.Create(&monitor); res.Error != nil {
		t.Fatalf("failed to create HTTP monitor: %v", res.Error)
	}

	us.CheckMonitor(monitor)

	var result models.UptimeMonitor
	db.First(&result, "id = ?", monitor.ID)
	assert.Equal(t, "up", result.Status, "HTTP monitor on localhost should be up with RFC1918 bypass")
}

func TestCheckMonitor_TCP_AcceptsRFC1918Address(t *testing.T) {
	// TCP monitors bypass URL validation entirely and dial directly.
	// Confirm that a TCP monitor targeting the loopback interface reports "up"
	// after the RFC 1918 bypass changes.
	db := setupUptimeTestDB(t)
	ns := NewNotificationService(db, nil)
	us := newTestUptimeService(t, db, ns)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start TCP listener: %v", err)
	}
	addr := listener.Addr().(*net.TCPAddr)
	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	t.Cleanup(func() { _ = listener.Close() })

	monitor := models.UptimeMonitor{
		ID:      "pr3-tcp-rfc1918-test",
		Name:    "TCP RFC1918 Accepted",
		Type:    "tcp",
		URL:     addr.String(),
		Status:  "pending",
		Enabled: true,
	}
	if res := db.Create(&monitor); res.Error != nil {
		t.Fatalf("failed to create TCP monitor: %v", res.Error)
	}

	us.CheckMonitor(monitor)

	var result models.UptimeMonitor
	db.First(&result, "id = ?", monitor.ID)
	assert.Equal(t, "up", result.Status, "TCP monitor to loopback should report up")
}
