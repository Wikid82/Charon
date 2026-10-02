package dbmaint

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Wikid82/charon/backend/internal/database"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/util"
)

// Reasons a run ends without converting, in addition to those Decide produces.
const (
	ReasonIntegrityCheckFailed Reason = "integrity_check_failed"
	ReasonDatabaseBusy         Reason = "database_busy"
	ReasonCaddyNotReady        Reason = "caddy_not_ready"
	ReasonStartupTimeout       Reason = "startup_timeout"
	ReasonShuttingDown         Reason = "shutting_down"
	ReasonConversionFailed     Reason = "conversion_failed"
	ReasonInternalError        Reason = "internal_error"
)

// ErrNoConvert is returned when a run is planned but no conversion is wired.
var ErrNoConvert = errors.New("no conversion function configured")

// ConvertFunc performs the conversion on the pinned connection. It returns nil
// when the database is in incremental mode, an error otherwise. Checkpointing
// and settings writes belong to Run.
type ConvertFunc func(ctx context.Context, conn *sql.Conn) error

// QuickCheckFunc exposes the boot quick_check (database.QuickCheckStatus).
type QuickCheckFunc func() (done <-chan struct{}, result func() string)

// CheckpointFunc runs wal_checkpoint(TRUNCATE) and reports whether a reader
// blocked it.
type CheckpointFunc func(ctx context.Context, q Querier) (busy bool, err error)

// ProbeFunc checks that no other writer holds the database.
type ProbeFunc func(ctx context.Context, conn *sql.Conn) error

// Timings bounds every wait of a run. Zero fields take the package defaults.
type Timings struct {
	PlannedMaxWait       time.Duration
	QuickCheckMaxWait    time.Duration
	ConnAcquireTimeout   time.Duration
	ProbeRetries         int
	ProbeRetryGap        time.Duration
	MarkerWriteTimeout   time.Duration
	CheckpointRetries    int
	CheckpointBackoff    time.Duration
	CheckpointBackoffCap time.Duration
}

func (t Timings) withDefaults() Timings {
	if t.PlannedMaxWait <= 0 {
		t.PlannedMaxWait = PlannedMaxWait
	}
	if t.QuickCheckMaxWait <= 0 {
		t.QuickCheckMaxWait = QuickCheckMaxWait
	}
	if t.ConnAcquireTimeout <= 0 {
		t.ConnAcquireTimeout = ConnAcquireTimeout
	}
	if t.ProbeRetries <= 0 {
		t.ProbeRetries = ProbeRetries
	}
	if t.ProbeRetryGap <= 0 {
		t.ProbeRetryGap = ProbeRetryGap
	}
	if t.MarkerWriteTimeout <= 0 {
		t.MarkerWriteTimeout = MarkerWriteTimeout
	}
	if t.CheckpointRetries <= 0 {
		t.CheckpointRetries = CheckpointRetries
	}
	if t.CheckpointBackoff <= 0 {
		t.CheckpointBackoff = CheckpointBackoff
	}
	if t.CheckpointBackoffCap <= 0 {
		t.CheckpointBackoffCap = CheckpointBackoffCap
	}
	return t
}

// Deps is everything Run needs. Convert, QuickCheck, Checkpoint, Probe and Now
// are seams: production passes the real ones, tests pass fakes.
type Deps struct {
	DB         *sql.DB
	DBPath     string
	Gate       *Gate
	Convert    ConvertFunc
	QuickCheck QuickCheckFunc
	Checkpoint CheckpointFunc
	Probe      ProbeFunc
	Now        func() time.Time
	// BootTime is the process start, stored with the in-progress marker.
	BootTime time.Time
	Timings  Timings
}

func (d Deps) withDefaults() Deps {
	if d.Checkpoint == nil {
		d.Checkpoint = checkpointTruncate
	}
	if d.Probe == nil {
		d.Probe = ProbeWriterLock
	}
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.BootTime.IsZero() {
		d.BootTime = d.Now()
	}
	d.Timings = d.Timings.withDefaults()
	return d
}

// Outcome is what a run did.
type Outcome struct {
	Result      Result
	Reason      Reason
	BytesBefore int64
	BytesAfter  int64
	Err         error
	// countFailure marks a failed attempt that counts toward MaxConvertAttempts:
	// a conversion that ran and failed, or a failure before it could start.
	countFailure bool
	// keepFlag leaves the user's reclaim request set although the failure is
	// counted: a failure before the conversion started consumed nothing, and
	// the back-off bounds the retries.
	keepFlag bool
}

