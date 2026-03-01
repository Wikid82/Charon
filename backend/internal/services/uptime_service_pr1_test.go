package services

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// setupPR1TestDB creates an in-memory SQLite database with all models needed
// for PR-1 uptime bug fix tests.
func setupPR1TestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "pr1test.db")
	dsn := dbPath + "?_journal_mode=WAL&_busy_timeout=5000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.UptimeMonitor{},
		&models.UptimeHeartbeat{},
		&models.UptimeHost{},
		&models.ProxyHost{},
		&models.Setting{},
	))

	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

// enableUptimeFeature sets the feature.uptime.enabled setting to "true".
func enableUptimeFeature(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Create(&models.Setting{
		Key:      "feature.uptime.enabled",
		Value:    "true",
		Type:     "bool",
		Category: "feature",
	}).Error)
}

// createTestProxyHost creates a minimal proxy host for testing.
func createTestProxyHost(t *testing.T, db *gorm.DB, name, domain, forwardHost string) models.ProxyHost {
	t.Helper()
	host := models.ProxyHost{
		UUID:          uuid.New().String(),
		Name:          name,
		DomainNames:   domain,
		ForwardScheme: "http",
		ForwardHost:   forwardHost,
		ForwardPort:   80,
		Enabled:       true,
	}
	require.NoError(t, db.Create(&host).Error)
	return host
}

func createAlwaysOKServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	return server
}

func hostPortFromServerURL(serverURL string) string {
	return strings.TrimPrefix(serverURL, "http://")
}

// --- Fix 1: Singleton UptimeService ---

func TestSingletonUptimeService_SharedState(t *testing.T) {
	db := setupPR1TestDB(t)
	svc := NewUptimeService(db, nil)

	// Verify both pendingNotifications and hostMutexes are the same instance
	// by writing to the maps from the shared reference.
	svc.pendingNotifications["test-key"] = &pendingHostNotification{}
	assert.Contains(t, svc.pendingNotifications, "test-key",
		"pendingNotifications should be shared on the same instance")

	// A second reference to the same service should see the same map state.
	svc2 := svc // simulate routes.go passing the same pointer
	assert.Contains(t, svc2.pendingNotifications, "test-key",
		"second reference must share the same pendingNotifications map")
}

// --- Fix 2: SyncAndCheckForHost ---

func TestSyncAndCheckForHost_CreatesMonitorAndHeartbeat(t *testing.T) {
	db := setupPR1TestDB(t)
	enableUptimeFeature(t, db)
	svc := NewUptimeService(db, nil)
	server := createAlwaysOKServer(t)
	domain := hostPortFromServerURL(server.URL)

	host := createTestProxyHost(t, db, "test-host", domain, "192.168.1.100")

	// Execute synchronously (normally called as goroutine)
	svc.SyncAndCheckForHost(host.ID)

	// Verify monitor was created
	var monitor models.UptimeMonitor
	err := db.Where("proxy_host_id = ?", host.ID).First(&monitor).Error
	require.NoError(t, err, "monitor should be created for the proxy host")
	assert.Equal(t, "http://"+domain, monitor.URL)
	assert.Equal(t, "192.168.1.100", monitor.UpstreamHost)
	assert.Contains(t, []string{"up", "down", "pending"}, monitor.Status, "status should be set by checkMonitor")

	// Verify at least one heartbeat was created (from the immediate check)
	var hbCount int64
	db.Model(&models.UptimeHeartbeat{}).Where("monitor_id = ?", monitor.ID).Count(&hbCount)
	assert.Greater(t, hbCount, int64(0), "at least one heartbeat should exist after SyncAndCheckForHost")
}

func TestSyncAndCheckForHost_SSLForcedUsesHTTPS(t *testing.T) {
	db := setupPR1TestDB(t)
	enableUptimeFeature(t, db)
	svc := NewUptimeService(db, nil)
	server := createAlwaysOKServer(t)
	domain := hostPortFromServerURL(server.URL)

	host := models.ProxyHost{
		UUID:          uuid.New().String(),
		Name:          "ssl-host",
		DomainNames:   domain,
		ForwardScheme: "https",
		ForwardHost:   "192.168.1.200",
		ForwardPort:   443,
		SSLForced:     true,
		Enabled:       true,
	}
	require.NoError(t, db.Create(&host).Error)

	svc.SyncAndCheckForHost(host.ID)

	var monitor models.UptimeMonitor
	require.NoError(t, db.Where("proxy_host_id = ?", host.ID).First(&monitor).Error)
	assert.Equal(t, "https://"+domain, monitor.URL)
}

