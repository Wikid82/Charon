package services

import (
	"context"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// seedHeartbeats inserts n heartbeat rows all stamped at createdAt.
func seedHeartbeats(t *testing.T, db *gorm.DB, monitorID string, createdAt time.Time, n int) {
	t.Helper()
	rows := make([]models.UptimeHeartbeat, 0, n)
	for i := 0; i < n; i++ {
		rows = append(rows, models.UptimeHeartbeat{
			MonitorID: monitorID,
			Status:    "up",
			Latency:   10,
			CreatedAt: createdAt,
		})
	}
	require.NoError(t, db.CreateInBatches(rows, 1000).Error)
}

func countHeartbeats(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&models.UptimeHeartbeat{}).Count(&n).Error)
	return n
}

func heartbeatIndexNames(t *testing.T, db *gorm.DB) []string {
	t.Helper()
	var names []string
	require.NoError(t, db.Raw(
		"SELECT name FROM sqlite_master WHERE type='index' AND tbl_name='uptime_heartbeats'").
		Scan(&names).Error)
	return names
}

func hasDeferredIndex(t *testing.T, db *gorm.DB) bool {
	t.Helper()
	for _, n := range heartbeatIndexNames(t, db) {
		if n == "idx_heartbeat_monitor_created" {
			return true
		}
	}
	return false
}

// newTestPruner builds a pruner over a pinned in-memory DB with a fixed clock
// and the fast steady-state pause (firstPassDone pre-set) unless a test needs
// the wider first-pass behaviour.
func newTestPruner(t *testing.T, db *gorm.DB, nowFn func() time.Time) *UptimePruner {
	t.Helper()
	p := newUptimePruner(db, newUptimeConfig(db))
	p.now = nowFn
	p.firstPassDone.Store(true)
	return p
}

func TestUptimePruner_DeletesOnlyRowsBeforeCutoff(t *testing.T) {
	db := newPinnedUptimeDB(t)
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	p := newTestPruner(t, db, func() time.Time { return now })

	seedHeartbeats(t, db, "m-old", now.AddDate(0, 0, -100), 40) // older than 90d -> delete
	seedHeartbeats(t, db, "m-new", now.AddDate(0, 0, -10), 25)  // within 90d -> keep

	deleted, err := p.pruneOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(40), deleted)
	require.Equal(t, int64(25), countHeartbeats(t, db))

	var remainingOld int64
	require.NoError(t, db.Model(&models.UptimeHeartbeat{}).
		Where("monitor_id = ?", "m-old").Count(&remainingOld).Error)
	require.Zero(t, remainingOld)
}

func TestUptimePruner_ChunkLoopTerminates(t *testing.T) {
	db := newPinnedUptimeDB(t)
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	p := newTestPruner(t, db, func() time.Time { return now })

	// 12000 stale rows => 5000 + 5000 + 2000, the last chunk (< pruneChunkSize)
	// breaks the loop.
	seedHeartbeats(t, db, "m1", now.AddDate(0, 0, -200), 12000)

	deleted, err := p.pruneOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(12000), deleted)
	require.Zero(t, countHeartbeats(t, db))
}

func TestUptimePruner_HonorsHotRetentionChange(t *testing.T) {
	db := newPinnedUptimeDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}))
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	p := newTestPruner(t, db, func() time.Time { return now })

	seedHeartbeats(t, db, "m1", now.AddDate(0, 0, -100), 10) // > 90d
	seedHeartbeats(t, db, "m1", now.AddDate(0, 0, -30), 10)  // 30d old
	seedHeartbeats(t, db, "m1", now.Add(-1*time.Hour), 5)    // fresh

	// Pass 1: default 90-day retention -> only the -100d rows go.
	deleted, err := p.pruneOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(10), deleted)
	require.Equal(t, int64(15), countHeartbeats(t, db))

	// Operator tightens retention to 1 day between passes.
	require.NoError(t, db.Create(&models.Setting{
		Key: "uptime.heartbeat_retention_days", Value: "1", Type: "int", Category: "uptime",
	}).Error)
	p.cfg.forceRefresh()

	// Pass 2: the 30-day-old rows are now stale; the fresh rows survive.
	deleted, err = p.pruneOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(10), deleted)
	require.Equal(t, int64(5), countHeartbeats(t, db))
}

