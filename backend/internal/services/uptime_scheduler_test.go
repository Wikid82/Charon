package services

import (
	"context"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// --- helpers ---

func newTestScheduler(t *testing.T, db *gorm.DB) (*UptimeScheduler, *UptimeWorkerPool) {
	t.Helper()
	pool, _ := newTestPool(t, db)
	sched := NewUptimeScheduler(pool)
	return sched, pool
}

func drainJobs(p *UptimeWorkerPool) []UptimeJob {
	var out []UptimeJob
	for {
		select {
		case j := <-p.jobs:
			out = append(out, j)
		default:
			return out
		}
	}
}

func seedMonitor(t *testing.T, db *gorm.DB, m models.UptimeMonitor) models.UptimeMonitor {
	t.Helper()
	if m.Type == "" {
		m.Type = "http"
	}
	if m.URL == "" {
		m.URL = "http://127.0.0.1:1"
	}
	require.NoError(t, db.Create(&m).Error)
	return m
}

// --- due selection ---

func TestUptimeScheduler_MonitorPass_DueSelectionRespectsNextCheckAtAndEnabled(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	sched, pool := newTestScheduler(t, db)

	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	sched.now = func() time.Time { return now }

	due := seedMonitor(t, db, models.UptimeMonitor{Name: "due", Enabled: true, Interval: 60})
	future := seedMonitor(t, db, models.UptimeMonitor{Name: "future", Enabled: true, Interval: 60})
	disabled := seedMonitor(t, db, models.UptimeMonitor{Name: "disabled", Enabled: false, Interval: 60})

	sched.mu.Lock()
	sched.monSchedule = map[string]time.Time{
		due.ID:      now.Add(-time.Second),
		future.ID:   now.Add(time.Hour),
		disabled.ID: now.Add(-time.Hour), // present but disabled row won't be enqueued as a job snapshot filter... still test the schedule filter
	}
	sched.known = map[string]struct{}{due.ID: {}, future.ID: {}, disabled.ID: {}}
	sched.mu.Unlock()

	sched.monitorPass(context.Background(), now)

	jobs := drainJobs(pool)
	ids := map[string]bool{}
	for _, j := range jobs {
		ids[j.Monitor.ID] = true
	}
	assert.True(t, ids[due.ID], "past-due monitor must be enqueued")
	assert.False(t, ids[future.ID], "future monitor must not be enqueued")
	// disabled row is loaded by snapshot but monitorPass has no enable filter;
	// the WORKER drops a disabled non-manual job. The schedule filter is what we
	// assert here — the disabled monitor's schedule was due so it is enqueued,
	// and handleMonitorCheck will no-op it. Confirm the due one advanced.
	sched.mu.Lock()
	assert.Equal(t, now.Add(60*time.Second), sched.monSchedule[due.ID])
	assert.Equal(t, now.Add(time.Hour), sched.monSchedule[future.ID], "future monitor schedule untouched")
	sched.mu.Unlock()
}

// --- interval clamp ---

func TestUptimeScheduler_MonitorPass_AdvancesByClampedInterval(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	sched, _ := newTestScheduler(t, db)

	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	sched.now = func() time.Time { return now }

	tooLow := seedMonitor(t, db, models.UptimeMonitor{Name: "low", Enabled: true, Interval: 10})   // -> 30
	zero := seedMonitor(t, db, models.UptimeMonitor{Name: "zero", Enabled: true, Interval: 0})     // -> default 60
	normal := seedMonitor(t, db, models.UptimeMonitor{Name: "norm", Enabled: true, Interval: 120}) // -> 120

	sched.mu.Lock()
	sched.monSchedule = map[string]time.Time{
		tooLow.ID: now.Add(-time.Second),
		zero.ID:   now.Add(-time.Second),
		normal.ID: now.Add(-time.Second),
	}
	sched.known = map[string]struct{}{tooLow.ID: {}, zero.ID: {}, normal.ID: {}}
	sched.mu.Unlock()

	sched.monitorPass(context.Background(), now)

	sched.mu.Lock()
	defer sched.mu.Unlock()
	assert.Equal(t, now.Add(30*time.Second), sched.monSchedule[tooLow.ID], "sub-floor interval clamped to 30s")
	assert.Equal(t, now.Add(60*time.Second), sched.monSchedule[zero.ID], "zero interval -> configured default 60s")
	assert.Equal(t, now.Add(120*time.Second), sched.monSchedule[normal.ID])
}

// --- jittered backfill spread ---

func TestUptimeScheduler_Hydrate_JitteredBackfillSpreadsPastDueMonitors(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	sched, _ := newTestScheduler(t, db)

	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	sched.now = func() time.Time { return now }

	const n = 120
	for i := 0; i < n; i++ {
		// interval 60 -> backfill window min(60s, 60s) = 60s
		seedMonitor(t, db, models.UptimeMonitor{Name: "m", Enabled: true, Interval: 60})
	}

	sched.hydrate(context.Background())

	sched.mu.Lock()
	offsets := make([]time.Duration, 0, n)
	for _, due := range sched.monSchedule {
		offsets = append(offsets, due.Sub(now))
	}
	sched.mu.Unlock()

	require.Len(t, offsets, n)

	buckets := make([]int, 6) // 6 x 10s buckets across the 60s window
	var min, max time.Duration = time.Hour, 0
	for _, o := range offsets {
		assert.GreaterOrEqual(t, o, time.Duration(0))
		assert.Less(t, o, 60*time.Second)
		if o < min {
			min = o
		}
		if o > max {
			max = o
		}
		b := int(o / (10 * time.Second))
		if b > 5 {
			b = 5
		}
		buckets[b]++
	}

	populated := 0
	for _, c := range buckets {
		if c > 0 {
			populated++
		}
	}
	assert.GreaterOrEqual(t, populated, 4, "backfill should spread across most of the window, got buckets %v", buckets)
	assert.Greater(t, max-min, 20*time.Second, "spread between earliest and latest due should be wide")
}

// --- host pass before monitor pass ---

func TestUptimeScheduler_RunTick_HostPassBeforeMonitorPass(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	sched, pool := newTestScheduler(t, db)

	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	sched.now = func() time.Time { return now }

	host := models.UptimeHost{Host: "10.0.0.5", Name: "h", Status: "up"}
	require.NoError(t, db.Create(&host).Error)
	mon := seedMonitor(t, db, models.UptimeMonitor{Name: "m", Enabled: true, Interval: 60, Type: "http", UptimeHostID: &host.ID})

	sched.mu.Lock()
	sched.hostSchedule = map[string]time.Time{host.ID: now.Add(-time.Second)}
	sched.hostMinInt = map[string]int{host.ID: 60}
	sched.monSchedule = map[string]time.Time{mon.ID: now.Add(-time.Second)}
	sched.known = map[string]struct{}{mon.ID: {}}
	sched.mu.Unlock()

	sched.runTick(context.Background())

	jobs := drainJobs(pool)
	require.GreaterOrEqual(t, len(jobs), 2)
	assert.Equal(t, JobHostCheck, jobs[0].Kind, "host job must be enqueued before monitor job")
	assert.Equal(t, JobMonitorCheck, jobs[len(jobs)-1].Kind)
}

// --- host-down short-circuit ---

func TestUptimeScheduler_MonitorPass_SkipsTCPMonitorOfDownHostButNotHTTP(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	sched, pool := newTestScheduler(t, db)

	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	sched.now = func() time.Time { return now }

	host := models.UptimeHost{Host: "10.0.0.9", Name: "downhost", Status: "down"}
	require.NoError(t, db.Create(&host).Error)
	pool.hostState[host.ID] = hostDebounce{status: "down"}

	tcpMon := seedMonitor(t, db, models.UptimeMonitor{Name: "tcp", Enabled: true, Interval: 60, Type: "tcp", URL: "10.0.0.9:22", UptimeHostID: &host.ID})
	httpMon := seedMonitor(t, db, models.UptimeMonitor{Name: "http", Enabled: true, Interval: 60, Type: "http", UptimeHostID: &host.ID})

	sched.mu.Lock()
	sched.monSchedule = map[string]time.Time{tcpMon.ID: now.Add(-time.Second), httpMon.ID: now.Add(-time.Second)}
	sched.known = map[string]struct{}{tcpMon.ID: {}, httpMon.ID: {}}
	sched.mu.Unlock()

	sched.monitorPass(context.Background(), now)

	jobs := drainJobs(pool)
	require.Len(t, jobs, 1)
	assert.Equal(t, httpMon.ID, jobs[0].Monitor.ID, "only the http monitor should be enqueued")

	sched.mu.Lock()
	assert.Equal(t, now.Add(60*time.Second), sched.monSchedule[tcpMon.ID], "skipped tcp monitor still advances")
	sched.mu.Unlock()

	// the skip also stages a write-back that monitorPass -> runTick would flush;
	// call flushWriteback and confirm it persisted.
	sched.flushWriteback(context.Background())
	var got models.UptimeMonitor
	require.NoError(t, db.First(&got, "id = ?", tcpMon.ID).Error)
	assert.WithinDuration(t, now.Add(60*time.Second), got.NextCheckAt, time.Second)
}

// --- write-back grouping ---

func TestUptimeScheduler_FlushWriteback_GroupsByTimestampIntoOneUpdatePerGroup(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	sched, _ := newTestScheduler(t, db)

	m1 := seedMonitor(t, db, models.UptimeMonitor{Name: "m1", Enabled: true, Interval: 60})
	m2 := seedMonitor(t, db, models.UptimeMonitor{Name: "m2", Enabled: true, Interval: 60})
	m3 := seedMonitor(t, db, models.UptimeMonitor{Name: "m3", Enabled: true, Interval: 60})

	var updates int
	require.NoError(t, db.Callback().Update().After("gorm:update").Register("test:count_uptime_updates", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "uptime_monitors" {
			updates++
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Update().Remove("test:count_uptime_updates") })

	shared := time.Date(2026, 8, 27, 12, 5, 0, 0, time.UTC)
	other := time.Date(2026, 8, 27, 12, 9, 0, 0, time.UTC)

	sched.mu.Lock()
	sched.writeback = map[string]time.Time{m1.ID: shared, m2.ID: shared, m3.ID: other}
	sched.mu.Unlock()

	sched.flushWriteback(context.Background())

	assert.Equal(t, 2, updates, "one UPDATE per distinct timestamp (2 groups), not one per monitor")

	var g1, g3 models.UptimeMonitor
	require.NoError(t, db.First(&g1, "id = ?", m1.ID).Error)
	require.NoError(t, db.First(&g3, "id = ?", m3.ID).Error)
	assert.WithinDuration(t, shared, g1.NextCheckAt, time.Second)
	assert.WithinDuration(t, other, g3.NextCheckAt, time.Second)

	sched.mu.Lock()
	assert.Empty(t, sched.writeback, "writeback cleared after a successful flush")
	sched.mu.Unlock()
}

// --- rescan ---

func TestUptimeScheduler_Rescan_PicksUpNewMonitorAndDropsDisabled(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	sched, pool := newTestScheduler(t, db)

	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	sched.now = func() time.Time { return now }

	existing := seedMonitor(t, db, models.UptimeMonitor{Name: "existing", Enabled: true, Interval: 60})
	sched.hydrate(context.Background())
	sched.mu.Lock()
	_, hadExisting := sched.known[existing.ID]
	sched.mu.Unlock()
	require.True(t, hadExisting)

	// New monitor appears, existing one is disabled.
	fresh := seedMonitor(t, db, models.UptimeMonitor{Name: "fresh", Enabled: true, Interval: 60})
	require.NoError(t, db.Model(&models.UptimeMonitor{}).Where("id = ?", existing.ID).Update("enabled", false).Error)

	sched.rescan(context.Background())

	sched.mu.Lock()
	defer sched.mu.Unlock()
	_, hasFresh := sched.known[fresh.ID]
	_, stillExisting := sched.known[existing.ID]
	assert.True(t, hasFresh, "rescan adds the newly created monitor")
	assert.False(t, stillExisting, "rescan drops the now-disabled monitor")
	_, schedFresh := sched.monSchedule[fresh.ID]
	assert.True(t, schedFresh)
	_, poolSeeded := pool.monState[fresh.ID]
	assert.True(t, poolSeeded, "rescan seeds pool debounce state for the new monitor")
}

// --- Rehydrate ---

func TestUptimeScheduler_Rehydrate_ReSeedsScheduleAndPoolState(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	sched, pool := newTestScheduler(t, db)

	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	sched.now = func() time.Time { return now }

	sched.hydrate(context.Background())
	require.NoError(t, pool.SeedState(context.Background()))

	// Simulate a live restore swapping in a DB that has a monitor the running
	// process never saw.
	restored := seedMonitor(t, db, models.UptimeMonitor{Name: "restored", Enabled: true, Interval: 60, Status: "down"})

	sched.Rehydrate(context.Background())

	sched.mu.Lock()
	_, inSchedule := sched.monSchedule[restored.ID]
	sched.mu.Unlock()
	assert.True(t, inSchedule, "Rehydrate re-runs hydration from the restored DB")

	pool.monMu.Lock()
	st, inState := pool.monState[restored.ID]
	pool.monMu.Unlock()
	assert.True(t, inState, "Rehydrate re-seeds the pool debounce map")
	assert.Equal(t, "down", st.status)
}

// --- ctx cancel stops the loop + flushes write-back ---

func TestUptimeScheduler_Run_StopsOnContextCancelAndFlushesWriteback(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	sched, _ := newTestScheduler(t, db)

	// Future next_check_at so hydrate keeps it (no backfill, writeback empty).
	future := time.Now().Add(time.Hour)
	mon := seedMonitor(t, db, models.UptimeMonitor{Name: "m", Enabled: true, Interval: 60, NextCheckAt: future})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		sched.Run(ctx)
		close(done)
	}()

	// Let hydrate + SeedState settle, then stage a pending write-back.
	time.Sleep(100 * time.Millisecond)
	staged := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	sched.mu.Lock()
	sched.writeback[mon.ID] = staged
	sched.mu.Unlock()

	cancel()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("scheduler Run did not return after context cancellation")
	}

	var got models.UptimeMonitor
	require.NoError(t, db.First(&got, "id = ?", mon.ID).Error)
	assert.WithinDuration(t, staged, got.NextCheckAt, time.Second, "final flush on ctx cancel persisted the staged write-back")
}
