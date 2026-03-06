package services

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupUptimeRaceTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.UptimeHost{},
		&models.UptimeMonitor{},
		&models.UptimeHeartbeat{},
		&models.NotificationProvider{},
		&models.Notification{},
	))
	return db
}

func TestCheckHost_RetryLogic(t *testing.T) {
	db := setupUptimeRaceTestDB(t)
	ns := NewNotificationService(db, nil)
	svc := NewUptimeService(db, ns)
	svc.config.TCPTimeout = 500 * time.Millisecond
	svc.config.MaxRetries = 2

	// Verify retry config is set correctly
	assert.Equal(t, 2, svc.config.MaxRetries, "MaxRetries should be configurable")
	assert.Equal(t, 500*time.Millisecond, svc.config.TCPTimeout, "TCPTimeout should be configurable")

	// Test with a non-existent port (will fail all retries)
	host := models.UptimeHost{
		Host:   "127.0.0.1",
		Name:   "Test Host",
		Status: "pending",
	}
	db.Create(&host)

	monitor := models.UptimeMonitor{
		UptimeHostID: &host.ID,
		Name:         "Test Monitor",
		Type:         "tcp",
		URL:          "tcp://127.0.0.1:9", // port 9 is discard, will refuse connection
	}
	db.Create(&monitor)

	// Run check - should fail but complete within reasonable time
	ctx := context.Background()
	start := time.Now()
	svc.checkHost(ctx, &host)
	elapsed := time.Since(start)

	// With 2 retries and 500ms timeout, should complete in < 3s (500ms * 3 attempts + delays)
	assert.Less(t, elapsed, 5*time.Second, "Should complete within expected time with retries")

	// Verify host is down after retries
	var updatedHost models.UptimeHost
	db.First(&updatedHost, "id = ?", host.ID)
	assert.Greater(t, updatedHost.FailureCount, 0, "Failure count should be incremented")
}

func TestCheckHost_Debouncing(t *testing.T) {
	db := setupUptimeRaceTestDB(t)
	ns := NewNotificationService(db, nil)
	svc := NewUptimeService(db, ns)
	svc.config.FailureThreshold = 2         // Require 2 failures
	svc.config.TCPTimeout = 1 * time.Second // Shorter timeout for test
	svc.config.MaxRetries = 0               // No retries for this test

	host := models.UptimeHost{
		Host:   "192.0.2.1", // TEST-NET-1, guaranteed to fail
		Name:   "Test Host",
		Status: "up",
	}
	db.Create(&host)

	monitor := models.UptimeMonitor{
		UptimeHostID: &host.ID,
		Name:         "Test Monitor",
		Type:         "tcp",
		URL:          "tcp://192.0.2.1:9999",
	}
	db.Create(&monitor)

	ctx := context.Background()

	// First failure - should NOT mark as down
	svc.checkHost(ctx, &host)
	db.Where("id = ?", host.ID).First(&host)
	assert.Equal(t, "up", host.Status, "Host should remain up after first failure")
	assert.Equal(t, 1, host.FailureCount, "Failure count should be 1")

	// Second failure - should mark as down
	svc.checkHost(ctx, &host)
	db.Where("id = ?", host.ID).First(&host)
	assert.Equal(t, "down", host.Status, "Host should be down after second failure")
	assert.Equal(t, 2, host.FailureCount, "Failure count should be 2")
}

func TestCheckHost_FailureCountReset(t *testing.T) {
	db := setupUptimeRaceTestDB(t)
	ns := NewNotificationService(db, nil)
	svc := NewUptimeService(db, ns)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = listener.Close() }()

	port := listener.Addr().(*net.TCPAddr).Port

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	host := models.UptimeHost{
		Host:         "127.0.0.1",
		Name:         "Test Host",
		Status:       "down",
		FailureCount: 3,
	}
	db.Create(&host)

	monitor := models.UptimeMonitor{
		UptimeHostID: &host.ID,
		Name:         "Test Monitor",
		Type:         "tcp",
		URL:          fmt.Sprintf("tcp://127.0.0.1:%d", port),
	}
	db.Create(&monitor)

	ctx := context.Background()
	svc.checkHost(ctx, &host)

	// Verify failure count is reset on success
	db.Where("id = ?", host.ID).First(&host)
	assert.Equal(t, "up", host.Status, "Host should be up")
	assert.Equal(t, 0, host.FailureCount, "Failure count should be reset to 0 on success")
}

