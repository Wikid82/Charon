package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUptimePipeline_S4_InFlightCheckHeartbeatSurvivesImmediateCancel wires the
// real ingester + worker pool and proves the S4 teardown guarantee: a check
// that is in flight when ctx is cancelled still has its heartbeat persisted,
// because the ingester's Run loop terminates on the pool CLOSING the results
// channel (after workerWG.Wait()), not on ctx.Done() (spec §3.1.4).
func TestUptimePipeline_S4_InFlightCheckHeartbeatSurvivesImmediateCancel(t *testing.T) {
	db := setupUptimeTestDB(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(600 * time.Millisecond):
		case <-r.Context().Done():
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	svc := NewUptimeService(db, NewNotificationService(db, nil))
	svc.config.FailureThreshold = 2
	pool := NewUptimeWorkerPool(svc) // shares svc.Ingester (newTestPool would not)
	pool.checker.notifier = &fakeNotifier{}
	pool.notifyTimeout = 150 * time.Millisecond
	svc.Pool = pool

	mon := seedMonitor(t, db, models.UptimeMonitor{Name: "inflight", Enabled: true, Interval: 60, Type: "http", URL: srv.URL})

	ctx, cancel := context.WithCancel(context.Background())

	ingesterDone := make(chan struct{})
	go func() {
		svc.Ingester.Run(ctx)
		close(ingesterDone)
	}()
	go pool.Run(ctx)

	require.NoError(t, pool.Enqueue(ctx, UptimeJob{Kind: JobMonitorCheck, Monitor: mon, Manual: true}))

	// Let a worker pick the job up and enter the probe, then cancel mid-flight.
	time.Sleep(80 * time.Millisecond)
	cancel()

	select {
	case <-ingesterDone:
	case <-time.After(30 * time.Second):
		t.Fatal("ingester did not finish its ordered drain after cancellation")
	}

	var count int64
	require.NoError(t, db.Model(&models.UptimeHeartbeat{}).Where("monitor_id = ?", mon.ID).Count(&count).Error)
	assert.GreaterOrEqual(t, count, int64(1), "in-flight check's heartbeat must be persisted by the final flush")
}

// TestUptimePipeline_SchedulerToIngester_EndToEnd drives scheduler.Run +
// pool.Run + ingester.Run against a pinned DB and asserts a due monitor gets
// checked on its own cadence and a heartbeat lands, then everything tears down
// cleanly on ctx cancel.
func TestUptimePipeline_SchedulerToIngester_EndToEnd(t *testing.T) {
	db := setupUptimeTestDB(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	svc := NewUptimeService(db, NewNotificationService(db, nil))
	svc.config.FailureThreshold = 2
	pool := NewUptimeWorkerPool(svc) // shares svc.Ingester (newTestPool would not)
	pool.checker.notifier = &fakeNotifier{}
	pool.notifyTimeout = 150 * time.Millisecond
	svc.Pool = pool
	sched := NewUptimeScheduler(pool)
	sched.tick = 200 * time.Millisecond

	// Near-future NextCheckAt: hydrate keeps it (not past-due, no backfill) and
	// it falls due within a couple of ticks.
	mon := seedMonitor(t, db, models.UptimeMonitor{
		Name: "e2e", Enabled: true, Interval: 30, Type: "http", URL: srv.URL,
		NextCheckAt: time.Now().Add(300 * time.Millisecond),
	})

	ctx, cancel := context.WithCancel(context.Background())
	ingesterDone := make(chan struct{})
	go func() { svc.Ingester.Run(ctx); close(ingesterDone) }()
	go pool.Run(ctx)
	go sched.Run(ctx)

	require.Eventually(t, func() bool {
		var c int64
		db.Model(&models.UptimeHeartbeat{}).Where("monitor_id = ?", mon.ID).Count(&c)
		return c >= 1
	}, 5*time.Second, 50*time.Millisecond, "scheduler should drive a check that produces a heartbeat")

	// next_check_at advanced by the clamped interval (~30s out).
	var got models.UptimeMonitor
	require.NoError(t, db.First(&got, "id = ?", mon.ID).Error)
	assert.True(t, got.NextCheckAt.After(time.Now().Add(20*time.Second)), "next_check_at advanced by the per-monitor interval")

	cancel()
	select {
	case <-ingesterDone:
	case <-time.After(30 * time.Second):
		t.Fatal("pipeline did not tear down cleanly")
	}
}

// TestUptimePipeline_MixedIntervals_EachMonitorAdvancesByItsOwnCadence is the
// automated stand-in for the C5 manual smoke (go run against a scratch DB is not
// possible in this sandbox — config pins /app/data). It runs the real
// scheduler+pool+ingester over three monitors at 30s / 60s / 120s and asserts
// each monitor's next_check_at is pushed out by ITS OWN interval, not a uniform
// 60s, then verifies an ordered clean teardown.
func TestUptimePipeline_MixedIntervals_EachMonitorAdvancesByItsOwnCadence(t *testing.T) {
	db := setupUptimeTestDB(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	t.Cleanup(srv.Close)

	svc := NewUptimeService(db, NewNotificationService(db, nil))
	svc.config.FailureThreshold = 2
	pool := NewUptimeWorkerPool(svc)
	pool.checker.notifier = &fakeNotifier{}
	pool.notifyTimeout = 150 * time.Millisecond
	svc.Pool = pool
	sched := NewUptimeScheduler(pool)
	sched.tick = 150 * time.Millisecond

	soon := time.Now().Add(250 * time.Millisecond)
	m30 := seedMonitor(t, db, models.UptimeMonitor{Name: "m30", Enabled: true, Interval: 30, Type: "http", URL: srv.URL, NextCheckAt: soon})
	m60 := seedMonitor(t, db, models.UptimeMonitor{Name: "m60", Enabled: true, Interval: 60, Type: "http", URL: srv.URL, NextCheckAt: soon})
	m120 := seedMonitor(t, db, models.UptimeMonitor{Name: "m120", Enabled: true, Interval: 120, Type: "http", URL: srv.URL, NextCheckAt: soon})

	ctx, cancel := context.WithCancel(context.Background())
	ingesterDone := make(chan struct{})
	go func() { svc.Ingester.Run(ctx); close(ingesterDone) }()
	go pool.Run(ctx)
	go sched.Run(ctx)

	require.Eventually(t, func() bool {
		var a, b, c int64
		db.Model(&models.UptimeHeartbeat{}).Where("monitor_id = ?", m30.ID).Count(&a)
		db.Model(&models.UptimeHeartbeat{}).Where("monitor_id = ?", m60.ID).Count(&b)
		db.Model(&models.UptimeHeartbeat{}).Where("monitor_id = ?", m120.ID).Count(&c)
		return a >= 1 && b >= 1 && c >= 1
	}, 6*time.Second, 50*time.Millisecond, "all three monitors should be checked")

	now := time.Now()
	var g30, g60, g120 models.UptimeMonitor
	require.NoError(t, db.First(&g30, "id = ?", m30.ID).Error)
	require.NoError(t, db.First(&g60, "id = ?", m60.ID).Error)
	require.NoError(t, db.First(&g120, "id = ?", m120.ID).Error)

	// Each advanced by ~its own interval from when it was enqueued — bands are
	// wide enough to absorb tick slack but disjoint enough to prove the cadence
	// is per-monitor, not a uniform 60s.
	assert.InDelta(t, 30, g30.NextCheckAt.Sub(now).Seconds(), 10, "m30 next check ~30s out")
	assert.InDelta(t, 60, g60.NextCheckAt.Sub(now).Seconds(), 12, "m60 next check ~60s out")
	assert.InDelta(t, 120, g120.NextCheckAt.Sub(now).Seconds(), 12, "m120 next check ~120s out")

	cancel()
	select {
	case <-ingesterDone:
	case <-time.After(30 * time.Second):
		t.Fatal("pipeline did not tear down cleanly on cancel")
	}
}
