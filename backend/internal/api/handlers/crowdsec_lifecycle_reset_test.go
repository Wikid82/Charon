package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// recordingEngine is a thread-safe CrowdsecExecutor that records its calls in order. When stopGate is set, Stop
// waits until the gate is closed, so a test can keep an engine stop in flight.
type recordingEngine struct {
	mu       sync.Mutex
	running  bool
	startErr error
	stopErr  error
	calls    []string
	stopGate chan struct{}
}

func (e *recordingEngine) Start(_ context.Context, _, _ string) (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.calls = append(e.calls, "start")
	if e.startErr != nil {
		return 0, e.startErr
	}
	e.running = true
	return 4242, nil
}

func (e *recordingEngine) Stop(_ context.Context, _ string) error {
	e.mu.Lock()
	gate := e.stopGate
	e.mu.Unlock()
	if gate != nil {
		<-gate
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.calls = append(e.calls, "stop")
	if e.stopErr != nil {
		return e.stopErr
	}
	e.running = false
	return nil
}

func (e *recordingEngine) Status(_ context.Context, _ string) (running bool, pid int, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.calls = append(e.calls, "status")
	if e.running {
		return true, 4242, nil
	}
	return false, 0, nil
}

func (e *recordingEngine) callLog() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return slices.Clone(e.calls)
}

func (e *recordingEngine) isRunning() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.running
}

// gatedApplier stands in for the Caddy manager. Every apply waits until the gate opens; the first apply also
// signals entered, so a test knows the caller is inside the apply. The gate opens at test cleanup at the latest.
type gatedApplier struct {
	entered chan struct{}
	release chan struct{}
	enter   sync.Once
	open    sync.Once
}

func newGatedApplier(t *testing.T) *gatedApplier {
	t.Helper()
	g := &gatedApplier{entered: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(g.openGate)
	return g
}

func (g *gatedApplier) openGate() {
	g.open.Do(func() { close(g.release) })
}

func (g *gatedApplier) ApplyConfig(context.Context) error {
	g.enter.Do(func() { close(g.entered) })
	<-g.release
	return nil
}

// newResetLifecycleHandler builds a CrowdSec handler on a fresh database whose CrowdSec is off: the default row is
// disabled and the enabled setting is false. The engine, the cscli stand-in and the proxy applier belong to the test.
func newResetLifecycleHandler(t *testing.T, eng CrowdsecExecutor, cmd CommandExecutor, applier CaddyConfigManager) *CrowdsecHandler {
	t.Helper()
	db := setupCrowdDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, Enabled: false, CrowdSecMode: "disabled"}).Error)
	require.NoError(t, db.Create(&models.Setting{Key: models.CrowdSecEnabledSettingKey, Value: "false", Category: "security", Type: "bool"}).Error)
	h := newTestCrowdsecHandler(t, db, eng, "/bin/false", t.TempDir())
	h.CmdExec = cmd
	h.caddyManager = applier
	h.LAPIMaxWait = 300 * time.Millisecond
	h.LAPIPollInterval = 5 * time.Millisecond
	return h
}

// newResetEmergencyHandler builds the emergency reset handler that shares h's database, proxy applier and lifecycle.
func newResetEmergencyHandler(t *testing.T, h *CrowdsecHandler) *EmergencyHandler {
	t.Helper()
	em := NewEmergencyHandlerWithDeps(h.DB, h.caddyManager, nil).WithCrowdsecLifecycle(h)
	t.Cleanup(em.Close)
	return em
}

// runEmergencyReset runs the reset's reply path and returns the recorded answer.
func runEmergencyReset(t *testing.T, em *EmergencyHandler) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/emergency/security-reset", http.NoBody)
	em.performSecurityReset(c, "127.0.0.1", time.Now())
	return w
}

// seedCrowdSecOn sets the stored decision to on: the enabled setting is true and the default row is local.
func seedCrowdSecOn(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Model(&models.Setting{}).Where("key = ?", models.CrowdSecEnabledSettingKey).Update("value", "true").Error)
	require.NoError(t, db.Model(&models.SecurityConfig{}).Where("name = ?", models.DefaultSecurityConfigName).Update("crowd_sec_mode", "local").Error)
}