func TestCheckAllHosts_Synchronization(t *testing.T) {
	db := setupUptimeRaceTestDB(t)
	ns := NewNotificationService(db, nil)
	svc := NewUptimeService(db, ns)
	svc.config.TCPTimeout = 500 * time.Millisecond // Shorter timeout for test
	svc.config.MaxRetries = 0                      // No retries for this test
	svc.config.CheckTimeout = 10 * time.Second     // Shorter overall timeout

	// Create multiple hosts
	numHosts := 5
	for i := 0; i < numHosts; i++ {
		host := models.UptimeHost{
			Host:   fmt.Sprintf("192.0.2.%d", i+1),
			Name:   fmt.Sprintf("Host %d", i+1),
			Status: "pending",
		}
		db.Create(&host)

		monitor := models.UptimeMonitor{
			UptimeHostID: &host.ID,
			Name:         fmt.Sprintf("Monitor %d", i+1),
			Type:         "tcp",
			URL:          fmt.Sprintf("tcp://192.0.2.%d:9999", i+1),
		}
		db.Create(&monitor)
	}

	start := time.Now()
	svc.checkAllHosts()
	elapsed := time.Since(start)

	// Verify all hosts were checked
	var hosts []models.UptimeHost
	db.Find(&hosts)
	assert.Len(t, hosts, numHosts)

	for _, host := range hosts {
		assert.NotEmpty(t, host.Status, "Host status should be set")
		assert.False(t, host.LastCheck.IsZero(), "LastCheck should be set")
	}

	// With concurrent checks and timeout, should complete reasonably fast
	// Not all hosts will succeed (using TEST-NET addresses), but function should return
	assert.Less(t, elapsed, 15*time.Second, "checkAllHosts should complete within timeout+buffer")
}

func TestCheckHost_ConcurrentChecks(t *testing.T) {
	db := setupUptimeRaceTestDB(t)
	ns := NewNotificationService(db, nil)
	svc := NewUptimeService(db, ns)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = listener.Close() }()

	port := listener.Addr().(*net.TCPAddr).Port

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	host := models.UptimeHost{
		Host:   "127.0.0.1",
		Name:   "Test Host",
		Status: "pending",
	}
	db.Create(&host)

	monitor := models.UptimeMonitor{
		UptimeHostID: &host.ID,
		Name:         "Test Monitor",
		Type:         "tcp",
		URL:          fmt.Sprintf("tcp://127.0.0.1:%d", port),
	}
	db.Create(&monitor)

	// Run multiple concurrent checks
	var wg sync.WaitGroup
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			svc.checkHost(ctx, &host)
		}()
	}

	wg.Wait()

	// Verify no race conditions or deadlocks
	var updatedHost models.UptimeHost
	db.Where("id = ?", host.ID).First(&updatedHost)
	assert.Equal(t, "up", updatedHost.Status, "Host should be up")
	assert.NotZero(t, updatedHost.LastCheck, "LastCheck should be set")
}

func TestCheckHost_ContextCancellation(t *testing.T) {
	db := setupUptimeRaceTestDB(t)
	ns := NewNotificationService(db, nil)
	svc := NewUptimeService(db, ns)
	svc.config.TCPTimeout = 5 * time.Second // Normal timeout
	svc.config.MaxRetries = 0               // No retries for this test

	host := models.UptimeHost{
		Host:   "192.0.2.1", // Will timeout
		Name:   "Test Host",
		Status: "pending",
	}
	db.Create(&host)

	monitor := models.UptimeMonitor{
		UptimeHostID: &host.ID,
		Name:         "Test Monitor",
		Type:         "tcp",
		URL:          "tcp://192.0.2.1:9999",
	}
	db.Create(&monitor)

	// Create context that will cancel immediately
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	time.Sleep(5 * time.Millisecond) // Ensure context is cancelled

	start := time.Now()
	svc.checkHost(ctx, &host)
	elapsed := time.Since(start)

	// Should return quickly due to context cancellation
	assert.Less(t, elapsed, 2*time.Second, "checkHost should respect context cancellation")
}

