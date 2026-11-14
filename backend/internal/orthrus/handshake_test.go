package orthrus

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/ratelimit"
)

const (
	hsUUIDA       = "11111111-1111-4111-8111-111111111111"
	hsUUIDB       = "22222222-2222-4222-8222-222222222222"
	hsUUIDMissing = "33333333-3333-4333-8333-333333333333"
	hsClientIP    = "192.0.2.10"
	hsKeyA        = "ch_orthrus_hs_key_a" //nolint:gosec // G101: test credential
	hsKeyB        = "ch_orthrus_hs_key_b" //nolint:gosec // G101: test credential
	hsKeyC        = "ch_orthrus_hs_key_c" //nolint:gosec // G101: test credential
	hsWrongKey    = "ch_orthrus_hs_wrong" //nolint:gosec // G101: test credential
)

// fakeHandshakeClock is a settable clock so budget refills are deterministic.
type fakeHandshakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeHandshakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeHandshakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// compareLog records the stored hash of every bcrypt comparison the guard runs.
type compareLog struct {
	mu     sync.Mutex
	hashes [][]byte
}

func (l *compareLog) install(g *handshakeGuard) {
	g.compare = func(hash, password []byte) error {
		l.mu.Lock()
		l.hashes = append(l.hashes, append([]byte(nil), hash...))
		l.mu.Unlock()
		return bcrypt.CompareHashAndPassword(hash, password)
	}
}

func (l *compareLog) count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.hashes)
}

func (l *compareLog) last() []byte {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.hashes) == 0 {
		return nil
	}
	return l.hashes[len(l.hashes)-1]
}

func (l *compareLog) reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.hashes = nil
}

type handshakeFixture struct {
	t     *testing.T
	db    *gorm.DB
	srv   *OrthrusServer
	clock *fakeHandshakeClock
	log   *compareLog
}

func newHandshakeFixture(t *testing.T) *handshakeFixture {
	t.Helper()
	db := setupServerTestDB(t)
	srv, err := NewOrthrusServer(db, setupTestCA(t))
	require.NoError(t, err)

	clock := &fakeHandshakeClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	guard, err := newHandshakeGuard(clock.Now)
	require.NoError(t, err)
	log := &compareLog{}
	log.install(guard)
	srv.handshake = guard

	return &handshakeFixture{t: t, db: db, srv: srv, clock: clock, log: log}
}

// addAgent stores an agent with MinCost hashes to keep tests fast. Failure
// paths compare against the cost-12 dummy hash, which is still exercised.
func (f *handshakeFixture) addAgent(agentUUID, name, key string) {
	f.t.Helper()
	hash, err := bcrypt.GenerateFromPassword(BcryptInput(key), bcrypt.MinCost)
	require.NoError(f.t, err)
	require.NoError(f.t, f.db.Create(&models.OrthrusAgent{
		UUID:        agentUUID,
		Name:        name,
		AuthKeyHash: string(hash),
		Status:      models.OrthrusStatusPending,
	}).Error)
}

func (f *handshakeFixture) storedHash(agentUUID string) []byte {
	f.t.Helper()
	var agent models.OrthrusAgent
	require.NoError(f.t, f.db.Where("uuid = ?", agentUUID).First(&agent).Error)
	return []byte(agent.AuthKeyHash)
}

func (f *handshakeFixture) auth(agentRef, key string) (*models.OrthrusAgent, *handshakeRejection) {
	return f.srv.authenticate(context.Background(), hsClientIP, agentRef, key)
}

func (f *handshakeFixture) legacyWarnings() int {
	n := 0
	f.srv.handshake.warnedLegacy.Range(func(_, _ any) bool {
		n++
		return true
	})
	return n
}

func (f *handshakeFixture) holdAllSlots() {
	f.srv.handshake.slots <- struct{}{}
	f.srv.handshake.slots <- struct{}{}
}

func (f *handshakeFixture) exhaustFailures(agentRef string) {
	f.t.Helper()
	for i := 0; i < handshakeFailuresPerWindow; i++ {
		_, rej := f.auth(agentRef, hsWrongKey)
		require.NotNil(f.t, rej)
		require.Equal(f.t, http.StatusUnauthorized, rej.status)
	}
}

func TestBcryptInput_TruncatesLongKeys(t *testing.T) {
	short := "ch_orthrus_short"
	assert.Equal(t, []byte(short), BcryptInput(short))

	exact72 := strings.Repeat("b", 72)
	assert.Equal(t, []byte(exact72[:71]), BcryptInput(exact72))

	long := strings.Repeat("a", 80)
	assert.Equal(t, []byte(long[:71]), BcryptInput(long))
}

