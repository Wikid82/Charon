package dbmaint

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/util"
)

// ErrNotIncremental is returned by Drain for a database that is not in
// incremental auto-vacuum mode, where PRAGMA incremental_vacuum does nothing.
var ErrNotIncremental = errors.New("database is not in incremental auto_vacuum mode")

// DrainOptions tunes one drain pass. Zero values select the package defaults.
type DrainOptions struct {
	// PagesPerStep is N of PRAGMA incremental_vacuum(N).
	PagesPerStep int
	// StepPause is the yield between steps; negative disables it.
	StepPause time.Duration
	// Budget is the maximum wall time of the pass.
	Budget time.Duration
	// KeepFreeBytes is the free-list floor the pass leaves in place; negative
	// drains everything.
	KeepFreeBytes int64
}

func (o DrainOptions) withDefaults() DrainOptions {
	if o.PagesPerStep <= 0 {
		o.PagesPerStep = DrainPagesPerStep
	}
	if o.StepPause == 0 {
		o.StepPause = DrainStepPause
	}
	if o.Budget <= 0 {
		o.Budget = DrainBudgetPerPass
	}
	if o.KeepFreeBytes == 0 {
		o.KeepFreeBytes = KeepFreeBytes
	}
	return o
}

// DrainResult summarizes a drain pass.
type DrainResult struct {
	Steps int
	// PagesFreed is measured by the page_count delta of each step.
	PagesFreed int64
	// BudgetExhausted is true when the pass stopped because Budget ran out.
	BudgetExhausted bool
	// Checkpointed is true when the final wal_checkpoint(TRUNCATE) completed
	// (a busy checkpoint is left for the next pass).
	Checkpointed bool
}

// Drain returns free pages to the OS in bounded PRAGMA incremental_vacuum steps
// on a database in incremental mode, then truncates the WAL so the file shrinks.
//
// Every step is issued with QueryContext and ALL of its rows are iterated: on
// this driver Exec steps the statement once and frees a single page per call
// (driver_behavior_test.go). The rows are closed before the next pool query, because with
// a one-connection pool open rows block every other statement. The freed-page
// sanity check is warn-and-continue only. It compares page_count, not
// freelist_count, since concurrent inserts reuse free pages.
func Drain(ctx context.Context, q Querier, opts DrainOptions) (DrainResult, error) {
	opts = opts.withDefaults()
	var res DrainResult

	mode, err := readPragma(ctx, q, "auto_vacuum")
	if err != nil {
		return res, err
	}
	if mode != AutoVacuumIncremental {
		return res, ErrNotIncremental
	}
	pageSize, err := readPragma(ctx, q, "page_size")
	if err != nil {
		return res, err
	}
	keepPages := int64(0)
	if opts.KeepFreeBytes > 0 && pageSize > 0 {
		keepPages = opts.KeepFreeBytes / pageSize
	}

	deadline := time.Now().Add(opts.Budget)
	for {
		if err := ctx.Err(); err != nil {
			return res, fmt.Errorf("drain: %w", err)
		}
		if !time.Now().Before(deadline) {
			res.BudgetExhausted = true
			break
		}

		free, err := readPragma(ctx, q, "freelist_count")
		if err != nil {
			return res, err
		}
		n := min(int64(opts.PagesPerStep), free-keepPages)
		if n <= 0 {
			break
		}

		freed, err := drainStep(ctx, q, n)
		if err != nil {
			return res, err
		}
		res.Steps++
		res.PagesFreed += freed
		if freed < n/2 {
			logger.Log().WithField("requested_pages", n).WithField("freed_pages", freed).
				Warn("incremental vacuum freed fewer pages than requested; the SQLite driver may have changed")
		}

		if opts.StepPause > 0 {
			if err := util.SleepContext(ctx, opts.StepPause); err != nil {
				return res, fmt.Errorf("drain: %w", err)
			}
		}
	}

	if res.PagesFreed > 0 {
		busy, err := checkpointTruncate(ctx, q)
		if err != nil {
			return res, err
		}
		res.Checkpointed = !busy
		if busy {
			logger.Log().Debug("wal checkpoint busy after drain; the next pass finishes the shrink")
		}
	}
	return res, nil
}

// drainStep runs one PRAGMA incremental_vacuum(n) and returns the pages freed,
// measured by the page_count delta.
func drainStep(ctx context.Context, q Querier, n int64) (int64, error) {
	before, err := readPragma(ctx, q, "page_count")
	if err != nil {
		return 0, err
	}
	rows, err := q.QueryContext(ctx, fmt.Sprintf("PRAGMA incremental_vacuum(%d)", n))
	if err != nil {
		return 0, fmt.Errorf("incremental_vacuum: %w", err)
	}
	for rows.Next() {
		// Each row is one freed page; the work happens while iterating.
	}
	iterErr := rows.Err()
	closeErr := rows.Close()
	if stepErr := errors.Join(iterErr, closeErr); stepErr != nil {
		return 0, fmt.Errorf("incremental_vacuum: %w", stepErr)
	}
	after, err := readPragma(ctx, q, "page_count")
	if err != nil {
		return 0, err
	}
	return before - after, nil
}

func readPragma(ctx context.Context, q Querier, name string) (int64, error) {
	var v int64
	if err := q.QueryRowContext(ctx, "PRAGMA "+name).Scan(&v); err != nil {
		return 0, fmt.Errorf("read %s: %w", name, err)
	}
	return v, nil
}

// checkpointTruncate runs wal_checkpoint(TRUNCATE) and reports whether a reader
// blocked it. SQLite signals that through the busy column of the result row;
// the error stays nil, so the column is what must be read.
func checkpointTruncate(ctx context.Context, q Querier) (busy bool, err error) {
	var busyCol, logFrames, checkpointed int64
	if err := q.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busyCol, &logFrames, &checkpointed); err != nil {
		return false, fmt.Errorf("wal_checkpoint: %w", err)
	}
	return busyCol != 0 || checkpointed < logFrames, nil
}
