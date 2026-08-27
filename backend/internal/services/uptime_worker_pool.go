package services

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"gorm.io/gorm"
)

// UptimeJobKind discriminates the two kinds of work the pool processes.
type UptimeJobKind uint8

const (
	// JobMonitorCheck runs one monitor's configured probe (http/https/tcp/orthrus).
	JobMonitorCheck UptimeJobKind = iota
	// JobHostCheck runs one host's TCP connectivity pre-check (single dial).
	JobHostCheck
)

// UptimeJob is one unit of work on the bounded queue.
type UptimeJob struct {
	Kind    UptimeJobKind
	Monitor models.UptimeMonitor // populated for JobMonitorCheck
	Host    *models.UptimeHost   // populated for JobHostCheck
	Manual  bool                 // true for POST /uptime/monitors/:id/check
}

// monDebounce is the AUTHORITATIVE per-monitor debounce state (B3). Seeded once
// from uptime_monitors at pool start, then read-modify-written synchronously by
// the worker under monMu on every check result. Transition detection and
// notification dispatch read/write THIS, never the scheduler's DB snapshot — so
// a dropped CheckResult can never suppress a down/up transition or its alert.
type monDebounce struct {
	status           string
	failureCount     int
	lastStatusChange time.Time
	lastNotifiedDown time.Time
}

// hostDebounce is the AUTHORITATIVE per-host connectivity state (B2). Written by
// the worker running a JobHostCheck; read (RLock) by the scheduler to decide
// whether to skip enqueueing a down host's TCP monitors.
type hostDebounce struct {
	status           string
	failureCount     int
	lastStatusChange time.Time
}

const (
	// uptimeQueueCapacity bounds the job channel: headroom for a cold-start
	// thundering herd (500 monitors + their hosts) without unbounded memory
	// (~200KB worst case). When full, TryEnqueue drops and the scheduler
	// retries next tick (spec §3.2.1).
	uptimeQueueCapacity = 512

	// uptimeDefaultWorkerPoolSize is the fallback worker count when
	// uptime.worker_pool_size is unset/invalid.
	uptimeDefaultWorkerPoolSize = 30

	// uptimeCheckHardCap is the ceiling on any single check's wall-clock budget
	// (spec §3.2.1). The effective budget is min(clampInterval(interval), this).
	uptimeCheckHardCap = 20 * time.Second

	// uptimeManualEnqueueTimeout bounds the blocking Enqueue used by manual
	// checks before it returns an error (handler maps to HTTP 503).
	uptimeManualEnqueueTimeout = 2 * time.Second

	// uptimeNotifyDispatchTimeout bounds the worker's synchronous notification
	// dispatch so a hung webhook cannot wedge the worker — or, at shutdown,
	// block the teardown chain past workerWG.Wait() (supervisor change #1).
	uptimeNotifyDispatchTimeout = 10 * time.Second
)

// UptimeWorkerPool is a fixed-size worker set over a bounded job queue. It owns
// the authoritative in-memory debounce maps and one shared SSRF-safe keep-alive
// HTTP client reused across every check.
//
// LOCK ORDERING INVARIANT: hostMu is always acquired before monMu, never the
// reverse. Only the host-down fan-out path nests them, and it releases hostMu
// BEFORE taking monMu (see handleHostCheck / fanOutHostDown). Do not introduce
// a monMu -> hostMu path or the pool can deadlock.
type UptimeWorkerPool struct {
	db       *gorm.DB
	svc      *UptimeService // thresholds, child-monitor loads, notification sink
	ingester *UptimeIngester
	cfg      *uptimeConfig
	checker  *uptimeChecker

	size int
	jobs chan UptimeJob

	monMu    sync.Mutex
	monState map[string]monDebounce

	hostMu    sync.RWMutex
	hostState map[string]hostDebounce

	workerWG   sync.WaitGroup
	enqDropped atomic.Int64
	seeded     atomic.Bool

	runMu  sync.Mutex
	runCtx context.Context //nolint:containedctx // stored only to derive bounded notification-dispatch contexts

	// test seams
	now           func() time.Time
	notifyTimeout time.Duration
}

