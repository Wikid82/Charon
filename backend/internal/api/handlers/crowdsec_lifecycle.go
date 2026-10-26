package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/util"
)

// engineStopTimeout bounds one Executor.Stop: the SIGTERM wait, the SIGKILL wait and the process scan.
const engineStopTimeout = 15 * time.Second

// resetStopTimeout bounds the engine stop that an emergency reset starts after its reply.
const resetStopTimeout = 15 * time.Second

// CrowdsecLifecycle is the one owner of the CrowdSec engine state. The engine process, the default row's
// crowd_sec_mode, the security.crowdsec.enabled setting and the proxy configuration change together, and only
// through it. The HTTP switches and the settings and emergency handlers call it; none of them drives the engine
// on its own. The request context passed in only supplies values: an operation does not stop when it is cancelled
// (see operationContext).
type CrowdsecLifecycle interface {
	StartCrowdSec(reqCtx context.Context) (CrowdsecStartResult, error)
	StopCrowdSec(reqCtx context.Context) (CrowdsecStopResult, error)
	// ResetCrowdSecState runs an emergency reset's writes under stateMu. Before the writes, in the same critical
	// section, it counts one pending engine stop and moves the state generation, so a Start in flight stops at its
	// next checkpoint and a later Start waits for the counted stop. Every call must be followed by exactly one
	// StopAfterReset, whether the writes succeed, fail or panic.
	ResetCrowdSecState(write func() error) error
	// StopAfterReset consumes the stop counted by ResetCrowdSecState. resetErr is the reset's result, or an error
	// when the writes did not complete. The stop runs in a tracked goroutine, so the caller never waits for it.
	StopAfterReset(resetErr error)
}

// CrowdsecStartResult is the outcome of a Start that did not fail.
type CrowdsecStartResult struct {
	Status       string // "started" or "already_running"
	PID          int
	LAPIReady    bool
	Warning      string // set when the engine started but LAPI was not ready in time
	ProxyApplied bool   // the real outcome of the one proxy apply
}

// CrowdsecStopResult is the outcome of a Stop that did not fail.
type CrowdsecStopResult struct {
	ProxyApplied bool
}

var _ CrowdsecLifecycle = (*CrowdsecHandler)(nil)

// CrowdsecBusyError reports that a Start or Stop was refused because another change holds the engine. JobID names
// the background install when one is the cause; it is empty for every other holder, and for a second Start or Stop.
type CrowdsecBusyError struct {
	JobID string
}

func (e *CrowdsecBusyError) Error() string {
	return "crowdsec is busy"
}

// msgCrowdsecUnavailable is the answer when no lifecycle is wired to a switch, so the switch cannot drive the engine.
const msgCrowdsecUnavailable = "CrowdSec is not available on this server."

// Sentinel causes of a failed Start or Stop. They are wrapped with the underlying error and mapped to HTTP answers
// by respondCrowdsecLifecycleError, so every caller answers the same way.
var (
	errCrowdsecStatus  = errors.New("read crowdsec status")
	errCrowdsecPersist = errors.New("persist crowdsec configuration")
	errCrowdsecStart   = errors.New("start crowdsec")
	errCrowdsecStop    = errors.New("stop crowdsec")
	// errCrowdsecStateChanged reports that an emergency reset turned CrowdSec off while a Start was running. The Start
	// stopped the engine it had begun and kept the reset's values.
	errCrowdsecStateChanged = errors.New("crowdsec was turned off during start")
)

