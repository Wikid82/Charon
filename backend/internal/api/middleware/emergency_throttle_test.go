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

// failHandler answers like a reset with a wrong or missing token.
func failHandler(c *gin.Context) { c.Status(http.StatusUnauthorized) }

func TestEmergencyResetThrottle_BudgetExhaustionAnswers429AndAudits(t *testing.T) {
	clock := newFakeThrottleClock()
	auditor := &recordingAuditor{}
	th := NewEmergencyResetThrottle("main", auditor, WithEmergencyResetClock(clock.now))

	calls := 0
	r := throttleRouter(th, func(c *gin.Context) {
		calls++
		c.Status(http.StatusUnauthorized)
	})

	for i := 0; i < EmergencyResetAttemptsPerWindow; i++ {
		assert.Equal(t, http.StatusUnauthorized, doReset(r, "192.0.2.10:1000").Code, "failed attempt %d within budget", i+1)
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
	assert.Equal(t, http.StatusUnauthorized, doReset(r, "192.0.2.10:1000").Code)
}

func TestEmergencyResetThrottle_SuccessesNeverExhaustBudget(t *testing.T) {
	th := NewEmergencyResetThrottle("main", nil, WithEmergencyResetClock(newFakeThrottleClock().now))

	calls := 0
	r := throttleRouter(th, func(c *gin.Context) {
		calls++
		c.Status(http.StatusOK)
	})

	const successes = 4 * EmergencyResetAttemptsPerWindow
	for i := 0; i < successes; i++ {
		require.Equal(t, http.StatusOK, doReset(r, "192.0.2.12:1000").Code, "successful reset %d must not be throttled", i+1)
	}
	assert.Equal(t, successes, calls)
}

func TestEmergencyResetThrottle_InterleavedSuccessesDoNotCountAsFailures(t *testing.T) {
	th := NewEmergencyResetThrottle("main", nil, WithEmergencyResetClock(newFakeThrottleClock().now))

	// Five failures spread among successes: the budget is spent on the failures only.
	outcomes := []int{
		http.StatusOK, http.StatusUnauthorized, http.StatusOK, http.StatusUnauthorized,
		http.StatusOK, http.StatusOK, http.StatusUnauthorized, http.StatusOK,
		http.StatusUnauthorized, http.StatusOK, http.StatusOK, http.StatusUnauthorized,
	}
	require.Len(t, outcomes, 12)
	idx := 0
	r := throttleRouter(th, func(c *gin.Context) {
		c.Status(outcomes[idx])
		idx++
	})

	for i, want := range outcomes {
		assert.Equal(t, want, doReset(r, "192.0.2.13:1000").Code, "outcome %d", i+1)
	}
	assert.Equal(t, http.StatusTooManyRequests, doReset(r, "192.0.2.13:1000").Code, "the sixth failure is refused")
}

func TestEmergencyResetThrottle_OnlyUnauthorizedChargesBudget(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		charged bool
	}{
		{"ok", http.StatusOK, false},
		{"no content", http.StatusNoContent, false},
		{"throttle status from handler", http.StatusTooManyRequests, false},
		{"unauthorized", http.StatusUnauthorized, true},
		{"bad request", http.StatusBadRequest, false},
		{"forbidden", http.StatusForbidden, false},
		{"server error", http.StatusInternalServerError, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			th := NewEmergencyResetThrottle("main", nil, WithEmergencyResetClock(newFakeThrottleClock().now))
			calls := 0
			r := throttleRouter(th, func(c *gin.Context) {
				calls++
				c.Status(tc.status)
			})

			const attempts = EmergencyResetAttemptsPerWindow + 3
			for i := 0; i < attempts; i++ {
				doReset(r, "192.0.2.14:1000")
			}
			if tc.charged {
				assert.Equal(t, EmergencyResetAttemptsPerWindow, calls)
				assert.Equal(t, http.StatusTooManyRequests, doReset(r, "192.0.2.14:1000").Code)
			} else {
				assert.Equal(t, attempts, calls, "a %d response must not consume the budget", tc.status)
			}
		})
	}
}

