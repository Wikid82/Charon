package services

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/database"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Legacy summary SQL, kept only as a test oracle (GH #1441): the ROW_NUMBER()
// window query and the grouped uptime query that scanned every retained
// heartbeat. The inner select also exposes id so the tie-broken variant can
// order on it.
const (
	legacyRecentBeatsSQL = `
SELECT monitor_id, status, latency, created_at
FROM (
  SELECT id, monitor_id, status, latency, created_at,
         ROW_NUMBER() OVER (PARTITION BY monitor_id ORDER BY created_at DESC) AS rn
  FROM uptime_heartbeats
  WHERE created_at >= ?
)
WHERE rn <= ?
ORDER BY monitor_id, created_at ASC`

	legacyUptime24hSQL = `
SELECT monitor_id,
       SUM(CASE WHEN status = 'up' THEN 1 ELSE 0 END) * 100.0 / COUNT(*) AS pct
FROM uptime_heartbeats
WHERE created_at >= ?
GROUP BY monitor_id`
)

// legacyUptimeRow is the row shape of legacyUptime24hSQL.
type legacyUptimeRow struct {
	MonitorID string
	Pct       float64
}

// equivalenceNow is the fixed clock for the equivalence fixtures.
var equivalenceNow = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

// seedEquivalenceFixture fills db with monitors and heartbeats that exercise
// every branch of the summary: more in-window beats than the 60 cap, a monitor
// with only stale rows, a monitor with no rows, and orphan rows of a deleted
// monitor. When withTies is true, every fifth timestamp is shared by a second
// heartbeat with a different status and latency; otherwise every timestamp is
// unique across the whole table.
func seedEquivalenceFixture(t *testing.T, db *gorm.DB, withTies bool) {
	t.Helper()

	monitorIDs := []string{"m-a", "m-b", "m-c", "m-d", "m-e", "m-stale", "m-empty"}
	for i, id := range monitorIDs {
		smMonitor(t, db, id, fmt.Sprintf("Monitor %02d", i), "up")
	}

	statuses := []string{"up", "up", "up", "down", "pending"}
	var beats []models.UptimeHeartbeat
	offset := 0 // distinct sub-second offset keeps tie-free timestamps unique table-wide
	add := func(monitorID string, at time.Time, status string) {
		beats = append(beats, models.UptimeHeartbeat{
			MonitorID: monitorID,
			Status:    status,
			Latency:   int64(len(beats) * 37 % 500),
			CreatedAt: at,
		})
	}

	for _, id := range []string{"m-a", "m-b", "m-c", "m-d", "m-e", "ghost"} {
		for j := 0; j < 130; j++ {
			offset++
			at := equivalenceNow.Add(-time.Duration(j)*10*time.Minute - time.Duration(offset)*time.Millisecond)
			if withTies && j%5 == 0 {
				at = equivalenceNow.Add(-time.Duration(j) * 10 * time.Minute)
				add(id, at, "up")
				add(id, at, "down")
				continue
			}
			add(id, at, statuses[len(beats)*7%len(statuses)])
		}
	}
	// Rows older than the 24h window must be ignored by both queries.
	for j := 0; j < 20; j++ {
		offset++
		add("m-stale", equivalenceNow.Add(-30*time.Hour-time.Duration(j)*time.Minute-time.Duration(offset)*time.Millisecond), "up")
	}
	for j := 0; j < 20; j++ {
		offset++
		add("m-a", equivalenceNow.Add(-48*time.Hour-time.Duration(j)*time.Minute-time.Duration(offset)*time.Millisecond), "down")
	}

	require.NoError(t, db.CreateInBatches(&beats, 200).Error)
}

// oracleSummary assembles the summary exactly like GetSummary but runs the
// given legacy beats SQL and the legacy uptime SQL.
func oracleSummary(t *testing.T, svc *UptimeSummaryService, beatsSQL string) []MonitorSummary {
	t.Helper()
	ctx := context.Background()
	windowStart := svc.now().Add(-uptimeSummaryWindow)

	monitors, err := svc.loadMonitors(ctx)
	require.NoError(t, err)

	var beatRows []recentBeatRow
	require.NoError(t, svc.db.WithContext(ctx).Raw(beatsSQL, windowStart, uptimeSummaryMaxBeats).Scan(&beatRows).Error)
	beats := make(map[string][]BeatDTO)
	for _, r := range beatRows {
		beats[r.MonitorID] = append(beats[r.MonitorID], BeatDTO{Status: r.Status, Latency: r.Latency, CreatedAt: r.CreatedAt})
	}

	var uptimeRows []legacyUptimeRow
	require.NoError(t, svc.db.WithContext(ctx).Raw(legacyUptime24hSQL, windowStart).Scan(&uptimeRows).Error)
	uptime := make(map[string]float64)
	for _, r := range uptimeRows {
		uptime[r.MonitorID] = r.Pct
	}

	return assembleSummaries(monitors, beats, uptime)
}