// NewUptimeWorkerPool constructs the pool and its one shared keep-alive
// SSRF-safe client. It does NOT start Run — the scheduler commit (C5) does that.
// Worker count comes from uptime.worker_pool_size (falling back to 30).
func NewUptimeWorkerPool(svc *UptimeService) *UptimeWorkerPool {
	size := uptimeDefaultWorkerPoolSize
	if svc.uptimeCfg != nil {
		if n := svc.uptimeCfg.WorkerPoolSize(); n >= 1 {
			size = n
		}
	}

	p := &UptimeWorkerPool{
		db:            svc.DB,
		svc:           svc,
		ingester:      svc.Ingester,
		cfg:           svc.uptimeCfg,
		size:          size,
		jobs:          make(chan UptimeJob, uptimeQueueCapacity),
		monState:      make(map[string]monDebounce),
		hostState:     make(map[string]hostDebounce),
		now:           time.Now,
		notifyTimeout: uptimeNotifyDispatchTimeout,
	}
	// Reuse the service's shared checker (one keep-alive client + one copy of
	// the probe switch). Fall back to a fresh one only if a caller built the
	// pool from a service that has none (defensive; production always does).
	if svc.checker != nil {
		p.checker = svc.checker
	} else {
		p.checker = newUptimeChecker(svc)
	}
	return p
}

// --- state seeding (B2/B3) ---

// SeedState populates monState/hostState from the DB. Called once before Run;
// idempotent and safe to call again (ReseedState). Maps are built OUTSIDE the
// locks and swapped in under them (supervisor change #4).
func (p *UptimeWorkerPool) SeedState(ctx context.Context) error {
	mon, err := p.loadMonState(ctx)
	if err != nil {
		return fmt.Errorf("seed monState: %w", err)
	}
	host, err := p.loadHostState(ctx)
	if err != nil {
		return fmt.Errorf("seed hostState: %w", err)
	}

	p.monMu.Lock()
	p.monState = mon
	p.monMu.Unlock()

	p.hostMu.Lock()
	p.hostState = host
	p.hostMu.Unlock()

	p.seeded.Store(true)
	return nil
}

// ReseedState rebuilds the debounce maps from the (restored) DB after a live
// backup restore (spec §3.9). Identical to SeedState; separate name for call-site
// clarity.
func (p *UptimeWorkerPool) ReseedState(ctx context.Context) error {
	if err := p.SeedState(ctx); err != nil {
		return fmt.Errorf("reseed state: %w", err)
	}
	return nil
}

// EnsureMonitorState adds a zero ("pending") entry for a newly-created monitor
// so its first check has a baseline to debounce against (spec §3.1.2 rescan()).
func (p *UptimeWorkerPool) EnsureMonitorState(id string) {
	p.monMu.Lock()
	if _, ok := p.monState[id]; !ok {
		p.monState[id] = monDebounce{status: "pending"}
	}
	p.monMu.Unlock()
}

func (p *UptimeWorkerPool) loadMonState(ctx context.Context) (map[string]monDebounce, error) {
	type row struct {
		ID               string
		Status           string
		FailureCount     int
		LastStatusChange time.Time
		LastNotifiedDown time.Time
	}
	var rows []row
	if err := p.db.WithContext(ctx).Model(&models.UptimeMonitor{}).
		Select("id", "status", "failure_count", "last_status_change", "last_notified_down").
		Where("enabled = ?", true).Scan(&rows).Error; err != nil {
		return nil, err
	}
	m := make(map[string]monDebounce, len(rows))
	for _, r := range rows {
		st := r.Status
		if st == "" {
			st = "pending"
		}
		m[r.ID] = monDebounce{
			status:           st,
			failureCount:     r.FailureCount,
			lastStatusChange: r.LastStatusChange,
			lastNotifiedDown: r.LastNotifiedDown,
		}
	}
	return m, nil
}

func (p *UptimeWorkerPool) loadHostState(ctx context.Context) (map[string]hostDebounce, error) {
	type row struct {
		ID               string
		Status           string
		FailureCount     int
		LastStatusChange time.Time
	}
	var rows []row
	if err := p.db.WithContext(ctx).Model(&models.UptimeHost{}).
		Select("id", "status", "failure_count", "last_status_change").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	m := make(map[string]hostDebounce, len(rows))
	for _, r := range rows {
		st := r.Status
		if st == "" {
			st = "pending"
		}
		m[r.ID] = hostDebounce{status: st, failureCount: r.FailureCount, lastStatusChange: r.LastStatusChange}
	}
	return m, nil
}

// --- enqueue / metrics ---

// TryEnqueue is a non-blocking enqueue. On a full queue it returns false and
// increments the drop metric; the scheduler leaves the job due and retries.
func (p *UptimeWorkerPool) TryEnqueue(j UptimeJob) bool {
	select {
	case p.jobs <- j:
		return true
	default:
		p.enqDropped.Add(1)
		return false
	}
}

