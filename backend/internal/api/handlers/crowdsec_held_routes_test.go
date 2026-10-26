package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// These tests cover the hold side of the busy check (#1730): a request that changes DataDir holds the install
// manager while it runs, so a background preset install cannot start in the middle of it.

const (
	probeJobID         = "00000000-0000-4000-8000-0000000000b1"
	heldWhitelistUUID  = "00000000-0000-4000-8000-0000000000aa"
	holdWaitForRequest = 10 * time.Second
)

// gatedCrowdsec stands in for every executor a held route can reach: the engine, the cscli command runner and the
// console. Each call waits until open is called, so a test can observe the handler while its request is in flight.
type gatedCrowdsec struct {
	gate    chan struct{}
	entered chan struct{}
	enter   sync.Once
	opened  sync.Once
}

func newGatedCrowdsec(t *testing.T) *gatedCrowdsec {
	t.Helper()
	g := &gatedCrowdsec{gate: make(chan struct{}), entered: make(chan struct{})}
	t.Cleanup(g.open)
	return g
}

// open lets every waiting and future call through. It is idempotent and also runs as a cleanup.
func (g *gatedCrowdsec) open() { g.opened.Do(func() { close(g.gate) }) }

func (g *gatedCrowdsec) wait() {
	g.enter.Do(func() { close(g.entered) })
	<-g.gate
}

func (g *gatedCrowdsec) Start(context.Context, string, string) (int, error) {
	g.wait()
	return 999, nil
}

func (g *gatedCrowdsec) Stop(context.Context, string) error {
	g.wait()
	return nil
}

func (g *gatedCrowdsec) Status(context.Context, string) (running bool, pid int, err error) {
	g.wait()
	return true, 4242, nil
}

func (g *gatedCrowdsec) Execute(context.Context, string, ...string) ([]byte, error) {
	g.wait()
	return []byte("ok"), nil
}

func (g *gatedCrowdsec) ExecuteWithEnv(context.Context, string, []string, map[string]string) ([]byte, error) {
	g.wait()
	return []byte("ok"), nil
}

// gateHeldRoutes points every executor of f at one gate. The whitelist reload signals a PID, so that is stubbed too.
func gateHeldRoutes(t *testing.T, f *busyFixture) *gatedCrowdsec {
	t.Helper()
	g := newGatedCrowdsec(t)
	f.h.Executor = g
	f.h.CmdExec = g
	f.h.Console = crowdsec.NewConsoleEnrollmentService(f.h.DB, g, f.dir, "test-secret")
	f.h.signalProcess = func(int, syscall.Signal) error { return nil }
	return g
}

// requireBusyOtherRefusal asserts a start refused only by a request in flight: 409, the busy code, and no job id,
// because no job exists to name.
func requireBusyOtherRefusal(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	resp := decodeBody(t, w)
	require.Equal(t, codeCrowdsecBusy, resp["error_code"])
	require.Equal(t, msgCrowdsecBusyOther, resp["error"])
	require.NotContains(t, resp, "job_id", "a refusal by a request names no job")
}

// asyncApplyRequest is the body of an async hub preset install, which is refused while a hold is out.
func asyncApplyRequest(f *busyFixture) (method, path string, body []byte, contentType string) {
	return http.MethodPost, "/admin/crowdsec/presets/apply", f.presetBody, "application/json"
}

