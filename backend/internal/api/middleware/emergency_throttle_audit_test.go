package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// floodRefusals sends n requests from one address; the budget admits the first
// EmergencyResetAttemptsPerWindow and refuses the rest.
func floodRefusals(t *testing.T, r http.Handler, addr string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		doReset(r, addr)
	}
}

func TestEmergencyResetThrottle_FloodAuditsOncePerWindowPerAddress(t *testing.T) {
	clock := newFakeThrottleClock()
	auditor := &recordingAuditor{}
	th, err := NewEmergencyResetThrottle("main", auditor, WithEmergencyResetClock(clock.now))
	require.NoError(t, err)
	r := throttleRouter(th, failHandler)

	const flood = 200
	floodRefusals(t, r, "192.0.2.50:1", EmergencyResetAttemptsPerWindow+flood)

	rows := auditor.snapshot()
	require.Len(t, rows, 1, "a flood within one window must write one audit row")
	assert.Equal(t, "192.0.2.50", rows[0].Actor)
	assert.Equal(t, EmergencyResetThrottledAction, rows[0].Action)
	assert.NotContains(t, rows[0].Details, "suppressed_refusals", "the first refusal of a window carries no suppressed count")

	// Still refused on every request: the 429 behaviour is unchanged.
	assert.Equal(t, http.StatusTooManyRequests, doReset(r, "192.0.2.50:1").Code)

	// After the window resets, the next refusal is audited and carries the suppressed count.
	clock.advance(EmergencyResetWindow)
	floodRefusals(t, r, "192.0.2.50:1", EmergencyResetAttemptsPerWindow+flood)

	rows = auditor.snapshot()
	require.Len(t, rows, 2, "a second window must write exactly one more audit row")
	assert.Contains(t, rows[1].Details, fmt.Sprintf("suppressed_refusals=%d", flood))
}

func TestEmergencyResetThrottle_SeparateAddressesAuditedSeparately(t *testing.T) {
	clock := newFakeThrottleClock()
	auditor := &recordingAuditor{}
	th, err := NewEmergencyResetThrottle("main", auditor, WithEmergencyResetClock(clock.now))
	require.NoError(t, err)
	r := throttleRouter(th, failHandler)

	floodRefusals(t, r, "192.0.2.60:1", EmergencyResetAttemptsPerWindow+10)
	floodRefusals(t, r, "192.0.2.61:1", EmergencyResetAttemptsPerWindow+10)
	floodRefusals(t, r, "192.0.2.60:2", 3)

	rows := auditor.snapshot()
	require.Len(t, rows, 2, "each address gets its own audit row per window")
	actors := []string{rows[0].Actor, rows[1].Actor}
	assert.ElementsMatch(t, []string{"192.0.2.60", "192.0.2.61"}, actors)
}

func TestEmergencyResetThrottle_ConcurrentFloodAuditsOnce(t *testing.T) {
	clock := newFakeThrottleClock()
	auditor := &recordingAuditor{}
	th, err := NewEmergencyResetThrottle("main", auditor, WithEmergencyResetClock(clock.now))
	require.NoError(t, err)
	r := throttleRouter(th, failHandler)

	const workers, perWorker = 20, 25
	var wg sync.WaitGroup
	codes := make(chan int, workers*perWorker)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				codes <- doReset(r, "192.0.2.70:1").Code
			}
		}()
	}
	wg.Wait()
	close(codes)

	admitted := 0
	for code := range codes {
		if code == http.StatusUnauthorized {
			admitted++
		}
	}
	assert.Equal(t, EmergencyResetAttemptsPerWindow, admitted)
	assert.Len(t, auditor.snapshot(), 1, "concurrent refusals from one address must write one audit row")
}

func TestEmergencyResetThrottle_ExpiredAuditStateIsPruned(t *testing.T) {
	clock := newFakeThrottleClock()
	th, err := NewEmergencyResetThrottle("main", nil, WithEmergencyResetClock(clock.now))
	require.NoError(t, err)
	r := throttleRouter(th, failHandler)

	for i := 0; i < 50; i++ {
		floodRefusals(t, r, fmt.Sprintf("192.0.2.%d:1", 100+i), EmergencyResetAttemptsPerWindow+1)
	}
	assert.Len(t, th.auditState, 50)

	clock.advance(EmergencyResetWindow)
	floodRefusals(t, r, "198.51.100.1:1", EmergencyResetAttemptsPerWindow+1)
	assert.Len(t, th.auditState, 1, "entries whose window has expired must be dropped")
}

func TestEmergencyResetThrottle_NilAuditorStillTracksWindows(t *testing.T) {
	th, err := NewEmergencyResetThrottle("main", nil)
	require.NoError(t, err)
	r := throttleRouter(th, failHandler)
	floodRefusals(t, r, "192.0.2.80:1", EmergencyResetAttemptsPerWindow+5)
	assert.Len(t, th.auditState, 1)
}