// Enqueue blocks up to uptimeManualEnqueueTimeout (or until ctx is done) for a
// slot. Used by manual checks; the handler maps a non-nil error to HTTP 503.
func (p *UptimeWorkerPool) Enqueue(ctx context.Context, j UptimeJob) error {
	timer := time.NewTimer(uptimeManualEnqueueTimeout)
	defer timer.Stop()
	select {
	case p.jobs <- j:
		return nil
	case <-timer.C:
		p.enqDropped.Add(1)
		return fmt.Errorf("uptime check queue full after %s", uptimeManualEnqueueTimeout)
	case <-ctx.Done():
		return fmt.Errorf("uptime enqueue cancelled: %w", ctx.Err())
	}
}

// QueueDepth is the number of jobs waiting to be picked up.
func (p *UptimeWorkerPool) QueueDepth() int { return len(p.jobs) }

// EnqueueDropped is the running count of jobs dropped on a full queue.
func (p *UptimeWorkerPool) EnqueueDropped() int64 { return p.enqDropped.Load() }

// WorkerPoolSize is the active worker count. The pool is sized once at
// construction from uptime.worker_pool_size; changing that setting needs a
// restart to take effect, and GET /api/v1/uptime/health surfaces this live
// value so operators can confirm a restart landed (spec §3.5.5, resolved
// decision #1).
func (p *UptimeWorkerPool) WorkerPoolSize() int { return p.size }

// HostState returns the authoritative connectivity state for a host (RLock).
// Consumed by the scheduler's host-down short-circuit (spec §3.1.2).
func (p *UptimeWorkerPool) HostState(hostID string) (hostDebounce, bool) {
	p.hostMu.RLock()
	defer p.hostMu.RUnlock()
	st, ok := p.hostState[hostID]
	return st, ok
}

// --- run / teardown ---

// Run seeds state (if not already), spawns p.size workers, and owns the S4
// teardown chain (spec §3.1.4): on ctx cancel the workers stop pulling new jobs,
// finish any in-flight check, then the pool — the SOLE sender — closes the
// ingester's results channel.
func (p *UptimeWorkerPool) Run(ctx context.Context) {
	p.runMu.Lock()
	p.runCtx = ctx
	p.runMu.Unlock()

	if !p.seeded.Load() {
		if err := p.SeedState(ctx); err != nil {
			logger.Log().WithError(err).Error("UptimeWorkerPool: initial SeedState failed; debounce maps start empty")
		}
	}

	for i := 0; i < p.size; i++ {
		p.workerWG.Add(1)
		go p.worker(ctx)
	}

	<-ctx.Done()
	p.workerWG.Wait()         // every in-flight handle() is bounded by uptimeCheckHardCap
	p.ingester.closeResults() // pool is the sole sender — safe to close now
}

func (p *UptimeWorkerPool) worker(ctx context.Context) {
	defer p.workerWG.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case j := <-p.jobs:
			p.handle(j)
		}
	}
}

func (p *UptimeWorkerPool) handle(j UptimeJob) {
	switch j.Kind {
	case JobHostCheck:
		p.handleHostCheck(j)
	default:
		p.handleMonitorCheck(j)
	}
}

// baseCtx is the parent for per-check and notification-dispatch contexts. It is
// p.runCtx once Run has started; context.Background() otherwise (tests calling
// handle() directly).
func (p *UptimeWorkerPool) baseCtx() context.Context {
	p.runMu.Lock()
	defer p.runMu.Unlock()
	if p.runCtx != nil {
		return p.runCtx
	}
	return context.Background()
}

func (p *UptimeWorkerPool) checkBudget(intervalSeconds int) time.Duration {
	d := time.Duration(clampInterval(intervalSeconds, p.cfg)) * time.Second
	if d > uptimeCheckHardCap {
		d = uptimeCheckHardCap
	}
	return d
}

// --- monitor check path (B3) ---

