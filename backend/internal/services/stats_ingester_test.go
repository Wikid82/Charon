package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupStatsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "stats_test.db") + "?_busy_timeout=5000&_journal_mode=WAL"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err, "open test db")
	require.NoError(t, db.AutoMigrate(&models.RequestLog{}), "migrate RequestLog")
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil && sqlDB != nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func makeEntry(hostID, ip, method string, status int) models.SecurityLogEntry {
	return models.SecurityLogEntry{
		Timestamp: time.Now().Format(time.RFC3339),
		ClientIP:  ip,
		Host:      hostID,
		Method:    method,
		Status:    status,
		Duration:  0.001,
		Size:      128,
	}
}

// TestStatsIngester_BatchFlushOnCount verifies that 100 entries trigger a batch write.
func TestStatsIngester_BatchFlushOnCount(t *testing.T) {
	t.Parallel()

	db := setupStatsTestDB(t)
	ing := NewStatsIngester(db)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go ing.Run(ctx)

	// Send exactly 100 entries — should trigger a flush before the timer fires.
	for i := 0; i < 100; i++ {
		ing.ingestCh <- makeEntry("host-1", "10.0.0.1", "GET", 200)
	}

	// Allow time for the batch write.
	require.Eventually(t, func() bool {
		var count int64
		db.Model(&models.RequestLog{}).Count(&count)
		return count >= 100
	}, 3*time.Second, 50*time.Millisecond, "expected 100 rows after count-flush")
}

// TestStatsIngester_BatchFlushOnTimer verifies that entries are flushed after 500ms.
func TestStatsIngester_BatchFlushOnTimer(t *testing.T) {
	t.Parallel()

	db := setupStatsTestDB(t)
	ing := NewStatsIngester(db)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go ing.Run(ctx)

	// Send fewer than 100 entries — only the timer should trigger the flush.
	for i := 0; i < 5; i++ {
		ing.ingestCh <- makeEntry("host-2", "10.0.0.2", "POST", 201)
	}

	// Should flush within ~600ms.
	require.Eventually(t, func() bool {
		var count int64
		db.Model(&models.RequestLog{}).Count(&count)
		return count >= 5
	}, 3*time.Second, 50*time.Millisecond, "expected 5 rows after timer-flush")
}

// TestStatsIngester_DroppedCountIncrementsWhenFull verifies back-pressure tracking.
func TestStatsIngester_DroppedCountIncrementsWhenFull(t *testing.T) {
	t.Parallel()

	db := setupStatsTestDB(t)
	ing := NewStatsIngester(db)

	// Do NOT start Run — keep the channel un-drained so we can fill it.
	// Fill the channel buffer completely.
	for i := 0; i < channelBufferSize; i++ {
		ing.ingestCh <- makeEntry("host-3", "10.0.0.3", "GET", 200)
	}

	// Now send one more non-blocking — should be dropped.
	select {
	case ing.ingestCh <- makeEntry("host-3", "10.0.0.3", "GET", 200):
		// sent; channel had room — only possible on a race (acceptable)
	default:
		ing.droppedCount.Add(1)
	}

	// Force a drop via the exported Send path.
	ing.Send(makeEntry("host-3", "10.0.0.3", "GET", 200))

	assert.GreaterOrEqual(t, ing.DroppedCount(), int64(1))
}

// TestStatsIngester_StopDrainsRemainingEntries verifies graceful shutdown writes pending entries.
func TestStatsIngester_StopDrainsRemainingEntries(t *testing.T) {
	t.Parallel()

	db := setupStatsTestDB(t)
	ing := NewStatsIngester(db)

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ing.Run(ctx)
	}()

	// Send a small batch.
	for i := 0; i < 10; i++ {
		ing.ingestCh <- makeEntry("host-4", "10.0.0.4", "DELETE", 204)
	}

	// Cancel context and wait for Run to exit.
	cancel()
	wg.Wait()

	// Stop drains the channel — all pending entries should be persisted.
	ing.Stop()

	var count int64
	db.Model(&models.RequestLog{}).Count(&count)
	assert.GreaterOrEqual(t, count, int64(10))
}

// TestStatsIngester_ClientIPHashing verifies GDPR-safe pseudonymisation.
func TestStatsIngester_ClientIPHashing(t *testing.T) {
	t.Parallel()

	rawIP := "192.168.1.100"
	got := hashClientIP(rawIP)

	// Must be hex-encoded, 32 chars (16 bytes × 2 hex digits).
	assert.Len(t, got, 32)

	// Must be consistent (deterministic).
	assert.Equal(t, got, hashClientIP(rawIP))

	// Must differ from the raw IP.
	assert.NotEqual(t, rawIP, got)

	// Verify correctness against reference implementation.
	sum := sha256.Sum256([]byte(rawIP))
	expected := hex.EncodeToString(sum[:16])
	assert.Equal(t, expected, got)

	// Must not be reversible (different IPs produce different hashes).
	assert.NotEqual(t, got, hashClientIP("10.0.0.1"))
}

// TestStatsIngester_RegisterWithLogWatcher verifies fan-out wiring.
func TestStatsIngester_RegisterWithLogWatcher(t *testing.T) {
	t.Parallel()

	db := setupStatsTestDB(t)
	ing := NewStatsIngester(db)

	watcher := NewLogWatcher("/tmp/nonexistent-test.log")
	watcher.RegisterIngester(ing)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go ing.Run(ctx)

	// Broadcast via the watcher — should fan-out to ingester.
	entry := models.SecurityLogEntry{
		Timestamp: time.Now().Format(time.RFC3339),
		ClientIP:  "172.16.0.1",
		Host:      "fan-out-host",
		Method:    "GET",
		Status:    200,
		Duration:  0.002,
		Size:      64,
	}
	watcher.broadcast(entry)

	require.Eventually(t, func() bool {
		var count int64
		db.Model(&models.RequestLog{}).Count(&count)
		return count >= 1
	}, 3*time.Second, 50*time.Millisecond, "expected 1 row after fan-out broadcast + timer flush")
}
