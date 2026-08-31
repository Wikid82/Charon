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
	"gorm.io/gorm"
)

// --- test doubles ---

type fakeNotifier struct {
	mu             sync.Mutex
	downN          int
	upN            int
	lastDownReason string
	upSleep        time.Duration // if >0, NotifyMonitorUp sleeps this long IGNORING ctx
}

func (f *fakeNotifier) NotifyMonitorDown(_ context.Context, _ models.UptimeMonitor, reason, _ string) {
	f.mu.Lock()
	f.downN++
	f.lastDownReason = reason
	f.mu.Unlock()
}

func (f *fakeNotifier) NotifyMonitorUp(_ context.Context, _ models.UptimeMonitor, _ string) {
	if f.upSleep > 0 {
		time.Sleep(f.upSleep) // deliberately not ctx-aware — proves dispatch bounds wall-clock
	}
	f.mu.Lock()
	f.upN++
	f.mu.Unlock()
}

func (f *fakeNotifier) downs() int { f.mu.Lock(); defer f.mu.Unlock(); return f.downN }
func (f *fakeNotifier) ups() int   { f.mu.Lock(); defer f.mu.Unlock(); return f.upN }

// newTestPool builds a pool over db with a fake notifier and a fast
// notification-dispatch deadline. The pool's Run loop is NOT started.
func newTestPool(t *testing.T, db *gorm.DB) (*UptimeWorkerPool, *fakeNotifier) {
	t.Helper()
	ns := NewNotificationService(db, nil)
	svc := NewUptimeService(db, ns)
	svc.config.FailureThreshold = 2

	p := NewUptimeWorkerPool(svc)
	fn := &fakeNotifier{}
	p.checker.notifier = fn
	p.notifyTimeout = 150 * time.Millisecond

	t.Cleanup(func() { svc.FlushPendingNotifications() })
	return p, fn
}

// tcpListener starts an accept-and-close listener and returns "host:port".
func tcpListener(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	return ln.Addr().String()
}

func (p *UptimeWorkerPool) setRunCtx(ctx context.Context) {
	p.runMu.Lock()
	p.runCtx = ctx
	p.runMu.Unlock()
}

// --- enqueue / metrics ---

func TestUptimeWorkerPool_TryEnqueue_DropsAndCountsWhenFull(t *testing.T) {
	t.Parallel()
	p, _ := newTestPool(t, setupUptimeTestDB(t))

	// Fill the bounded job channel.
	for i := 0; i < uptimeQueueCapacity; i++ {
		require.True(t, p.TryEnqueue(UptimeJob{Kind: JobMonitorCheck}))
	}
	assert.Equal(t, uptimeQueueCapacity, p.QueueDepth())

	assert.False(t, p.TryEnqueue(UptimeJob{Kind: JobMonitorCheck}), "enqueue must fail on a full queue")
	assert.False(t, p.TryEnqueue(UptimeJob{Kind: JobMonitorCheck}))
	assert.Equal(t, int64(2), p.EnqueueDropped())
}

func TestUptimeWorkerPool_Enqueue_BlocksThenTimesOut(t *testing.T) {
	t.Parallel()
	p, _ := newTestPool(t, setupUptimeTestDB(t))
	for i := 0; i < uptimeQueueCapacity; i++ {
		require.True(t, p.TryEnqueue(UptimeJob{Kind: JobMonitorCheck}))
	}

	start := time.Now()
	err := p.Enqueue(context.Background(), UptimeJob{Kind: JobMonitorCheck})
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "queue full")
	assert.GreaterOrEqual(t, elapsed, uptimeManualEnqueueTimeout-200*time.Millisecond)
	assert.Less(t, elapsed, uptimeManualEnqueueTimeout+2*time.Second)
	assert.Equal(t, int64(1), p.EnqueueDropped())
}

func TestUptimeWorkerPool_Enqueue_SucceedsWhenSpaceAvailable(t *testing.T) {
	t.Parallel()
	p, _ := newTestPool(t, setupUptimeTestDB(t))
	require.NoError(t, p.Enqueue(context.Background(), UptimeJob{Kind: JobMonitorCheck}))
	assert.Equal(t, 1, p.QueueDepth())
}