// hookSettingWrites runs onWrite before every write of a settings row, counting from 1 after it is installed. A
// returned error fails that write; a panic propagates like a panic inside the write itself.
func hookSettingWrites(t *testing.T, db *gorm.DB, name string, onWrite func(n int) error) {
	t.Helper()
	n := 0
	hook := func(tx *gorm.DB) {
		if tx.Statement.Table != "settings" {
			return
		}
		n++
		if err := onWrite(n); err != nil {
			_ = tx.AddError(err)
		}
	}
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register(name+":create", hook))
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register(name+":update", hook))
}

// waitForResetStops waits until every engine stop that an emergency reset started has finished.
func waitForResetStops(t *testing.T, h *CrowdsecHandler) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, h.WaitResetStops(ctx))
}

// requireCrowdSecOff asserts the stored state a reset leaves: the enabled setting is false and the default row is disabled.
func requireCrowdSecOff(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.Equal(t, "false", lifecycleSettingValue(t, db))
	var cfg models.SecurityConfig
	require.NoError(t, db.Where("name = ?", models.DefaultSecurityConfigName).First(&cfg).Error)
	require.Equal(t, "disabled", cfg.CrowdSecMode)
}

// TestCrowdsecLifecycleReset_ResetBeforePersistAnswersStateChanged: a reset that lands before the Start's first
// write wins. The Start writes nothing, never starts the engine and answers state changed.
func TestCrowdsecLifecycleReset_ResetBeforePersistAnswersStateChanged(t *testing.T) {
	eng := &recordingEngine{}
	h := newResetLifecycleHandler(t, eng, &lifecycleLAPIFake{readyAt: 1}, &recordingApplier{})
	em := newResetEmergencyHandler(t, h)
	h.hooks.afterStatus = func() { runEmergencyReset(t, em) }

	_, err := h.StartCrowdSec(context.Background())

	require.ErrorIs(t, err, errCrowdsecStateChanged)
	require.NotContains(t, eng.callLog(), "start", "the engine must not start after a reset overtook the Start")
	waitForResetStops(t, h)
	require.False(t, eng.isRunning())
	requireCrowdSecOff(t, h.DB)
}

// TestCrowdsecLifecycleReset_ResetThenFailedStartKeepsResetValues: a Start whose engine start fails after a reset
// does not revert to its snapshot. The state stays exactly as the reset and the Start's persist left it.
func TestCrowdsecLifecycleReset_ResetThenFailedStartKeepsResetValues(t *testing.T) {
	eng := &recordingEngine{startErr: errors.New("engine refused to start")}
	h := newResetLifecycleHandler(t, eng, &lifecycleLAPIFake{readyAt: 1}, &recordingApplier{})
	em := newResetEmergencyHandler(t, h)
	preState, err := h.snapshotCrowdsecStart()
	require.NoError(t, err)
	var afterReset crowdsecStartSnapshot
	h.hooks.afterPersist = func() {
		runEmergencyReset(t, em)
		afterReset, err = h.snapshotCrowdsecStart()
		require.NoError(t, err)
	}

	_, err = h.StartCrowdSec(context.Background())

	require.ErrorIs(t, err, errCrowdsecStateChanged)
	waitForResetStops(t, h)
	finalState, err := h.snapshotCrowdsecStart()
	require.NoError(t, err)
	require.NotEqual(t, preState, afterReset, "the Start's persist must have changed state the reset did not restore")
	require.Equal(t, afterReset, finalState, "a Start overtaken by a reset must not revert to its snapshot")
	require.False(t, eng.isRunning())
	requireCrowdSecOff(t, h.DB)
}

// TestCrowdsecLifecycleReset_ResetDuringLAPIWaitStopsEngine: a reset that lands while the Start waits for LAPI ends
// the wait at the next poll, stops the engine and answers state changed.
func TestCrowdsecLifecycleReset_ResetDuringLAPIWaitStopsEngine(t *testing.T) {
	eng := &recordingEngine{}
	h := newResetLifecycleHandler(t, eng, &lifecycleLAPIFake{readyAt: 0}, &recordingApplier{})
	em := newResetEmergencyHandler(t, h)
	resetRan := false
	h.hooks.duringLAPIWait = func() {
		if resetRan {
			return
		}
		resetRan = true
		require.Equal(t, http.StatusOK, runEmergencyReset(t, em).Code)
	}

	start := time.Now()
	_, err := h.StartCrowdSec(context.Background())

	require.ErrorIs(t, err, errCrowdsecStateChanged)
	require.Less(t, time.Since(start), h.LAPIMaxWait, "the reset must end the LAPI wait")
	require.Contains(t, eng.callLog(), "start")
	waitForResetStops(t, h)
	require.False(t, eng.isRunning())
	requireCrowdSecOff(t, h.DB)
}