// runner carries the state of one Run call.
type runner struct {
	d      Deps
	fileID string
	store  *Store
	// before is the database statistics read on the pinned connection just
	// ahead of the conversion; the file-size verification compares against it.
	before Stats
}

// Run is the maintenance goroutine body: it waits for the two readiness
// signals and the boot quick_check (all inside the planned phase, where the UI
// is served normally), takes the pool's only connection, converts, and settles
// the persisted state. It never returns an error and never panics: every exit
// path releases the gate and signals the shutdown waiter.
func Run(ctx context.Context, d Deps) (out Outcome) {
	d = d.withDefaults()
	defer d.Gate.RunnerExited()
	defer d.Gate.Release()
	defer func() {
		if rec := recover(); rec != nil {
			logger.Log().WithField("panic", rec).
				Error("database maintenance: recovered from a panic; continuing without optimization")
			out = Outcome{Result: ResultFailed, Reason: ReasonInternalError, Err: fmt.Errorf("panic: %v", rec)}
			d.Gate.Finish(FinishInfo{Phase: PhaseFailed, Reason: ReasonInternalError})
		}
	}()

	r := &runner{d: d, store: NewStore(d.DB)}
	if id, err := FileID(d.DBPath); err != nil {
		logger.Log().WithError(err).Warn("database maintenance: could not identify the database file")
	} else {
		r.fileID = id
	}
	return r.run(ctx)
}

func (r *runner) run(ctx context.Context) Outcome {
	if out, stop := r.awaitReady(ctx); stop {
		return out
	}
	if out, stop := r.awaitQuickCheck(ctx); stop {
		return out
	}
	conn, out, stop := r.acquireConn(ctx)
	if stop {
		return out
	}
	return r.convert(ctx, conn)
}

// awaitReady waits, in planned, for the config-applied and listener-bound
// signals.
func (r *runner) awaitReady(ctx context.Context) (Outcome, bool) {
	err := r.d.Gate.AwaitReady(ctx, r.d.Timings.PlannedMaxWait)
	switch {
	case err == nil:
		return Outcome{}, false
	case errors.Is(err, ErrConfigNotApplied):
		return r.settle(ctx, Outcome{Result: ResultSkipped, Reason: ReasonCaddyNotReady}), true
	case errors.Is(err, ErrStartupTimeout):
		return r.settle(ctx, Outcome{Result: ResultSkipped, Reason: ReasonStartupTimeout}), true
	default: // context cancelled, or the gate was released elsewhere
		return r.settle(ctx, Outcome{Result: ResultCancelled, Reason: ReasonShuttingDown}), true
	}
}

// awaitQuickCheck waits, still in planned, for the boot quick_check. A timeout
// proceeds (the checkpoint retry covers a still-running reader); a corrupt
// database is skipped.
func (r *runner) awaitQuickCheck(ctx context.Context) (Outcome, bool) {
	if r.d.QuickCheck == nil {
		return Outcome{}, false
	}
	done, result := r.d.QuickCheck()
	if done == nil {
		return Outcome{}, false
	}

	timer := time.NewTimer(r.d.Timings.QuickCheckMaxWait)
	defer timer.Stop()
	select {
	case <-done:
		if verdict := result(); verdict != "" && verdict != database.QuickCheckOK {
			return r.settle(ctx, Outcome{Result: ResultSkipped, Reason: ReasonIntegrityCheckFailed}), true
		}
	case <-timer.C:
		logger.Log().Info("database maintenance: the boot integrity check is still running; continuing")
	case <-ctx.Done():
		return r.settle(ctx, Outcome{Result: ResultCancelled, Reason: ReasonShuttingDown}), true
	}
	return Outcome{}, false
}

