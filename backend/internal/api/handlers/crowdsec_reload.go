package handlers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
	"github.com/Wikid82/charon/backend/internal/logger"
)

// Reload timing. The engine arms its SIGHUP handler about one second after it starts, and a SIGHUP that arrives
// before that ends the process. engineSettleWindow (5 s) keeps a wide margin over that second. reloadMinInterval
// spaces out the signals the engine has to process.
const (
	engineSettleWindow = 5 * time.Second
	reloadMinInterval  = 5 * time.Second
	// reloadPendingBound limits how long a queued reload waits for an engine whose LAPI never answers.
	reloadPendingBound = 60 * time.Second
	// reloadPollInterval is how often a queued reload checks whether the engine is ready.
	reloadPollInterval = 250 * time.Millisecond
	// reloadProbeTimeout caps the LAPI readiness probe.
	reloadProbeTimeout = 2 * time.Second
)

// engineAger is implemented by executors that can say how long an engine has been running. Executors without it
// (test fakes) are treated as old enough.
type engineAger interface {
	EngineAge(pid int) (age time.Duration, known bool)
}

// reloadDeps are the collaborators of a crowdsecReloader. The handler wires them to its executor, its signal
// function and its LAPI probe; tests replace them.
type reloadDeps struct {
	// status reports whether the managed engine runs and its pid.
	status func(ctx context.Context) (running bool, pid int, err error)
	// signal delivers a signal to the engine.
	signal func(pid int, sig syscall.Signal) error
	// age reports how long the engine has run; nil or known=false counts as old enough.
	age func(pid int) (time.Duration, bool)
	// probe reports whether LAPI answers.
	probe func(ctx context.Context) bool
	// now is the clock; tests replace it.
	now func() time.Time
	// appCtx returns the application context. Cancelling it ends queued reloads.
	appCtx func() context.Context
}

// crowdsecReloader is the only sender of SIGHUP to the managed CrowdSec engine. A reload request is delivered at
// once when the engine is ready. Otherwise one reload is queued and delivered by a tracked goroutine as soon as
// the engine is ready, so a signal never reaches an engine that has not armed its handler yet.
type crowdsecReloader struct {
	deps        reloadDeps
	settle      time.Duration
	minInterval time.Duration
	bound       time.Duration
	poll        time.Duration

	// mu guards every field below, so a request and the queued delivery decide together.
	mu         sync.Mutex
	pending    bool
	lastSignal time.Time
	// The readiness probe result is cached per engine for one settle window.
	probedPid int
	probedAt  time.Time
	probedOK  bool

	wg sync.WaitGroup
}

func newCrowdsecReloader(deps reloadDeps) *crowdsecReloader {
	return &crowdsecReloader{
		deps:        deps,
		settle:      engineSettleWindow,
		minInterval: reloadMinInterval,
		bound:       reloadPendingBound,
		poll:        reloadPollInterval,
	}
}

// reload asks the engine to reload its configuration. It returns crowdsec.ErrCrowdSecNotRunning when no engine
// runs, nil after a signal the engine received now, crowdsec.ErrReloadDeferred when the reload is queued, and the
// signal's error otherwise.
func (r *crowdsecReloader) reload(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	running, pid, err := r.deps.status(ctx)
	if err != nil {
		return fmt.Errorf("crowdsec status: %w", err)
	}
	if !running || pid <= 0 {
		return crowdsec.ErrCrowdSecNotRunning
	}
	if r.pending {
		return crowdsec.ErrReloadDeferred
	}
	if r.ready(ctx, pid) {
		return r.signalLocked(pid)
	}

	r.pending = true
	r.wg.Add(1)
	go r.deliver(r.deps.appCtx(), r.deps.now().Add(r.bound))
	return crowdsec.ErrReloadDeferred
}

// ready reports whether a signal to pid can be delivered now: the engine has run for the settle window, the
// minimum interval since the last signal has passed, and LAPI answers. Callers hold r.mu.
func (r *crowdsecReloader) ready(ctx context.Context, pid int) bool {
	if age, known := r.ageOf(pid); known && age < r.settle {
		return false
	}
	now := r.deps.now()
	if !r.lastSignal.IsZero() && now.Sub(r.lastSignal) < r.minInterval {
		return false
	}
	return r.probeLocked(ctx, pid, now)
}

// ageOf returns the engine's age when the executor can tell it.
func (r *crowdsecReloader) ageOf(pid int) (time.Duration, bool) {
	if r.deps.age == nil {
		return 0, false
	}
	return r.deps.age(pid)
}

// probeLocked answers the LAPI probe from the cache while it is inside the settle window for pid, and probes
// otherwise. Callers hold r.mu.
func (r *crowdsecReloader) probeLocked(ctx context.Context, pid int, now time.Time) bool {
	if r.probedPid == pid && !r.probedAt.IsZero() && now.Sub(r.probedAt) < r.settle {
		return r.probedOK
	}
	ok := r.deps.probe(ctx)
	r.probedPid, r.probedAt, r.probedOK = pid, now, ok
	return ok
}

