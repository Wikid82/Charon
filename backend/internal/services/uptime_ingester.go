package services

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"gorm.io/gorm"
)

const (
	// uptimeChannelBufferSize bounds the in-flight result buffer: ~500 monitors
	// with a few checks in flight per cycle (spec §3.3.2).
	uptimeChannelBufferSize = 2048
	// uptimeBatchSize is the row count that forces an early flush.
	uptimeBatchSize = 100
	// uptimeFlushInterval is the steady-state flush cadence.
	uptimeFlushInterval = 500 * time.Millisecond

	// uptimeShutdownFlushInterval is the tightened cadence used after ctx is
	// cancelled so the tail drains promptly while Run keeps ranging until the
	// pool closes the results channel (spec §3.1.4 / S4).
	uptimeShutdownFlushInterval = 50 * time.Millisecond

	// uptimeMaxFlushRetries bounds how many consecutive failed flushes a batch
	// is retained across before it is discarded. Mirrors the transient-lock
	// retry convention in createMonitorWithRetry — monitoring data is not worth
	// stalling the pipeline indefinitely, and a dropped heartbeat self-heals on
	// the monitor's next check.
	uptimeMaxFlushRetries = 3

	// uptimeDropLogInterval rate-limits the "channel full" warning.
	uptimeDropLogInterval = int64(time.Second)
)

// CheckResult is the worker→ingester message for one monitor check.
//
// Every status/debounce field is ALREADY RESOLVED by the worker against the
// authoritative in-memory monState map (spec §3.2.1 / §3.3.3) before the result
// is sent. The ingester performs NO transition logic and NO read-modify-write —
// it is a pure persistence mirror that copies these columns into
// uptime_heartbeats and uptime_monitors. A dropped CheckResult costs at most
// one heartbeat row plus one stale column refresh; it can never corrupt or
// suppress a transition, because runtime detection never reads these columns
// back (B3).
type CheckResult struct {
	MonitorID       string
	HostID          string // UptimeMonitor.UptimeHostID, "" when the monitor has no parent host
	HeartbeatStatus string // "up" | "down" — raw probe outcome, stored verbatim on the heartbeat row
	Latency         int64  // milliseconds
	Message         string
	CheckedAt       time.Time

	// Worker-resolved (authoritative) fields — copied verbatim, never recomputed.
	NewMonitorStatus string    // resolved monitor status after debounce
	FailureCount     int       // post-check consecutive-failure counter (from monState)
	StatusChanged    bool      // did the resolved status change on this check?
	StatusChangedAt  time.Time // set iff StatusChanged
	Synthetic        bool      // host-down child fan-out result (spec §3.2.3) — informational only
}

// HostCheckResult is the worker→ingester message for one host connectivity
// check. Like CheckResult, every field is pre-resolved by the worker; the
// ingester only mirrors them into uptime_hosts.
type HostCheckResult struct {
	HostID          string
	Status          string // resolved after the FailureThreshold debounce
	FailureCount    int
	Latency         int64
	Message         string
	CheckedAt       time.Time
	StatusChanged   bool
	StatusChangedAt time.Time
}

// UptimeIngester is the buffered, batched write path for uptime check results,
// mirroring StatsIngester. It is a dumb persistence mirror: all status and
// debounce resolution happens upstream in the worker pool, which is the sole
// sender on the results channel and closes it during teardown (spec §3.1.4).
type UptimeIngester struct {
	db           *gorm.DB
	results      chan any // CheckResult | HostCheckResult
	droppedCount atomic.Int64
	lastDropLog  atomic.Int64 // unix-nanos of the last emitted drop warning

	// failedFlushes tracks consecutive flush failures for the current retained
	// batch. Touched only by the single goroutine driving doFlush (Run, or a
	// test) — no synchronisation required.
	failedFlushes int
}

// newUptimeIngester builds an ingester bound to db and creates the buffered
// results channel it owns. Run is NOT started here — in production the worker
// pool commit wires and starts it; until then the ingester is inert.
func newUptimeIngester(db *gorm.DB) *UptimeIngester {
	return &UptimeIngester{
		db:      db,
		results: make(chan any, uptimeChannelBufferSize),
	}
}

// Send performs a non-blocking enqueue of a monitor check result. When the
// buffer is full the result is dropped and the drop counter incremented; the
// lost heartbeat self-heals on the monitor's next check (spec §3.3.2).
func (i *UptimeIngester) Send(r CheckResult) { i.enqueue(r) }

// SendHost performs a non-blocking enqueue of a host check result.
func (i *UptimeIngester) SendHost(r HostCheckResult) { i.enqueue(r) }

func (i *UptimeIngester) enqueue(r any) {
	select {
	case i.results <- r:
	default:
		i.noteDropped(1)
	}
}

// noteDropped increments the drop counter by n and emits a rate-limited warning.
func (i *UptimeIngester) noteDropped(n int64) {
	total := i.droppedCount.Add(n)

	now := time.Now().UnixNano()
	last := i.lastDropLog.Load()
	if now-last > uptimeDropLogInterval && i.lastDropLog.CompareAndSwap(last, now) {
		logger.Log().WithField("dropped_total", total).
			Warn("UptimeIngester results channel full — check result dropped")
	}
}