func (p *UptimeWorkerPool) handleMonitorCheck(j UptimeJob) {
	m := j.Monitor
	// Guard the <=30s enable/disable race: a monitor disabled after being
	// snapshotted emits nothing (spec §3.7). Manual checks always run.
	if !m.Enabled && !j.Manual {
		return
	}

	cctx, cancel := context.WithTimeout(p.baseCtx(), p.checkBudget(m.Interval))
	defer cancel()

	raw := p.checker.probe(cctx, m)
	now := p.now()

	maxRetries := m.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3 // legacy rows
	}

	p.monMu.Lock()
	prev := p.monState[m.ID]
	if prev.status == "" {
		prev.status = "pending"
	}
	next, changed, durationStr := applyMonitorDebounce(prev, raw, maxRetries, now)
	if changed && next.status == "down" {
		next.lastNotifiedDown = now
	}
	p.monState[m.ID] = next
	p.monMu.Unlock()

	// Notification dispatch AFTER releasing monMu (spec §3.3.3).
	if changed {
		p.dispatchMonitorTransition(m, next.status, raw.Message, durationStr)
	}

	p.emit(CheckResult{
		MonitorID:        m.ID,
		HostID:           derefString(m.UptimeHostID),
		HeartbeatStatus:  raw.heartbeatStatus(),
		Latency:          raw.Latency,
		Message:          raw.Message,
		CheckedAt:        now,
		NewMonitorStatus: next.status,
		FailureCount:     next.failureCount,
		StatusChanged:    changed,
		StatusChangedAt:  statusChangedAt(changed, now),
	})
}

// applyMonitorDebounce is the pure transition function extracted from
// uptime_service.go:checkMonitor (~lines 946-1003): success => up + reset;
// failure => failureCount++ then "down" at >= maxRetries. changed follows the
// legacy rule (old != new && old != "pending"). durationStr is the humanised
// gap since the previous status change (previous-uptime for a down, downtime for
// an up), matching legacy behaviour.
func applyMonitorDebounce(prev monDebounce, raw rawCheckResult, maxRetries int, now time.Time) (next monDebounce, changed bool, durationStr string) {
	next = prev
	newStatus := prev.status

	if raw.Success {
		if prev.status != "up" {
			newStatus = "up"
		}
		next.failureCount = 0
	} else {
		next.failureCount = prev.failureCount + 1
		if next.failureCount >= maxRetries {
			newStatus = "down"
		}
	}

	changed = prev.status != newStatus && prev.status != "pending"
	if changed && !prev.lastStatusChange.IsZero() {
		durationStr = formatDuration(now.Sub(prev.lastStatusChange))
	}

	next.status = newStatus
	if changed {
		next.lastStatusChange = now
	}
	return next, changed, durationStr
}

// forceMonitorDown transitions a child monitor straight to "down" because its
// parent host is unreachable (spec §3.2.3: child failureCount -> max). Caller
// must have already checked prev.status != "down".
func forceMonitorDown(prev monDebounce, maxRetries int, now time.Time) (next monDebounce, changed bool, durationStr string) {
	next = prev
	next.failureCount = maxRetries
	changed = prev.status != "down" && prev.status != "pending"
	if changed && !prev.lastStatusChange.IsZero() {
		durationStr = formatDuration(now.Sub(prev.lastStatusChange))
	}
	next.status = "down"
	if changed {
		next.lastStatusChange = now
	}
	return next, changed, durationStr
}

func (p *UptimeWorkerPool) dispatchMonitorTransition(m models.UptimeMonitor, newStatus, reason, durationStr string) {
	p.dispatch(func(ctx context.Context) {
		switch newStatus {
		case "down":
			p.checker.notifier.NotifyMonitorDown(ctx, m, reason, durationStr)
		case "up":
			p.checker.notifier.NotifyMonitorUp(ctx, m, durationStr)
		}
	})
}

// dispatch runs fn with a deadline-bounded context derived from the pool's run
// ctx and — critically — never blocks the caller past notifyTimeout even if fn
// ignores ctx entirely (supervisor change #1). A slow/hung webhook therefore
// cannot wedge a worker or stall shutdown; the fn goroutine drains on its own.
func (p *UptimeWorkerPool) dispatch(fn func(context.Context)) {
	nctx, cancel := context.WithTimeout(p.baseCtx(), p.notifyTimeout)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		fn(nctx)
	}()

	select {
	case <-done:
	case <-nctx.Done():
		// Deadline hit (or pool shutting down): stop waiting. cancel() (deferred)
		// signals a ctx-aware fn to abort; a non-cooperative fn finishes in the
		// background.
	}
}

// --- host check path (B2) ---