func TestNewHandshakeGuard_Defaults(t *testing.T) {
	g, err := newHandshakeGuard(time.Now)
	require.NoError(t, err)
	assert.Equal(t, handshakeVerifySlots, cap(g.slots))
	assert.Empty(t, g.slots)
	assert.Equal(t, handshakeVerifyWait, g.verifyWait)
	assert.NotNil(t, g.compare)
}

func TestHandshake_UUIDTarget_SingleCompare(t *testing.T) {
	f := newHandshakeFixture(t)
	f.addAgent(hsUUIDA, "agent-a", hsKeyA)
	f.addAgent(hsUUIDB, "agent-b", hsKeyB)

	agent, rej := f.auth(hsUUIDA, hsKeyA)
	require.Nil(t, rej)
	assert.Equal(t, hsUUIDA, agent.UUID)
	assert.Equal(t, 1, f.log.count())
	assert.Equal(t, 0, f.legacyWarnings(), "uuid match must not warn")
}

func TestHandshake_UUIDTarget_WrongKey_NoFallbackToOtherAgents(t *testing.T) {
	f := newHandshakeFixture(t)
	f.addAgent(hsUUIDA, "agent-a", hsKeyA)
	f.addAgent(hsUUIDB, "agent-b", hsKeyB)

	// The presented key belongs to B, but the header names A: the failure
	// must not fall back to scanning every agent.
	agent, rej := f.auth(hsUUIDA, hsKeyB)
	assert.Nil(t, agent)
	require.NotNil(t, rej)
	assert.Equal(t, http.StatusUnauthorized, rej.status)
	assert.Equal(t, 1, f.log.count())
	assert.Equal(t, f.storedHash(hsUUIDA), f.log.last())
}

func TestHandshake_UUIDShapedWithoutRow_FailsAtOnceWithDummyCompare(t *testing.T) {
	f := newHandshakeFixture(t)
	f.addAgent(hsUUIDB, "agent-b", hsKeyB)
	// An agent whose name equals the UUID-shaped header must not be matched:
	// UUID-shaped headers never take the name lookup or the scan.
	f.addAgent(hsUUIDA, hsUUIDMissing, hsKeyC)

	agent, rej := f.auth(hsUUIDMissing, hsKeyC)
	assert.Nil(t, agent)
	require.NotNil(t, rej)
	assert.Equal(t, http.StatusUnauthorized, rej.status)
	assert.Equal(t, 1, f.log.count())
	assert.Equal(t, dummyHash(), f.log.last())
	assert.Equal(t, 0, f.legacyWarnings())
}

func TestHandshake_NameTarget_SingleRowWarnsOnce(t *testing.T) {
	f := newHandshakeFixture(t)
	f.addAgent(hsUUIDA, "edge-one", hsKeyA)

	for i := 0; i < 2; i++ {
		agent, rej := f.auth("edge-one", hsKeyA)
		require.Nil(t, rej)
		assert.Equal(t, hsUUIDA, agent.UUID)
	}
	assert.Equal(t, 2, f.log.count(), "name path costs one compare per attempt")
	assert.Equal(t, 1, f.legacyWarnings(), "legacy match warns once per agent")
}

func TestHandshake_NameAmbiguous_FallsBackToScan(t *testing.T) {
	f := newHandshakeFixture(t)
	f.addAgent(hsUUIDA, "shared", hsKeyA)
	f.addAgent(hsUUIDB, "shared", hsKeyB)

	agent, rej := f.auth("shared", hsKeyB)
	require.Nil(t, rej)
	assert.Equal(t, hsUUIDB, agent.UUID)
	assert.GreaterOrEqual(t, f.log.count(), 2, "scan compares each candidate")
}

func TestHandshake_LegacyScan_MatchesWithoutUsableHeader(t *testing.T) {
	refs := map[string]string{
		"empty":     "",
		"unknown":   "unknown",
		"unmatched": "no-such-name",
	}
	for label, ref := range refs {
		t.Run(label, func(t *testing.T) {
			f := newHandshakeFixture(t)
			f.addAgent(hsUUIDA, "agent-a", hsKeyA)

			agent, rej := f.auth(ref, hsKeyA)
			require.Nil(t, rej)
			assert.Equal(t, hsUUIDA, agent.UUID)
			assert.Equal(t, 1, f.legacyWarnings())
		})
	}
}