// respondCrowdsecLifecycleError maps an error from StartCrowdSec or StopCrowdSec to its HTTP answer.
func respondCrowdsecLifecycleError(c *gin.Context, err error) {
	var busy *CrowdsecBusyError
	switch {
	case errors.As(err, &busy):
		respondBusy(c, busy.JobID)
	case errors.Is(err, errCrowdsecStatus):
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read CrowdSec status"})
	case errors.Is(err, errCrowdsecPersist):
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to persist configuration"})
	case errors.Is(err, errCrowdsecStart):
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start CrowdSec"})
	case errors.Is(err, errCrowdsecStop):
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to stop CrowdSec"})
	case errors.Is(err, errCrowdsecStateChanged):
		respondCoded(c, http.StatusConflict, "crowdsec_state_changed", "CrowdSec was turned off while it was starting", nil)
	default:
		logger.Log().WithError(err).Error("CrowdSec lifecycle request failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "CrowdSec request failed"})
	}
}

// operationContext returns the context one Start or Stop runs on, and the func that releases it. The context does
// not follow reqCtx, so a client that leaves never cuts an operation in half. It ends when the application context
// is cancelled at shutdown, which is the one signal that stops a wait early.
func (h *CrowdsecHandler) operationContext(reqCtx context.Context) (ctx context.Context, release func()) {
	ctx, cancel := context.WithCancel(context.WithoutCancel(reqCtx))
	appCtx := h.appCtx
	if appCtx == nil {
		appCtx = context.Background()
	}
	stopWatch := context.AfterFunc(appCtx, cancel)
	return ctx, func() {
		stopWatch()
		cancel()
	}
}

// LifecycleOrNil returns nil for a nil interface or for a nil *CrowdsecHandler stored in one, so a handler that
// was never built is detectable as not wired. It is a type switch rather than reflection, so no kind can panic.
func LifecycleOrNil(l CrowdsecLifecycle) CrowdsecLifecycle {
	switch h := l.(type) {
	case nil:
		return nil
	case *CrowdsecHandler:
		if h == nil {
			return nil
		}
	}
	return l
}

// lifecycleHooks are test seams that run at fixed points of a Start. Production leaves every field nil.
type lifecycleHooks struct {
	afterStatus        func() // after the status read, before the first write
	afterPersist       func() // after the persist write group, before the already_running check and the engine start
	afterFailedPersist func() // after a failed persist write group, before the revert
	duringLAPIWait     func() // at the top of each LAPI poll, before the poll's state check
	inWrite            func() // inside writeIfCurrent, with stateMu held
}

// runHook calls a test seam when it is set.
func runHook(hook func()) {
	if hook != nil {
		hook()
	}
}

// resetStopTracker counts the engine stops that emergency resets started and that have not finished. It is a
// mutex-guarded counter rather than a sync.WaitGroup: begin may run while a wait is blocked (the Tier 2 server keeps
// serving during shutdown), which a WaitGroup forbids. The zero value is ready to use and has nothing pending.
type resetStopTracker struct {
	mu      sync.Mutex
	pending int
	idle    chan struct{} // open while pending is above zero; closed when it returns to zero
}

// begin counts one pending stop.
func (t *resetStopTracker) begin() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.pending == 0 {
		t.idle = make(chan struct{})
	}
	t.pending++
}

// done releases one pending stop.
func (t *resetStopTracker) done() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.pending == 0 {
		return
	}
	t.pending--
	if t.pending == 0 {
		close(t.idle)
	}
}

