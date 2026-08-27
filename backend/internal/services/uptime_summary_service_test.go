package services

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// smMonitor inserts a monitor with an explicit status/last-check so the
// no-heartbeat fallback path can be asserted.
func smMonitor(t *testing.T, db *gorm.DB, id, name, status string) models.UptimeMonitor {
	t.Helper()
	m := models.UptimeMonitor{
		ID:        id,
		Name:      name,
		Type:      "http",
		URL:       "https://" + id + ".example.com",
		Enabled:   true,
		Interval:  30,
		Status:    status,
		Latency:   42,
		LastCheck: time.Now().Add(-30 * time.Second),
	}
	require.NoError(t, db.Create(&m).Error)
	return m
}

func smBeat(t *testing.T, db *gorm.DB, monitorID, status string, latency int64, at time.Time) {
	t.Helper()
	require.NoError(t, db.Create(&models.UptimeHeartbeat{
		MonitorID: monitorID,
		Status:    status,
		Latency:   latency,
		CreatedAt: at,
	}).Error)
}

func smIndex(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Exec(
		"CREATE INDEX IF NOT EXISTS idx_heartbeat_monitor_created ON uptime_heartbeats (monitor_id, created_at)",
	).Error)
}

func TestUptimeSummary_WindowedBeats_LimitAndChronologicalOrder(t *testing.T) {
	db := setupUptimeTestDB(t)
	svc := NewUptimeSummaryService(db)

	m := smMonitor(t, db, "mon-order", "Order Monitor", "up")
	base := time.Now().Add(-1 * time.Hour)
	for i := 0; i < 10; i++ {
		smBeat(t, db, m.ID, "up", int64(i), base.Add(time.Duration(i)*time.Minute))
	}

	res, err := svc.GetSummary(context.Background(), 3)
	require.NoError(t, err)
	require.Len(t, res, 1)

	beats := res[0].RecentBeats
	require.Len(t, beats, 3, "must return at most beats rows per monitor")

	// The three most recent (latencies 7,8,9), emitted oldest-first.
	assert.Equal(t, int64(7), beats[0].Latency)
	assert.Equal(t, int64(9), beats[2].Latency)
	assert.True(t, beats[0].CreatedAt.Before(beats[1].CreatedAt))
	assert.True(t, beats[1].CreatedAt.Before(beats[2].CreatedAt))
}

func TestUptimeSummary_Uptime24hMath(t *testing.T) {
	db := setupUptimeTestDB(t)
	svc := NewUptimeSummaryService(db)

	m := smMonitor(t, db, "mon-pct", "Pct Monitor", "up")
	base := time.Now().Add(-2 * time.Hour)
	smBeat(t, db, m.ID, "up", 1, base)
	smBeat(t, db, m.ID, "up", 1, base.Add(1*time.Minute))
	smBeat(t, db, m.ID, "up", 1, base.Add(2*time.Minute))
	smBeat(t, db, m.ID, "down", 0, base.Add(3*time.Minute))

	res, err := svc.GetSummary(context.Background(), 30)
	require.NoError(t, err)
	require.Len(t, res, 1)
	require.NotNil(t, res[0].Uptime24h)
	assert.InDelta(t, 75.0, *res[0].Uptime24h, 0.0001)
}

func TestUptimeSummary_EmptyHistoryMonitor(t *testing.T) {
	db := setupUptimeTestDB(t)
	svc := NewUptimeSummaryService(db)

	smMonitor(t, db, "mon-empty", "Empty Monitor", "down")

	res, err := svc.GetSummary(context.Background(), 30)
	require.NoError(t, err)
	require.Len(t, res, 1)

	assert.Nil(t, res[0].Uptime24h, "no in-window data -> uptime_24h is null")
	assert.NotNil(t, res[0].RecentBeats, "recent_beats must be [] not null")
	assert.Len(t, res[0].RecentBeats, 0)
	assert.Equal(t, "down", res[0].Status, "status falls back to the monitor row")
}

func TestUptimeSummary_BeatsClamp(t *testing.T) {
	db := setupUptimeTestDB(t)

	m := smMonitor(t, db, "mon-clamp", "Clamp Monitor", "up")
	base := time.Now().Add(-3 * time.Hour)
	for i := 0; i < 70; i++ {
		smBeat(t, db, m.ID, "up", int64(i), base.Add(time.Duration(i)*time.Minute))
	}

	t.Run("above cap clamps to 60", func(t *testing.T) {
		svc := NewUptimeSummaryService(db)
		res, err := svc.GetSummary(context.Background(), 999)
		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Len(t, res[0].RecentBeats, 60)
	})

	t.Run("below floor clamps to 1", func(t *testing.T) {
		svc := NewUptimeSummaryService(db)
		res, err := svc.GetSummary(context.Background(), 0)
		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Len(t, res[0].RecentBeats, 1)
	})
}