func TestHandshake_LegacyScan_NoAgents_OneDummyCompare(t *testing.T) {
	f := newHandshakeFixture(t)

	agent, rej := f.auth("", hsKeyA)
	assert.Nil(t, agent)
	require.NotNil(t, rej)
	assert.Equal(t, http.StatusUnauthorized, rej.status)
	assert.Equal(t, 1, f.log.count())
	assert.Equal(t, dummyHash(), f.log.last())
}

func TestHandshake_LegacyScan_BudgetLimited(t *testing.T) {
	f := newHandshakeFixture(t)
	f.addAgent(hsUUIDA, "agent-a", hsKeyA)

	for i := 0; i < legacyScanBurst; i++ {
		_, rej := f.auth("", hsWrongKey)
		require.NotNil(t, rej)
	}

	// The scan budget is spent: the valid key is refused after one dummy
	// compare, never a scan.
	f.log.reset()
	agent, rej := f.auth("", hsKeyA)
	assert.Nil(t, agent)
	require.NotNil(t, rej)
	assert.Equal(t, http.StatusUnauthorized, rej.status)
	assert.Equal(t, 1, f.log.count())
	assert.Equal(t, dummyHash(), f.log.last())

	// The budget refills over time.
	f.clock.Advance(legacyScanEvery)
	agent, rej = f.auth("", hsKeyA)
	require.Nil(t, rej)
	assert.Equal(t, hsUUIDA, agent.UUID)
}

func TestHandshake_FailureBudget_PerClientAndAgent(t *testing.T) {
	f := newHandshakeFixture(t)
	f.addAgent(hsUUIDA, "agent-a", hsKeyA)
	f.addAgent(hsUUIDB, "agent-b", hsKeyB)

	f.exhaustFailures(hsUUIDA)

	f.log.reset()
	agent, rej := f.auth(hsUUIDA, hsKeyA)
	assert.Nil(t, agent)
	require.NotNil(t, rej)
	assert.Equal(t, http.StatusTooManyRequests, rej.status)
	assert.Positive(t, rej.retryAfter)
	assert.Equal(t, 0, f.log.count(), "throttled attempts are not verified")

	// Another agent behind the same client address is not affected.
	agent, rej = f.auth(hsUUIDB, hsKeyB)
	require.Nil(t, rej)
	assert.Equal(t, hsUUIDB, agent.UUID)
}

func TestHandshake_FailureBudget_UnknownTargetDoesNotBlockKnownAgent(t *testing.T) {
	f := newHandshakeFixture(t)
	f.addAgent(hsUUIDA, "agent-a", hsKeyA)

	f.exhaustFailures(hsUUIDMissing)
	_, rej := f.auth(hsUUIDMissing, hsWrongKey)
	require.NotNil(t, rej)
	assert.Equal(t, http.StatusTooManyRequests, rej.status)

	agent, rej := f.auth(hsUUIDA, hsKeyA)
	require.Nil(t, rej)
	assert.Equal(t, hsUUIDA, agent.UUID)
}

func TestHandshake_SuccessesNeverCharge(t *testing.T) {
	f := newHandshakeFixture(t)
	f.addAgent(hsUUIDA, "agent-a", hsKeyA)

	for i := 0; i < handshakeFailuresPerWindow+5; i++ {
		agent, rej := f.auth(hsUUIDA, hsKeyA)
		require.Nil(t, rej, "attempt %d", i)
		assert.Equal(t, hsUUIDA, agent.UUID)
	}
}

func TestHandshake_RePeekAfterTakingSlot(t *testing.T) {
	f := newHandshakeFixture(t)
	f.addAgent(hsUUIDA, "agent-a", hsKeyA)
	g := f.srv.handshake

	// Both slots are busy, so the request under test waits for one.
	f.holdAllSlots()
	reached := make(chan struct{})
	g.beforeSlot = func() { close(reached) }

	done := make(chan *handshakeRejection, 1)
	go func() {
		_, rej := f.auth(hsUUIDA, hsWrongKey)
		done <- rej
	}()
	<-reached

	// Failures recorded while the request waits must count when it gets a slot.
	budgetKey := ratelimit.ClientKey(hsClientIP) + "|" + hsUUIDA
	for i := 0; i < handshakeFailuresPerWindow; i++ {
		g.failures.Allow(budgetKey)
	}
	<-g.slots

	rej := <-done
	require.NotNil(t, rej)
	assert.Equal(t, http.StatusTooManyRequests, rej.status)
	assert.Equal(t, 0, f.log.count(), "re-peek must refuse before any verification")

	// Only the test's own hold remains; the request released its slot.
	assert.Len(t, g.slots, 1)
	<-g.slots
	assert.Empty(t, g.slots)
}