// TestCrowdsecLifecycleReset_AlreadyRunningResetAfterPersistStopsEngine: the already_running branch checks the state
// after its persist, so a reset that lands there stops the engine and keeps the reset values.
func TestCrowdsecLifecycleReset_AlreadyRunningResetAfterPersistStopsEngine(t *testing.T) {
	eng := &recordingEngine{running: true}
	h := newResetLifecycleHandler(t, eng, &lifecycleLAPIFake{readyAt: 1}, &recordingApplier{})
	em := newResetEmergencyHandler(t, h)
	h.hooks.afterPersist = func() { runEmergencyReset(t, em) }

	_, err := h.StartCrowdSec(context.Background())

	require.ErrorIs(t, err, errCrowdsecStateChanged)
	waitForResetStops(t, h)
	require.False(t, eng.isRunning())
	require.NotContains(t, eng.callLog(), "start", "an already running engine is not started again")
	requireCrowdSecOff(t, h.DB)
}

// TestCrowdsecLifecycleReset_ResetWaitsForStartWriteGroup: a reset that arrives during a Start's write group waits
// for the group to end. It then runs in full, and the Start's next checkpoint answers state changed.
func TestCrowdsecLifecycleReset_ResetWaitsForStartWriteGroup(t *testing.T) {
	eng := &recordingEngine{}
	h := newResetLifecycleHandler(t, eng, &lifecycleLAPIFake{readyAt: 1}, &recordingApplier{})
	em := newResetEmergencyHandler(t, h)
	resetDone := make(chan struct{})
	var once sync.Once
	h.hooks.inWrite = func() {
		once.Do(func() {
			go func() {
				defer close(resetDone)
				err := h.ResetCrowdSecState(func() error {
					_, writeErr := em.disableAllSecurityModules()
					return writeErr
				})
				h.StopAfterReset(err)
			}()
			time.Sleep(50 * time.Millisecond)
			select {
			case <-resetDone:
				t.Error("the reset ran inside the start's write group")
			default:
			}
		})
	}
	h.hooks.afterPersist = func() { <-resetDone }

	_, err := h.StartCrowdSec(context.Background())

	require.ErrorIs(t, err, errCrowdsecStateChanged)
	waitForResetStops(t, h)
	require.False(t, eng.isRunning())
	requireCrowdSecOff(t, h.DB)
}

// TestCrowdsecLifecycleReset_StartDuringResetApplyWaitsForStop: while a reset is held inside its proxy apply, a
// Start waits for the reset's engine stop before it reads the status. The stop comes first, then the start, and
// the Start ends with the engine running and the setting on.
func TestCrowdsecLifecycleReset_StartDuringResetApplyWaitsForStop(t *testing.T) {
	eng := &recordingEngine{running: true}
	applier := newGatedApplier(t)
	h := newResetLifecycleHandler(t, eng, &lifecycleLAPIFake{readyAt: 1}, applier)
	seedCrowdSecOn(t, h.DB)
	em := newResetEmergencyHandler(t, h)

	resetCode := make(chan int, 1)
	go func() { resetCode <- runEmergencyReset(t, em).Code }()
	<-applier.entered

	type startOutcome struct {
		res CrowdsecStartResult
		err error
	}
	startDone := make(chan startOutcome, 1)
	go func() {
		res, err := h.StartCrowdSec(context.Background())
		startDone <- startOutcome{res: res, err: err}
	}()
	time.Sleep(100 * time.Millisecond)
	require.NotContains(t, eng.callLog(), "status", "the start must wait for the reset's stop before reading the status")

	applier.openGate()
	require.Equal(t, http.StatusOK, <-resetCode)
	out := <-startDone
	require.NoError(t, out.err)
	require.Equal(t, "started", out.res.Status)

	calls := eng.callLog()
	require.Less(t, slices.Index(calls, "stop"), slices.Index(calls, "status"))
	require.Less(t, slices.Index(calls, "stop"), slices.Index(calls, "start"))
	require.True(t, eng.isRunning())
	require.Equal(t, "true", lifecycleSettingValue(t, h.DB))
}