// DroppedCount returns the number of results lost to a full buffer or to a
// batch discarded after repeated flush failures. Surfaced at
// GET /api/v1/uptime/health.
func (i *UptimeIngester) DroppedCount() int64 { return i.droppedCount.Load() }

// Run consumes results, batching writes to SQLite. Per spec §3.1.4 (S4) it
// terminates ONLY when the results channel is closed by the worker pool (the
// sole sender), after one final flush. ctx is observed only to tighten the
// flush ticker so the tail drains quickly at shutdown — it does not end the
// loop while results is still open, which is what guarantees no in-flight
// result is lost.
func (i *UptimeIngester) Run(ctx context.Context) {
	ticker := time.NewTicker(uptimeFlushInterval)
	defer ticker.Stop()

	batch := make([]any, 0, uptimeBatchSize)
	done := ctx.Done()

	for {
		select {
		case r, ok := <-i.results:
			if !ok {
				i.doFlush(&batch) // channel closed — final flush, then stop
				return
			}
			batch = append(batch, r)
			if len(batch) >= uptimeBatchSize {
				i.doFlush(&batch)
			}

		case <-ticker.C:
			i.doFlush(&batch)

		case <-done:
			// Fire once: nil out the channel so this case blocks forever
			// afterwards, then tighten the flush cadence for a fast tail drain.
			done = nil
			ticker.Reset(uptimeShutdownFlushInterval)
		}
	}
}

// Stop drains any results still buffered and flushes them. Intended for tests
// and post-Run cleanup where the pool-closes-results handshake did not run.
func (i *UptimeIngester) Stop() {
	batch := make([]any, 0, uptimeBatchSize)
	for {
		select {
		case r, ok := <-i.results:
			if !ok {
				i.doFlush(&batch)
				return
			}
			batch = append(batch, r)
			if len(batch) >= uptimeBatchSize {
				i.doFlush(&batch)
			}
		default:
			i.doFlush(&batch)
			return
		}
	}
}

// doFlush attempts to persist batch. On success the batch is cleared and the
// failure counter reset. On failure the batch is RETAINED for the next attempt;
// after uptimeMaxFlushRetries consecutive failures it is discarded (rows
// counted as dropped) so a persistent DB fault cannot stall the pipeline.
func (i *UptimeIngester) doFlush(batch *[]any) {
	if len(*batch) == 0 {
		return
	}

	if err := i.flush(*batch); err != nil {
		i.failedFlushes++
		logger.Log().WithError(err).WithField("attempt", i.failedFlushes).
			Error("UptimeIngester: batch flush failed; retaining for retry")

		if i.failedFlushes >= uptimeMaxFlushRetries {
			logger.Log().WithField("dropped_rows", len(*batch)).
				Error("UptimeIngester: discarding batch after repeated flush failures")
			i.noteDropped(int64(len(*batch)))
			*batch = (*batch)[:0]
			i.failedFlushes = 0
		}
		return
	}

	i.failedFlushes = 0
	*batch = (*batch)[:0]
}

// flush persists one batch inside a single transaction: heartbeat inserts, then
// coalesced uptime_monitors updates (latest result per monitor wins), then
// coalesced uptime_hosts updates. next_check_at is never touched — the
// scheduler owns it.
func (i *UptimeIngester) flush(batch []any) error {
	heartbeats := make([]models.UptimeHeartbeat, 0, len(batch))
	monUpdates := make(map[string]CheckResult, len(batch))
	hostUpdates := make(map[string]HostCheckResult, len(batch))

	for _, item := range batch {
		switch r := item.(type) {
		case CheckResult:
			heartbeats = append(heartbeats, models.UptimeHeartbeat{
				MonitorID: r.MonitorID,
				Status:    r.HeartbeatStatus,
				Latency:   r.Latency,
				Message:   r.Message,
				CreatedAt: r.CheckedAt,
			})
			monUpdates[r.MonitorID] = r // batch is in arrival order → latest wins
		case HostCheckResult:
			hostUpdates[r.HostID] = r
		default:
			logger.Log().Warnf("UptimeIngester: unknown result type %T — skipped", item)
		}
	}

	return i.db.Transaction(func(tx *gorm.DB) error {
		if len(heartbeats) > 0 {
			if err := tx.CreateInBatches(heartbeats, uptimeBatchSize).Error; err != nil {
				return fmt.Errorf("insert heartbeats: %w", err)
			}
		}

		for id, r := range monUpdates {
			updates := map[string]any{
				"status":        r.NewMonitorStatus,
				"last_check":    r.CheckedAt,
				"latency":       r.Latency,
				"failure_count": r.FailureCount,
			}
			if r.StatusChanged {
				updates["last_status_change"] = r.StatusChangedAt
			}
			if err := tx.Model(&models.UptimeMonitor{}).Where("id = ?", id).Updates(updates).Error; err != nil {
				return fmt.Errorf("update monitor %s: %w", id, err)
			}
		}

		for id, r := range hostUpdates {
			updates := map[string]any{
				"status":        r.Status,
				"last_check":    r.CheckedAt,
				"latency":       r.Latency,
				"failure_count": r.FailureCount,
			}
			if r.StatusChanged {
				updates["last_status_change"] = r.StatusChangedAt
			}
			if err := tx.Model(&models.UptimeHost{}).Where("id = ?", id).Updates(updates).Error; err != nil {
				return fmt.Errorf("update host %s: %w", id, err)
			}
		}

		return nil
	})
}
