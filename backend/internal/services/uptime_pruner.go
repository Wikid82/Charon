package services

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"gorm.io/gorm"
)

const (
	// prunerInterval is the steady-state cadence of the retention pruner.
	prunerInterval = 1 * time.Hour
	// prunerFirstRunDelay lets the rest of the boot sequence (Caddy, DB warm-up,
	// the scheduler's cold-start backfill) settle before the first prune pass.
	prunerFirstRunDelay = 30 * time.Second

	// pruneChunkSize is the row count deleted per statement. Small enough that a
	// single chunk never holds the one SQLite write connection for long.
	pruneChunkSize = 5000
	// pruneChunkPause is the steady-state yield between chunks (tiny hourly
	// passes) so the ingester flush and API writes get the connection back.
	pruneChunkPause = 50 * time.Millisecond
	// firstPassChunkPause is the wider yield used until the first clean pass
	// completes — a cold, huge table can take 100-500 ms per chunk (N1).
	firstPassChunkPause = 250 * time.Millisecond

	// walCheckpointRowThreshold is the deleted-row count above which a pass
	// issues PRAGMA wal_checkpoint(TRUNCATE) to reclaim WAL file growth.
	walCheckpointRowThreshold = 50_000
	// optimizeEveryPasses runs PRAGMA optimize on a ~daily sub-cadence (every
	// Nth clean pass at the hourly interval). VACUUM is deliberately not used.
	optimizeEveryPasses = 24
)

// UptimePruner hard-deletes uptime_heartbeats rows older than
// uptime.heartbeat_retention_days on an hourly cadence, in paused chunks so the
// single SQLite write connection stays available (spec §3.4). It also owns the
// deferred idx_heartbeat_monitor_created index (spec §3.5.6): rather than a
// struct tag / AutoMigrate build over a potentially huge table at boot, the
// pruner issues CREATE INDEX IF NOT EXISTS at the end of every clean, caught-up
// pass — idempotent, and retried hourly until it lands, so a transient early
// failure can never leave the summary endpoint's index permanently unbuilt.
type UptimePruner struct {
	db  *gorm.DB
	cfg *uptimeConfig
	now func() time.Time

	// firstPassDone widens the inter-chunk pause until the first clean pass
	// completes. atomic because a future restore hook may flip it off-loop.
	firstPassDone atomic.Bool
	// indexCreated is a cheap in-memory short-circuit for ensureIndex. It is
	// only ever set after a successful CREATE INDEX IF NOT EXISTS; the DDL's own
	// idempotency is the real guarantee, so a stale false simply costs one
	// extra no-op DDL on the next pass.
	indexCreated atomic.Bool
	// passCount counts clean prune passes, for the PRAGMA optimize sub-cadence.
	passCount atomic.Int64

	// firstRunDelay / interval are seeded from the consts by the constructor;
	// tests override them so Run's loop is exercisable without hour-long waits.
	firstRunDelay time.Duration
	interval      time.Duration

	// beforeChunkHook, when non-nil, is invoked at the top of every prune-loop
	// iteration (1-based). Test seam only — lets a test abort ctx deterministically
	// between chunks. Always nil in production.
	beforeChunkHook func(iteration int)
	// walRowThreshold overrides walCheckpointRowThreshold when > 0 (test seam).
	walRowThreshold int64
}

// NewUptimePruner builds a pruner over the worker pool's DB handle and shared
// hot-reloading uptime.* config snapshot (same wiring pattern as
// NewUptimeScheduler). Run is started by routes.go on the shared request ctx.
func NewUptimePruner(pool *UptimeWorkerPool) *UptimePruner {
	return newUptimePruner(pool.db, pool.cfg)
}

func newUptimePruner(db *gorm.DB, cfg *uptimeConfig) *UptimePruner {
	return &UptimePruner{
		db:            db,
		cfg:           cfg,
		now:           time.Now,
		firstRunDelay: prunerFirstRunDelay,
		interval:      prunerInterval,
	}
}

// Run performs a first prune pass ~30 s after boot, then one every hour until
// ctx is cancelled. It is an independent goroutine: it aborts on ctx between
// chunks and can be cut at any chunk boundary with no persistence impact, so it
// is not part of the ordered ingester-drain teardown chain (spec §3.1.4).
func (p *UptimePruner) Run(ctx context.Context) {
	delay := p.firstRunDelay
	if delay <= 0 {
		delay = prunerFirstRunDelay
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return
	case <-timer.C:
	}
	p.tick(ctx)

	iv := p.interval
	if iv <= 0 {
		iv = prunerInterval
	}
	ticker := time.NewTicker(iv)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.tick(ctx)
		}
	}
}