func TestCheckAllHosts_StaggeredStartup(t *testing.T) {
	db := setupUptimeRaceTestDB(t)
	ns := NewNotificationService(db, nil)
	svc := NewUptimeService(db, ns)
	svc.config.StaggerDelay = 50 * time.Millisecond
	svc.config.TCPTimeout = 500 * time.Millisecond // Shorter timeout for test
	svc.config.MaxRetries = 0                      // No retries for this test
	svc.config.CheckTimeout = 10 * time.Second     // Shorter overall timeout

	// Create multiple hosts
	numHosts := 3
	for i := 0; i < numHosts; i++ {
		host := models.UptimeHost{
			Host:   fmt.Sprintf("192.0.2.%d", i+1),
			Name:   fmt.Sprintf("Host %d", i+1),
			Status: "pending",
		}
		db.Create(&host)

		monitor := models.UptimeMonitor{
			UptimeHostID: &host.ID,
			Name:         fmt.Sprintf("Monitor %d", i+1),
			Type:         "tcp",
			URL:          fmt.Sprintf("tcp://192.0.2.%d:9999", i+1),
		}
		db.Create(&monitor)
	}

	start := time.Now()
	svc.checkAllHosts()
	elapsed := time.Since(start)

	// With staggered startup (50ms * 2 delays between 3 hosts) + check time
	// Should take at least 100ms due to stagger delays
	assert.GreaterOrEqual(t, elapsed, 100*time.Millisecond, "Should include stagger delays")
}

func TestUptimeConfig_Defaults(t *testing.T) {
	db := setupUptimeRaceTestDB(t)
	ns := NewNotificationService(db, nil)
	svc := NewUptimeService(db, ns)

	assert.Equal(t, 10*time.Second, svc.config.TCPTimeout, "TCP timeout should be 10s")
	assert.Equal(t, 2, svc.config.MaxRetries, "Max retries should be 2")
	assert.Equal(t, 2, svc.config.FailureThreshold, "Failure threshold should be 2")
	assert.Equal(t, 60*time.Second, svc.config.CheckTimeout, "Check timeout should be 60s")
	assert.Equal(t, 100*time.Millisecond, svc.config.StaggerDelay, "Stagger delay should be 100ms")
}

func TestCheckHost_HostMutexPreventsRaceCondition(t *testing.T) {
	db := setupUptimeRaceTestDB(t)
	ns := NewNotificationService(db, nil)
	svc := NewUptimeService(db, ns)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = listener.Close() }()

	port := listener.Addr().(*net.TCPAddr).Port

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			time.Sleep(10 * time.Millisecond) // Simulate slow response
			_ = conn.Close()
		}
	}()

	host := models.UptimeHost{
		Host:   "127.0.0.1",
		Name:   "Test Host",
		Status: "pending",
	}
	db.Create(&host)

	monitor := models.UptimeMonitor{
		UptimeHostID: &host.ID,
		Name:         "Test Monitor",
		Type:         "tcp",
		URL:          fmt.Sprintf("tcp://127.0.0.1:%d", port),
	}
	db.Create(&monitor)

	// Run multiple concurrent checks to test mutex
	var wg sync.WaitGroup
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			svc.checkHost(ctx, &host)
		}()
	}

	wg.Wait()

	// Verify database consistency (no corruption from race conditions)
	var updatedHost models.UptimeHost
	db.Where("id = ?", host.ID).First(&updatedHost)
	assert.NotEmpty(t, updatedHost.Status, "Host status should be set")
	assert.Equal(t, "up", updatedHost.Status, "Host should be up")
	assert.GreaterOrEqual(t, updatedHost.Latency, int64(0), "Latency should be non-negative")
}