// requireSameSummaryJSON compares the marshalled API payload, i.e. the wire
// format clients see, byte for byte.
func requireSameSummaryJSON(t *testing.T, want, got []MonitorSummary) {
	t.Helper()
	wantJSON, err := json.Marshal(want)
	require.NoError(t, err)
	gotJSON, err := json.Marshal(got)
	require.NoError(t, err)
	require.Equal(t, string(wantJSON), string(gotJSON))
}

// forEachIndexState runs fn against a freshly seeded database in both index
// states: without and with the pruner's deferred idx_heartbeat_monitor_created.
func forEachIndexState(t *testing.T, withTies bool, fn func(t *testing.T, svc *UptimeSummaryService)) {
	t.Helper()
	for _, withIndex := range []bool{false, true} {
		name := "without idx_heartbeat_monitor_created"
		if withIndex {
			name = "with idx_heartbeat_monitor_created"
		}
		t.Run(name, func(t *testing.T) {
			db := setupUptimeTestDB(t)
			seedEquivalenceFixture(t, db, withTies)
			if withIndex {
				smIndex(t, db)
			}
			svc := NewUptimeSummaryService(db)
			svc.now = func() time.Time { return equivalenceNow }
			fn(t, svc)
		})
	}
}

// On a fixture without duplicate timestamps the untouched legacy SQL has a
// unique answer, so GetSummary must serialise to exactly the same JSON.
func TestUptimeSummary_MatchesLegacySQL_TieFree(t *testing.T) {
	forEachIndexState(t, false, func(t *testing.T, svc *UptimeSummaryService) {
		want := oracleSummary(t, svc, legacyRecentBeatsSQL)
		for _, beats := range []int{1, 30, 60} {
			t.Run(fmt.Sprintf("beats=%d", beats), func(t *testing.T) {
				got, err := svc.GetSummary(context.Background(), beats)
				require.NoError(t, err)
				requireSameSummaryJSON(t, sliceSummaries(want, beats), got)
			})
		}
	})
}

// BenchmarkUptimeSummaryCold measures a cache-miss GetSummary on a database
// opened through database.Connect (production pragmas, single connection):
// 50 monitors x 2 days of one-minute heartbeats (144,000 rows), with and
// without the pruner's deferred composite index. Scale the constants to
// reproduce docs/performance/database.md.
func BenchmarkUptimeSummaryCold(b *testing.B) {
	const (
		monitorCount = 50
		days         = 2
	)
	for _, withIndex := range []bool{false, true} {
		name := "no_index"
		if withIndex {
			name = "composite_index"
		}
		b.Run(name, func(b *testing.B) {
			db := seedColdSummaryDB(b, monitorCount, days)
			if withIndex {
				require.NoError(b, db.Exec(
					"CREATE INDEX IF NOT EXISTS idx_heartbeat_monitor_created ON uptime_heartbeats (monitor_id, created_at)",
				).Error)
			}
			ctx := context.Background()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// A new service per iteration guarantees a cache miss.
				res, err := NewUptimeSummaryService(db).GetSummary(ctx, uptimeSummaryMaxBeats)
				if err != nil || len(res) != monitorCount {
					b.Fatalf("GetSummary: %d monitors, err %v", len(res), err)
				}
			}
		})
	}
}