// TestHeldRoutesBlockAJobWhileTheyRun blocks every held route mid-request and proves a job cannot start until the
// request ends. A route classed routeHeld without a hold fails here.
func TestHeldRoutesBlockAJobWhileTheyRun(t *testing.T) {
	for route, c := range mutatingCrowdsecRoutes {
		if c.class != routeHeld {
			continue
		}
		t.Run(route, func(t *testing.T) {
			f := newBusyFixture(t)
			g := gateHeldRoutes(t, f)
			require.NoError(t, f.h.DB.Create(&models.CrowdSecWhitelist{UUID: heldWhitelistUUID, IPOrCIDR: "192.0.2.30"}).Error)
			method, path, body, ct := busyCases[route].request(t, f)

			done := make(chan *httptest.ResponseRecorder, 1)
			go func() { done <- f.do(method, path, body, ct) }()
			select {
			case <-g.entered:
			case <-time.After(holdWaitForRequest):
				require.FailNow(t, "the held request never reached its executor")
			}

			require.False(t, f.h.ApplyJobs().TryAcquire(probeJobID), "a job must wait out a request in flight")

			g.open()
			<-done
			require.True(t, f.h.ApplyJobs().TryAcquire(probeJobID), "the job may start once the request ended")
			f.h.ApplyJobs().Release(probeJobID)
		})
	}
}

// TestBusyRefusalWhileARequestIsInFlightNamesNoJob checks the HTTP answer a preset install gets while Stop holds the
// manager: 409 with the busy code and the "finishing another change" text, and no job id.
func TestBusyRefusalWhileARequestIsInFlightNamesNoJob(t *testing.T) {
	f := newBusyFixture(t)
	g := gateHeldRoutes(t, f)

	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- f.do(http.MethodPost, "/admin/crowdsec/stop", nil, "") }()
	select {
	case <-g.entered:
	case <-time.After(holdWaitForRequest):
		require.FailNow(t, "stop never reached its executor")
	}

	requireBusyOtherRefusal(t, f.do(asyncApplyRequest(f)))

	g.open()
	<-done
	require.Equal(t, http.StatusOK, f.do(http.MethodPost, "/admin/crowdsec/stop", nil, "").Code, "the install may start again once Stop has returned")
}

// TestRefusedHoldNeverNamesAJob pins T1 at the HTTP level: while a hold is out and no job exists, no refusal may
// carry a job id, so the UI never resumes a job that does not exist.
func TestRefusedHoldNeverNamesAJob(t *testing.T) {
	f := newBusyFixture(t)
	release, _, ok := f.h.ApplyJobs().Hold()
	require.True(t, ok)
	defer release()

	for i := 0; i < 200; i++ {
		requireBusyOtherRefusal(t, f.do(asyncApplyRequest(f)))
	}
	_, busy := f.h.ApplyJobs().Busy()
	require.False(t, busy, "a refused start must never leave a job flag behind")
}

// TestSecondRestoreAnswersRestoreRunningWhileOneRuns keeps restore-vs-restore exclusion: holds never refuse each
// other, so restore.running has to refuse the second restore with its own message.
func TestSecondRestoreAnswersRestoreRunningWhileOneRuns(t *testing.T) {
	f := newBusyFixture(t)
	g := gateHeldRoutes(t, f)
	restore := func() *httptest.ResponseRecorder {
		return f.do(http.MethodPost, "/admin/crowdsec/hub-items/restore",
			restoreBody(crowdsec.HubItem{Type: "scenarios", Name: "a/one"}), "application/json")
	}

	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- restore() }()
	select {
	case <-g.entered:
	case <-time.After(holdWaitForRequest):
		require.FailNow(t, "the first restore never started")
	}

	w := restore()
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	require.Equal(t, msgRestoreRunning, decodeBody(t, w)["error"])

	g.open()
	<-done
}

// TestHeldRouteHoldIsReleasedWhenTheHandlerPanics pins the panic path: the hold is released by a defer right after a
// successful hold, so gin's recovery leaves the manager free for the next install.
func TestHeldRouteHoldIsReleasedWhenTheHandlerPanics(t *testing.T) {
	f := newBusyFixture(t)
	r := gin.New()
	r.Use(gin.Recovery())
	r.POST("/panic", func(c *gin.Context) {
		release, ok := f.h.holdUnlessBusy(c)
		if !ok {
			return
		}
		defer release()
		panic("boom")
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/panic", http.NoBody))
	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.True(t, f.h.ApplyJobs().TryAcquire(probeJobID), "a panicking request must not leave a hold behind")
	f.h.ApplyJobs().Release(probeJobID)
}