// TestCrowdsecLifecycleReset_StartWaitsForPendingStopBeforeStatus: a Start that arrives right after a reset's
// counted stop waits for that stop before it reads the status, so the stop cannot kill the engine the Start starts.
func TestCrowdsecLifecycleReset_StartWaitsForPendingStopBeforeStatus(t *testing.T) {
	eng := &recordingEngine{stopGate: make(chan struct{})}
	h := newResetLifecycleHandler(t, eng, &lifecycleLAPIFake{readyAt: 1}, &recordingApplier{})
	err := h.ResetCrowdSecState(func() error { return nil })
	require.NoError(t, err)
	h.StopAfterReset(nil)

	startDone := make(chan error, 1)
	go func() {
		_, startErr := h.StartCrowdSec(context.Background())
		startDone <- startErr
	}()
	time.Sleep(100 * time.Millisecond)
	require.NotContains(t, eng.callLog(), "status", "the start must wait for the pending stop")

	close(eng.stopGate)
	require.NoError(t, <-startDone)
	calls := eng.callLog()
	require.Less(t, slices.Index(calls, "stop"), slices.Index(calls, "status"))
	require.True(t, eng.isRunning())
	require.Equal(t, "true", lifecycleSettingValue(t, h.DB))
}

// TestCrowdsecLifecycleReset_StartRefusedWhenStopWaitTimesOut: when the pending reset stop does not finish within
// the wait, the Start answers busy without a job id and writes nothing.
func TestCrowdsecLifecycleReset_StartRefusedWhenStopWaitTimesOut(t *testing.T) {
	eng := &recordingEngine{stopGate: make(chan struct{})}
	h := newResetLifecycleHandler(t, eng, &lifecycleLAPIFake{readyAt: 1}, &recordingApplier{})
	h.resetStopWaitMax = 100 * time.Millisecond
	require.NoError(t, h.ResetCrowdSecState(func() error { return nil }))
	h.StopAfterReset(nil)

	_, err := h.StartCrowdSec(context.Background())

	var busy *CrowdsecBusyError
	require.ErrorAs(t, err, &busy)
	require.Empty(t, busy.JobID)
	require.NotContains(t, eng.callLog(), "start")
	requireCrowdSecOff(t, h.DB)

	close(eng.stopGate)
	waitForResetStops(t, h)
}

// TestCrowdsecLifecycleReset_PanickingWriteReleasesPendingStop: a reset whose write panics still releases the
// pending stop it counted, so no later Start waits for it and the state lock is free again.
func TestCrowdsecLifecycleReset_PanickingWriteReleasesPendingStop(t *testing.T) {
	eng := &recordingEngine{running: true}
	h := newResetLifecycleHandler(t, eng, &lifecycleLAPIFake{readyAt: 1}, &recordingApplier{})
	seedCrowdSecOn(t, h.DB)
	em := newResetEmergencyHandler(t, h)
	armed := true
	hookSettingWrites(t, h.DB, "test:panic", func(int) error {
		if armed {
			panic("forced write panic")
		}
		return nil
	})

	require.Panics(t, func() { runEmergencyReset(t, em) })

	waitForResetStops(t, h)
	require.True(t, h.stateMu.TryLock(), "the state lock must be free after the panic")
	h.stateMu.Unlock()

	armed = false
	res, err := h.StartCrowdSec(context.Background())
	require.NoError(t, err, "no stop is pending, so the Start must not be refused")
	require.Equal(t, "already_running", res.Status)
}

// TestCrowdsecLifecycleReset_FirstWriteFailureLeavesStartRunning: a reset that fails at its first write leaves
// CrowdSec on, so the Start that is waiting for LAPI carries on and answers 200.
func TestCrowdsecLifecycleReset_FirstWriteFailureLeavesStartRunning(t *testing.T) {
	eng := &recordingEngine{}
	h := newResetLifecycleHandler(t, eng, &lifecycleLAPIFake{readyAt: 2}, &recordingApplier{})
	em := newResetEmergencyHandler(t, h)
	resetCode := 0
	h.hooks.duringLAPIWait = func() {
		if resetCode != 0 {
			return
		}
		hookSettingWrites(t, h.DB, "test:first_write", func(n int) error {
			if n == 1 {
				return errors.New("forced settings write failure")
			}
			return nil
		})
		resetCode = runEmergencyReset(t, em).Code
	}

	res, err := h.StartCrowdSec(context.Background())

	require.NoError(t, err)
	require.Equal(t, "started", res.Status)
	require.True(t, res.LAPIReady)
	require.Equal(t, http.StatusInternalServerError, resetCode)
	waitForResetStops(t, h)
	require.True(t, eng.isRunning())
	require.Equal(t, "true", lifecycleSettingValue(t, h.DB))
}