func TestHandshake_SlotWaitTimeout_503(t *testing.T) {
	f := newHandshakeFixture(t)
	f.addAgent(hsUUIDA, "agent-a", hsKeyA)
	f.srv.handshake.verifyWait = 20 * time.Millisecond
	f.holdAllSlots()

	agent, rej := f.auth(hsUUIDA, hsKeyA)
	assert.Nil(t, agent)
	require.NotNil(t, rej)
	assert.Equal(t, http.StatusServiceUnavailable, rej.status)
	assert.Equal(t, 20*time.Millisecond, rej.retryAfter)
	assert.Equal(t, 0, f.log.count())
}

func TestHandshake_SlotWaitCancelled_503(t *testing.T) {
	f := newHandshakeFixture(t)
	f.addAgent(hsUUIDA, "agent-a", hsKeyA)
	f.holdAllSlots()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	agent, rej := f.srv.authenticate(ctx, hsClientIP, hsUUIDA, hsKeyA)
	assert.Nil(t, agent)
	require.NotNil(t, rej)
	assert.Equal(t, http.StatusServiceUnavailable, rej.status)
}

func TestHandshake_ConcurrentAttemptsReleaseSlots(t *testing.T) {
	f := newHandshakeFixture(t)
	f.addAgent(hsUUIDA, "agent-a", hsKeyA)
	f.addAgent(hsUUIDB, "agent-b", hsKeyB)

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				agent, rej := f.auth(hsUUIDA, hsKeyA)
				assert.Nil(t, rej)
				if assert.NotNil(t, agent) {
					assert.Equal(t, hsUUIDA, agent.UUID)
				}
				return
			}
			_, rej := f.auth(hsUUIDA, hsWrongKey)
			if assert.NotNil(t, rej) {
				assert.Equal(t, http.StatusUnauthorized, rej.status)
			}
		}(i)
	}
	wg.Wait()
	assert.Empty(t, f.srv.handshake.slots)
}

func TestHandshake_DatabaseErrors_500(t *testing.T) {
	t.Run("uuid lookup", func(t *testing.T) {
		f := newHandshakeFixture(t)
		f.closeDB()
		agent, rej := f.auth(hsUUIDA, hsKeyA)
		assert.Nil(t, agent)
		require.NotNil(t, rej)
		assert.Equal(t, http.StatusInternalServerError, rej.status)
	})
	t.Run("legacy scan", func(t *testing.T) {
		f := newHandshakeFixture(t)
		f.closeDB()
		agent, rej := f.auth("", hsKeyA)
		assert.Nil(t, agent)
		require.NotNil(t, rej)
		assert.Equal(t, http.StatusInternalServerError, rej.status)
	})
}

func (f *handshakeFixture) closeDB() {
	f.t.Helper()
	sqlDB, err := f.db.DB()
	require.NoError(f.t, err)
	require.NoError(f.t, sqlDB.Close())
}

func TestHandleWebSocket_ThrottledResponseCarriesRetryAfter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newHandshakeFixture(t)
	f.addAgent(hsUUIDA, "agent-a", hsKeyA)
	f.exhaustFailures(hsUUIDA)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/ws", http.NoBody)
	req.RemoteAddr = hsClientIP + ":4000"
	req.Header.Set("Authorization", "Bearer "+hsKeyA)
	req.Header.Set("X-Orthrus-Name", hsUUIDA)
	c.Request = req

	f.srv.HandleWebSocket(c)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	secs, err := strconv.Atoi(w.Header().Get("Retry-After"))
	require.NoError(t, err)
	assert.Positive(t, secs)
	assert.Contains(t, w.Body.String(), ratelimit.TooManyRequestsMessage)
}

func TestHandleWebSocket_SlotTimeoutResponseIs503WithRetryAfter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newHandshakeFixture(t)
	f.addAgent(hsUUIDA, "agent-a", hsKeyA)
	f.srv.handshake.verifyWait = 10 * time.Millisecond
	f.holdAllSlots()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/ws", http.NoBody)
	req.RemoteAddr = hsClientIP + ":4000"
	req.Header.Set("Authorization", "Bearer "+hsKeyA)
	req.Header.Set("X-Orthrus-Name", hsUUIDA)
	c.Request = req

	f.srv.HandleWebSocket(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Equal(t, "1", w.Header().Get("Retry-After"))
}

