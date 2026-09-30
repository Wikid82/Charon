// Package dbmaint keeps the SQLite database file small: it inspects the file,
// decides when a one-time conversion to incremental auto-vacuum is worthwhile,
// and returns free pages to the OS in small steps (GH #1422).
//
// The package may import internal/database; internal/database must never
// import it. internal/services talks to it only through small interfaces.
package dbmaint

import "time"

// Thresholds and tuning values. Each is unit-tested through Decide or the
// helpers that consume it.
const (
	// MinFreeRatio is the fraction of free pages at which a conversion pays off.
	MinFreeRatio = 0.20
	// MinReclaimableBytes is the hard floor: never convert for less than this.
	MinReclaimableBytes int64 = 100 << 20
	// ReclaimableTriggerBytes converts even below MinFreeRatio once this much is
	// reclaimable (the MinReclaimableBytes floor still applies).
	ReclaimableTriggerBytes int64 = 1 << 30

	// DiskSafetyMultiplier and DiskSlackBytes add headroom to the disk estimate.
	DiskSafetyMultiplier       = 1.15
	DiskSlackBytes       int64 = 64 << 20

	// DrainPagesPerStep is the page count of one PRAGMA incremental_vacuum(N).
	DrainPagesPerStep = 2000
	// DrainStepPause is the yield between drain steps.
	DrainStepPause = 50 * time.Millisecond
	// DrainBudgetPerPass is the maximum wall time of one drain pass.
	DrainBudgetPerPass = 60 * time.Second
	// KeepFreeBytes is the free-list floor a drain leaves in place.
	KeepFreeBytes int64 = 32 << 20

	// MinShrinkFraction is the share of the reclaimable space a finished
	// conversion must have removed from the main file (checked by file size)
	// before it is reported as converted rather than pending a checkpoint.
	MinShrinkFraction = 0.5

	// MaxConvertAttempts is the number of consecutive failed conversions after
	// which the boot path backs off until the user requests it again.
	MaxConvertAttempts = 3
)

// SQLite auto_vacuum modes (PRAGMA auto_vacuum).
const (
	AutoVacuumNone        = 0
	AutoVacuumIncremental = 2
)

// Bounds of the boot-time maintenance run (3.1 of the plan).
const (
	// PlannedMaxWait is the longest the gate stays planned waiting for the two
	// readiness signals. It excludes the quick_check wait.
	PlannedMaxWait = 3 * time.Minute
	// QuickCheckMaxWait is the longest the runner waits, still in planned, for
	// the boot quick_check once both signals arrived.
	QuickCheckMaxWait = 15 * time.Minute
	// ConnAcquireTimeout bounds the wait for the pool's only connection; on
	// expiry the run is skipped as database_busy instead of leaving the
	// management plane at 503 forever behind a leaked connection.
	ConnAcquireTimeout = 45 * time.Second
	// ShutdownRunnerWait is how long the shutdown path waits for the runner. It
	// is sized against Docker's 10 s default stop grace.
	ShutdownRunnerWait = 4 * time.Second
	// MarkerWriteTimeout bounds the detached final settings writes at shutdown.
	MarkerWriteTimeout = 3 * time.Second

	// ProbeRetries and ProbeRetryGap govern the writer-lock probe retries.
	ProbeRetries  = 3
	ProbeRetryGap = 20 * time.Second

	// CheckpointRetries, CheckpointBackoff and CheckpointBackoffCap bound the
	// retry of wal_checkpoint(TRUNCATE) while a reader pins the WAL.
	CheckpointRetries    = 6
	CheckpointBackoff    = 2 * time.Second
	CheckpointBackoffCap = 30 * time.Second
	busyTimeoutMillis    = 5000 // database.Connect's busy_timeout
)
