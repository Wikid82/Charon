package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// gatedStartExec is a CrowdSec engine stand-in whose Start blocks until unblock is called.
type gatedStartExec struct {
	entered     chan struct{}
	release     chan struct{}
	enteredOnce sync.Once
	releaseOnce sync.Once
	mu          sync.Mutex
	stopCalls   int
	started     bool
}

func newGatedStartExec() *gatedStartExec {
	return &gatedStartExec{entered: make(chan struct{}), release: make(chan struct{})}
}

func (g *gatedStartExec) unblock() {
	g.releaseOnce.Do(func() { close(g.release) })
}

func (g *gatedStartExec) Start(_ context.Context, _, _ string) (int, error) {
	g.enteredOnce.Do(func() { close(g.entered) })
	<-g.release
	g.mu.Lock()
	defer g.mu.Unlock()
	g.started = true
	return 4242, nil
}

func (g *gatedStartExec) Stop(_ context.Context, _ string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.stopCalls++
	return nil
}

// Status reports the engine running once the gated Start has returned, as a real engine would.
func (g *gatedStartExec) Status(_ context.Context, _ string) (running bool, pid int, err error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.started {
		return true, 4242, nil
	}
	return false, 0, nil
}

func (g *gatedStartExec) stops() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.stopCalls
}

// TestCrowdsecLifecycle_StopRefusedWhileStartRuns pins the lifecycle lock: while a Start is inside the engine
// executor, a second Stop is refused with the generic busy answer and never reaches the executor.
func TestCrowdsecLifecycle_StopRefusedWhileStartRuns(t *testing.T) {
	db := setupCrowdDB(t)
	exec := newGatedStartExec()
	t.Cleanup(exec.unblock)
	h := newTestCrowdsecHandler(t, db, exec, "/bin/false", t.TempDir())

	startDone := make(chan *httptest.ResponseRecorder, 1)
	go func() { startDone <- startRequest(t, h, context.Background()) }()

	select {
	case <-exec.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("Start never reached the engine executor")
	}

	stopW := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(stopW)
	c.Request = httptest.NewRequest(http.MethodPost, "/admin/crowdsec/stop", http.NoBody)
	h.Stop(c)

	require.Equal(t, http.StatusConflict, stopW.Code, stopW.Body.String())
	require.JSONEq(t,
		`{"error":"CrowdSec is finishing another change; try again in a moment","error_code":"crowdsec_busy"}`,
		stopW.Body.String())
	require.Equal(t, 0, exec.stops(), "a refused Stop must not reach the engine executor")

	exec.unblock()
	select {
	case w := <-startDone:
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not finish after the executor was released")
	}
}

// TestLifecycleOrNil pins that a typed-nil handler counts as not wired, and that a real handler passes through.
func TestLifecycleOrNil(t *testing.T) {
	var typedNil *CrowdsecHandler
	live := newTestCrowdsecHandler(t, setupCrowdDB(t), &fakeExec{}, "/bin/false", t.TempDir())

	require.Nil(t, LifecycleOrNil(nil))
	require.Nil(t, LifecycleOrNil(CrowdsecLifecycle(typedNil)))
	require.True(t, LifecycleOrNil(live) == CrowdsecLifecycle(live))
}

// TestWithCrowdsecLifecycle_SetterStoresInstanceOrNil pins the setter of each handler that receives the lifecycle:
// the same instance is stored, and a typed-nil handler is stored as not wired.
func TestWithCrowdsecLifecycle_SetterStoresInstanceOrNil(t *testing.T) {
	var typedNil *CrowdsecHandler
	live := newTestCrowdsecHandler(t, setupCrowdDB(t), &fakeExec{}, "/bin/false", t.TempDir())

	t.Run("emergency handler", func(t *testing.T) {
		h := NewEmergencyHandler(setupCrowdDB(t)).WithCrowdsecLifecycle(live)
		require.True(t, h.crowdsec == CrowdsecLifecycle(live))
		h = NewEmergencyHandler(setupCrowdDB(t)).WithCrowdsecLifecycle(CrowdsecLifecycle(typedNil))
		require.True(t, h.crowdsec == nil, "a typed-nil lifecycle must count as not wired")
	})
	t.Run("settings handler", func(t *testing.T) {
		h := (&SettingsHandler{}).WithCrowdsecLifecycle(live)
		require.True(t, h.crowdsec == CrowdsecLifecycle(live))
		h = (&SettingsHandler{}).WithCrowdsecLifecycle(CrowdsecLifecycle(typedNil))
		require.True(t, h.crowdsec == nil, "a typed-nil lifecycle must count as not wired")
	})
	t.Run("security handler", func(t *testing.T) {
		h := (&SecurityHandler{}).WithCrowdsecLifecycle(live)
		require.True(t, h.crowdsec == CrowdsecLifecycle(live))
		h = (&SecurityHandler{}).WithCrowdsecLifecycle(CrowdsecLifecycle(typedNil))
		require.True(t, h.crowdsec == nil, "a typed-nil lifecycle must count as not wired")
	})
}

// TestRespondCrowdsecLifecycleError_MapsEachCause pins the one mapping every lifecycle caller uses: the busy
// refusal (with and without a job) and each wrapped cause keep today's status, code and text.
func TestRespondCrowdsecLifecycleError_MapsEachCause(t *testing.T) {
	cases := []struct {
		name string
		err  error
		code int
		body string
	}{
		{
			name: "busy without a job",
			err:  &CrowdsecBusyError{},
			code: http.StatusConflict,
			body: `{"error":"CrowdSec is finishing another change; try again in a moment","error_code":"crowdsec_busy"}`,
		},
		{
			name: "busy with a job",
			err:  &CrowdsecBusyError{JobID: "job-1"},
			code: http.StatusConflict,
			body: `{"error":"CrowdSec is busy installing a preset; try again when it finishes","error_code":"crowdsec_busy","job_id":"job-1"}`,
		},
		{
			name: "status read",
			err:  fmt.Errorf("%w: boom", errCrowdsecStatus),
			code: http.StatusInternalServerError,
			body: `{"error":"failed to read CrowdSec status"}`,
		},
		{
			name: "persist",
			err:  fmt.Errorf("%w: boom", errCrowdsecPersist),
			code: http.StatusInternalServerError,
			body: `{"error":"Failed to persist configuration"}`,
		},
		{
			name: "start",
			err:  fmt.Errorf("%w: boom", errCrowdsecStart),
			code: http.StatusInternalServerError,
			body: `{"error":"failed to start CrowdSec"}`,
		},
		{
			name: "stop",
			err:  fmt.Errorf("%w: boom", errCrowdsecStop),
			code: http.StatusInternalServerError,
			body: `{"error":"failed to stop CrowdSec"}`,
		},
		{
			name: "engine exited",
			err:  fmt.Errorf("%w: pid 4242", errCrowdsecEngineExited),
			code: http.StatusInternalServerError,
			body: `{"error":"CrowdSec stopped right after it started. Check the CrowdSec log for the reason.","error_code":"crowdsec_engine_exited"}`,
		},
		{
			name: "unknown cause",
			err:  errors.New("unexpected"),
			code: http.StatusInternalServerError,
			body: `{"error":"CrowdSec request failed"}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			respondCrowdsecLifecycleError(c, tc.err)
			require.Equal(t, tc.code, w.Code)
			require.JSONEq(t, tc.body, w.Body.String())
		})
	}
	require.NotEmpty(t, (&CrowdsecBusyError{}).Error())
}