// failureProfile runs n refused handshakes for agentRef and returns each
// refusal status and the number of bcrypt comparisons each attempt ran.
func (f *handshakeFixture) failureProfile(agentRef string, n int) (statuses, compares []int) {
	f.t.Helper()
	statuses = make([]int, 0, n)
	compares = make([]int, 0, n)
	for i := 0; i < n; i++ {
		f.log.reset()
		_, rej := f.auth(agentRef, hsWrongKey)
		require.NotNil(f.t, rej)
		statuses = append(statuses, rej.status)
		compares = append(compares, f.log.count())
	}
	return statuses, compares
}

func TestHandshake_NamePathFailures_DoNotSpendUUIDBudget(t *testing.T) {
	f := newHandshakeFixture(t)
	f.addAgent(hsUUIDA, "edge-one", hsKeyA)

	f.exhaustFailures("edge-one")
	_, rej := f.auth("edge-one", hsWrongKey)
	require.NotNil(t, rej)
	assert.Equal(t, http.StatusTooManyRequests, rej.status, "name budget is spent")

	budgetKey := ratelimit.ClientKey(hsClientIP) + "|" + hsUUIDA
	assert.True(t, f.srv.handshake.failures.Peek(budgetKey).Allowed, "uuid budget must stay untouched")
}

func TestHandshake_RealAgentConnectsAfterNamePathFailures(t *testing.T) {
	refs := map[string]string{
		"name hit":    "edge-one",
		"name miss":   "no-such-name",
		"legacy scan": "",
	}
	for label, ref := range refs {
		t.Run(label, func(t *testing.T) {
			f := newHandshakeFixture(t)
			f.addAgent(hsUUIDA, "edge-one", hsKeyA)

			f.exhaustFailures(ref)
			agent, rej := f.auth(hsUUIDA, hsKeyA)
			require.Nil(t, rej)
			assert.Equal(t, hsUUIDA, agent.UUID)
		})
	}
}

func TestHandshake_NameHitAndMiss_AreIndistinguishable(t *testing.T) {
	hit := newHandshakeFixture(t)
	hit.addAgent(hsUUIDA, "edge-one", hsKeyA)
	miss := newHandshakeFixture(t)
	miss.addAgent(hsUUIDA, "edge-one", hsKeyA)

	n := handshakeFailuresPerWindow + 2
	hitStatuses, hitCompares := hit.failureProfile("edge-one", n)
	missStatuses, missCompares := miss.failureProfile("no-such-name", n)

	assert.Equal(t, hitStatuses, missStatuses, "refusal statuses must match")
	assert.Equal(t, hitCompares, missCompares, "bcrypt cost per attempt must match")
	for i, status := range hitStatuses {
		if status == http.StatusUnauthorized {
			assert.Equal(t, 1, hitCompares[i], "attempt %d: one bcrypt per failure", i)
		}
	}
}

func TestHandshake_NameHitAndMiss_ShareOneBudget(t *testing.T) {
	f := newHandshakeFixture(t)
	f.addAgent(hsUUIDA, "edge-one", hsKeyA)

	f.exhaustFailures("edge-one")
	f.log.reset()
	agent, rej := f.auth("no-such-name", hsWrongKey)
	assert.Nil(t, agent)
	require.NotNil(t, rej)
	assert.Equal(t, http.StatusTooManyRequests, rej.status)
	assert.Equal(t, 0, f.log.count(), "throttled miss is not verified")
}

func TestNewHandshakeGuard_PrecomputesDummyHash(t *testing.T) {
	g, err := newHandshakeGuard(time.Now)
	require.NoError(t, err)
	require.NotNil(t, g.dummy, "dummy hash must exist before the first failure")
	assert.Equal(t, dummyHash(), g.dummy)
	cost, err := bcrypt.Cost(g.dummy)
	require.NoError(t, err)
	assert.Equal(t, BcryptCost, cost)
}

func TestHandshake_FirstUnknownUUIDFailure_ComparesPrecomputedDummy(t *testing.T) {
	f := newHandshakeFixture(t)
	f.addAgent(hsUUIDA, "agent-a", hsKeyA)

	_, rej := f.auth(hsUUIDMissing, hsWrongKey)
	require.NotNil(t, rej)
	assert.Equal(t, 1, f.log.count())
	assert.Equal(t, f.srv.handshake.dummy, f.log.last())
}

func TestDummyHash_IsCostTwelveAndStable(t *testing.T) {
	h := dummyHash()
	cost, err := bcrypt.Cost(h)
	require.NoError(t, err)
	assert.Equal(t, BcryptCost, cost)
	assert.Equal(t, h, dummyHash())
}
