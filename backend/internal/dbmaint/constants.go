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

	// MaxConvertAttempts is the number of consecutive failed conversions after
	// which the boot path backs off until the user requests it again.
	MaxConvertAttempts = 3
)

// SQLite auto_vacuum modes (PRAGMA auto_vacuum).
const (
	AutoVacuumNone        = 0
	AutoVacuumIncremental = 2
)
