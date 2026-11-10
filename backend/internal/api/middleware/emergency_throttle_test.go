package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// recordingAuditor captures audit rows written by the throttle.
type recordingAuditor struct {
	mu   sync.Mutex
	rows []models.SecurityAudit
	err  error
}

func (r *recordingAuditor) LogAudit(a *models.SecurityAudit) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rows = append(r.rows, *a)
	return r.err
}

func (r *recordingAuditor) snapshot() []models.SecurityAudit {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]models.SecurityAudit(nil), r.rows...)
}

// fakeThrottleClock is a settable clock for the per-client budget.
type fakeThrottleClock struct {
	mu sync.Mutex
	t  time.Time
}

func (f *fakeThrottleClock) now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.t
}

func (f *fakeThrottleClock) advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.t = f.t.Add(d)
}

func newFakeThrottleClock() *fakeThrottleClock {
	return &fakeThrottleClock{t: time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)}
}

// throttleRouter mounts the throttle in front of a handler that counts its calls.
func throttleRouter(th *EmergencyResetThrottle, handler gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	r.POST("/reset", th.Middleware(), handler)
	return r
}

func doReset(r http.Handler, remoteAddr string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/reset", http.NoBody)
	req.RemoteAddr = remoteAddr
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func okHandler(c *gin.Context) { c.Status(http.StatusOK) }

func TestEmergencyResetThrottle_BudgetExhaustionAnswers429AndAudits(t *testing.T) {
	clock := newFakeThrottleClock()
	auditor := &recordingAuditor{}
	th, err := NewEmergencyResetThrottle("main", auditor, WithEmergencyResetClock(clock.now))
	require.NoError(t, err)

	calls := 0
	r := throttleRouter(th, func(c *gin.Context) {
		calls++
		c.Status(http.StatusOK)
	})

	for i := 0; i < EmergencyResetAttemptsPerWindow; i++ {
		assert.Equal(t, http.StatusOK, doReset(r, "192.0.2.10:1000").Code, "attempt %d within budget", i+1)
	}

	w := doReset(r, "192.0.2.10:1000")
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.NotEmpty(t, w.Header().Get("Retry-After"))
	assert.Equal(t, EmergencyResetAttemptsPerWindow, calls, "over-budget request must not reach the handler")

	rows := auditor.snapshot()
	require.Len(t, rows, 1)
	assert.Equal(t, "emergency_reset_throttled", rows[0].Action)
	assert.Equal(t, "192.0.2.10", rows[0].Actor)
	assert.Contains(t, rows[0].Details, "listener=main")

	// The budget refills once the window has passed.
	clock.advance(EmergencyResetWindow)
	assert.Equal(t, http.StatusOK, doReset(r, "192.0.2.10:1000").Code)
}

func TestEmergencyResetThrottle_PerAddressIsolation(t *testing.T) {
	clock := newFakeThrottleClock()
	th, err := NewEmergencyResetThrottle("main", nil, WithEmergencyResetClock(clock.now))
	require.NoError(t, err)
	r := throttleRouter(th, okHandler)

	for i := 0; i < EmergencyResetAttemptsPerWindow; i++ {
		doReset(r, "192.0.2.10:1000")
	}
	assert.Equal(t, http.StatusTooManyRequests, doReset(r, "192.0.2.10:2000").Code)
	assert.Equal(t, http.StatusOK, doReset(r, "192.0.2.11:1000").Code, "a different address keeps its own budget")
}

func TestEmergencyResetThrottle_SemaphoreWaitsForFreeSlot(t *testing.T) {
	th, err := NewEmergencyResetThrottle("main", nil, WithEmergencyResetWait(5*time.Second))
	require.NoError(t, err)

	entered := make(chan struct{}, EmergencyResetConcurrency+1)
	gate := make(chan struct{})
	r := throttleRouter(th, func(c *gin.Context) {
		entered <- struct{}{}
		<-gate
		c.Status(http.StatusOK)
	})

	var wg sync.WaitGroup
	results := make(chan int, 3)
	for _, addr := range []string{"192.0.2.1:1", "192.0.2.2:1", "192.0.2.3:1"} {
		wg.Add(1)
		go func(a string) {
			defer wg.Done()
			results <- doReset(r, a).Code
		}(addr)
	}

	// Two checks hold the slots; the third must wait rather than enter.
	<-entered
	<-entered
	select {
	case <-entered:
		t.Fatal("third check entered while both slots were held")
	case <-time.After(100 * time.Millisecond):
	}

	// Freeing one slot lets the waiting request proceed.
	gate <- struct{}{}
	<-entered
	close(gate)
	wg.Wait()
	close(results)

	for code := range results {
		assert.Equal(t, http.StatusOK, code)
	}
}

func TestEmergencyResetThrottle_SemaphoreWaitTimesOutWith429(t *testing.T) {
	auditor := &recordingAuditor{}
	const wait = 50 * time.Millisecond
	th, err := NewEmergencyResetThrottle("loopback", auditor, WithEmergencyResetWait(wait))
	require.NoError(t, err)

	entered := make(chan struct{}, EmergencyResetConcurrency)
	gate := make(chan struct{})
	calls := make(chan struct{}, 10)
	r := throttleRouter(th, func(c *gin.Context) {
		calls <- struct{}{}
		entered <- struct{}{}
		<-gate
		c.Status(http.StatusOK)
	})

	var wg sync.WaitGroup
	for _, addr := range []string{"192.0.2.1:1", "192.0.2.2:1"} {
		wg.Add(1)
		go func(a string) {
			defer wg.Done()
			doReset(r, a)
		}(addr)
	}
	<-entered
	<-entered

	start := time.Now()
	w := doReset(r, "192.0.2.3:1")
	elapsed := time.Since(start)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.GreaterOrEqual(t, elapsed, wait, "the request must wait for a slot before answering 429")
	assert.Len(t, calls, EmergencyResetConcurrency, "the timed-out request must not reach the handler")

	rows := auditor.snapshot()
	require.Len(t, rows, 1)
	assert.Equal(t, "emergency_reset_throttled", rows[0].Action)
	assert.Equal(t, "192.0.2.3", rows[0].Actor)

	close(gate)
	wg.Wait()
}

func TestEmergencyResetThrottle_SeparateInstancesDoNotShareBudgetOrSlots(t *testing.T) {
	clock := newFakeThrottleClock()
	mainTh, err := NewEmergencyResetThrottle("main", nil, WithEmergencyResetClock(clock.now), WithEmergencyResetWait(50*time.Millisecond))
	require.NoError(t, err)
	loopbackTh, err := NewEmergencyResetThrottle("loopback", nil, WithEmergencyResetClock(clock.now), WithEmergencyResetWait(50*time.Millisecond))
	require.NoError(t, err)

	// Exhaust the main instance's budget for one address.
	mainRouter := throttleRouter(mainTh, okHandler)
	for i := 0; i < EmergencyResetAttemptsPerWindow; i++ {
		doReset(mainRouter, "127.0.0.1:1")
	}
	assert.Equal(t, http.StatusTooManyRequests, doReset(mainRouter, "127.0.0.1:1").Code)

	// The loopback instance keeps its full budget for the same address.
	loopbackRouter := throttleRouter(loopbackTh, okHandler)
	assert.Equal(t, http.StatusOK, doReset(loopbackRouter, "127.0.0.1:1").Code)

	// Holding both slots on the main instance must not block the loopback instance.
	entered := make(chan struct{}, EmergencyResetConcurrency)
	gate := make(chan struct{})
	holdRouter := throttleRouter(mainTh, func(c *gin.Context) {
		entered <- struct{}{}
		<-gate
		c.Status(http.StatusOK)
	})
	var wg sync.WaitGroup
	for _, addr := range []string{"192.0.2.1:1", "192.0.2.2:1"} {
		wg.Add(1)
		go func(a string) {
			defer wg.Done()
			doReset(holdRouter, a)
		}(addr)
	}
	<-entered
	<-entered

	assert.Equal(t, http.StatusOK, doReset(loopbackRouter, "192.0.2.9:1").Code)

	close(gate)
	wg.Wait()
}

func TestEmergencyResetThrottle_NilAuditorAndFailingAuditorStillAnswer429(t *testing.T) {
	failing := &recordingAuditor{err: errors.New("db down")}
	for _, auditor := range []EmergencyThrottleAuditor{nil, failing} {
		th, err := NewEmergencyResetThrottle("main", auditor)
		require.NoError(t, err)
		r := throttleRouter(th, okHandler)
		for i := 0; i < EmergencyResetAttemptsPerWindow; i++ {
			doReset(r, "192.0.2.20:1")
		}
		assert.Equal(t, http.StatusTooManyRequests, doReset(r, "192.0.2.20:1").Code)
	}
	assert.Len(t, failing.snapshot(), 1, "a failing auditor is still invoked once")
}

func TestNewEmergencyResetThrottle_DefaultsMatchSpec(t *testing.T) {
	th, err := NewEmergencyResetThrottle("main", nil)
	require.NoError(t, err)
	assert.Equal(t, EmergencyResetConcurrency, cap(th.slots))
	assert.Equal(t, EmergencyResetSlotWait, th.wait)
}
