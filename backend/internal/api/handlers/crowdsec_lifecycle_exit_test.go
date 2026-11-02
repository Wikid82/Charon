package handlers

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// exitScriptEngine is a CrowdSec engine stand-in for the LAPI wait. Status reports not running until Start, then
// follows running in call order; the last entry repeats. errCalls lists the post-start Status calls (1-based) that
// fail. onStatus, when set, runs after each post-start Status call and outside the engine lock, so a test can let a
// reset land between two polls.
type exitScriptEngine struct {
	mu       sync.Mutex
	started  bool
	running  []bool
	errCalls map[int]bool
	calls    int
	stops    int
	onStatus func(call int)
}

func (e *exitScriptEngine) Start(_ context.Context, _, _ string) (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.started = true
	return 4242, nil
}

func (e *exitScriptEngine) Stop(_ context.Context, _ string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stops++
	return nil
}

func (e *exitScriptEngine) Status(_ context.Context, _ string) (running bool, pid int, err error) {
	e.mu.Lock()
	if !e.started {
		e.mu.Unlock()
		return false, 0, nil
	}
	e.calls++
	call := e.calls
	up := e.running[min(call-1, len(e.running)-1)]
	failed := e.errCalls[call]
	hook := e.onStatus
	e.mu.Unlock()

	if hook != nil {
		hook(call)
	}
	if failed {
		return false, 0, errors.New("status read failed")
	}
	if !up {
		return false, 0, nil
	}
	return true, 4242, nil
}

// statusCallsAfterStart returns how many Status calls ran once the engine was started.
func (e *exitScriptEngine) statusCallsAfterStart() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.calls
}

// TestStartCrowdSec_EngineExitsDuringLAPIWait: an engine that dies before LAPI answers ends the wait after two
// consecutive not-running polls. The Start reverts to its exact snapshot, never applies the proxy and answers
// errCrowdsecEngineExited.
func TestStartCrowdSec_EngineExitsDuringLAPIWait(t *testing.T) {
	eng := &exitScriptEngine{running: []bool{false}}
	applier := &recordingApplier{}
	h := newResetLifecycleHandler(t, eng, &lifecycleLAPIFake{readyAt: 0}, applier)
	before, err := h.snapshotCrowdsecStart()
	require.NoError(t, err)

	_, err = h.StartCrowdSec(context.Background())

	require.ErrorIs(t, err, errCrowdsecEngineExited)
	require.Equal(t, 2, eng.statusCallsAfterStart(), "the exit needs two consecutive not-running polls")
	after, err := h.snapshotCrowdsecStart()
	require.NoError(t, err)
	require.Equal(t, before, after, "a Start whose engine exited must revert to its exact snapshot")
	requireCrowdSecOff(t, h.DB)
	require.Equal(t, 0, applier.count(), "a failed start must not apply the proxy")
}

// TestStartCrowdSec_EngineExitAnswersCodedError: the HTTP answer of a Start whose engine exited is a 500 with the
// stable error_code and the English text.
func TestStartCrowdSec_EngineExitAnswersCodedError(t *testing.T) {
	eng := &exitScriptEngine{running: []bool{false}}
	h := newResetLifecycleHandler(t, eng, &lifecycleLAPIFake{readyAt: 0}, &recordingApplier{})

	w := startRequest(t, h, context.Background())

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	require.JSONEq(t,
		`{"error":"CrowdSec stopped right after it started. Check the CrowdSec log for the reason.","error_code":"crowdsec_engine_exited"}`,
		w.Body.String())
}

// TestStartCrowdSec_SingleNotRunningPollDoesNotTrigger: one not-running poll followed by a running one is not an
// exit, so the Start carries on to LAPI ready and answers 200.
func TestStartCrowdSec_SingleNotRunningPollDoesNotTrigger(t *testing.T) {
	eng := &exitScriptEngine{running: []bool{false, true}}
	applier := &recordingApplier{}
	h := newResetLifecycleHandler(t, eng, &lifecycleLAPIFake{readyAt: 3}, applier)

	res, err := h.StartCrowdSec(context.Background())

	require.NoError(t, err)
	require.Equal(t, "started", res.Status)
	require.True(t, res.LAPIReady)
	require.Equal(t, 1, applier.count(), "the proxy is applied once")
	require.Equal(t, "true", lifecycleSettingValue(t, h.DB))
}

// TestStartCrowdSec_FailedStatusPollBreaksStreak: a Status read error is not a running=false poll, so it does not
// count towards the exit. The streak restarts after it, and the exit needs two not-running polls after the error.
func TestStartCrowdSec_FailedStatusPollBreaksStreak(t *testing.T) {
	eng := &exitScriptEngine{running: []bool{false}, errCalls: map[int]bool{1: true}}
	h := newResetLifecycleHandler(t, eng, &lifecycleLAPIFake{readyAt: 0}, &recordingApplier{})

	_, err := h.StartCrowdSec(context.Background())

	require.ErrorIs(t, err, errCrowdsecEngineExited)
	require.Equal(t, 3, eng.statusCallsAfterStart(), "the failed read must not count, so the exit takes three polls")
}

// TestStartCrowdSec_ResetRacingExitWins: a reset that lands between the two not-running polls wins over the exit.
// The Start answers errCrowdsecStateChanged, and the reset's values stay.
func TestStartCrowdSec_ResetRacingExitWins(t *testing.T) {
	eng := &exitScriptEngine{running: []bool{false}}
	h := newResetLifecycleHandler(t, eng, &lifecycleLAPIFake{readyAt: 0}, &recordingApplier{})
	em := newResetEmergencyHandler(t, h)
	eng.onStatus = func(call int) {
		if call == 2 {
			require.Equal(t, http.StatusOK, runEmergencyReset(t, em).Code)
		}
	}

	_, err := h.StartCrowdSec(context.Background())

	require.ErrorIs(t, err, errCrowdsecStateChanged)
	require.NotErrorIs(t, err, errCrowdsecEngineExited)
	waitForResetStops(t, h)
	requireCrowdSecOff(t, h.DB)
}

// TestStartCrowdSec_EngineExitAfterLAPIReadyIsUnaffected: once LAPI answers the wait ends, so an engine that dies
// afterwards is not polled again and the Start answers 200.
func TestStartCrowdSec_EngineExitAfterLAPIReadyIsUnaffected(t *testing.T) {
	eng := &exitScriptEngine{running: []bool{true, false}}
	h := newResetLifecycleHandler(t, eng, &lifecycleLAPIFake{readyAt: 1}, &recordingApplier{})

	res, err := h.StartCrowdSec(context.Background())

	require.NoError(t, err)
	require.Equal(t, "started", res.Status)
	require.True(t, res.LAPIReady)
	require.Equal(t, 1, eng.statusCallsAfterStart(), "no poll runs once LAPI is ready")
}