// tick runs one prune pass and, only when that pass was clean and caught up
// (pruneOnce returned a nil error — a ctx abort or a chunk error both surface as
// non-nil), retries the deferred index build and advances the PRAGMA optimize
// sub-cadence.
func (p *UptimePruner) tick(ctx context.Context) {
	deleted, err := p.pruneOnce(ctx)
	if err != nil {
		if ctx.Err() == nil {
			logger.Log().WithError(err).Warn("UptimePruner: prune pass failed; will retry next interval")
		}
		return
	}
	if deleted > 0 {
		logger.Log().WithField("deleted", deleted).Info("UptimePruner: heartbeat retention prune complete")
	}

	// Deferred index (§3.5.6): idempotent CREATE INDEX IF NOT EXISTS at the end
	// of every clean, caught-up pass, retried hourly until it lands.
	_ = p.ensureIndex(ctx)

	if n := p.passCount.Add(1); n%optimizeEveryPasses == 0 {
		// Best-effort maintenance hint; a failure here is not actionable.
		p.db.WithContext(ctx).Exec(`PRAGMA optimize`)
	}
}

// pruneOnce deletes every heartbeat older than the current retention window in
// paused chunks. It returns the total rows removed and a non-nil error only on
// a ctx abort or a failed chunk — a nil error means the table is caught up.
func (p *UptimePruner) pruneOnce(ctx context.Context) (int64, error) {
	days := p.cfg.RetentionDays()
	cutoff := p.now().Add(-time.Duration(days) * 24 * time.Hour)

	pause := pruneChunkPause
	if !p.firstPassDone.Load() {
		pause = firstPassChunkPause
	}

	var total int64
	for i := 1; ; i++ {
		if p.beforeChunkHook != nil {
			p.beforeChunkHook(i)
		}
		if err := ctx.Err(); err != nil {
			return total, err
		}

		// Subquery form (not DELETE ... LIMIT, which modernc.org/sqlite does not
		// compile in). ORDER BY id deletes the oldest rows first and keeps the
		// plan on the primary key. Fully parameterised on cutoff + LIMIT.
		res := p.db.WithContext(ctx).Exec(
			`DELETE FROM uptime_heartbeats
			 WHERE id IN (
			     SELECT id FROM uptime_heartbeats
			     WHERE created_at < ?
			     ORDER BY id
			     LIMIT ?
			 )`, cutoff, pruneChunkSize)
		if res.Error != nil {
			return total, fmt.Errorf("uptime prune chunk: %w", res.Error)
		}
		total += res.RowsAffected
		if res.RowsAffected < pruneChunkSize {
			break // fewer than a full chunk deleted => caught up
		}
		time.Sleep(pause)
	}

	threshold := p.walRowThreshold
	if threshold == 0 {
		threshold = walCheckpointRowThreshold
	}
	if total >= threshold {
		// Reclaim WAL file growth from a large prune; best-effort.
		p.db.WithContext(ctx).Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	}

	p.firstPassDone.Store(true)
	return total, nil
}

// ensureIndex attempts the deferred composite index the summary endpoint needs.
// It is safe to call on every pass: the in-memory short-circuit skips the DDL
// once it has succeeded, and CREATE INDEX IF NOT EXISTS is itself idempotent, so
// a failed or interrupted earlier attempt simply retries here next hour.
func (p *UptimePruner) ensureIndex(ctx context.Context) error {
	if p.indexCreated.Load() {
		return nil
	}
	if err := p.db.WithContext(ctx).Exec(
		`CREATE INDEX IF NOT EXISTS idx_heartbeat_monitor_created ON uptime_heartbeats (monitor_id, created_at)`,
	).Error; err != nil {
		logger.Log().WithError(err).Warn(
			"UptimePruner: deferred index idx_heartbeat_monitor_created not built; will retry next pass")
		return fmt.Errorf("create idx_heartbeat_monitor_created: %w", err)
	}
	if !p.indexCreated.Swap(true) {
		logger.Log().Info("UptimePruner: idx_heartbeat_monitor_created is present")
	}
	return nil
}
