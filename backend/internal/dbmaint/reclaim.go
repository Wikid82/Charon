package dbmaint

import (
	"context"
	"errors"

	"github.com/Wikid82/charon/backend/internal/logger"
)

// Reclaimer runs the steady-state space maintenance after a retention prune
// pass: an incremental-mode database is drained, a legacy one is only advised.
// It satisfies the small interface the uptime pruner declares.
type Reclaimer struct {
	q       Querier
	dbPath  string
	envMode string
	advisor *Advisor
	opts    DrainOptions
}

// NewReclaimer builds a Reclaimer for the database at dbPath. The advisor is
// shared so the status endpoint reads what the reclaimer learned.
func NewReclaimer(q Querier, dbPath, envMode string, advisor *Advisor) *Reclaimer {
	return &Reclaimer{q: q, dbPath: dbPath, envMode: envMode, advisor: advisor}
}

// AfterPrune is called after a clean prune pass that deleted the given number
// of rows. Failures are logged, never returned: maintenance must not disturb
// the pruner.
func (r *Reclaimer) AfterPrune(ctx context.Context, deleted int64) {
	stats, err := Inspect(ctx, r.q, r.dbPath)
	if err != nil {
		if ctx.Err() == nil {
			logger.Log().WithError(err).Warn("database maintenance: could not inspect the database")
		}
		return
	}

	if !stats.IsIncremental() {
		r.advisor.Advise(ctx, r.q, PlanConfig{DBPath: r.dbPath, EnvMode: r.envMode})
		return
	}
	if deleted <= 0 && stats.ReclaimableBytes() <= KeepFreeBytes {
		return
	}

	res, err := Drain(ctx, r.q, r.opts)
	switch {
	case err == nil:
		if res.PagesFreed > 0 {
			logger.Log().WithField("pages_freed", res.PagesFreed).WithField("steps", res.Steps).
				Info("database maintenance: returned free pages to the operating system")
		}
	case errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded):
		// Shutting down; the next pass continues where this one stopped.
	default:
		logger.Log().WithError(err).Warn("database maintenance: drain pass failed; will retry after the next prune")
	}
}
