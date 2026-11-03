package main

import (
	"context"
	"sync"
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

// registrationDrainTimeout bounds the wait for bouncer registrations in flight at shutdown. A swap takes two
// local cscli calls, so the wait is normally empty; the bound keeps shutdown inside the container stop grace.
const registrationDrainTimeout = 30 * time.Second

// registrationWaiter is the part of the CrowdSec lifecycle the shutdown path needs.
type registrationWaiter interface {
	WaitRegistrations(ctx context.Context) error
}

// drainRegistrations waits, for at most registrationDrainTimeout, for the bouncer registrations in flight. A nil
// lifecycle has none. A wait that times out is logged; the registration is then retried on the next start.
func drainRegistrations(w registrationWaiter) {
	if w == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), registrationDrainTimeout)
	defer cancel()
	if err := w.WaitRegistrations(ctx); err != nil {
		logger.Log().WithError(err).Warn("bouncer registration did not finish before shutdown; it is retried on the next start")
	}
}

// runConcurrently runs every step at the same time and returns after all of them have returned.
func runConcurrently(steps ...func()) {
	var wg sync.WaitGroup
	for _, step := range steps {
		wg.Add(1)
		go func(run func()) {
			defer wg.Done()
			run()
		}(step)
	}
	wg.Wait()
}