// TestCrowdsecLifecycleReset_ThirdWriteFailureStopsEngine: a reset that fails after the CrowdSec keys are written
// has turned CrowdSec off, so the Start stops the engine it started and answers state changed.
func TestCrowdsecLifecycleReset_ThirdWriteFailureStopsEngine(t *testing.T) {
	eng := &recordingEngine{}
	h := newResetLifecycleHandler(t, eng, &lifecycleLAPIFake{readyAt: 0}, &recordingApplier{})
	em := newResetEmergencyHandler(t, h)
	resetCode := 0
	h.hooks.duringLAPIWait = func() {
		if resetCode != 0 {
			return
		}
		hookSettingWrites(t, h.DB, "test:third_write", func(n int) error {
			if n == 3 {
				return errors.New("forced settings write failure")
			}
			return nil
		})
		resetCode = runEmergencyReset(t, em).Code
	}

	_, err := h.StartCrowdSec(context.Background())

	require.ErrorIs(t, err, errCrowdsecStateChanged)
	require.Equal(t, http.StatusInternalServerError, resetCode)
	waitForResetStops(t, h)
	require.False(t, eng.isRunning())
	// The reset failed before its row update, so only the CrowdSec keys it wrote are asserted: both are off.
	require.Equal(t, "false", lifecycleSettingValue(t, h.DB))
	var mode models.Setting
	require.NoError(t, h.DB.Where("key = ?", models.CrowdSecDeprecatedModeSettingKey).First(&mode).Error)
	require.Equal(t, "disabled", mode.Value)
}

// TestCrowdsecLifecycleReset_ReadErrorAfterFailedResetDoesNotStop: when a failed reset leaves the decision unreadable,
// the stop is not run, because the reset did not succeed and the engine state is unknown.
func TestCrowdsecLifecycleReset_ReadErrorAfterFailedResetDoesNotStop(t *testing.T) {
	eng := &recordingEngine{running: true}
	h := newResetLifecycleHandler(t, eng, &lifecycleLAPIFake{readyAt: 1}, &recordingApplier{})
	resetErr := h.ResetCrowdSecState(func() error { return errors.New("forced reset failure") })
	require.Error(t, resetErr)
	sqlDB, err := h.DB.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	h.StopAfterReset(resetErr)

	waitForResetStops(t, h)
	require.True(t, eng.isRunning())
	require.NotContains(t, eng.callLog(), "stop")
}

// TestCrowdsecLifecycleReset_EngineStopFollowsReplyAndStopsEngine: the reset replies while its engine stop is still
// in flight, and the stop then runs and ends with the engine stopped. The 200 body keeps the disabled modules.
func TestCrowdsecLifecycleReset_EngineStopFollowsReplyAndStopsEngine(t *testing.T) {
	eng := &recordingEngine{running: true, stopGate: make(chan struct{})}
	h := newResetLifecycleHandler(t, eng, &lifecycleLAPIFake{readyAt: 1}, &recordingApplier{})
	seedCrowdSecOn(t, h.DB)
	em := newResetEmergencyHandler(t, h)

	start := time.Now()
	res := runEmergencyReset(t, em)

	require.Equal(t, http.StatusOK, res.Code)
	require.Less(t, time.Since(start), 5*time.Second, "the reply must not wait for the engine stop")
	require.True(t, eng.isRunning(), "the stop is still waiting on its gate")
	close(eng.stopGate)
	waitForResetStops(t, h)
	require.False(t, eng.isRunning())

	var body struct {
		Success         bool     `json:"success"`
		DisabledModules []string `json:"disabled_modules"`
	}
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.Len(t, body.DisabledModules, 5)
}

