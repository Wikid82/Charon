package services

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/google/uuid"
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

// TestSyncAndCheckForHost_RetriesOnTransientLockError verifies that a
// transient "database is locked" error from the monitor-create write inside
// SyncAndCheckForHost is retried with backoff instead of being treated as
// permanent, mirroring the established convention in
// credential_service.go's Delete and security_service.go's
// persistAuditWithRetry.
//
// Two independent connections attach to the same named, shared-cache
// in-memory database (no _busy_timeout DSN param on either, so SQLite's own
// busy handler is not in play -- a lock conflict surfaces to the Go driver
// immediately as "database is locked" rather than blocking internally).
// This deliberately mirrors the locking mechanism confirmed during the
// investigation behind this fix: SQLite's shared-cache table locking is a
// pure in-process mechanism enforced by the SQLite library itself, unlike
// on-disk file locking which depends on OS-level advisory locks that are
// not reliably enforced on every filesystem this suite may run against.
// One connection holds a write lock (via an uncommitted transaction) for a
// short window immediately before SyncAndCheckForHost runs on the other,
// so this test exercises this package's own retry loop, not SQLite's.
func TestSyncAndCheckForHost_RetriesOnTransientLockError(t *testing.T) {
	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	t.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	require.NoError(t, db.AutoMigrate(
		&models.ProxyHost{},
		&models.UptimeHost{},
		&models.UptimeMonitor{},
		&models.UptimeHeartbeat{},
		&models.Setting{},
	))

	ns := NewNotificationService(db, nil)
	svc := NewUptimeService(db, ns)

	host := models.ProxyHost{
		UUID:        uuid.NewString(),
		Name:        "Retry Target",
		DomainNames: "retry-target.example.com",
		ForwardHost: "retry-upstream",
		ForwardPort: 8080,
		Enabled:     true,
	}
	require.NoError(t, db.Create(&host).Error)

	lockDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	t.Cleanup(func() {
		if sqlDB, dbErr := lockDB.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	lockAcquired := make(chan struct{})
	lockReleased := make(chan struct{})
	go func() {
		defer close(lockReleased)
		_ = lockDB.Transaction(func(tx *gorm.DB) error {
			// Any write statement takes SQLite's shared-cache write lock
			// (locking is at the database level, not just this table),
			// which is what blocks the primary connection's later INSERT
			// into uptime_monitors below.
			if txErr := tx.Exec("INSERT INTO settings (key, value) VALUES (?, ?)", "lock-holder", "1").Error; txErr != nil {
				return txErr
			}
			close(lockAcquired)
			time.Sleep(75 * time.Millisecond)
			return nil
		})
	}()

	<-lockAcquired
	t.Cleanup(func() { <-lockReleased })

	svc.SyncAndCheckForHost(host.ID)

	var monitor models.UptimeMonitor
	require.NoError(t, db.Where("proxy_host_id = ?", host.ID).First(&monitor).Error,
		"monitor should eventually be created despite transient lock contention from a concurrent writer")
}

// newPinnedUptimeDB opens an in-memory SQLite DB pinned to a single connection
// (SetMaxOpenConns(1)), mirroring production's configurePool. A single shared
// connection means all goroutines see the same in-memory database and SQLite
// writes serialise, so the only race left to exercise is the Go-level
// check-then-act interleaving inside ensureUptimeHost.
func newPinnedUptimeDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(
		&models.UptimeHost{},
		&models.UptimeMonitor{},
		&models.UptimeHeartbeat{},
		&models.NotificationProvider{},
		&models.Notification{},
	))
	return db
}

// TestEnsureUptimeHost_ConcurrentSameHost is the regression test for GitHub
// issue #1221: a check-then-act race in ensureUptimeHost. Before the fix, two
// goroutines could both observe gorm.ErrRecordNotFound for the same host and
// both attempt an INSERT; the loser hit "UNIQUE constraint failed:
// uptime_hosts.host", logged an error, and returned "" — silently dropping its
// monitor out of host-level notification grouping.
//
// Must pass under -race.
func TestEnsureUptimeHost_ConcurrentSameHost(t *testing.T) {
	db := newPinnedUptimeDB(t)
	ns := NewNotificationService(db, nil)
	svc := NewUptimeService(db, ns)

	const goroutines = 12
	const sharedHost = "shared-upstream.internal"

	var wg sync.WaitGroup
	ids := make([]string, goroutines)
	start := make(chan struct{})

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start // release all goroutines at once to maximise interleaving
			ids[i] = svc.ensureUptimeHost(sharedHost, fmt.Sprintf("caller-%d", i))
		}(i)
	}

	close(start)
	wg.Wait()

	// Exactly one row exists for the shared host.
	var count int64
	require.NoError(t, db.Model(&models.UptimeHost{}).Where("host = ?", sharedHost).Count(&count).Error)
	assert.Equal(t, int64(1), count, "exactly one UptimeHost row must exist for the shared host")

	var winner models.UptimeHost
	require.NoError(t, db.Where("host = ?", sharedHost).First(&winner).Error)
	require.NotEmpty(t, winner.ID)

	// Every caller returned the same, non-empty ID — nobody lost the race.
	for i, id := range ids {
		assert.NotEmptyf(t, id, "goroutine %d returned an empty UptimeHost ID", i)
		assert.Equalf(t, winner.ID, id, "goroutine %d returned a different UptimeHost ID", i)
	}
}

// TestEnsureUptimeHost_UnexpectedQueryError_ReturnsEmpty verifies the
// unexpected-error branch: a non-ErrRecordNotFound failure from the initial
// lookup must be logged and return "" explicitly rather than falling through
// to return an empty uptimeHost.ID.
func TestEnsureUptimeHost_UnexpectedQueryError_ReturnsEmpty(t *testing.T) {
	db := newPinnedUptimeDB(t)
	ns := NewNotificationService(db, nil)
	svc := NewUptimeService(db, ns)

	// Drop the table so the SELECT fails with "no such table: uptime_hosts".
	require.NoError(t, db.Migrator().DropTable(&models.UptimeHost{}))

	got := svc.ensureUptimeHost("some-host.internal", "Some Host")
	assert.Empty(t, got, "unexpected query error must return an empty ID, not fall through")
}
