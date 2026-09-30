package main

import (
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
)

// runnerWaiter is the part of the maintenance gate the shutdown path needs.
type runnerWaiter interface {
	WaitRunner(d time.Duration) bool
}

// shutdownDrain runs the ordered shutdown steps that follow cancelling the
// application context. The database maintenance runner is waited on FIRST, for
// at most runnerWait: it needs a moment to record an interrupted or completed
// conversion and clear its in-progress marker, and nothing else may consume the
// stop grace period before that. The wait is bounded, so a runner stuck in the
// uninterruptible tail of a VACUUM cannot delay the remaining steps.
func shutdownDrain(runner runnerWaiter, runnerWait time.Duration, steps ...func()) {
	if !runner.WaitRunner(runnerWait) {
		logger.Log().Warn("database maintenance did not finish before shutdown; it is safe and will be retried on the next start")
	}
	for _, step := range steps {
		step()
	}
}