// TestCrowdsecLifecycleReset_ManyPairsEndConsistent: concurrent Starts and resets end with no stop pending, and the
// engine state matches the stored decision.
func TestCrowdsecLifecycleReset_ManyPairsEndConsistent(t *testing.T) {
	eng := &recordingEngine{}
	h := newResetLifecycleHandler(t, eng, &lifecycleLAPIFake{readyAt: 1}, &recordingApplier{})
	sqlDB, err := h.DB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	em := newResetEmergencyHandler(t, h)

	var wg sync.WaitGroup
	for range 6 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _ = h.StartCrowdSec(context.Background())
		}()
		go func() {
			defer wg.Done()
			_ = runEmergencyReset(t, em)
		}()
	}
	wg.Wait()

	waitForResetStops(t, h)
	var row models.SecurityConfig
	require.NoError(t, h.DB.Where("name = ?", models.DefaultSecurityConfigName).First(&row).Error)
	decision, err := models.CrowdSecEnabled(h.DB, row)
	require.NoError(t, err)
	require.Equal(t, decision, eng.isRunning(), "the engine must match the stored decision")
}

// TestCrowdsecLifecycleStateChanged_RespondsConflictWithCode: a state change during a Start answers 409 with the
// coded body, whether or not it is wrapped.
func TestCrowdsecLifecycleStateChanged_RespondsConflictWithCode(t *testing.T) {
	for _, err := range []error{errCrowdsecStateChanged, errors.Join(errCrowdsecStateChanged)} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		respondCrowdsecLifecycleError(c, err)

		require.Equal(t, http.StatusConflict, w.Code)
		var body map[string]string
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		require.Equal(t, "crowdsec_state_changed", body["error_code"])
		require.Equal(t, "CrowdSec was turned off while it was starting", body["error"])
	}
}

// TestResetStopTracker_ZeroValueWaitReturnsAtOnce: the zero tracker has nothing pending, so a wait returns at once.
func TestResetStopTracker_ZeroValueWaitReturnsAtOnce(t *testing.T) {
	var tr resetStopTracker
	require.NoError(t, tr.wait(context.Background()))
}

// TestResetStopTracker_WaitBlocksUntilDone: a pending stop holds a wait until it is done.
func TestResetStopTracker_WaitBlocksUntilDone(t *testing.T) {
	var tr resetStopTracker
	tr.begin()
	released := make(chan error, 1)
	go func() { released <- tr.wait(context.Background()) }()

	select {
	case err := <-released:
		t.Fatalf("wait returned while a stop was pending: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	tr.done()
	select {
	case err := <-released:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("wait did not return after the stop was done")
	}
}

// TestResetStopTracker_WaitHonoursContext: a wait ends with the context's error when the context ends first.
func TestResetStopTracker_WaitHonoursContext(t *testing.T) {
	var tr resetStopTracker
	tr.begin()
	defer tr.done()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.ErrorIs(t, tr.wait(ctx), context.Canceled)
}

// TestResetStopTracker_ConcurrentBeginDoneWait: many stops begin, finish and wait concurrently, and the count
// returns to zero.
func TestResetStopTracker_ConcurrentBeginDoneWait(t *testing.T) {
	var tr resetStopTracker
	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tr.begin()
			tr.done()
			_ = tr.wait(context.Background())
		}()
	}
	wg.Wait()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, tr.wait(ctx))
}

// TestResetStopTracker_DoneWithNothingPendingIsNoop: a release without a counted stop never drives the count below zero.
func TestResetStopTracker_DoneWithNothingPendingIsNoop(t *testing.T) {
	var tr resetStopTracker
	tr.done()
	require.NoError(t, tr.wait(context.Background()))
}

// bouncerCallHook is the cscli stand-in for a registration that a reset overtakes. LAPI answers as lifecycleLAPIFake
// does. The first bouncers call runs onCall, then fails, as a registration without a key would.
type bouncerCallHook struct {
	inner  *lifecycleLAPIFake
	once   sync.Once
	onCall func()
}

func (b *bouncerCallHook) Execute(ctx context.Context, name string, args ...string) ([]byte, error) {
	if slices.Contains(args, "bouncers") {
		b.once.Do(b.onCall)
		return nil, errors.New("bouncer registration unavailable")
	}
	return b.inner.Execute(ctx, name, args...)
}

// TestCrowdsecLifecycleReset_ResetAfterFailedPersistAnswersStateChanged: a reset that lands after a failed persist
// group means the revert finds the state moved, so it does not run and the Start answers state changed.
func TestCrowdsecLifecycleReset_ResetAfterFailedPersistAnswersStateChanged(t *testing.T) {
	eng := &recordingEngine{}
	h := newResetLifecycleHandler(t, eng, &lifecycleLAPIFake{readyAt: 1}, &recordingApplier{})
	em := newResetEmergencyHandler(t, h)
	hookSettingWrites(t, h.DB, "test:persist_failure", func(n int) error {
		if n == 1 {
			return errors.New("forced settings write failure")
		}
		return nil
	})
	h.hooks.afterFailedPersist = func() { runEmergencyReset(t, em) }

	_, err := h.StartCrowdSec(context.Background())

	require.ErrorIs(t, err, errCrowdsecStateChanged)
	require.NotContains(t, eng.callLog(), "start")
	waitForResetStops(t, h)
	requireCrowdSecOff(t, h.DB)
}

