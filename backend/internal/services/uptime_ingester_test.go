package services

import (
	"context"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/require"
)

// makeIngesterCheckResult builds a minimal, already-resolved CheckResult for
// ingester tests. All debounce fields are pre-computed by the worker in
// production; the ingester only mirrors them.
func makeIngesterCheckResult(monitorID, status string) CheckResult {
	return CheckResult{
		MonitorID:        monitorID,
		HeartbeatStatus:  status,
		Latency:          20,
		Message:          "probe",
		CheckedAt:        time.Now().UTC(),
		NewMonitorStatus: status,
		FailureCount:     0,
	}
}

// TestUptimeIngester_SendDropsWhenFull verifies non-blocking Send/SendHost
// increment the drop counter once the buffer is saturated (mirrors
// StatsIngester back-pressure semantics).
func TestUptimeIngester_SendDropsWhenFull(t *testing.T) {
	i := newUptimeIngester(nil)

	// Fill the buffer directly so nothing is draining it.
	for k := 0; k < uptimeChannelBufferSize; k++ {
		i.results <- makeIngesterCheckResult("m1", "up")
	}

	i.Send(makeIngesterCheckResult("m1", "up"))
	i.SendHost(HostCheckResult{HostID: "h1", Status: "up"})

	require.GreaterOrEqual(t, i.DroppedCount(), int64(2),
		"both overflow sends must be counted as drops")
}

// TestUptimeIngester_FlushOnCountThreshold verifies a batch of uptimeBatchSize
// results is persisted without waiting for a timer tick.
func TestUptimeIngester_FlushOnCountThreshold(t *testing.T) {
	db := newPinnedUptimeDB(t)
	i := newUptimeIngester(db)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go i.Run(ctx)

	for k := 0; k < uptimeBatchSize; k++ {
		i.Send(makeIngesterCheckResult("m1", "up"))
	}

	require.Eventually(t, func() bool {
		var n int64
		db.Model(&models.UptimeHeartbeat{}).Count(&n)
		return n >= int64(uptimeBatchSize)
	}, 3*time.Second, 25*time.Millisecond, "count-threshold flush must persist the batch")
}

// TestUptimeIngester_FlushOnTimer verifies a sub-threshold batch is flushed by
// the periodic ticker.
func TestUptimeIngester_FlushOnTimer(t *testing.T) {
	db := newPinnedUptimeDB(t)
	i := newUptimeIngester(db)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go i.Run(ctx)

	for k := 0; k < 5; k++ {
		i.Send(makeIngesterCheckResult("m2", "down"))
	}

	require.Eventually(t, func() bool {
		var n int64
		db.Model(&models.UptimeHeartbeat{}).Count(&n)
		return n >= 5
	}, 3*time.Second, 25*time.Millisecond, "timer flush must persist a sub-threshold batch")
}

// TestUptimeIngester_RunTerminatesOnChannelCloseNotCtx is the S4 contract:
// Run must keep draining while results is open even after ctx is cancelled,
// and return only once the pool (sole sender) closes results — after a final
// flush that loses no buffered result.
func TestUptimeIngester_RunTerminatesOnChannelCloseNotCtx(t *testing.T) {
	db := newPinnedUptimeDB(t)
	i := newUptimeIngester(db)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		i.Run(ctx)
		close(done)
	}()

	i.Send(makeIngesterCheckResult("m1", "up"))
	i.Send(makeIngesterCheckResult("m1", "up"))

	// Cancelling ctx alone must NOT end Run while results is still open.
	cancel()
	select {
	case <-done:
		t.Fatal("Run returned on ctx cancel with an open results channel")
	case <-time.After(300 * time.Millisecond):
	}

	// Closing results ends Run after one final flush.
	close(i.results)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after results was closed")
	}

	var n int64
	db.Model(&models.UptimeHeartbeat{}).Count(&n)
	require.GreaterOrEqual(t, n, int64(2), "final flush must persist all buffered results")
}

// TestUptimeIngester_StopDrainsAndFlushes verifies the post-Run cleanup drain.
func TestUptimeIngester_StopDrainsAndFlushes(t *testing.T) {
	db := newPinnedUptimeDB(t)
	i := newUptimeIngester(db)

	for k := 0; k < 10; k++ {
		i.Send(makeIngesterCheckResult("m3", "up"))
	}

	i.Stop()

	var n int64
	db.Model(&models.UptimeHeartbeat{}).Count(&n)
	require.Equal(t, int64(10), n, "Stop must drain and flush every buffered result")
}