// wait returns once no stop is pending, or with ctx's error when ctx ends first.
func (t *resetStopTracker) wait(ctx context.Context) error {
	t.mu.Lock()
	if t.pending == 0 {
		t.mu.Unlock()
		return nil
	}
	idle := t.idle
	t.mu.Unlock()

	select {
	case <-idle:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// StartCrowdSec starts the CrowdSec engine and waits for LAPI to be ready. It is the request-independent core of
// the Start switch. It refuses with *CrowdsecBusyError while another Start or Stop runs, while a background change
// holds the manager, or while an emergency reset's engine stop is still pending. Every outcome that changes or
// confirms the CrowdSec state re-applies the proxy configuration exactly once. When an emergency reset turns
// CrowdSec off while the Start runs, the Start stops the engine, keeps the reset's values and answers
// errCrowdsecStateChanged.
//
// Before the engine starts, Charon's whitelist file is rewritten from the database under dataMu, so the engine never
// reads an out-of-date list. A running engine gets no signal from Start: it may still be loading its parsers, so
// it picks the list up at its next reload (see syncWhitelistFile).
func (h *CrowdsecHandler) StartCrowdSec(reqCtx context.Context) (CrowdsecStartResult, error) {
	ctx, releaseOp := h.operationContext(reqCtx)
	defer releaseOp()
	if !h.lifecycleMu.TryLock() {
		return CrowdsecStartResult{}, &CrowdsecBusyError{}
	}
	defer h.lifecycleMu.Unlock()
	release, jobID, ok := h.ApplyJobs().Hold()
	if !ok {
		return CrowdsecStartResult{}, &CrowdsecBusyError{JobID: jobID}
	}
	defer release()

	// Record the generation before the wait for pending reset stops. A reset that counted its stop before this load
	// is seen by the wait; a reset that starts later moves the generation, which the checkpoints see.
	gen := h.stateGen.Load()
	if err := h.waitResetStops(ctx); err != nil {
		logger.Log().WithField("error", sanitizeForLog(err.Error())).Warn("CrowdSec start refused: an emergency reset is still stopping the engine")
		return CrowdsecStartResult{}, &CrowdsecBusyError{}
	}

	// Read the status first so a failure here leaves the persisted config untouched.
	// A second spawn cannot bind LAPI and would clobber the pid file of the engine
	// that is already running, so an existing process is reported instead. Status
	// can rewrite the pid file when it adopts an engine the file did not track.
	running, runningPid, statusErr := h.Executor.Status(ctx, h.DataDir)
	if statusErr != nil {
		logger.Log().WithField("error", sanitizeForLog(statusErr.Error())).Warn("failed to read crowdsec status before start")
		return CrowdsecStartResult{}, fmt.Errorf("%w: %w", errCrowdsecStatus, statusErr)
	}
	runHook(h.hooks.afterStatus)

	// Capture the state Start writes, so a failed Start can put back exactly what was there.
	snap, err := h.snapshotCrowdsecStart()
	if err != nil {
		logger.Log().WithError(err).Error("Failed to snapshot SecurityConfig before CrowdSec start")
		return CrowdsecStartResult{}, fmt.Errorf("%w: %w", errCrowdsecPersist, err)
	}

	if persistErr := h.writeIfCurrent(gen, h.persistCrowdsecStart); persistErr != nil {
		if errors.Is(persistErr, errCrowdsecStateChanged) {
			return CrowdsecStartResult{}, h.abortStartAfterReset(ctx)
		}
		logger.Log().WithError(persistErr).Error("Failed to persist SecurityConfig")
		runHook(h.hooks.afterFailedPersist)
		if revertErr := h.revertCrowdsecStartIfCurrent(gen, snap); errors.Is(revertErr, errCrowdsecStateChanged) {
			return CrowdsecStartResult{}, h.abortStartAfterReset(ctx)
		}
		return CrowdsecStartResult{}, fmt.Errorf("%w: %w", errCrowdsecPersist, persistErr)
	}
	runHook(h.hooks.afterPersist)

	if running {
		h.syncWhitelistFile(ctx)
		_, lapiErr := h.runCSCLI(ctx, 2*time.Second, "lapi", "status")
		if h.stateChangedSince(gen) {
			return CrowdsecStartResult{}, h.abortStartAfterReset(ctx)
		}
		logger.Log().WithField("pid", runningPid).Info("CrowdSec start requested but it is already running")
		return CrowdsecStartResult{
			Status:       "already_running",
			PID:          runningPid,
			LAPIReady:    lapiErr == nil,
			ProxyApplied: h.applyProxyConfig(ctx) == nil,
		}, nil
	}

	h.syncWhitelistFile(ctx)

	// Start the process. The executor re-checks under its own lock, so a
	// concurrent Start that wins the race hands back the existing pid.
	pid, err := h.Executor.Start(ctx, h.BinPath, h.DataDir)
	if err != nil {
		if revertErr := h.revertCrowdsecStartIfCurrent(gen, snap); errors.Is(revertErr, errCrowdsecStateChanged) {
			return CrowdsecStartResult{}, h.abortStartAfterReset(ctx)
		}
		logger.Log().WithField("error", sanitizeForLog(err.Error())).Warn("failed to start crowdsec")
		return CrowdsecStartResult{}, fmt.Errorf("%w: %w", errCrowdsecStart, err)
	}

	// Wait for LAPI to be ready (with timeout)
	lapiReady := false
	maxWait := h.LAPIMaxWait
	if maxWait == 0 {
		maxWait = 60 * time.Second
	}
	pollInterval := h.LAPIPollInterval
	if pollInterval == 0 {
		pollInterval = 500 * time.Millisecond
	}
	deadline := time.Now().Add(maxWait)

	for time.Now().Before(deadline) {
		runHook(h.hooks.duringLAPIWait)
		// A reset that turned CrowdSec off ends the wait at once; the checkpoint below stops the engine.
		if h.stateChangedSince(gen) {
			break
		}

		// Check LAPI status using cscli
		_, err := h.runCSCLI(ctx, 2*time.Second, "lapi", "status")

		if err == nil {
			lapiReady = true
			break
		}

		// A shutdown ends the wait at once, the same as the deadline does.
		if util.SleepContext(ctx, pollInterval) != nil {
			break
		}
	}

	if !lapiReady {
		if h.stateChangedSince(gen) {
			return CrowdsecStartResult{}, h.abortStartAfterReset(ctx)
		}
		// The mode is already local, so the proxy needs the apply even though no registration ran.
		logger.Log().WithField("pid", pid).Warn("CrowdSec started but LAPI not ready within timeout")
		return CrowdsecStartResult{
			Status:       "started",
			PID:          pid,
			LAPIReady:    false,
			Warning:      "Process started but LAPI initialization may take additional time",
			ProxyApplied: h.applyProxyConfig(ctx) == nil,
		}, nil
	}

	// After confirming LAPI is ready, ensure bouncer is registered
	res, regErr := h.ensureBouncerRegistration(ctx)
	h.handleBouncerRegistration(res, regErr)
	if h.stateChangedSince(gen) {
		return CrowdsecStartResult{}, h.abortStartAfterReset(ctx)
	}

	logger.Log().WithField("pid", pid).Info("CrowdSec started and LAPI is ready")
	return CrowdsecStartResult{
		Status:       "started",
		PID:          pid,
		LAPIReady:    true,
		ProxyApplied: h.applyProxyConfig(ctx) == nil,
	}, nil
}

// StopCrowdSec stops the CrowdSec engine and turns its state off. It is the request-independent core of the Stop
// switch, and it refuses with *CrowdsecBusyError under the same rules as StartCrowdSec.
func (h *CrowdsecHandler) StopCrowdSec(reqCtx context.Context) (CrowdsecStopResult, error) {
	ctx, releaseOp := h.operationContext(reqCtx)
	defer releaseOp()
	if !h.lifecycleMu.TryLock() {
		return CrowdsecStopResult{}, &CrowdsecBusyError{}
	}
	defer h.lifecycleMu.Unlock()
	release, jobID, ok := h.ApplyJobs().Hold()
	if !ok {
		return CrowdsecStopResult{}, &CrowdsecBusyError{JobID: jobID}
	}
	defer release()

	// Executor.Stop gives up without SIGKILL once its context is done, so it gets its own bound, detached from
	// the operation: a client leaving or a shutdown never leaves a SIGTERMed engine with the setting still on.
	stopCtx, cancelStop := context.WithTimeout(context.WithoutCancel(ctx), engineStopTimeout)
	defer cancelStop()
	if err := h.Executor.Stop(stopCtx, h.DataDir); err != nil {
		logger.Log().WithField("error", sanitizeForLog(err.Error())).Warn("failed to stop crowdsec")
		return CrowdsecStopResult{}, fmt.Errorf("%w: %w", errCrowdsecStop, err)
	}

	h.stateMu.Lock()
	h.persistCrowdsecStopped()
	h.stateMu.Unlock()

	// The engine is stopped and the mode is written, so the proxy must stop routing through CrowdSec.
	return CrowdsecStopResult{ProxyApplied: h.applyProxyConfig(ctx) == nil}, nil
}

// persistCrowdsecStopped writes the stopped state: the default row, the enabled setting and the deprecated mode
// setting. Each write is logged on failure and does not fail the Stop, which has already stopped the engine.
// The caller holds stateMu.
func (h *CrowdsecHandler) persistCrowdsecStopped() {
	// Only the CrowdSec mode and its setting change here. The global security switch is left alone.
	if _, err := setDefaultSecurityConfigState(h.DB, map[string]any{"crowd_sec_mode": "disabled"}); err != nil {
		logger.Log().WithError(err).Warn("Failed to update SecurityConfig after stopping CrowdSec")
	}
	if err := upsertSetting(h.DB, models.Setting{Key: models.CrowdSecEnabledSettingKey, Value: "false", Category: "security", Type: "bool"}); err != nil {
		logger.Log().WithError(err).Warn("Failed to update CrowdSec setting after stopping CrowdSec")
	}
	if err := upsertSetting(h.DB, models.Setting{Key: models.CrowdSecDeprecatedModeSettingKey, Value: "disabled", Category: "security", Type: "string"}); err != nil {
		logger.Log().WithError(err).Warn("Failed to update CrowdSec mode setting after stopping CrowdSec")
	}
}

// ResetCrowdSecState implements CrowdsecLifecycle. The reset's writes run under stateMu. Inside that critical section
// the pending stop is counted before the generation moves, so a Start that loads the new generation also sees the
// stop, and a Start that loaded the old one is caught by its checkpoints.
func (h *CrowdsecHandler) ResetCrowdSecState(write func() error) error {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	h.resetStops.begin()
	h.stateGen.Add(1)
	return write()
}

// StopAfterReset implements CrowdsecLifecycle. It returns at once: the stop runs in a goroutine that holds no handler
// lock, so neither the reply nor a Start waits for a slow engine stop beyond resetStopTimeout.
func (h *CrowdsecHandler) StopAfterReset(resetErr error) {
	go h.stopEngineAfterReset(resetErr)
}

// stopEngineAfterReset decides whether a reset leaves the engine to stop, and stops it when it does. It always
// consumes the stop that ResetCrowdSecState counted. A reset that wrote the CrowdSec keys has left CrowdSec off, so
// the engine stops. A reset that failed before writing them leaves CrowdSec on, and the engine keeps running. When
// the decision cannot be read, the engine stops only if the reset itself succeeded.
func (h *CrowdsecHandler) stopEngineAfterReset(resetErr error) {
	defer h.resetStops.done()

	on, err := h.crowdSecOn()
	stop := !on
	if err != nil {
		logger.Log().WithField("error", sanitizeForLog(err.Error())).Warn("could not read the CrowdSec state after an emergency reset")
		stop = resetErr == nil
	}
	if !stop {
		logger.Log().Info("emergency reset did not stop the CrowdSec engine")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), resetStopTimeout)
	defer cancel()
	if err := h.Executor.Stop(ctx, h.DataDir); err != nil {
		logger.Log().WithField("error", sanitizeForLog(err.Error())).Warn("failed to stop crowdsec after an emergency reset")
	}
}

// WaitResetStops waits until no emergency reset's engine stop is pending, or until ctx ends. The shutdown path and
// the tests use it.
func (h *CrowdsecHandler) WaitResetStops(ctx context.Context) error {
	return h.resetStops.wait(ctx)
}

// waitResetStops is the Start's bounded wait for pending reset stops: the stop goroutine ends within resetStopTimeout,
// so the extra second only absorbs scheduling. The test seam resetStopWaitMax overrides the bound.
func (h *CrowdsecHandler) waitResetStops(ctx context.Context) error {
	bound := h.resetStopWaitMax
	if bound == 0 {
		bound = resetStopTimeout + time.Second
	}
	waitCtx, cancel := context.WithTimeout(ctx, bound)
	defer cancel()
	return h.resetStops.wait(waitCtx)
}

// crowdSecOn returns the CrowdSec decision that the proxy and the status read (models.CrowdSecEnabled).
func (h *CrowdsecHandler) crowdSecOn() (bool, error) {
	row, err := models.FindDefaultSecurityConfig(h.DB)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, fmt.Errorf("load default security config: %w", err)
	}
	return models.CrowdSecEnabled(h.DB, row)
}