// TestCrowdsecLifecycleReset_ResetDuringRegistrationAnswersStateChanged: a reset that lands while the bouncer
// registration runs, after LAPI answered, is caught by the checkpoint after the registration.
func TestCrowdsecLifecycleReset_ResetDuringRegistrationAnswersStateChanged(t *testing.T) {
	eng := &recordingEngine{}
	cmd := &bouncerCallHook{inner: &lifecycleLAPIFake{readyAt: 1}}
	h := newResetLifecycleHandler(t, eng, cmd, &recordingApplier{})
	em := newResetEmergencyHandler(t, h)
	cmd.onCall = func() { runEmergencyReset(t, em) }

	_, err := h.StartCrowdSec(context.Background())

	require.ErrorIs(t, err, errCrowdsecStateChanged)
	waitForResetStops(t, h)
	require.False(t, eng.isRunning())
	requireCrowdSecOff(t, h.DB)
}

// TestCrowdsecLifecycleReset_ReadErrorDuringWaitAbandonsStart: when the state cannot be read after a reset, the Start
// does not carry on: it stops the engine and answers state changed.
func TestCrowdsecLifecycleReset_ReadErrorDuringWaitAbandonsStart(t *testing.T) {
	eng := &recordingEngine{}
	h := newResetLifecycleHandler(t, eng, &lifecycleLAPIFake{readyAt: 0}, &recordingApplier{})
	sqlDB, err := h.DB.DB()
	require.NoError(t, err)
	resetRan := false
	h.hooks.duringLAPIWait = func() {
		if resetRan {
			return
		}
		resetRan = true
		require.NoError(t, h.ResetCrowdSecState(func() error { return nil }))
		h.StopAfterReset(nil)
		require.NoError(t, sqlDB.Close())
	}

	_, err = h.StartCrowdSec(context.Background())

	require.ErrorIs(t, err, errCrowdsecStateChanged)
	waitForResetStops(t, h)
	require.False(t, eng.isRunning())
}

// TestCrowdsecLifecycleReset_StopFailureAfterResetIsLoggedOnly: a failed engine stop after a reset does not change the
// reply, which was already sent, and the stop is still attempted.
func TestCrowdsecLifecycleReset_StopFailureAfterResetIsLoggedOnly(t *testing.T) {
	eng := &recordingEngine{running: true, stopErr: errors.New("engine did not exit")}
	h := newResetLifecycleHandler(t, eng, &lifecycleLAPIFake{readyAt: 1}, &recordingApplier{})
	seedCrowdSecOn(t, h.DB)
	em := newResetEmergencyHandler(t, h)

	res := runEmergencyReset(t, em)

	require.Equal(t, http.StatusOK, res.Code)
	waitForResetStops(t, h)
	require.Contains(t, eng.callLog(), "stop")
	require.True(t, eng.isRunning(), "the failed stop leaves the engine as it was")
}

// TestCrowdsecLifecycleReset_StopFailureDuringAbortStillAnswersStateChanged: a failed stop while a Start is abandoned
// is logged, and the Start still answers state changed.
func TestCrowdsecLifecycleReset_StopFailureDuringAbortStillAnswersStateChanged(t *testing.T) {
	eng := &recordingEngine{stopErr: errors.New("engine did not exit")}
	h := newResetLifecycleHandler(t, eng, &lifecycleLAPIFake{readyAt: 0}, &recordingApplier{})
	em := newResetEmergencyHandler(t, h)
	resetRan := false
	h.hooks.duringLAPIWait = func() {
		if resetRan {
			return
		}
		resetRan = true
		require.Equal(t, http.StatusOK, runEmergencyReset(t, em).Code)
	}

	_, err := h.StartCrowdSec(context.Background())

	require.ErrorIs(t, err, errCrowdsecStateChanged)
	waitForResetStops(t, h)
	require.Contains(t, eng.callLog(), "stop")
}