// signalLocked sends SIGHUP and records the time, so the minimum interval also counts a failed attempt. Callers
// hold r.mu.
func (r *crowdsecReloader) signalLocked(pid int) error {
	r.lastSignal = r.deps.now()
	return r.deps.signal(pid, syscall.SIGHUP)
}

// deliver waits for the queued reload and sends it once the engine is ready. It ends when the reload is sent,
// dropped (the engine exited, or the bound passed), or cancelled by the application context.
func (r *crowdsecReloader) deliver(appCtx context.Context, deadline time.Time) {
	defer r.wg.Done()
	ticker := time.NewTicker(r.poll)
	defer ticker.Stop()
	for {
		select {
		case <-appCtx.Done():
			r.dropPending()
			logger.Log().Info("queued CrowdSec reload cancelled at shutdown")
			return
		case <-ticker.C:
		}
		if r.step(appCtx, deadline) {
			return
		}
	}
}

// step runs one check of a queued reload and reports whether the reload is finished.
func (r *crowdsecReloader) step(ctx context.Context, deadline time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	running, pid, err := r.deps.status(ctx)
	if err == nil && !running {
		r.pending = false
		logger.Log().Info("CrowdSec exited before the queued reload was delivered; the reload was dropped")
		return true
	}
	if err == nil && r.ready(ctx, pid) {
		sigErr := r.signalLocked(pid)
		r.pending = false
		if sigErr != nil {
			logger.Log().WithError(sigErr).Warn("queued CrowdSec reload failed")
		} else {
			logger.Log().Info("queued CrowdSec reload delivered")
		}
		return true
	}
	if r.deps.now().After(deadline) {
		r.pending = false
		logger.Log().Warn("CrowdSec did not become ready in time; the queued reload was dropped")
		return true
	}
	return false
}

// dropPending clears the queued reload without sending it.
func (r *crowdsecReloader) dropPending() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pending = false
}

// wait blocks until every queued delivery goroutine has finished. Tests use it to end cleanly.
func (r *crowdsecReloader) wait() {
	r.wg.Wait()
}

// reloader returns the handler's reloader. It is built on first use; its dependencies read the handler's fields
// when they are called, so a test may change the executor or the signal function afterwards.
func (h *CrowdsecHandler) reloader() *crowdsecReloader {
	h.reloaderOnce.Do(func() {
		h.reloaderState = newCrowdsecReloader(reloadDeps{
			status: func(ctx context.Context) (bool, int, error) {
				return h.Executor.Status(ctx, h.DataDir)
			},
			signal: func(pid int, sig syscall.Signal) error {
				if h.signalProcess != nil {
					return h.signalProcess(pid, sig)
				}
				return signalOSProcess(pid, sig)
			},
			age: func(pid int) (time.Duration, bool) {
				ager, ok := h.Executor.(engineAger)
				if !ok {
					return 0, false
				}
				return ager.EngineAge(pid)
			},
			probe: h.lapiAnswers,
			now:   time.Now,
			appCtx: func() context.Context {
				if h.appCtx != nil {
					return h.appCtx
				}
				return context.Background()
			},
		})
	})
	return h.reloaderState
}

// lapiAnswers reports whether the engine's LAPI answers a status query, the readiness probe for a reload.
func (h *CrowdsecHandler) lapiAnswers(ctx context.Context) bool {
	if h.reloadProbe != nil {
		return h.reloadProbe(ctx)
	}
	_, err := h.runCSCLI(ctx, reloadProbeTimeout, "lapi", "status")
	return err == nil
}

// signalOSProcess sends sig to the process with the given PID.
func signalOSProcess(pid int, sig syscall.Signal) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process %d: %w", pid, err)
	}
	if err := proc.Signal(sig); err != nil {
		return fmt.Errorf("signal process %d: %w", pid, err)
	}
	return nil
}

// reloadCrowdSec asks the managed CrowdSec engine to reload its parsers, scenarios, whitelists and hub items.
// It is the only path that sends SIGHUP. It returns crowdsec.ErrCrowdSecNotRunning when no managed process is
// running, nil when the engine was signalled now, and crowdsec.ErrReloadDeferred when the engine is still starting
// and the reload is queued.
func (h *CrowdsecHandler) reloadCrowdSec(ctx context.Context) error {
	if h.Executor == nil {
		return crowdsec.ErrCrowdSecNotRunning
	}
	return h.reloader().reload(ctx)
}

// reloadAfterWhitelistChange reloads CrowdSec best-effort; failures never fail the request.
func (h *CrowdsecHandler) reloadAfterWhitelistChange(ctx context.Context, action string) {
	switch err := h.reloadCrowdSec(ctx); {
	case err == nil, errors.Is(err, crowdsec.ErrCrowdSecNotRunning):
	case errors.Is(err, crowdsec.ErrReloadDeferred):
		logger.Log().WithField("action", action).Info("crowdsec is starting; the whitelist change applies once it is ready")
	default:
		logger.Log().WithError(err).Warnf("crowdsec reload failed after whitelist %s (non-fatal)", action)
	}
}