func TestUptimePruner_ContextCancelledMidLoop(t *testing.T) {
	db := newPinnedUptimeDB(t)
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	// Raw pruner (firstPassDone still false) so we can assert an aborted pass
	// does not flip it.
	p := newUptimePruner(db, newUptimeConfig(db))
	p.now = func() time.Time { return now }

	seedHeartbeats(t, db, "m1", now.AddDate(0, 0, -200), 12000)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Cancel right before the second chunk: chunk 1 (5000 rows) has committed,
	// the loop's ctx.Err() guard then aborts the pass.
	p.beforeChunkHook = func(iteration int) {
		if iteration == 2 {
			cancel()
		}
	}

	deleted, err := p.pruneOnce(ctx)
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, int64(pruneChunkSize), deleted)
	require.Equal(t, int64(12000-pruneChunkSize), countHeartbeats(t, db))
	require.False(t, p.firstPassDone.Load(), "an aborted pass must not flip firstPassDone")
}

func TestUptimePruner_NoopWhenCaughtUp(t *testing.T) {
	db := newPinnedUptimeDB(t)
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	p := newTestPruner(t, db, func() time.Time { return now })

	seedHeartbeats(t, db, "m1", now.Add(-2*time.Hour), 30) // all fresh

	deleted, err := p.pruneOnce(context.Background())
	require.NoError(t, err)
	require.Zero(t, deleted)
	require.Equal(t, int64(30), countHeartbeats(t, db))
}

func TestUptimePruner_FirstPassFlipsFirstPassDone(t *testing.T) {
	db := newPinnedUptimeDB(t)
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	p := newUptimePruner(db, newUptimeConfig(db))
	p.now = func() time.Time { return now }
	require.False(t, p.firstPassDone.Load(), "starts false -> wider first-pass pause")

	seedHeartbeats(t, db, "m1", now.AddDate(0, 0, -100), 20)

	_, err := p.pruneOnce(context.Background())
	require.NoError(t, err)
	require.True(t, p.firstPassDone.Load(), "a clean pass flips to steady-state pause")
}

func TestUptimePruner_EnsureIndexAfterCleanPass(t *testing.T) {
	db := newPinnedUptimeDB(t)
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	p := newTestPruner(t, db, func() time.Time { return now })

	seedHeartbeats(t, db, "m1", now.AddDate(0, 0, -100), 20)
	require.False(t, hasDeferredIndex(t, db))

	p.tick(context.Background())

	require.True(t, hasDeferredIndex(t, db))
	require.True(t, p.indexCreated.Load())

	// A second attempt short-circuits on the in-memory bool and stays a no-op.
	require.NoError(t, p.ensureIndex(context.Background()))
	require.True(t, hasDeferredIndex(t, db))
}

func TestUptimePruner_EnsureIndexSkippedAfterContextAbort(t *testing.T) {
	db := newPinnedUptimeDB(t)
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	p := newTestPruner(t, db, func() time.Time { return now })

	seedHeartbeats(t, db, "m1", now.AddDate(0, 0, -200), 12000)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.beforeChunkHook = func(iteration int) {
		if iteration == 1 {
			cancel()
		}
	}

	p.tick(ctx)

	require.False(t, hasDeferredIndex(t, db), "a ctx-aborted pass must not build the index")
	require.False(t, p.indexCreated.Load())
}

func TestUptimePruner_EnsureIndexRetriedAfterErroredPass(t *testing.T) {
	db := newPinnedUptimeDB(t)
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	p := newTestPruner(t, db, func() time.Time { return now })

	// First pass: table missing -> pruneOnce returns an error, no index attempt.
	require.NoError(t, db.Migrator().DropTable(&models.UptimeHeartbeat{}))
	p.tick(context.Background())
	require.False(t, p.indexCreated.Load())

	// Next pass on a healthy table: clean prune -> the index is finally built.
	require.NoError(t, db.AutoMigrate(&models.UptimeHeartbeat{}))
	seedHeartbeats(t, db, "m1", now.AddDate(0, 0, -100), 10)
	p.tick(context.Background())
	require.True(t, hasDeferredIndex(t, db))
	require.True(t, p.indexCreated.Load())
}

func TestUptimePruner_ErroredChunkReturnsWrappedError(t *testing.T) {
	db := newPinnedUptimeDB(t)
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	p := newTestPruner(t, db, func() time.Time { return now })

	require.NoError(t, db.Migrator().DropTable(&models.UptimeHeartbeat{}))

	deleted, err := p.pruneOnce(context.Background())
	require.Error(t, err)
	require.Zero(t, deleted)
	require.Contains(t, err.Error(), "uptime prune chunk")
}

func TestUptimePruner_WALCheckpointPastThreshold(t *testing.T) {
	db := newPinnedUptimeDB(t)
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	p := newTestPruner(t, db, func() time.Time { return now })
	p.walRowThreshold = 20 // cross the threshold without seeding 50k rows

	seedHeartbeats(t, db, "m1", now.AddDate(0, 0, -200), 12000)

	deleted, err := p.pruneOnce(context.Background())
	require.NoError(t, err, "PRAGMA wal_checkpoint(TRUNCATE) must run cleanly after a large prune")
	require.Equal(t, int64(12000), deleted)
	require.Zero(t, countHeartbeats(t, db))
}