func TestEmergencyResetThrottle_PerAddressIsolation(t *testing.T) {
	clock := newFakeThrottleClock()
	th := NewEmergencyResetThrottle("main", nil, WithEmergencyResetClock(clock.now))
	r := throttleRouter(th, failHandler)

	for i := 0; i < EmergencyResetAttemptsPerWindow; i++ {
		doReset(r, "192.0.2.10:1000")
	}
	assert.Equal(t, http.StatusTooManyRequests, doReset(r, "192.0.2.10:2000").Code)
	assert.Equal(t, http.StatusUnauthorized, doReset(r, "192.0.2.11:1000").Code, "a different address keeps its own budget")
}

func TestEmergencyResetThrottle_QueuedRequestRechecksBudgetUnderSlot(t *testing.T) {
	clock := newFakeThrottleClock()
	th := NewEmergencyResetThrottle("main", nil, WithEmergencyResetClock(clock.now), WithEmergencyResetWait(5*time.Second))

	// Holding requests (header X-Gate) block in the handler until gate closes.
	entered := make(chan struct{}, EmergencyResetConcurrency)
	gate := make(chan struct{})
	calls := make(chan struct{}, 10)
	r := gin.New()
	gin.SetMode(gin.TestMode)
	_ = r.SetTrustedProxies(nil)
	r.POST("/reset", th.Middleware(), func(c *gin.Context) {
		calls <- struct{}{}
		if c.GetHeader("X-Gate") != "" {
			entered <- struct{}{}
			<-gate
		}
		c.Status(http.StatusUnauthorized)
	})
	send := func(addr, gated string) <-chan int {
		out := make(chan int, 1)
		go func() {
			req := httptest.NewRequest(http.MethodPost, "/reset", http.NoBody)
			req.RemoteAddr = addr
			if gated != "" {
				req.Header.Set("X-Gate", gated)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			out <- w.Code
		}()
		return out
	}

	// Four failures leave one token for the address.
	for i := 0; i < EmergencyResetAttemptsPerWindow-1; i++ {
		require.Equal(t, http.StatusUnauthorized, doReset(r, "192.0.2.15:1").Code)
	}

	// Two gated requests from the same address hold both slots, each with the last token peeked.
	holderA := send("192.0.2.15:1", "a")
	holderB := send("192.0.2.15:1", "b")
	<-entered
	<-entered

	// This request peeks a token, then queues for a slot.
	queued := send("192.0.2.15:1", "")
	time.Sleep(50 * time.Millisecond)

	close(gate)
	assert.Equal(t, http.StatusUnauthorized, <-holderA)
	assert.Equal(t, http.StatusUnauthorized, <-holderB)

	// The holders spent the last token, so the queued request is refused before the handler.
	assert.Equal(t, http.StatusTooManyRequests, <-queued)
	assert.Len(t, calls, EmergencyResetAttemptsPerWindow+1, "the queued request must not reach the handler")
}

func TestEmergencyResetThrottle_SemaphoreWaitsForFreeSlot(t *testing.T) {
	th := NewEmergencyResetThrottle("main", nil, WithEmergencyResetWait(5*time.Second))

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
	th := NewEmergencyResetThrottle("loopback", auditor, WithEmergencyResetWait(wait))

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
	mainTh := NewEmergencyResetThrottle("main", nil, WithEmergencyResetClock(clock.now), WithEmergencyResetWait(50*time.Millisecond))
	loopbackTh := NewEmergencyResetThrottle("loopback", nil, WithEmergencyResetClock(clock.now), WithEmergencyResetWait(50*time.Millisecond))

	// Exhaust the main instance's budget for one address.
	mainRouter := throttleRouter(mainTh, failHandler)
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
		th := NewEmergencyResetThrottle("main", auditor)
		r := throttleRouter(th, failHandler)
		for i := 0; i < EmergencyResetAttemptsPerWindow; i++ {
			doReset(r, "192.0.2.20:1")
		}
		assert.Equal(t, http.StatusTooManyRequests, doReset(r, "192.0.2.20:1").Code)
	}
	assert.Len(t, failing.snapshot(), 1, "a failing auditor is still invoked once")
}

func TestNewEmergencyResetThrottle_DefaultsMatchSpec(t *testing.T) {
	th := NewEmergencyResetThrottle("main", nil)
	assert.Equal(t, EmergencyResetConcurrency, cap(th.slots))
	assert.Equal(t, EmergencyResetSlotWait, th.wait)
}
