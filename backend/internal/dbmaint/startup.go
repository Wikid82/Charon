package dbmaint

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Wikid82/charon/backend/internal/database"
	"github.com/Wikid82/charon/backend/internal/logger"
)

// PlanFunc produces the boot decision.
type PlanFunc func(ctx context.Context) (PlanResult, error)

// SafePlan runs plan and turns a panic or an error into an idle result (a zero
// PlanResult) plus one warning: planning runs synchronously during startup, so
// it must never block or fail it.
func SafePlan(ctx context.Context, plan PlanFunc) (res PlanResult) {
	defer func() {
		if rec := recover(); rec != nil {
			logger.Log().WithField("panic", rec).
				Warn("database maintenance: planning panicked; skipping optimization this start")
			res = PlanResult{}
		}
	}()
	planned, err := plan(ctx)
	if err != nil {
		logger.Log().WithError(err).Warn("database maintenance: planning failed; skipping optimization this start")
		return PlanResult{}
	}
	return planned
}

// StartupPlan is the production planner: it reads the persisted request and
// failure state of the current file and evaluates the boot decision.
func StartupPlan(ctx context.Context, db *sql.DB, dbPath, envMode string) (PlanResult, error) {
	fileID, err := FileID(dbPath)
	if err != nil {
		return PlanResult{}, err
	}
	discardMarkerOfOptimizedFile(ctx, db, fileID)
	state, err := NewStore(db).Load(ctx, fileID)
	if err != nil {
		return PlanResult{}, fmt.Errorf("load maintenance state: %w", err)
	}
	return Plan(ctx, db, PlanConfig{
		DBPath:        dbPath,
		EnvMode:       envMode,
		FlagRequested: state.FlagRequested,
		Attempts:      state.Attempts,
		Interruptions: state.Interruptions,
	})
}

// discardMarkerOfOptimizedFile clears the leftover marker of a file that is
// already incremental (killed after the VACUUM, before the marker was cleared),
// so Load does not count a completed conversion as a failure. It is cosmetic:
// an error only logs a warning and Load then behaves as before.
func discardMarkerOfOptimizedFile(ctx context.Context, db *sql.DB, fileID string) {
	mode, err := readPragma(ctx, db, "auto_vacuum")
	if err != nil || mode != AutoVacuumIncremental {
		return
	}
	discarded, err := NewStore(db).DiscardMarkerIfConverted(ctx, fileID)
	switch {
	case err != nil:
		logger.Log().WithError(err).Warn("database maintenance: could not clear the leftover marker of an optimized database")
	case discarded:
		logger.Log().Info("database optimization had completed before an earlier shutdown; clearing the leftover marker")
	}
}

// StartParams configures Start. Plan and Convert are seams; nil selects
// StartupPlan and Convert.
type StartParams struct {
	Gate    *Gate
	DB      *sql.DB
	DBPath  string
	EnvMode string
	Plan    PlanFunc
	Convert ConvertFunc
	Timings Timings
}

// Start plans synchronously and, only when a conversion is likely, marks the
// gate planned and launches the runner goroutine on ctx. It reports whether a
// run was started. Planning is wrapped by SafePlan, so Start never blocks or
// fails startup.
func Start(ctx context.Context, p StartParams) bool {
	if p.Gate == nil {
		return false
	}
	plan := p.Plan
	if plan == nil {
		plan = func(ctx context.Context) (PlanResult, error) {
			return StartupPlan(ctx, p.DB, p.DBPath, p.EnvMode)
		}
	}

	res := SafePlan(ctx, plan)
	if !res.Decision.Run {
		logPlanSkip(res.Decision)
		settlePlanSkip(ctx, p, res)
		return false
	}

	convert := p.Convert
	if convert == nil {
		convert = Convert
	}

	p.Gate.MarkPlanned()
	go Run(ctx, Deps{
		DB:      p.DB,
		DBPath:  p.DBPath,
		Gate:    p.Gate,
		Convert: convert,
		QuickCheck: func() (<-chan struct{}, func() string) {
			return database.QuickCheckStatus(p.DBPath)
		},
		BootTime: time.Now(),
		Timings:  p.Timings,
	})
	return true
}

func logPlanSkip(d Decision) {
	switch d.Reason {
	case "", ReasonBelowThreshold:
		// Nothing worth saying: the common case of an install that needs no work.
	case ReasonTooManyFailures:
		logger.Log().WithField("reason", string(d.Reason)).
			Warn("database optimization stopped: it failed or was interrupted too many times; use the reclaim button to try again")
	case ReasonInsufficientDisk:
		logger.Log().WithField("required_bytes", d.RequiredBytes).WithField("available_bytes", d.AvailableBytes).
			Warn("database optimization skipped: not enough free disk space")
	default:
		logger.Log().WithField("reason", string(d.Reason)).Info("database optimization not needed at this start")
	}
}

// settlePlanSkip persists what a plan-time skip implies. The user's request is
// cleared only when there is nothing left to optimize or the failure back-off
// stopped the run (any other skip keeps it for the next start), and a refusal
// that will repeat every boot is remembered so Advise does not promise a
// conversion the next start will refuse.
func settlePlanSkip(ctx context.Context, p StartParams, res PlanResult) {
	d := res.Decision
	if p.DB == nil || (!d.ClearFlag && d.Reason != ReasonTooManyFailures) {
		return
	}
	store := NewStore(p.DB)
	var errs error
	if d.ClearFlag {
		errs = errors.Join(errs, store.ClearFlag(ctx))
	}
	if d.Reason == ReasonTooManyFailures {
		fileID, err := FileID(p.DBPath)
		if err == nil {
			err = store.WriteLastResult(ctx, LastResult{
				At:          time.Now().UTC(),
				Outcome:     ResultSkipped,
				Reason:      d.Reason,
				BytesBefore: res.Stats.MainBytes + res.Stats.WALBytes,
				FileID:      fileID,
			})
		}
		errs = errors.Join(errs, err)
	}
	if errs != nil {
		logger.Log().WithError(errs).Warn("database maintenance: could not record the skipped optimization")
	}
}