// acquireConn flips the gate to checking (503), takes the pool's only
// connection within ConnAcquireTimeout and probes for another writer. The
// connection is held between probe retries so the gate state stays truthful.
func (r *runner) acquireConn(ctx context.Context) (*sql.Conn, Outcome, bool) {
	if err := r.d.Gate.BeginChecking(); err != nil {
		return nil, r.settle(ctx, Outcome{Result: ResultCancelled, Reason: ReasonShuttingDown}), true
	}

	acquireCtx, cancel := context.WithTimeout(ctx, r.d.Timings.ConnAcquireTimeout)
	conn, err := r.d.DB.Conn(acquireCtx)
	cancel()
	if err != nil {
		if ctx.Err() != nil {
			return nil, r.settle(ctx, Outcome{Result: ResultCancelled, Reason: ReasonShuttingDown}), true
		}
		logger.Log().WithError(err).Warn("database maintenance: could not get a database connection in time; " +
			"a connection may have been leaked. Optimization postponed")
		return nil, r.settle(ctx, Outcome{Result: ResultSkipped, Reason: ReasonDatabaseBusy}), true
	}

	for attempt := 0; ; attempt++ {
		err = r.d.Probe(ctx, conn)
		switch {
		case err == nil:
			return conn, Outcome{}, false
		case ctx.Err() != nil:
			_ = conn.Close()
			return nil, r.settle(ctx, Outcome{Result: ResultCancelled, Reason: ReasonShuttingDown}), true
		case !errors.Is(err, ErrWriterBusy):
			_ = conn.Close()
			return nil, r.settle(ctx, Outcome{
				Result: ResultFailed, Reason: ReasonConversionFailed, Err: err, countFailure: true, keepFlag: true,
			}), true
		case attempt >= r.d.Timings.ProbeRetries:
			_ = conn.Close()
			return nil, r.settle(ctx, Outcome{Result: ResultSkipped, Reason: ReasonDatabaseBusy}), true
		}
		if sleepErr := util.SleepContext(ctx, r.d.Timings.ProbeRetryGap); sleepErr != nil {
			_ = conn.Close()
			return nil, r.settle(ctx, Outcome{Result: ResultCancelled, Reason: ReasonShuttingDown}), true
		}
	}
}

// convert runs the conversion on the pinned connection and settles the result.
// The connection is closed BEFORE any settings write: with a one-connection
// pool a write issued while it is held would block until its timeout and be
// lost.
func (r *runner) convert(ctx context.Context, conn *sql.Conn) Outcome {
	out := Outcome{BytesBefore: sizeOnDisk(r.d.DBPath)}
	if err := r.d.Gate.BeginConverting(out.BytesBefore); err != nil {
		_ = conn.Close()
		out.Result, out.Reason = ResultCancelled, ReasonShuttingDown
		return r.settle(ctx, out)
	}
	if r.d.Convert == nil {
		_ = conn.Close()
		out.Result, out.Reason, out.Err = ResultFailed, ReasonConversionFailed, ErrNoConvert
		return r.settle(ctx, out)
	}

	stats, err := Inspect(ctx, conn, r.d.DBPath)
	if err != nil {
		return r.abortBeforeConversion(ctx, conn, out, fmt.Errorf("inspect before conversion: %w", err))
	}
	r.before = stats

	logger.Log().Warn("database optimization started; the management UI, API and emergency server return 503 until it finishes; " +
		"set CHARON_DB_COMPACT_ON_START=off and restart to skip optimization and regain break-glass access")

	// The marker is written through the pinned connection: the pool has no other.
	if err := NewStore(conn).SetInProgress(ctx, r.fileID, r.d.BootTime); err != nil {
		return r.abortBeforeConversion(ctx, conn, out, err)
	}

	convErr := r.d.Convert(ctx, conn)
	_ = conn.Close()

	switch {
	case convErr == nil && ctx.Err() == nil:
		return r.settleConverted(ctx, out)
	case convErr == nil:
		// VACUUM's copy-back tail is not interruptible: the conversion finished
		// although the context was cancelled. Decide by the result, not ctx.Err().
		return r.settleConvertedAfterCancel(ctx, out)
	case ctx.Err() != nil:
		out.Result, out.Reason, out.Err = ResultInterrupted, ReasonShuttingDown, convErr
		return r.settle(ctx, out)
	default:
		out.Result, out.Reason, out.Err, out.countFailure = ResultFailed, ReasonConversionFailed, convErr, true
		return r.settle(ctx, out)
	}
}

