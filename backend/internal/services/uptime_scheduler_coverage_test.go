package services

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// closedUptimeDB returns a migrated uptime DB whose underlying connection pool
// has been closed, so every subsequent query returns "sql: database is closed".
// Used to drive the scheduler's DB-error branches deterministically.
func closedUptimeDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := setupUptimeTestDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	return db
}

func ptrString(s string) *string { return &s }

// --- loadHostSchedules: real host rows exercise the map-building loop ---

func TestUptimeScheduler_Hydrate_BuildsHostSchedulesFromHostLinkedMonitors(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	sched, _ := newTestScheduler(t, db)

	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	sched.now = func() time.Time { return now }

	host := models.UptimeHost{Host: "10.0.0.20", Name: "h20", Status: "up"}
	require.NoError(t, db.Create(&host).Error)
	// Two enabled children with different intervals: min(45, 90) -> 45.
	seedMonitor(t, db, models.UptimeMonitor{Name: "c-a", Enabled: true, Interval: 45, Type: "tcp", URL: "10.0.0.20:22", UptimeHostID: &host.ID})
	seedMonitor(t, db, models.UptimeMonitor{Name: "c-b", Enabled: true, Interval: 90, Type: "http", UptimeHostID: &host.ID})

	sched.hydrate(context.Background())

	sched.mu.Lock()
	defer sched.mu.Unlock()
	require.Contains(t, sched.hostMinInt, host.ID)
	assert.Equal(t, 45, sched.hostMinInt[host.ID], "host effective interval is min of enabled child intervals, clamped")
	due, ok := sched.hostSchedule[host.ID]
	require.True(t, ok, "host schedule seeded during hydration")
	off := due.Sub(now)
	assert.GreaterOrEqual(t, off, time.Duration(0))
	assert.Less(t, off, uptimeSchedulerBackfillWindow, "host first due-time is jittered inside the backfill window")
}

// --- rescan: host schedule adopt (new host) + drop (host lost all children) ---

func TestUptimeScheduler_Rescan_AdoptsNewHostAndDropsChildlessHost(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	sched, _ := newTestScheduler(t, db)

	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	sched.now = func() time.Time { return now }

	h1 := models.UptimeHost{Host: "10.0.0.31", Name: "h31", Status: "up"}
	require.NoError(t, db.Create(&h1).Error)
	m1 := seedMonitor(t, db, models.UptimeMonitor{Name: "m1", Enabled: true, Interval: 60, Type: "tcp", URL: "10.0.0.31:22", UptimeHostID: &h1.ID})

	sched.hydrate(context.Background())
	sched.mu.Lock()
	_, h1Seeded := sched.hostSchedule[h1.ID]
	sched.mu.Unlock()
	require.True(t, h1Seeded)

	// h1 loses its only child; a brand-new host h2 appears with a child.
	require.NoError(t, db.Model(&models.UptimeMonitor{}).Where("id = ?", m1.ID).Update("enabled", false).Error)
	h2 := models.UptimeHost{Host: "10.0.0.32", Name: "h32", Status: "up"}
	require.NoError(t, db.Create(&h2).Error)
	seedMonitor(t, db, models.UptimeMonitor{Name: "m2", Enabled: true, Interval: 60, Type: "tcp", URL: "10.0.0.32:22", UptimeHostID: &h2.ID})

	sched.rescan(context.Background())

	sched.mu.Lock()
	defer sched.mu.Unlock()
	_, stillH1 := sched.hostSchedule[h1.ID]
	_, hasH2 := sched.hostSchedule[h2.ID]
	assert.False(t, stillH1, "rescan drops a host that no longer has enabled child monitors")
	assert.True(t, hasH2, "rescan adopts a newly-appeared host with a jittered due-time")
	assert.Contains(t, sched.hostMinInt, h2.ID)
}

// --- runTick: rescan fires every Nth tick ---