func TestUptimeWorkerPool_Enqueue_RespectsCancelledContext(t *testing.T) {
	t.Parallel()
	p, _ := newTestPool(t, setupUptimeTestDB(t))
	for i := 0; i < uptimeQueueCapacity; i++ {
		require.True(t, p.TryEnqueue(UptimeJob{Kind: JobMonitorCheck}))
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := p.Enqueue(ctx, UptimeJob{Kind: JobMonitorCheck})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
}

// --- per-check deadline ---

func TestUptimeWorkerPool_CheckBudget_NeverExceedsHardCap(t *testing.T) {
	t.Parallel()
	p, _ := newTestPool(t, setupUptimeTestDB(t))
	for _, interval := range []int{0, 1, 5, 30, 60, 3600, 999999} {
		got := p.checkBudget(interval)
		assert.LessOrEqual(t, got, uptimeCheckHardCap, "interval=%d", interval)
		assert.Equal(t, uptimeCheckHardCap, got, "floored intervals always resolve to the hard cap")
	}
}

func TestUptimeWorkerPool_MonitorCheck_HonoursDeadlineFromRunCtx(t *testing.T) {
	t.Parallel()
	p, _ := newTestPool(t, setupUptimeTestDB(t))

	// A cancelled run ctx means every derived per-check ctx is already done, so
	// the probe must return promptly instead of waiting out a 20s budget.
	ctx, cancel := context.WithCancel(context.Background())
	p.setRunCtx(ctx)
	cancel()

	m := models.UptimeMonitor{ID: "m-deadline", Type: "http", URL: "http://192.0.2.1:9999", Enabled: true, MaxRetries: 2}
	p.EnsureMonitorState(m.ID)

	start := time.Now()
	p.handleMonitorCheck(UptimeJob{Kind: JobMonitorCheck, Monitor: m})
	assert.Less(t, time.Since(start), 3*time.Second, "a cancelled run ctx must abort the probe quickly")
}

// --- SeedState ---

func TestUptimeWorkerPool_SeedState_PopulatesMapsFromDB(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	p, _ := newTestPool(t, db)

	notified := time.Now().Add(-3 * time.Minute).UTC().Truncate(time.Second)
	require.NoError(t, db.Create(&models.UptimeMonitor{
		ID: "mon-up", Name: "up", Type: "http", URL: "http://x", Enabled: true,
		Status: "up", FailureCount: 0,
	}).Error)
	require.NoError(t, db.Create(&models.UptimeMonitor{
		ID: "mon-down", Name: "down", Type: "http", URL: "http://y", Enabled: true,
		Status: "down", FailureCount: 4, LastNotifiedDown: notified,
	}).Error)
	require.NoError(t, db.Create(&models.UptimeMonitor{
		ID: "mon-disabled", Name: "off", Type: "http", URL: "http://z", Enabled: false, Status: "up",
	}).Error)
	require.NoError(t, db.Create(&models.UptimeHost{ID: "host-1", Host: "10.0.0.1", Status: "down", FailureCount: 2}).Error)

	require.NoError(t, p.SeedState(context.Background()))

	p.monMu.Lock()
	defer p.monMu.Unlock()
	assert.Len(t, p.monState, 2, "only enabled monitors are seeded")
	assert.Equal(t, "up", p.monState["mon-up"].status)

	down := p.monState["mon-down"]
	assert.Equal(t, "down", down.status)
	assert.Equal(t, 4, down.failureCount)
	assert.WithinDuration(t, notified, down.lastNotifiedDown, time.Second)

	_, disabledSeeded := p.monState["mon-disabled"]
	assert.False(t, disabledSeeded)

	st, ok := p.HostState("host-1")
	require.True(t, ok)
	assert.Equal(t, "down", st.status)
	assert.Equal(t, 2, st.failureCount)
}

// --- B3: authoritative in-memory debounce survives a saturated ingester ---

func TestUptimeWorkerPool_B3_DownTransitionDetectedDespiteEveryResultDropped(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	p, fn := newTestPool(t, db)

	// Saturate the ingester's results channel so EVERY emit() drops.
	for len(p.ingester.results) < cap(p.ingester.results) {
		p.ingester.results <- CheckResult{}
	}
	require.Equal(t, cap(p.ingester.results), len(p.ingester.results))

	m := models.UptimeMonitor{
		ID: "m-b3", Name: "b3", Type: "tcp", URL: "127.0.0.1:9", // refused → always fails
		Enabled: true, MaxRetries: 2, Status: "up",
	}
	require.NoError(t, db.Create(&m).Error)
	p.monMu.Lock()
	p.monState[m.ID] = monDebounce{status: "up", lastStatusChange: time.Now().Add(-time.Hour)}
	p.monMu.Unlock()

	// Two consecutive failing checks — crosses MaxRetries=2.
	p.handleMonitorCheck(UptimeJob{Kind: JobMonitorCheck, Monitor: m})
	p.handleMonitorCheck(UptimeJob{Kind: JobMonitorCheck, Monitor: m})

	p.monMu.Lock()
	st := p.monState[m.ID]
	p.monMu.Unlock()

	assert.Equal(t, "down", st.status, "transition must be detected from the in-memory map, not the (dropped) DB write")
	assert.Equal(t, 2, st.failureCount)
	assert.Positive(t, p.ingester.DroppedCount(), "every CheckResult was in fact dropped")

	assert.Eventually(t, func() bool { return fn.downs() == 1 }, time.Second, 10*time.Millisecond,
		"the down notification must still fire")
}

func TestUptimeWorkerPool_ManualJob_SharesMonState(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	p, fn := newTestPool(t, db)

	m := models.UptimeMonitor{
		ID: "m-manual", Name: "manual", Type: "tcp", URL: "127.0.0.1:9",
		Enabled: true, MaxRetries: 2, Status: "up",
	}
	require.NoError(t, db.Create(&m).Error)
	p.monMu.Lock()
	p.monState[m.ID] = monDebounce{status: "up", lastStatusChange: time.Now().Add(-time.Hour)}
	p.monMu.Unlock()

	// One scheduled failing check, then one MANUAL failing check.
	p.handleMonitorCheck(UptimeJob{Kind: JobMonitorCheck, Monitor: m})
	p.handleMonitorCheck(UptimeJob{Kind: JobMonitorCheck, Monitor: m, Manual: true})

	p.monMu.Lock()
	st := p.monState[m.ID]
	p.monMu.Unlock()
	assert.Equal(t, 2, st.failureCount, "manual and scheduled checks accumulate the same streak")
	assert.Equal(t, "down", st.status)
	assert.Eventually(t, func() bool { return fn.downs() == 1 }, time.Second, 10*time.Millisecond)
}

// --- supervisor change #1: a slow notification hook cannot wedge the worker ---

func TestUptimeWorkerPool_Dispatch_SlowHookDoesNotExceedDeadline(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	p, fn := newTestPool(t, db)
	fn.upSleep = 3 * time.Second // hook ignores ctx and blocks far past notifyTimeout (150ms)

	addr := tcpListener(t) // probe will SUCCEED → recovery → NotifyMonitorUp
	m := models.UptimeMonitor{ID: "m-slow", Name: "slow", Type: "tcp", URL: addr, Enabled: true, MaxRetries: 2}
	require.NoError(t, db.Create(&m).Error)
	p.monMu.Lock()
	p.monState[m.ID] = monDebounce{status: "down", failureCount: 5, lastStatusChange: time.Now().Add(-time.Hour)}
	p.monMu.Unlock()

	start := time.Now()
	p.handleMonitorCheck(UptimeJob{Kind: JobMonitorCheck, Monitor: m})
	elapsed := time.Since(start)

	assert.Less(t, elapsed, time.Second, "worker must not block for the hook's full 3s — dispatch deadline is 150ms")
	// The hook still completes in the background.
	assert.Eventually(t, func() bool { return fn.ups() == 1 }, 5*time.Second, 25*time.Millisecond)
}

// --- B2: host up->down fan-out ---

func TestUptimeWorkerPool_B2_HostDownFanout(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	p, fn := newTestPool(t, db)

	host := models.UptimeHost{ID: "h-b2", Host: "127.0.0.1", Name: "h", Status: "up"}
	require.NoError(t, db.Create(&host).Error)
	child1 := models.UptimeMonitor{ID: "c1", UptimeHostID: &host.ID, Name: "c1", Type: "tcp", URL: "127.0.0.1:9", Enabled: true, MaxRetries: 2, Status: "up"}
	child2 := models.UptimeMonitor{ID: "c2", UptimeHostID: &host.ID, Name: "c2", Type: "tcp", URL: "127.0.0.1:9", Enabled: true, MaxRetries: 2, Status: "up"}
	require.NoError(t, db.Create(&child1).Error)
	require.NoError(t, db.Create(&child2).Error)

	require.NoError(t, p.SeedState(context.Background()))
	p.monMu.Lock()
	p.monState["c1"] = monDebounce{status: "up", lastStatusChange: time.Now().Add(-time.Hour)}
	p.monState["c2"] = monDebounce{status: "up", lastStatusChange: time.Now().Add(-time.Hour)}
	p.monMu.Unlock()
	p.hostMu.Lock()
	p.hostState[host.ID] = hostDebounce{status: "up"}
	p.hostMu.Unlock()

	// Two failing host checks — FailureThreshold=2 flips it to down on the 2nd.
	p.handleHostCheck(UptimeJob{Kind: JobHostCheck, Host: &host})
	st, _ := p.HostState(host.ID)
	assert.Equal(t, "up", st.status, "first failure must not flip the host")

	p.handleHostCheck(UptimeJob{Kind: JobHostCheck, Host: &host})

	st, ok := p.HostState(host.ID)
	require.True(t, ok)
	assert.Equal(t, "down", st.status, "HostState reflects the host as down")

	p.monMu.Lock()
	c1 := p.monState["c1"]
	c2 := p.monState["c2"]
	p.monMu.Unlock()
	assert.Equal(t, "down", c1.status, "child TCP monitor forced down by the fan-out")
	assert.Equal(t, "down", c2.status)
	assert.Equal(t, 2, c1.failureCount, "child failureCount maxed to MaxRetries")

	// One NotifyMonitorDown per transitioning child (the real notifier's 30s
	// batch window is what folds these into a single alert).
	assert.Eventually(t, func() bool { return fn.downs() == 2 }, time.Second, 10*time.Millisecond)
	assert.Equal(t, "Host unreachable", fn.lastDownReason)

	// A synthetic down heartbeat was queued for each child.
	drainAndCountSynthetic := func() (synthetic, total int) {
		for {
			select {
			case r := <-p.ingester.results:
				total++
				if cr, ok := r.(CheckResult); ok && cr.Synthetic {
					synthetic++
				}
			default:
				return
			}
		}
	}
	synthetic, _ := drainAndCountSynthetic()
	assert.Equal(t, 2, synthetic, "one synthetic down CheckResult per child")
}

func TestUptimeWorkerPool_B2_HostDownFanout_AlreadyDownChildIsSkipped(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	p, fn := newTestPool(t, db)

	host := models.UptimeHost{ID: "h-skip", Host: "127.0.0.1", Name: "h", Status: "up"}
	require.NoError(t, db.Create(&host).Error)
	require.NoError(t, db.Create(&models.UptimeMonitor{ID: "cd", UptimeHostID: &host.ID, Name: "cd", Type: "tcp", URL: "127.0.0.1:9", Enabled: true, MaxRetries: 2, Status: "down"}).Error)

	p.monMu.Lock()
	p.monState["cd"] = monDebounce{status: "down", failureCount: 2}
	p.monMu.Unlock()
	p.hostMu.Lock()
	p.hostState[host.ID] = hostDebounce{status: "up", failureCount: 1}
	p.hostMu.Unlock()

	p.handleHostCheck(UptimeJob{Kind: JobHostCheck, Host: &host}) // 2nd failure → host down

	st, _ := p.HostState(host.ID)
	assert.Equal(t, "down", st.status)
	assert.Equal(t, 0, fn.downs(), "an already-down child produces no synthetic result and no notification")
}

// --- lock ordering / hostMu not held across fan-out ---

func TestUptimeWorkerPool_HostDownFanout_ConcurrentHostStateReadsDoNotDeadlock(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	p, _ := newTestPool(t, db)

	host := models.UptimeHost{ID: "h-lock", Host: "127.0.0.1", Name: "h", Status: "up"}
	require.NoError(t, db.Create(&host).Error)
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("lc%d", i)
		require.NoError(t, db.Create(&models.UptimeMonitor{ID: id, UptimeHostID: &host.ID, Name: id, Type: "tcp", URL: "127.0.0.1:9", Enabled: true, MaxRetries: 2, Status: "up"}).Error)
		p.monMu.Lock()
		p.monState[id] = monDebounce{status: "up"}
		p.monMu.Unlock()
	}
	p.hostMu.Lock()
	p.hostState[host.ID] = hostDebounce{status: "up", failureCount: 1}
	p.hostMu.Unlock()

	stop := make(chan struct{})
	var readerWG sync.WaitGroup
	readerWG.Add(1)
	go func() {
		defer readerWG.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_, _ = p.HostState(host.ID) // must never block for the fan-out's duration
			}
		}
	}()

	done := make(chan struct{})
	go func() {
		p.handleHostCheck(UptimeJob{Kind: JobHostCheck, Host: &host})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		close(stop)
		readerWG.Wait()
		t.Fatal("handleHostCheck deadlocked (hostMu likely held across the fan-out)")
	}
	close(stop)
	readerWG.Wait()
}

// --- teardown: workerWG drains and results is closed on ctx cancel ---

func TestUptimeWorkerPool_Run_DrainsWorkersAndClosesResultsOnCancel(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	p, _ := newTestPool(t, db)
	p.size = 3

	ctx, cancel := context.WithCancel(context.Background())
	runReturned := make(chan struct{})
	go func() {
		p.Run(ctx)
		close(runReturned)
	}()

	time.Sleep(50 * time.Millisecond) // let workers spin up
	cancel()

	select {
	case <-runReturned:
	case <-time.After(3 * time.Second):
		t.Fatal("pool.Run did not return after ctx cancel")
	}

	// The pool is the sole closer of the ingester results channel.
	_, open := <-p.ingester.results
	assert.False(t, open, "results channel must be closed by the pool at teardown")

	// closeResults is idempotent.
	assert.NotPanics(t, func() { p.ingester.closeResults() })
}
