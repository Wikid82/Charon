package dbmaint

import (
	"context"
	"sync"

	"github.com/Wikid82/charon/backend/internal/logger"
)

// Advice is the latest answer to "would the next start convert this database?".
type Advice struct {
	// Pending is true when a conversion at the next start is likely.
	Pending          bool
	ReclaimableBytes int64
	// Reason is why no conversion is pending (empty when Pending).
	Reason Reason
}

// HistoryFunc returns the persisted last result for the current database file,
// or nil when there is none or it cannot be read.
type HistoryFunc func(ctx context.Context) *LastResult

// StoreHistory reads the last result of the file at dbPath from store. Any
// failure means "no history": advice must never fail because of it.
func StoreHistory(store *Store, dbPath string) HistoryFunc {
	return func(ctx context.Context) *LastResult {
		fileID, err := FileID(dbPath)
		if err != nil {
			return nil
		}
		st, err := store.Peek(ctx, fileID)
		if err != nil {
			return nil
		}
		return st.LastResult
	}
}

// Advisor re-evaluates the boot decision after prune passes and remembers the
// result for the status endpoint. The zero value is ready to use and has no
// history.
type Advisor struct {
	mu      sync.Mutex
	current Advice
	logged  bool
	history HistoryFunc
}

// NewAdvisor returns an Advisor that stays silent about a conversion the next
// boot would refuse, judged by history (nil means no history).
func NewAdvisor(history HistoryFunc) *Advisor { return &Advisor{history: history} }

// Current returns the last advice.
func (a *Advisor) Current() Advice {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.current
}

// Advise runs the boot decision as a dry run (the persisted request and the
// attempt counter are ignored, nothing is written) and records the outcome. It
// is skipped when the last run for this file ended in a terminal skip. On an
// error the previous advice is kept.
func (a *Advisor) Advise(ctx context.Context, q Querier, cfg PlanConfig) Advice {
	cfg.FlagRequested, cfg.Attempts = false, 0
	res, err := Plan(ctx, q, cfg)
	if err != nil {
		logger.Log().WithError(err).Warn("database maintenance: could not evaluate whether optimization is pending")
		return a.Current()
	}
	return a.adviseFrom(ctx, res)
}

// adviseFrom applies the persisted-history suppression to a dry-run result and
// records it.
func (a *Advisor) adviseFrom(ctx context.Context, res PlanResult) Advice {
	if res.Decision.Run && a.history != nil {
		if last := a.history(ctx); SuppressesPending(last) {
			res.Decision = skip(last.Reason)
		}
	}
	return a.record(res)
}

// record stores the advice derived from res and logs the first time a
// conversion becomes pending.
func (a *Advisor) record(res PlanResult) Advice {
	advice := Advice{
		Pending:          res.Decision.Run,
		ReclaimableBytes: res.Stats.ReclaimableBytes(),
		Reason:           res.Decision.Reason,
	}

	a.mu.Lock()
	first := advice.Pending && !a.logged
	if first {
		a.logged = true
	}
	a.current = advice
	a.mu.Unlock()

	if first {
		logger.Log().WithField("reclaimable_bytes", advice.ReclaimableBytes).Infof(
			"database optimization pending: about %.1f GB of the database file is reusable space; "+
				"it will be returned automatically the next time Charon starts (for example after an update)",
			float64(advice.ReclaimableBytes)/float64(1<<30))
	}
	return advice
}