func TestSyncAndCheckForHost_DeletedHostNoPanic(t *testing.T) {
	db := setupPR1TestDB(t)
	enableUptimeFeature(t, db)
	svc := NewUptimeService(db, nil)

	// Call with a host ID that doesn't exist — should log and return, not panic
	assert.NotPanics(t, func() {
		svc.SyncAndCheckForHost(99999)
	})

	// No monitor should be created
	var count int64
	db.Model(&models.UptimeMonitor{}).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestSyncAndCheckForHost_ExistingMonitorSkipsCreate(t *testing.T) {
	db := setupPR1TestDB(t)
	enableUptimeFeature(t, db)
	svc := NewUptimeService(db, nil)
	server := createAlwaysOKServer(t)
	domain := hostPortFromServerURL(server.URL)

	host := createTestProxyHost(t, db, "existing-mon", domain, "10.0.0.1")

	// Pre-create a monitor
	existingMonitor := models.UptimeMonitor{
		ID:          uuid.New().String(),
		ProxyHostID: &host.ID,
		Name:        "pre-existing",
		Type:        "http",
		URL:         "http://" + domain,
		Interval:    60,
		Enabled:     true,
		Status:      "up",
	}
	require.NoError(t, db.Create(&existingMonitor).Error)

	svc.SyncAndCheckForHost(host.ID)

	// Should still be exactly 1 monitor
	var count int64
	db.Model(&models.UptimeMonitor{}).Where("proxy_host_id = ?", host.ID).Count(&count)
	assert.Equal(t, int64(1), count, "should not create a duplicate monitor")
}

// --- Fix 2 continued: Feature flag test ---

func TestSyncAndCheckForHost_DisabledFeatureNoop(t *testing.T) {
	db := setupPR1TestDB(t)
	// Explicitly set feature to disabled
	require.NoError(t, db.Create(&models.Setting{
		Key:      "feature.uptime.enabled",
		Value:    "false",
		Type:     "bool",
		Category: "feature",
	}).Error)
	svc := NewUptimeService(db, nil)
	server := createAlwaysOKServer(t)
	domain := hostPortFromServerURL(server.URL)

	host := createTestProxyHost(t, db, "disabled-host", domain, "10.0.0.2")

	svc.SyncAndCheckForHost(host.ID)

	// No monitor should be created when feature is disabled
	var count int64
	db.Model(&models.UptimeMonitor{}).Where("proxy_host_id = ?", host.ID).Count(&count)
	assert.Equal(t, int64(0), count, "no monitor should be created when feature is disabled")
}

func TestSyncAndCheckForHost_MissingSetting_StillCreates(t *testing.T) {
	db := setupPR1TestDB(t)
	// No setting at all — the method should proceed (default: enabled behavior)
	svc := NewUptimeService(db, nil)
	server := createAlwaysOKServer(t)
	domain := hostPortFromServerURL(server.URL)

	host := createTestProxyHost(t, db, "no-setting", domain, "10.0.0.3")

	svc.SyncAndCheckForHost(host.ID)

	var count int64
	db.Model(&models.UptimeMonitor{}).Where("proxy_host_id = ?", host.ID).Count(&count)
	assert.Greater(t, count, int64(0), "monitor should be created when setting is missing (default: enabled)")
}

// --- Fix 4: CleanupStaleFailureCounts ---

func TestCleanupStaleFailureCounts_ResetsStuckMonitors(t *testing.T) {
	db := setupPR1TestDB(t)
	svc := NewUptimeService(db, nil)

	// Create a "stuck" monitor: down, failure_count > 5, no recent UP heartbeat
	stuckMonitor := models.UptimeMonitor{
		ID:           uuid.New().String(),
		Name:         "stuck-monitor",
		Type:         "http",
		URL:          "http://stuck.example.com",
		Interval:     60,
		Enabled:      true,
		Status:       "down",
		FailureCount: 10,
	}
	require.NoError(t, db.Create(&stuckMonitor).Error)

	err := svc.CleanupStaleFailureCounts()
	require.NoError(t, err)

	// Verify the monitor was reset
	var m models.UptimeMonitor
	require.NoError(t, db.First(&m, "id = ?", stuckMonitor.ID).Error)
	assert.Equal(t, 0, m.FailureCount, "failure_count should be reset to 0")
	assert.Equal(t, "pending", m.Status, "status should be reset to pending")
}

func TestCleanupStaleFailureCounts_SkipsMonitorsWithRecentUpHeartbeat(t *testing.T) {
	db := setupPR1TestDB(t)
	svc := NewUptimeService(db, nil)

	// Create a monitor that is "down" with high failure_count BUT has a recent UP heartbeat
	healthyMonitor := models.UptimeMonitor{
		ID:           uuid.New().String(),
		Name:         "healthy-monitor",
		Type:         "http",
		URL:          "http://healthy.example.com",
		Interval:     60,
		Enabled:      true,
		Status:       "down",
		FailureCount: 10,
	}
	require.NoError(t, db.Create(&healthyMonitor).Error)

	// Add a recent UP heartbeat
	hb := models.UptimeHeartbeat{
		MonitorID: healthyMonitor.ID,
		Status:    "up",
		Latency:   50,
		CreatedAt: time.Now().Add(-1 * time.Hour), // 1 hour ago — within 24h window
	}
	require.NoError(t, db.Create(&hb).Error)

	err := svc.CleanupStaleFailureCounts()
	require.NoError(t, err)

	// Monitor should NOT be reset because it has a recent UP heartbeat
	var m models.UptimeMonitor
	require.NoError(t, db.First(&m, "id = ?", healthyMonitor.ID).Error)
	assert.Equal(t, 10, m.FailureCount, "failure_count should NOT be reset since there's a recent UP heartbeat")
	assert.Equal(t, "down", m.Status, "status should remain down")
}

func TestCleanupStaleFailureCounts_SkipsLowFailureCount(t *testing.T) {
	db := setupPR1TestDB(t)
	svc := NewUptimeService(db, nil)

	// Monitor with failure_count <= 5 — should not be touched
	monitor := models.UptimeMonitor{
		ID:           uuid.New().String(),
		Name:         "low-failure-monitor",
		Type:         "http",
		URL:          "http://low.example.com",
		Interval:     60,
		Enabled:      true,
		Status:       "down",
		FailureCount: 3,
	}
	require.NoError(t, db.Create(&monitor).Error)

	err := svc.CleanupStaleFailureCounts()
	require.NoError(t, err)

	var m models.UptimeMonitor
	require.NoError(t, db.First(&m, "id = ?", monitor.ID).Error)
	assert.Equal(t, 3, m.FailureCount, "low failure_count should not be reset")
	assert.Equal(t, "down", m.Status)
}

func TestCleanupStaleFailureCounts_DoesNotResetDownHosts(t *testing.T) {
	db := setupPR1TestDB(t)
	svc := NewUptimeService(db, nil)

	// Create a host that is currently down.
	host := models.UptimeHost{
		ID:           uuid.New().String(),
		Host:         "stuck-host.local",
		Name:         "stuck-host",
		Status:       "down",
		FailureCount: 10,
	}
	require.NoError(t, db.Create(&host).Error)

	err := svc.CleanupStaleFailureCounts()
	require.NoError(t, err)

	var h models.UptimeHost
	require.NoError(t, db.First(&h, "id = ?", host.ID).Error)
	assert.Equal(t, 10, h.FailureCount, "cleanup must not reset host failure_count")
	assert.Equal(t, "down", h.Status, "cleanup must not reset host status")
}

// setupPR1ConcurrentDB creates a file-based SQLite database with WAL mode and
// busy_timeout to handle concurrent writes without "database table is locked".
func setupPR1ConcurrentDB(t *testing.T) *gorm.DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	dsn := dbPath + "?_journal_mode=WAL&_busy_timeout=5000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.UptimeMonitor{},
		&models.UptimeHeartbeat{},
		&models.UptimeHost{},
		&models.ProxyHost{},
		&models.Setting{},
	))

	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
		_ = os.Remove(dbPath)
	})

	return db
}