func TestUptimeScheduler_RunTick_RescanFiresEveryNthTick(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	sched, _ := newTestScheduler(t, db)

	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	sched.now = func() time.Time { return now }

	existing := seedMonitor(t, db, models.UptimeMonitor{Name: "existing", Enabled: true, Interval: 60, NextCheckAt: now.Add(time.Hour)})
	sched.hydrate(context.Background())

	fresh := seedMonitor(t, db, models.UptimeMonitor{Name: "fresh", Enabled: true, Interval: 60, NextCheckAt: now.Add(time.Hour)})

	for i := 0; i < uptimeSchedulerRescanEveryTicks; i++ {
		sched.runTick(context.Background())
	}

	sched.mu.Lock()
	defer sched.mu.Unlock()
	_, known := sched.known[fresh.ID]
	assert.True(t, known, "the periodic rescan inside runTick picks up a monitor created after hydration")
	_, stillExisting := sched.known[existing.ID]
	assert.True(t, stillExisting, "an already-known enabled monitor is left untouched by rescan (continue branch)")
}

// --- host/monitor pass: per-tick enqueue cap ---

func TestUptimeScheduler_Passes_CapEnqueuesPerTick(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	sched, _ := newTestScheduler(t, db)

	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	sched.now = func() time.Time { return now }

	overCap := uptimeSchedulerMaxEnqueuePerTick + 25
	hostSched := make(map[string]time.Time, overCap)
	monSched := make(map[string]time.Time, overCap)
	known := make(map[string]struct{}, overCap)
	for i := 0; i < overCap; i++ {
		hid := "h-" + strconv.Itoa(i)
		mid := "m-" + strconv.Itoa(i)
		hostSched[hid] = now.Add(-time.Minute)
		monSched[mid] = now.Add(-time.Minute)
		known[mid] = struct{}{}
	}
	sched.mu.Lock()
	sched.hostSchedule = hostSched
	sched.hostMinInt = map[string]int{}
	sched.monSchedule = monSched
	sched.known = known
	sched.mu.Unlock()

	// No DB rows exist for these synthetic ids, so nothing is actually enqueued;
	// the assertion is only that the cap slice executes without panic and the
	// passes complete. (loadHostSnapshots / loadJobSnapshots return nothing.)
	sched.hostPass(context.Background(), now)
	sched.monitorPass(context.Background(), now)
}

// --- hostDownSkips: nil pool short-circuit ---

func TestUptimeScheduler_HostDownSkips_NilPoolReturnsFalse(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	sched, _ := newTestScheduler(t, db)
	sched.pool = nil

	got := sched.hostDownSkips(models.UptimeMonitor{Type: "tcp", UptimeHostID: ptrString("any")})
	assert.False(t, got, "with no worker pool there is no authoritative host state to skip against")
}

// --- featureEnabled: reads and caches the setting row ---

func TestUptimeScheduler_FeatureEnabled_ReadsAndCachesSettingRow(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	sched, _ := newTestScheduler(t, db)

	base := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	sched.now = func() time.Time { return base }

	require.NoError(t, db.Create(&models.Setting{Key: "feature.uptime.enabled", Value: "false", Type: "bool", Category: "feature"}).Error)
	assert.False(t, sched.featureEnabled(context.Background()), "explicit false setting disables the scheduler")

	// Cached until the TTL elapses even though the row changed.
	require.NoError(t, db.Model(&models.Setting{}).Where("key = ?", "feature.uptime.enabled").Update("value", "true").Error)
	assert.False(t, sched.featureEnabled(context.Background()), "flag lookup is cached for the TTL")

	sched.now = func() time.Time { return base.Add(uptimeSchedulerFlagTTL + time.Second) }
	assert.True(t, sched.featureEnabled(context.Background()), "after the TTL the setting is re-read")
}

// --- Run: feature-disabled tick is a no-op ---