func (p *UptimeWorkerPool) handleHostCheck(j UptimeJob) {
	if j.Host == nil {
		return
	}
	host := *j.Host

	cctx, cancel := context.WithTimeout(p.baseCtx(), uptimeCheckHardCap)
	defer cancel()

	raw := p.checker.probeHost(cctx, host, p.db)
	now := p.now()

	threshold := p.svc.config.FailureThreshold
	if threshold <= 0 {
		threshold = 2
	}

	p.hostMu.Lock()
	prev := p.hostState[host.ID]
	if prev.status == "" {
		prev.status = host.Status
		if prev.status == "" {
			prev.status = "pending"
		}
	}
	next, changed := applyHostDebounce(prev, raw, threshold, now)
	p.hostState[host.ID] = next
	wentDown := changed && next.status == "down"
	p.hostMu.Unlock() // release BEFORE fan-out (supervisor change #2); lock order hostMu -> monMu.

	if wentDown {
		p.fanOutHostDown(host, now)
	}

	p.emitHost(HostCheckResult{
		HostID:          host.ID,
		Status:          next.status,
		FailureCount:    next.failureCount,
		Latency:         raw.Latency,
		Message:         raw.Message,
		CheckedAt:       now,
		StatusChanged:   changed,
		StatusChangedAt: statusChangedAt(changed, now),
	})
}

// applyHostDebounce mirrors uptime_service.go:checkHost (~lines 674-701):
// success => up + reset; failure => failureCount++ then "down" at >= threshold,
// otherwise keep the current status. changed follows old != new && old !=
// "pending" (so pending->up is not a transition).
func applyHostDebounce(prev hostDebounce, raw rawCheckResult, threshold int, now time.Time) (next hostDebounce, changed bool) {
	next = prev
	newStatus := prev.status

	if raw.Success {
		next.failureCount = 0
		newStatus = "up"
	} else {
		next.failureCount = prev.failureCount + 1
		if next.failureCount >= threshold {
			newStatus = "down"
		}
	}

	changed = prev.status != newStatus && prev.status != "pending"
	next.status = newStatus
	if changed {
		next.lastStatusChange = now
	}
	return next, changed
}

// fanOutHostDown synthesises "down" child results for the host's TCP monitors
// and fires ONE consolidated down-notification (the 30s batch window coalesces).
// Neither hostMu nor monMu is held across the DB read; monMu is taken per child
// for a short RMW (spec §3.2.3, supervisor change #2).
func (p *UptimeWorkerPool) fanOutHostDown(host models.UptimeHost, now time.Time) {
	var children []models.UptimeMonitor
	if err := p.db.Where("uptime_host_id = ? AND lower(type) = ?", host.ID, "tcp").
		Find(&children).Error; err != nil {
		logger.Log().WithError(err).WithField("host_id", host.ID).
			Error("UptimeWorkerPool: host-down fan-out could not load child monitors")
		return
	}

	const synthMsg = "Host unreachable"
	for _, child := range children {
		maxRetries := child.MaxRetries
		if maxRetries <= 0 {
			maxRetries = 3
		}

		p.monMu.Lock()
		prev := p.monState[child.ID]
		if prev.status == "" {
			prev.status = "pending"
		}
		if prev.status == "down" {
			p.monMu.Unlock()
			continue // already down — no synthetic result, no notification
		}
		next, changed, durationStr := forceMonitorDown(prev, maxRetries, now)
		if changed {
			next.lastNotifiedDown = now
		}
		p.monState[child.ID] = next
		p.monMu.Unlock()

		p.emit(CheckResult{
			MonitorID:        child.ID,
			HostID:           host.ID,
			HeartbeatStatus:  "down",
			Latency:          0,
			Message:          synthMsg,
			CheckedAt:        now,
			NewMonitorStatus: next.status,
			FailureCount:     next.failureCount,
			StatusChanged:    changed,
			StatusChangedAt:  statusChangedAt(changed, now),
			Synthetic:        true,
		})

		if changed {
			// Same batched down path as a real transition; the 30s window folds
			// every child into one "N services down on host X" alert.
			c := child
			dt := durationStr
			p.dispatch(func(ctx context.Context) {
				p.checker.notifier.NotifyMonitorDown(ctx, c, synthMsg, dt)
			})
		}
	}
}

// --- emit ---

func (p *UptimeWorkerPool) emit(r CheckResult)         { p.ingester.Send(r) }
func (p *UptimeWorkerPool) emitHost(r HostCheckResult) { p.ingester.SendHost(r) }

// --- small helpers ---

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func statusChangedAt(changed bool, now time.Time) time.Time {
	if changed {
		return now
	}
	return time.Time{}
}