func TestUptimePruner_WALCheckpointSkippedBelowThreshold(t *testing.T) {
	db := newPinnedUptimeDB(t)
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	p := newTestPruner(t, db, func() time.Time { return now })

	seedHeartbeats(t, db, "m1", now.AddDate(0, 0, -100), 100) // 100 < 50_000 default

	deleted, err := p.pruneOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(100), deleted)
}

func TestUptimePruner_OptimizeCadence(t *testing.T) {
	db := newPinnedUptimeDB(t)
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	p := newTestPruner(t, db, func() time.Time { return now })
	p.indexCreated.Store(true) // isolate the optimize-cadence branch
	p.passCount.Store(optimizeEveryPasses - 1)

	seedHeartbeats(t, db, "m1", now.Add(-1*time.Hour), 5) // fresh -> 0 deleted

	p.tick(context.Background())

	require.Equal(t, int64(optimizeEveryPasses), p.passCount.Load(),
		"a clean pass increments the pass counter and triggers PRAGMA optimize")
}

func TestUptimePruner_Run(t *testing.T) {
	db := newPinnedUptimeDB(t)
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	p := newTestPruner(t, db, func() time.Time { return now })
	p.firstRunDelay = 5 * time.Millisecond
	p.interval = 0 // exercise Run's iv<=0 guard -> default hourly ticker

	seedHeartbeats(t, db, "m1", now.AddDate(0, 0, -100), 30)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { p.Run(ctx); close(done) }()

	require.Eventually(t, func() bool {
		return countHeartbeats(t, db) == 0 && hasDeferredIndex(t, db)
	}, 3*time.Second, 10*time.Millisecond, "Run must prune then build the deferred index on the first pass")

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after ctx cancel")
	}
}

func TestUptimePruner_RunAbortsDuringInitialDelay(t *testing.T) {
	db := newPinnedUptimeDB(t)
	p := newTestPruner(t, db, time.Now)
	p.firstRunDelay = 0 // exercise the <=0 guard -> default, then abort via ctx
	p.interval = 0

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { p.Run(ctx); close(done) }()

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return when ctx was cancelled during the initial delay")
	}
}

func TestUptimePruner_RunTicksRepeatedly(t *testing.T) {
	db := newPinnedUptimeDB(t)
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	p := newTestPruner(t, db, func() time.Time { return now })
	p.firstRunDelay = 1 * time.Millisecond
	p.interval = 5 * time.Millisecond

	seedHeartbeats(t, db, "m1", now.Add(-1*time.Hour), 3) // always fresh -> every pass is clean, 0 deleted

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { p.Run(ctx); close(done) }()

	// passCount only advances inside tick, so >= 3 proves the ticker branch
	// (not just the initial pass) drove additional tick() calls.
	require.Eventually(t, func() bool { return p.passCount.Load() >= 3 },
		3*time.Second, 5*time.Millisecond, "the hourly ticker branch must keep calling tick()")

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after ctx cancel")
	}
}

func TestUptimePruner_EnsureIndexErrorPathThenRecovers(t *testing.T) {
	db := newPinnedUptimeDB(t)
	p := newTestPruner(t, db, time.Now)

	// CREATE INDEX ... ON uptime_heartbeats fails while the table is absent.
	require.NoError(t, db.Migrator().DropTable(&models.UptimeHeartbeat{}))
	err := p.ensureIndex(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "idx_heartbeat_monitor_created")
	require.False(t, p.indexCreated.Load(), "a failed attempt must leave the retry bool false")

	// Next attempt on a healthy table succeeds and latches the bool.
	require.NoError(t, db.AutoMigrate(&models.UptimeHeartbeat{}))
	require.NoError(t, p.ensureIndex(context.Background()))
	require.True(t, p.indexCreated.Load())
	require.True(t, hasDeferredIndex(t, db))
}

func TestNewUptimePruner_WiresPoolDBAndConfig(t *testing.T) {
	db := newPinnedUptimeDB(t)
	ns := NewNotificationService(db, nil)
	pool := NewUptimeWorkerPool(NewUptimeService(db, ns))

	p := NewUptimePruner(pool)
	require.NotNil(t, p)
	require.Same(t, pool.db, p.db)
	require.Same(t, pool.cfg, p.cfg)
	require.Equal(t, prunerInterval, p.interval)
	require.Equal(t, prunerFirstRunDelay, p.firstRunDelay)
	require.NotNil(t, p.now)
}