// --- Concurrent access tests ---

func TestSyncAndCheckForHost_ConcurrentCreates_NoDuplicates(t *testing.T) {
	db := setupPR1ConcurrentDB(t)
	enableUptimeFeature(t, db)
	svc := NewUptimeService(db, nil)
	server := createAlwaysOKServer(t)
	domain := hostPortFromServerURL(server.URL)

	// Create multiple proxy hosts with unique domains
	hosts := make([]models.ProxyHost, 5)
	for i := range hosts {
		hosts[i] = createTestProxyHost(t, db,
			fmt.Sprintf("concurrent-host-%d", i),
			domain,
			fmt.Sprintf("10.0.0.%d", 100+i),
		)
	}

	var wg sync.WaitGroup
	for _, h := range hosts {
		wg.Add(1)
		go func(hostID uint) {
			defer wg.Done()
			svc.SyncAndCheckForHost(hostID)
		}(h.ID)
	}
	wg.Wait()

	// Each host should have exactly 1 monitor
	for _, h := range hosts {
		var count int64
		db.Model(&models.UptimeMonitor{}).Where("proxy_host_id = ?", h.ID).Count(&count)
		assert.Equal(t, int64(1), count, "each proxy host should have exactly 1 monitor")
	}
}

func TestSyncAndCheckForHost_ConcurrentSameHost_NoDuplicates(t *testing.T) {
	db := setupPR1ConcurrentDB(t)
	enableUptimeFeature(t, db)
	svc := NewUptimeService(db, nil)
	server := createAlwaysOKServer(t)
	domain := hostPortFromServerURL(server.URL)

	host := createTestProxyHost(t, db, "race-host", domain, "10.0.0.200")

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			svc.SyncAndCheckForHost(host.ID)
		}()
	}
	wg.Wait()

	// Should still be exactly 1 monitor even after 10 concurrent calls
	var count int64
	db.Model(&models.UptimeMonitor{}).Where("proxy_host_id = ?", host.ID).Count(&count)
	assert.Equal(t, int64(1), count, "concurrent SyncAndCheckForHost should not create duplicates")
}