// abortBeforeConversion ends a run that failed before VACUUM started. A failure
// while the application is shutting down is a stop, not a failed attempt, and
// a writer that slipped in is a busy skip to retry; any other failure counts
// but keeps the user's request (the back-off bounds the retries).
func (r *runner) abortBeforeConversion(ctx context.Context, conn *sql.Conn, out Outcome, err error) Outcome {
	_ = conn.Close()
	switch {
	case ctx.Err() != nil:
		out.Result, out.Reason = ResultCancelled, ReasonShuttingDown
	case isWriterBusy(err):
		out.Result, out.Reason, out.Err = ResultSkipped, ReasonDatabaseBusy, err
	default:
		out.Result, out.Reason, out.Err = ResultFailed, ReasonConversionFailed, err
		out.countFailure, out.keepFlag = true, true
	}
	return r.settle(ctx, out)
}

// settleConverted finishes a normal conversion: a bounded checkpoint retry
// first (the gate stays active meanwhile), then the settings.
func (r *runner) settleConverted(ctx context.Context, out Outcome) Outcome {
	out.Result = ResultConverted
	if !r.checkpointWithRetry(ctx) {
		out.Result = ResultConvertedPendingCheckpoint
	}
	return r.settle(ctx, r.verifyShrink(out))
}

// settleConvertedAfterCancel finishes a conversion that completed after the
// context was cancelled. Order matters (the runner wait at shutdown is short):
// last_result and the marker clear are written FIRST, then a single checkpoint
// attempt without backoff. A checkpoint that never returns cannot lose them.
func (r *runner) settleConvertedAfterCancel(ctx context.Context, out Outcome) Outcome {
	out.Result = ResultConverted
	out.BytesAfter = sizeOnDisk(r.d.DBPath)
	r.persist(ctx, out)

	wctx, cancel := r.detached(ctx)
	defer cancel()
	busy, err := r.d.Checkpoint(wctx, r.d.DB)
	if err != nil || busy {
		out.Result = ResultConvertedPendingCheckpoint
	}
	return r.settle(ctx, r.verifyShrink(out))
}

// verifyShrink fills BytesAfter and checks the result by FILE SIZE, not by a
// pragma: in WAL mode the main file only shrinks at a completed checkpoint. A
// conversion whose file did not shrink as far as the free pages predicted is
// durable but not reported as converted; the pruner's later checkpoint finishes
// the shrink.
func (r *runner) verifyShrink(out Outcome) Outcome {
	out.BytesAfter = sizeOnDisk(r.d.DBPath)
	if out.Result != ResultConverted {
		return out
	}
	mainBytes, _, err := fileSizes(r.d.DBPath)
	if err != nil {
		logger.Log().WithError(err).Warn("database optimization finished but the file size could not be verified")
		out.Result = ResultConvertedPendingCheckpoint
		return out
	}
	if limit := r.maxMainBytesAfter(); mainBytes > limit {
		logger.Log().WithField("main_bytes", mainBytes).WithField("expected_at_most", limit).
			Warn("database optimization finished but the file has not shrunk as far as expected; " +
				"the next checkpoint will finish the shrink")
		out.Result = ResultConvertedPendingCheckpoint
	}
	return out
}

// maxMainBytesAfter is the largest main-file size that still counts as shrunk:
// the live data plus the part of the reclaimable space the conversion failed to
// return, tolerated up to 1-MinShrinkFraction of it.
func (r *runner) maxMainBytesAfter() int64 {
	return r.before.LiveBytes() + int64(float64(r.before.ReclaimableBytes())*(1-MinShrinkFraction))
}

// checkpointWithRetry truncates the WAL, retrying with a doubling backoff while
// a reader pins it. It reports whether the WAL was truncated.
func (r *runner) checkpointWithRetry(ctx context.Context) bool {
	t := r.d.Timings
	backoff := t.CheckpointBackoff
	for attempt := 0; ; attempt++ {
		busy, err := r.d.Checkpoint(ctx, r.d.DB)
		if err == nil && !busy {
			return true
		}
		if err != nil {
			logger.Log().WithError(err).Debug("database maintenance: wal checkpoint failed")
		}
		if attempt >= t.CheckpointRetries {
			return false
		}
		if util.SleepContext(ctx, backoff) != nil {
			return false
		}
		backoff = min(backoff*2, t.CheckpointBackoffCap)
	}
}

// detached returns a context that survives the cancellation of ctx, for the
// final writes of a shutdown, with MarkerWriteTimeout as its bound.
func (r *runner) detached(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), r.d.Timings.MarkerWriteTimeout)
}