func TestUptimeSummary_CacheHitSkipsQuery(t *testing.T) {
	db := setupUptimeTestDB(t)
	svc := NewUptimeSummaryService(db)

	m := smMonitor(t, db, "mon-cache", "Cache Monitor", "up")
	smBeat(t, db, m.ID, "up", 1, time.Now().Add(-1*time.Minute))

	var queries atomic.Int64
	const cbName = "test:summary_query_counter"
	require.NoError(t, db.Callback().Query().After("gorm:query").Register(cbName, func(*gorm.DB) {
		queries.Add(1)
	}))
	t.Cleanup(func() { _ = db.Callback().Query().Remove(cbName) })

	_, err := svc.GetSummary(context.Background(), 30)
	require.NoError(t, err)
	afterFirst := queries.Load()
	require.Greater(t, afterFirst, int64(0), "first call must hit the DB")

	_, err = svc.GetSummary(context.Background(), 30)
	require.NoError(t, err)
	assert.Equal(t, afterFirst, queries.Load(), "second call within TTL must be served from cache")
}

func TestUptimeSummary_ExcludesBeatsOlderThan24h(t *testing.T) {
	db := setupUptimeTestDB(t)
	svc := NewUptimeSummaryService(db)
	fixed := time.Now()
	svc.now = func() time.Time { return fixed }

	m := smMonitor(t, db, "mon-window", "Window Monitor", "up")
	smBeat(t, db, m.ID, "down", 0, fixed.Add(-25*time.Hour)) // outside window
	smBeat(t, db, m.ID, "up", 5, fixed.Add(-1*time.Hour))    // inside window

	res, err := svc.GetSummary(context.Background(), 30)
	require.NoError(t, err)
	require.Len(t, res, 1)

	require.Len(t, res[0].RecentBeats, 1, "only the in-window beat is returned")
	assert.Equal(t, "up", res[0].RecentBeats[0].Status)
	require.NotNil(t, res[0].Uptime24h)
	assert.InDelta(t, 100.0, *res[0].Uptime24h, 0.0001, "24h math only counts in-window beats")
}

func TestUptimeSummary_OrderedByNameASC(t *testing.T) {
	db := setupUptimeTestDB(t)
	svc := NewUptimeSummaryService(db)

	smMonitor(t, db, "mon-z", "Zeta", "up")
	smMonitor(t, db, "mon-a", "Alpha", "up")
	smMonitor(t, db, "mon-m", "Mike", "up")

	res, err := svc.GetSummary(context.Background(), 30)
	require.NoError(t, err)
	require.Len(t, res, 3)
	assert.Equal(t, "Alpha", res[0].Name)
	assert.Equal(t, "Mike", res[1].Name)
	assert.Equal(t, "Zeta", res[2].Name)
}

// TestUptimeSummary_CorrectWithAndWithoutIndex proves the windowed query is
// correct whether or not the pruner's deferred idx_heartbeat_monitor_created
// exists yet (spec §3.5.6 — never 503-gated on index absence).
func TestUptimeSummary_CorrectWithAndWithoutIndex(t *testing.T) {
	assertResult := func(t *testing.T, res []MonitorSummary) {
		t.Helper()
		require.Len(t, res, 1)
		assert.Equal(t, "up", res[0].Status)
		require.Len(t, res[0].RecentBeats, 3)
		assert.Equal(t, "down", res[0].RecentBeats[2].Status)
		require.NotNil(t, res[0].Uptime24h)
		assert.InDelta(t, 66.6667, *res[0].Uptime24h, 0.01)
	}

	seed := func(t *testing.T, db *gorm.DB) {
		m := smMonitor(t, db, "mon-idx", "Index Monitor", "up")
		base := time.Now().Add(-90 * time.Minute)
		smBeat(t, db, m.ID, "up", 1, base)
		smBeat(t, db, m.ID, "up", 2, base.Add(1*time.Minute))
		smBeat(t, db, m.ID, "down", 0, base.Add(2*time.Minute))
	}

	t.Run("without idx_heartbeat_monitor_created", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		seed(t, db)
		res, err := NewUptimeSummaryService(db).GetSummary(context.Background(), 30)
		require.NoError(t, err)
		assertResult(t, res)
	})

	t.Run("with idx_heartbeat_monitor_created", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		seed(t, db)
		smIndex(t, db)
		res, err := NewUptimeSummaryService(db).GetSummary(context.Background(), 30)
		require.NoError(t, err)
		assertResult(t, res)
	})
}