// TestUptimeIngester_CoalescesMonitorUpdatesLatestWins verifies that N results
// for one monitor produce N heartbeat rows but a single uptime_monitors update
// carrying only the latest result's values.
func TestUptimeIngester_CoalescesMonitorUpdatesLatestWins(t *testing.T) {
	db := newPinnedUptimeDB(t)
	require.NoError(t, db.Create(&models.UptimeMonitor{ID: "m1", Name: "M1", Enabled: true, Status: "pending"}).Error)

	i := newUptimeIngester(db)
	base := time.Now().UTC().Truncate(time.Second)

	var batch []any
	for k := 1; k <= 5; k++ {
		batch = append(batch, CheckResult{
			MonitorID:        "m1",
			HeartbeatStatus:  "down",
			Latency:          int64(k * 10),
			Message:          "fail",
			CheckedAt:        base.Add(time.Duration(k) * time.Second),
			NewMonitorStatus: "down",
			FailureCount:     k,
			StatusChanged:    k == 5,
			StatusChangedAt:  base.Add(5 * time.Second),
		})
	}
	require.NoError(t, i.flush(batch))

	var hb int64
	db.Model(&models.UptimeHeartbeat{}).Where("monitor_id = ?", "m1").Count(&hb)
	require.Equal(t, int64(5), hb, "one heartbeat row per result")

	var m models.UptimeMonitor
	require.NoError(t, db.First(&m, "id = ?", "m1").Error)
	require.Equal(t, "down", m.Status)
	require.Equal(t, 5, m.FailureCount, "monitor update coalesced to the latest result")
	require.Equal(t, int64(50), m.Latency)
	require.WithinDuration(t, base.Add(5*time.Second), m.LastStatusChange, time.Second)
}

// TestUptimeIngester_CoalescesHostUpdatesLatestWins verifies the same coalescing
// for HostCheckResult → uptime_hosts.
func TestUptimeIngester_CoalescesHostUpdatesLatestWins(t *testing.T) {
	db := newPinnedUptimeDB(t)
	require.NoError(t, db.Create(&models.UptimeHost{ID: "h1", Host: "h1.internal", Status: "pending"}).Error)

	i := newUptimeIngester(db)
	base := time.Now().UTC().Truncate(time.Second)

	batch := []any{
		HostCheckResult{HostID: "h1", Status: "up", FailureCount: 0, Latency: 3, CheckedAt: base},
		HostCheckResult{HostID: "h1", Status: "up", FailureCount: 1, Latency: 7, CheckedAt: base.Add(time.Second)},
		HostCheckResult{
			HostID: "h1", Status: "down", FailureCount: 2, Latency: 0,
			CheckedAt: base.Add(2 * time.Second), StatusChanged: true, StatusChangedAt: base.Add(2 * time.Second),
		},
	}
	require.NoError(t, i.flush(batch))

	var h models.UptimeHost
	require.NoError(t, db.First(&h, "id = ?", "h1").Error)
	require.Equal(t, "down", h.Status, "host update coalesced to the latest result")
	require.Equal(t, 2, h.FailureCount)
	require.WithinDuration(t, base.Add(2*time.Second), h.LastStatusChange, time.Second)
}

// TestUptimeIngester_RoutesResultTypes verifies the flush type-switch routes a
// CheckResult to heartbeats + uptime_monitors and a HostCheckResult to
// uptime_hosts in a single mixed batch.
func TestUptimeIngester_RoutesResultTypes(t *testing.T) {
	db := newPinnedUptimeDB(t)
	require.NoError(t, db.Create(&models.UptimeMonitor{ID: "m1", Status: "pending"}).Error)
	require.NoError(t, db.Create(&models.UptimeHost{ID: "h1", Host: "h1.internal", Status: "pending"}).Error)

	i := newUptimeIngester(db)
	now := time.Now().UTC().Truncate(time.Second)

	batch := []any{
		CheckResult{
			MonitorID: "m1", HostID: "h1", HeartbeatStatus: "up", Latency: 12,
			Message: "HTTP 200", CheckedAt: now, NewMonitorStatus: "up", FailureCount: 0,
		},
		HostCheckResult{HostID: "h1", Status: "up", FailureCount: 0, Latency: 5, CheckedAt: now},
	}
	require.NoError(t, i.flush(batch))

	var hb int64
	db.Model(&models.UptimeHeartbeat{}).Count(&hb)
	require.Equal(t, int64(1), hb)

	var m models.UptimeMonitor
	require.NoError(t, db.First(&m, "id = ?", "m1").Error)
	require.Equal(t, "up", m.Status)

	var h models.UptimeHost
	require.NoError(t, db.First(&h, "id = ?", "h1").Error)
	require.Equal(t, "up", h.Status)
}

