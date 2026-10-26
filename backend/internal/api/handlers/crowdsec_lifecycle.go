package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
)

// CrowdsecLifecycle is the one owner of the CrowdSec engine state. The engine process, the default row's
// crowd_sec_mode, the security.crowdsec.enabled setting and the proxy configuration change together, and only
// through it. The HTTP switches and the settings and emergency handlers call it; none of them drives the engine
// on its own.
type CrowdsecLifecycle interface {
	StartCrowdSec(ctx context.Context) (CrowdsecStartResult, error)
	StopCrowdSec(ctx context.Context) (CrowdsecStopResult, error)
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

// Sentinel causes of a failed Start or Stop. They are wrapped with the underlying error and mapped to HTTP answers
// by respondCrowdsecLifecycleError, so every caller answers the same way.
var (
	errCrowdsecStatus  = errors.New("read crowdsec status")
	errCrowdsecPersist = errors.New("persist crowdsec configuration")
	errCrowdsecStart   = errors.New("start crowdsec")
	errCrowdsecStop    = errors.New("stop crowdsec")
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
	default:
		logger.Log().WithError(err).Error("CrowdSec lifecycle request failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "CrowdSec request failed"})
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

// StartCrowdSec starts the CrowdSec engine and waits for LAPI to be ready. It is the request-independent core of
// the Start switch. It refuses with *CrowdsecBusyError while another Start or Stop runs, or while a background
// change holds the manager. Every outcome that changes or confirms the CrowdSec state re-applies the proxy
// configuration exactly once.
func (h *CrowdsecHandler) StartCrowdSec(ctx context.Context) (CrowdsecStartResult, error) {
	if !h.lifecycleMu.TryLock() {
		return CrowdsecStartResult{}, &CrowdsecBusyError{}
	}
	defer h.lifecycleMu.Unlock()
	release, jobID, ok := h.ApplyJobs().Hold()
	if !ok {
		return CrowdsecStartResult{}, &CrowdsecBusyError{JobID: jobID}
	}
	defer release()

	// Read the status first so a failure here leaves the persisted config untouched.
	// A second spawn cannot bind LAPI and would clobber the pid file of the engine
	// that is already running, so an existing process is reported instead. Status
	// can rewrite the pid file when it adopts an engine the file did not track.
	running, runningPid, statusErr := h.Executor.Status(ctx, h.DataDir)
	if statusErr != nil {
		logger.Log().WithField("error", sanitizeForLog(statusErr.Error())).Warn("failed to read crowdsec status before start")
		return CrowdsecStartResult{}, fmt.Errorf("%w: %w", errCrowdsecStatus, statusErr)
	}

	// Capture the state Start writes, so a failed Start can put back exactly what was there.
	snap, err := h.snapshotCrowdsecStart()
	if err != nil {
		logger.Log().WithError(err).Error("Failed to snapshot SecurityConfig before CrowdSec start")
		return CrowdsecStartResult{}, fmt.Errorf("%w: %w", errCrowdsecPersist, err)
	}

	if persistErr := h.persistCrowdsecStart(); persistErr != nil {
		logger.Log().WithError(persistErr).Error("Failed to persist SecurityConfig")
		h.revertCrowdsecStart(snap)
		return CrowdsecStartResult{}, fmt.Errorf("%w: %w", errCrowdsecPersist, persistErr)
	}

	if running {
		_, lapiErr := h.runCSCLI(ctx, 2*time.Second, "lapi", "status")
		logger.Log().WithField("pid", runningPid).Info("CrowdSec start requested but it is already running")
		return CrowdsecStartResult{
			Status:       "already_running",
			PID:          runningPid,
			LAPIReady:    lapiErr == nil,
			ProxyApplied: h.applyProxyConfig(ctx) == nil,
		}, nil
	}

	// Start the process. The executor re-checks under its own lock, so a
	// concurrent Start that wins the race hands back the existing pid.
	pid, err := h.Executor.Start(ctx, h.BinPath, h.DataDir)
	if err != nil {
		h.revertCrowdsecStart(snap)
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
		// Check LAPI status using cscli
		_, err := h.runCSCLI(ctx, 2*time.Second, "lapi", "status")

		if err == nil {
			lapiReady = true
			break
		}

		time.Sleep(pollInterval)
	}

	if !lapiReady {
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
func (h *CrowdsecHandler) StopCrowdSec(ctx context.Context) (CrowdsecStopResult, error) {
	if !h.lifecycleMu.TryLock() {
		return CrowdsecStopResult{}, &CrowdsecBusyError{}
	}
	defer h.lifecycleMu.Unlock()
	release, jobID, ok := h.ApplyJobs().Hold()
	if !ok {
		return CrowdsecStopResult{}, &CrowdsecBusyError{JobID: jobID}
	}
	defer release()

	if err := h.Executor.Stop(ctx, h.DataDir); err != nil {
		logger.Log().WithField("error", sanitizeForLog(err.Error())).Warn("failed to stop crowdsec")
		return CrowdsecStopResult{}, fmt.Errorf("%w: %w", errCrowdsecStop, err)
	}

	// Only the CrowdSec mode and its setting change here. The global security switch is left alone.
	if _, err := setDefaultSecurityConfigState(h.DB, map[string]any{"crowd_sec_mode": "disabled"}); err != nil {
		logger.Log().WithError(err).Warn("Failed to update SecurityConfig after stopping CrowdSec")
	}
	if err := upsertSetting(h.DB, models.Setting{Key: "security.crowdsec.enabled", Value: "false", Category: "security", Type: "bool"}); err != nil {
		logger.Log().WithError(err).Warn("Failed to update CrowdSec setting after stopping CrowdSec")
	}

	// The engine is stopped and the mode is written, so the proxy must stop routing through CrowdSec.
	return CrowdsecStopResult{ProxyApplied: h.applyProxyConfig(ctx) == nil}, nil
}