// stateChangedSince reports whether an emergency reset has turned CrowdSec off since gen was recorded. A moved
// generation alone is not a change: a reset that fails before its first write leaves CrowdSec on, and the Start
// carries on. The generation is read without the lock, so an unmoved state costs nothing.
func (h *CrowdsecHandler) stateChangedSince(gen uint64) bool {
	if h.stateGen.Load() == gen {
		return false
	}
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	return h.turnedOffSinceLocked(gen)
}

// turnedOffSinceLocked is stateChangedSince for a caller that holds stateMu. A failed read counts as a change, so the
// Start stops the engine rather than run it on a state it cannot read.
func (h *CrowdsecHandler) turnedOffSinceLocked(gen uint64) bool {
	if h.stateGen.Load() == gen {
		return false
	}
	on, err := h.crowdSecOn()
	if err != nil {
		logger.Log().WithField("error", sanitizeForLog(err.Error())).Warn("could not read the CrowdSec state while a start was running")
		return true
	}
	return !on
}

// writeIfCurrent runs one group of Start's DB writes under stateMu, unless an emergency reset has turned CrowdSec off
// since gen was recorded. Then nothing is written and errCrowdsecStateChanged is returned.
func (h *CrowdsecHandler) writeIfCurrent(gen uint64, write func() error) error {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	runHook(h.hooks.inWrite)
	if h.turnedOffSinceLocked(gen) {
		return errCrowdsecStateChanged
	}
	return write()
}