// TestUptimeIngester_RetainsBatchThenDropsAfterMaxRetries verifies a failing
// flush retains the batch for retry and drops it only after
// uptimeMaxFlushRetries consecutive failures (bounded, mirrors
// createMonitorWithRetry).
func TestUptimeIngester_RetainsBatchThenDropsAfterMaxRetries(t *testing.T) {
	db := newPinnedUptimeDB(t)
	require.NoError(t, db.Migrator().DropTable(&models.UptimeHeartbeat{}))

	i := newUptimeIngester(db)
	batch := []any{CheckResult{
		MonitorID: "m1", HeartbeatStatus: "up", CheckedAt: time.Now().UTC(), NewMonitorStatus: "up",
	}}

	i.doFlush(&batch)
	require.Len(t, batch, 1, "batch retained after 1st failed flush")
	require.Equal(t, 1, i.failedFlushes)

	i.doFlush(&batch)
	require.Len(t, batch, 1, "batch retained after 2nd failed flush")
	require.Equal(t, 2, i.failedFlushes)

	i.doFlush(&batch)
	require.Len(t, batch, 0, "batch dropped after the 3rd failed flush")
	require.Equal(t, 0, i.failedFlushes, "retry counter resets after a drop")
	require.GreaterOrEqual(t, i.DroppedCount(), int64(1), "dropped rows are counted")
}

// TestUptimeIngester_StopFlushesAtCountThresholdAndOnClose exercises Stop's
// mid-drain count flush and its closed-channel exit branch.
func TestUptimeIngester_StopFlushesAtCountThresholdAndOnClose(t *testing.T) {
	db := newPinnedUptimeDB(t)
	i := newUptimeIngester(db)

	for k := 0; k < uptimeBatchSize+5; k++ {
		i.Send(makeIngesterCheckResult("m4", "up"))
	}
	close(i.results) // Stop must handle a closed, still-draining channel

	i.Stop()

	var n int64
	db.Model(&models.UptimeHeartbeat{}).Count(&n)
	require.Equal(t, int64(uptimeBatchSize+5), n)
}

// TestUptimeIngester_FlushSkipsUnknownResultType verifies the flush type-switch
// default branch: an unrecognised element is skipped, the rest still persist.
func TestUptimeIngester_FlushSkipsUnknownResultType(t *testing.T) {
	db := newPinnedUptimeDB(t)
	i := newUptimeIngester(db)

	batch := []any{
		"not-a-result",
		makeIngesterCheckResult("m1", "up"),
	}
	require.NoError(t, i.flush(batch))

	var n int64
	db.Model(&models.UptimeHeartbeat{}).Count(&n)
	require.Equal(t, int64(1), n, "the valid result persists; the unknown element is skipped")
}

// TestUptimeIngester_FlushErrorsOnMonitorUpdateFailure covers the monitor
// coalesced-update error path: heartbeat insert succeeds, the uptime_monitors
// UPDATE fails, and the whole transaction reports the wrapped error.
func TestUptimeIngester_FlushErrorsOnMonitorUpdateFailure(t *testing.T) {
	db := newPinnedUptimeDB(t)
	require.NoError(t, db.Migrator().DropTable(&models.UptimeMonitor{}))

	i := newUptimeIngester(db)
	err := i.flush([]any{makeIngesterCheckResult("m1", "up")})
	require.Error(t, err)
	require.Contains(t, err.Error(), "update monitor")
}

// TestUptimeIngester_FlushErrorsOnHostUpdateFailure covers the host coalesced-
// update error path.
func TestUptimeIngester_FlushErrorsOnHostUpdateFailure(t *testing.T) {
	db := newPinnedUptimeDB(t)
	require.NoError(t, db.Migrator().DropTable(&models.UptimeHost{}))

	i := newUptimeIngester(db)
	err := i.flush([]any{HostCheckResult{HostID: "h1", Status: "up", CheckedAt: time.Now().UTC()}})
	require.Error(t, err)
	require.Contains(t, err.Error(), "update host")
}

// TestUptimeIngester_DoFlushClearsCounterOnSuccess verifies a successful flush
// after a transient failure resets the retry counter.
func TestUptimeIngester_DoFlushClearsCounterOnSuccess(t *testing.T) {
	db := newPinnedUptimeDB(t)
	i := newUptimeIngester(db)
	i.failedFlushes = 2 // simulate two prior transient failures

	batch := []any{CheckResult{
		MonitorID: "m1", HeartbeatStatus: "up", CheckedAt: time.Now().UTC(), NewMonitorStatus: "up",
	}}
	i.doFlush(&batch)

	require.Len(t, batch, 0)
	require.Equal(t, 0, i.failedFlushes)

	var n int64
	db.Model(&models.UptimeHeartbeat{}).Count(&n)
	require.Equal(t, int64(1), n)
}