// TestUptimeSummary_PerfBudget is the S5 gate: with the index built, a batch
// summary over a large-ish heartbeat table must complete well under a
// CI-stable 2s ceiling. Seed note: 500 monitors x 120 beats over 24h = 60k
// rows. This is a lean subset of the §3.5.3 production profile (500 x ~720 =
// ~360k). The <2s ceiling is a regression guard (it trips instantly if the
// 3-query strategy regresses to a per-monitor loop); the <300ms p95 at the
// full 360k profile is tracked in the QA run, not gated here (runner variance).
func TestUptimeSummary_PerfBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("perf budget seed is too heavy for -short")
	}

	db := setupUptimeTestDB(t)

	const (
		monitorCount   = 500
		beatsPerMon    = 120
		spacingMinutes = 12 // 120 * 12min = 24h
	)

	monitors := make([]models.UptimeMonitor, 0, monitorCount)
	now := time.Now()
	for i := 0; i < monitorCount; i++ {
		id := fmt.Sprintf("perf-mon-%03d", i)
		monitors = append(monitors, models.UptimeMonitor{
			ID:        id,
			Name:      fmt.Sprintf("Perf Monitor %03d", i),
			Type:      "http",
			URL:       "https://" + id + ".example.com",
			Enabled:   true,
			Interval:  30,
			Status:    "up",
			Latency:   50,
			LastCheck: now.Add(-30 * time.Second),
		})
	}
	require.NoError(t, db.CreateInBatches(&monitors, 200).Error)

	beats := make([]models.UptimeHeartbeat, 0, monitorCount*beatsPerMon)
	base := now.Add(-24 * time.Hour)
	for i := 0; i < monitorCount; i++ {
		mid := monitors[i].ID
		for j := 0; j < beatsPerMon; j++ {
			status := "up"
			if j%10 == 0 {
				status = "down"
			}
			beats = append(beats, models.UptimeHeartbeat{
				MonitorID: mid,
				Status:    status,
				Latency:   int64(40 + j%20),
				CreatedAt: base.Add(time.Duration(j*spacingMinutes) * time.Minute),
			})
		}
	}
	require.NoError(t, db.CreateInBatches(&beats, 2000).Error)

	smIndex(t, db)

	svc := NewUptimeSummaryService(db)
	start := time.Now()
	res, err := svc.GetSummary(context.Background(), 30)
	elapsed := time.Since(start)
	require.NoError(t, err)
	require.Len(t, res, monitorCount)

	t.Logf("GetSummary(%d monitors, %d total beats, beats=30) took %s",
		monitorCount, len(beats), elapsed)
	require.Less(t, elapsed, 2*time.Second, "S5 perf budget: batch summary must complete under 2s")
}

func TestUptimeSummary_ErrorPaths(t *testing.T) {
	t.Run("monitor metadata query fails", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		smMonitor(t, db, "mon-e1", "E1", "up")
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())

		_, err = NewUptimeSummaryService(db).GetSummary(context.Background(), 30)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "load monitors")
	})

	t.Run("recent-beats window query fails", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		smMonitor(t, db, "mon-e2", "E2", "up")

		// Raw .Scan() executes through the gorm:row processor (verified), and
		// tx.Statement.SQL is populated there.
		const cb = "test:fail_recent_beats"
		require.NoError(t, db.Callback().Row().Before("gorm:row").Register(cb, func(tx *gorm.DB) {
			if strings.Contains(tx.Statement.SQL.String(), "ROW_NUMBER()") {
				_ = tx.AddError(errRecentBeatsInjected)
			}
		}))
		t.Cleanup(func() { _ = db.Callback().Row().Remove(cb) })

		_, err := NewUptimeSummaryService(db).GetSummary(context.Background(), 30)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "load recent beats")
	})

	t.Run("24h uptime query fails", func(t *testing.T) {
		db := setupUptimeTestDB(t)
		smMonitor(t, db, "mon-e3", "E3", "up")

		const cb = "test:fail_uptime24h"
		require.NoError(t, db.Callback().Row().Before("gorm:row").Register(cb, func(tx *gorm.DB) {
			if strings.Contains(tx.Statement.SQL.String(), "GROUP BY monitor_id") {
				_ = tx.AddError(errUptime24hInjected)
			}
		}))
		t.Cleanup(func() { _ = db.Callback().Row().Remove(cb) })

		_, err := NewUptimeSummaryService(db).GetSummary(context.Background(), 30)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "load 24h uptime")
	})
}

var (
	errRecentBeatsInjected = injectedErr("injected: recent beats")
	errUptime24hInjected   = injectedErr("injected: 24h uptime")
)

type injectedErr string

func (e injectedErr) Error() string { return string(e) }