// revertCrowdsecStartIfCurrent restores the snapshot unless an emergency reset has taken the state since gen was
// recorded. A reset that did take it keeps its own values, and the caller answers with abortStartAfterReset.
func (h *CrowdsecHandler) revertCrowdsecStartIfCurrent(gen uint64, snap crowdsecStartSnapshot) error {
	return h.writeIfCurrent(gen, func() error {
		h.revertCrowdsecStart(snap)
		return nil
	})
}

// abortStartAfterReset ends a Start that an emergency reset overtook. The engine is stopped on its own bounded
// context, the reset's values stay as they are, and the proxy is applied once. It returns errCrowdsecStateChanged.
func (h *CrowdsecHandler) abortStartAfterReset(ctx context.Context) error {
	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), engineStopTimeout)
	defer cancel()
	if err := h.Executor.Stop(stopCtx, h.DataDir); err != nil {
		logger.Log().WithField("error", sanitizeForLog(err.Error())).Warn("failed to stop crowdsec after an emergency reset overtook its start")
	}
	_ = h.applyProxyConfig(ctx)
	return errCrowdsecStateChanged
}

// writeWhitelistFile regenerates Charon's whitelist file from the database and reports whether it did. A handler
// without a whitelist service (a bare test handler) has no file to write and reports false.
func (h *CrowdsecHandler) writeWhitelistFile(ctx context.Context) (bool, error) {
	if h.WhitelistSvc == nil {
		return false, nil
	}
	if err := h.WhitelistSvc.WriteYAML(ctx); err != nil {
		return false, fmt.Errorf("write whitelist file: %w", err)
	}
	return true, nil
}

// syncWhitelistFile rewrites Charon's whitelist file from the database under dataMu. It never signals the engine: a
// running engine reads the file at its next reload, and whitelist adds and deletes reload it. A lock wait that ends
// with ctx, and a failed write, are logged and never fail the Start (the same rule as boot).
func (h *CrowdsecHandler) syncWhitelistFile(ctx context.Context) {
	if err := h.lockDataCtx(ctx, "crowdsec start"); err != nil {
		logger.Log().WithField("error", sanitizeForLog(err.Error())).Warn("CrowdSec start: the whitelist file was not rewritten")
		return
	}
	defer h.dataMu.Unlock()
	if _, err := h.writeWhitelistFile(ctx); err != nil {
		logger.Log().WithField("error", sanitizeForLog(err.Error())).Warn("CrowdSec start: rewriting the whitelist file failed (non-fatal)")
	}
}