func TestUptimeScheduler_Run_FeatureDisabledTickIsNoop(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	sched, pool := newTestScheduler(t, db)
	sched.tick = 3 * time.Millisecond

	require.NoError(t, db.Create(&models.Setting{Key: "feature.uptime.enabled", Value: "false", Type: "bool", Category: "feature"}).Error)
	mon := seedMonitor(t, db, models.UptimeMonitor{Name: "m", Enabled: true, Interval: 60})

	sched.mu.Lock()
	sched.monSchedule = map[string]time.Time{mon.ID: time.Now().Add(-time.Hour)}
	sched.known = map[string]struct{}{mon.ID: {}}
	sched.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { sched.Run(ctx); close(done) }()
	time.Sleep(60 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return after cancel")
	}

	assert.Empty(t, drainJobs(pool), "a disabled feature flag must skip runTick entirely — nothing enqueued")
}

// --- duration helpers ---

func TestUptimeScheduler_DurationHelpers(t *testing.T) {
	t.Parallel()
	assert.Equal(t, time.Duration(0), jitterDuration(0))
	assert.Equal(t, time.Duration(0), jitterDuration(-5*time.Second))
	for i := 0; i < 50; i++ {
		d := jitterDuration(100 * time.Millisecond)
		assert.GreaterOrEqual(t, d, time.Duration(0))
		assert.Less(t, d, 100*time.Millisecond)
	}
	assert.Equal(t, time.Second, minDuration(time.Second, 2*time.Second))
	assert.Equal(t, time.Second, minDuration(3*time.Second, time.Second))
	assert.Equal(t, 5*time.Second, minDuration(5*time.Second, 5*time.Second))
}

// --- snapshot loaders: empty-id and DB-error branches ---

func TestUptimeScheduler_SnapshotLoaders_EmptyAndErrorBranches(t *testing.T) {
	t.Parallel()
	okDB := setupUptimeTestDB(t)
	schedOK, _ := newTestScheduler(t, okDB)

	assert.Nil(t, schedOK.loadJobSnapshots(context.Background(), nil))
	assert.Nil(t, schedOK.loadHostSnapshots(context.Background(), []string{}))

	badDB := closedUptimeDB(t)
	schedBad, _ := newTestScheduler(t, badDB)
	assert.Nil(t, schedBad.loadJobSnapshots(context.Background(), []string{"x"}), "a query error yields nil, not a partial slice")
	assert.Nil(t, schedBad.loadHostSnapshots(context.Background(), []string{"x"}))
}

// --- DB-error resilience: hydrate / loadHostSchedules / rescan / flushWriteback / Run / Rehydrate ---

func TestUptimeScheduler_DBErrors_AreLoggedAndSurvived(t *testing.T) {
	t.Parallel()
	db := closedUptimeDB(t)
	sched, _ := newTestScheduler(t, db)
	sched.now = func() time.Time { return time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC) }

	// hydrate: both the monitor query and the host query fail; must not panic.
	sched.hydrate(context.Background())
	sched.mu.Lock()
	assert.Empty(t, sched.monSchedule)
	sched.mu.Unlock()

	// loadHostSchedules directly: returns empty maps on query error.
	hmi, hs := sched.loadHostSchedules(context.Background(), time.Now())
	assert.Empty(t, hmi)
	assert.Empty(t, hs)

	// rescan: query error path.
	assert.NotPanics(t, func() { sched.rescan(context.Background()) })

	// flushWriteback: transaction fails -> staged entries are re-queued.
	staged := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	sched.mu.Lock()
	sched.writeback = map[string]time.Time{"m-x": staged}
	sched.mu.Unlock()
	sched.flushWriteback(context.Background())
	sched.mu.Lock()
	got, ok := sched.writeback["m-x"]
	sched.mu.Unlock()
	require.True(t, ok, "a failed write-back flush re-stages its entries for the next tick")
	assert.Equal(t, staged, got)

	// Run: hydrate + pool SeedState both fail, loop still starts and stops cleanly.
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { sched.Run(ctx); close(done) }()
	time.Sleep(30 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return after cancel despite DB errors")
	}

	// Rehydrate: hydrate + pool ReseedState both fail; must not panic.
	assert.NotPanics(t, func() { sched.Rehydrate(context.Background()) })
}