// seedColdSummaryDB builds a production-configured database holding
// monitorCount monitors with days of one-minute heartbeats ending now.
func seedColdSummaryDB(b *testing.B, monitorCount, days int) *gorm.DB {
	b.Helper()
	db, err := database.Connect(filepath.Join(b.TempDir(), "bench.db"))
	require.NoError(b, err)
	require.NoError(b, db.AutoMigrate(&models.UptimeMonitor{}, &models.UptimeHeartbeat{}))
	b.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	now := time.Now()
	perMonitor := days * 24 * 60
	monitors := make([]models.UptimeMonitor, 0, monitorCount)
	beats := make([]models.UptimeHeartbeat, 0, monitorCount*perMonitor)
	for i := 0; i < monitorCount; i++ {
		id := fmt.Sprintf("00000000-0000-4000-8000-%012d", i)
		monitors = append(monitors, models.UptimeMonitor{
			ID: id, Name: fmt.Sprintf("Monitor %04d", i), Type: "http",
			URL: "https://" + id + ".example.com", Enabled: true, Interval: 60, Status: "up",
		})
		for j := 0; j < perMonitor; j++ {
			status := "up"
			if j%50 == 0 {
				status = "down"
			}
			beats = append(beats, models.UptimeHeartbeat{
				MonitorID: id, Status: status, Latency: int64(20 + j%30),
				CreatedAt: now.Add(-time.Duration(j) * time.Minute),
			})
		}
	}
	require.NoError(b, db.CreateInBatches(&monitors, 200).Error)
	require.NoError(b, db.CreateInBatches(&beats, 2000).Error)
	return db
}

// Tie-broken legacy oracle: the legacy SQL plus the deterministic tie-breakers
// the rewrite adds (newest-first by created_at then id inside the cap,
// oldest-first by created_at then id in the output). The untouched legacy
// order is nondeterministic on duplicate created_at values, so it cannot be
// the reference there.
const tieBrokenLegacyRecentBeatsSQL = `
SELECT monitor_id, status, latency, created_at
FROM (
  SELECT id, monitor_id, status, latency, created_at,
         ROW_NUMBER() OVER (PARTITION BY monitor_id ORDER BY created_at DESC, id DESC) AS rn
  FROM uptime_heartbeats
  WHERE created_at >= ?
)
WHERE rn <= ?
ORDER BY monitor_id, created_at ASC, id ASC`

// On a fixture full of duplicate timestamps with differing statuses the
// rewrite must equal the tie-broken oracle, whichever index exists.
func TestUptimeSummary_MatchesTieBrokenLegacySQL_Ties(t *testing.T) {
	forEachIndexState(t, true, func(t *testing.T, svc *UptimeSummaryService) {
		want := oracleSummary(t, svc, tieBrokenLegacyRecentBeatsSQL)
		for _, beats := range []int{1, 30, 60} {
			t.Run(fmt.Sprintf("beats=%d", beats), func(t *testing.T) {
				got, err := svc.GetSummary(context.Background(), beats)
				require.NoError(t, err)
				requireSameSummaryJSON(t, sliceSummaries(want, beats), got)
			})
		}
	})
}

// Without ties the tie-breakers must change nothing: the untouched legacy SQL
// is still the reference.
func TestUptimeSummary_TieFreeUnaffectedByTieBreakers(t *testing.T) {
	forEachIndexState(t, false, func(t *testing.T, svc *UptimeSummaryService) {
		requireSameSummaryJSON(t,
			oracleSummary(t, svc, legacyRecentBeatsSQL),
			oracleSummary(t, svc, tieBrokenLegacyRecentBeatsSQL))
	})
}

// Regression tripwire for the 40x: with the composite index present, neither
// summary query may fall back to scanning every retained heartbeat, and the
// ranked query may only sort the final output (no temp b-tree inside the
// per-monitor top-N).
func TestUptimeSummary_QueryPlanUsesCompositeIndex(t *testing.T) {
	db := setupUptimeTestDB(t)
	smIndex(t, db)
	windowStart := equivalenceNow.Add(-uptimeSummaryWindow)

	beatsPlan := explainPlan(t, db, recentBeatsSQL, uptimeMonitorScanLimit, windowStart, uptimeSummaryMaxBeats)
	uptimePlan := explainPlan(t, db, uptime24hSQL, windowStart, windowStart, uptimeMonitorScanLimit)

	require.Contains(t, beatsPlan, "idx_heartbeat_monitor_created", "plan:\n%s", beatsPlan)
	require.NotContains(t, beatsPlan, "SCAN uptime_heartbeats", "plan:\n%s", beatsPlan)
	require.LessOrEqual(t, strings.Count(beatsPlan, "TEMP B-TREE"), 1,
		"only the final ORDER BY may sort; plan:\n%s", beatsPlan)

	require.Contains(t, uptimePlan, "idx_heartbeat_monitor_created", "plan:\n%s", uptimePlan)
	require.NotContains(t, uptimePlan, "SCAN uptime_heartbeats", "plan:\n%s", uptimePlan)
}