// persist writes last_result, then clears the in-progress marker. A failed
// conversion also counts as an attempt; a conversion stopped by shutdown counts
// as an interruption, which has its own, higher limit. Failures are logged, never returned.
func (r *runner) persist(ctx context.Context, out Outcome) {
	if out.Result == ResultCancelled {
		return
	}
	wctx, cancel := r.detached(ctx)
	defer cancel()

	// The counter is written BEFORE last_result and the marker clear: a kill in
	// between leaves the marker, which the next boot counts as one failed
	// attempt (a rare double count, never a missed one).
	var errs error
	switch {
	case out.countFailure:
		_, failErr := r.store.RecordFailure(wctx, r.fileID)
		errs = errors.Join(errs, failErr)
	case out.Result == ResultInterrupted:
		_, intErr := r.store.RecordInterruption(wctx, r.fileID)
		errs = errors.Join(errs, intErr)
	}
	errs = errors.Join(errs,
		r.store.WriteLastResult(wctx, LastResult{
			At:          r.d.Now().UTC(),
			Outcome:     out.Result,
			Reason:      out.Reason,
			BytesBefore: out.BytesBefore,
			BytesAfter:  out.BytesAfter,
			FileID:      r.fileID,
		}),
		r.store.ClearInProgress(wctx),
	)
	if out.Result.Converted() {
		// A success ends the streak of failures.
		errs = errors.Join(errs, r.store.ResetAttempts(wctx))
	}
	if out.consumesFlag() {
		errs = errors.Join(errs, r.store.ClearFlag(wctx))
	} else if out.keepsRequestWhileCounted() {
		errs = errors.Join(errs, r.dropRequestIfBackedOff(wctx))
	}
	if errs != nil {
		logger.Log().WithError(errs).Warn("database maintenance: could not record the result")
	}
}

// dropRequestIfBackedOff clears the user's request once the failure budget is
// exhausted. A failure that keeps the request leaves it set for a retry, but
// when this was the last allowed one the next boot would only discard it; until
// then the Database page would show "scheduled" next to the stopped notice.
func (r *runner) dropRequestIfBackedOff(ctx context.Context) error {
	st, err := r.store.Peek(ctx, r.fileID)
	if err != nil || !BackedOff(st.Attempts, st.Interruptions) {
		return err
	}
	return r.store.ClearFlag(ctx)
}

// settle records the outcome: persisted state, gate release and log line.
func (r *runner) settle(ctx context.Context, out Outcome) Outcome {
	r.persist(ctx, out)
	r.d.Gate.Finish(gateOutcome(out))
	logOutcome(out)
	return out
}

// consumesFlag reports whether the user's "reclaim on next restart" request is
// used up: a conversion worked, or it ran and failed (counted). A run that
// merely skipped, was interrupted or failed before the conversion started
// (keepFlag) leaves the request set so the next start retries it, unless that
// failure used up the budget (dropRequestIfBackedOff).
func (o Outcome) consumesFlag() bool { return o.Result.Converted() || (o.countFailure && !o.keepFlag) }

// keepsRequestWhileCounted reports whether the run counted against a failure
// budget but left the user's request set (the budget decides about a retry).
func (o Outcome) keepsRequestWhileCounted() bool {
	return o.Result == ResultInterrupted || (o.countFailure && o.keepFlag)
}

func gateOutcome(out Outcome) FinishInfo {
	info := FinishInfo{Reason: out.Reason, BytesAfter: out.BytesAfter}
	switch {
	case out.Result.Converted():
		info.Phase = PhaseDone
	case out.Result == ResultFailed:
		info.Phase = PhaseFailed
	default:
		info.Phase = PhaseSkipped
	}
	return info
}

func logOutcome(out Outcome) {
	entry := logger.Log().WithField("result", string(out.Result))
	if out.Reason != "" {
		entry = entry.WithField("reason", string(out.Reason))
	}
	switch {
	case out.Result.Converted():
		entry.WithField("bytes_before", out.BytesBefore).WithField("bytes_after", out.BytesAfter).
			Info("database optimization finished")
	case out.Result == ResultFailed:
		entry.WithError(out.Err).Error("database optimization failed; it is retried on the next start unless it has failed 3 times")
	case out.Result == ResultCancelled || out.Result == ResultInterrupted:
		entry.Info("database optimization stopped by shutdown")
	default:
		entry.Warn("database optimization skipped")
	}
}
