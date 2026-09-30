package dbmaint

import (
	"context"
	"database/sql"
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

// StartupPlan is the production planner. While conversionEnabled is false it
// returns an idle result without reading anything, so no install can enter
// planned, defer the pipeline or pin the pool before a conversion exists.
func StartupPlan(ctx context.Context, db *sql.DB, dbPath, envMode string) (PlanResult, error) {
	if !conversionEnabled {
		return PlanResult{}, nil
	}
	fileID, err := FileID(dbPath)
	if err != nil {
		return PlanResult{}, err
	}
	state, err := NewStore(db).Load(ctx, fileID)
	if err != nil {
		return PlanResult{}, fmt.Errorf("load maintenance state: %w", err)
	}
	return Plan(ctx, db, PlanConfig{
		DBPath:        dbPath,
		EnvMode:       envMode,
		FlagRequested: state.FlagRequested,
		Attempts:      state.Attempts,
	})
}

// StartParams configures Start. Plan and Convert are seams; nil Plan selects
// StartupPlan.
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
		return false
	}

	p.Gate.MarkPlanned()
	go Run(ctx, Deps{
		DB:      p.DB,
		DBPath:  p.DBPath,
		Gate:    p.Gate,
		Convert: p.Convert,
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
	case ReasonInsufficientDisk:
		logger.Log().WithField("required_bytes", d.RequiredBytes).WithField("available_bytes", d.AvailableBytes).
			Warn("database optimization skipped: not enough free disk space")
	default:
		logger.Log().WithField("reason", string(d.Reason)).Info("database optimization not needed at this start")
	}
}
