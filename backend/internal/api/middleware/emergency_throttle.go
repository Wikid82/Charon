package middleware

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/ratelimit"
	"github.com/Wikid82/charon/backend/internal/util"
)

const (
	// EmergencyResetAttemptsPerWindow is the per-address attempt budget for the reset routes.
	EmergencyResetAttemptsPerWindow = 5
	// EmergencyResetWindow is the period over which the per-address budget refills.
	EmergencyResetWindow = 5 * time.Minute
	// EmergencyResetConcurrency is the number of reset checks that may run at once per listener.
	EmergencyResetConcurrency = 2
	// EmergencyResetSlotWait is how long a request waits for a check slot before it is refused.
	EmergencyResetSlotWait = 2 * time.Second

	// EmergencyResetThrottledAction is the audit action written when a reset request is refused.
	EmergencyResetThrottledAction = "emergency_reset_throttled"
)

// EmergencyThrottleAuditor records audit rows. *services.SecurityService satisfies it.
type EmergencyThrottleAuditor interface {
	LogAudit(a *models.SecurityAudit) error
}

// EmergencyResetThrottleOption customizes NewEmergencyResetThrottle.
type EmergencyResetThrottleOption func(*EmergencyResetThrottle)

// WithEmergencyResetClock injects the clock used by the per-address budget (tests).
func WithEmergencyResetClock(now func() time.Time) EmergencyResetThrottleOption {
	return func(t *EmergencyResetThrottle) { t.clock = now }
}

// WithEmergencyResetWait overrides the slot wait (tests).
func WithEmergencyResetWait(d time.Duration) EmergencyResetThrottleOption {
	return func(t *EmergencyResetThrottle) { t.wait = d }
}

// emergencyAuditWindow is the audit state for one client address: when its
// current audit window opened and how many refusals were not audited since.
type emergencyAuditWindow struct {
	start      time.Time
	suppressed uint64
}

// EmergencyResetThrottle guards one emergency reset listener. Each listener
// owns its own instance, so a flood on one cannot use up the other's budget
// or check slots.
type EmergencyResetThrottle struct {
	listener string
	auditor  EmergencyThrottleAuditor
	clock    func() time.Time
	budget   *ratelimit.KeyedLimiter
	slots    chan struct{}
	wait     time.Duration
	gates    *addressGates

	// auditMu guards auditState and lastPrune. auditState holds one entry per
	// client address that was refused within the current window.
	auditMu    sync.Mutex
	auditState map[string]*emergencyAuditWindow
	lastPrune  time.Time
}

// NewEmergencyResetThrottle builds a throttle for the named listener. listener
// labels audit rows; auditor may be nil to skip audit writes.
func NewEmergencyResetThrottle(listener string, auditor EmergencyThrottleAuditor, opts ...EmergencyResetThrottleOption) *EmergencyResetThrottle {
	t := &EmergencyResetThrottle{
		listener:   listener,
		auditor:    auditor,
		clock:      time.Now,
		slots:      make(chan struct{}, EmergencyResetConcurrency),
		wait:       EmergencyResetSlotWait,
		gates:      newAddressGates(),
		auditState: make(map[string]*emergencyAuditWindow),
	}
	for _, opt := range opts {
		opt(t)
	}

	rate, burst := ratelimit.PerWindow(EmergencyResetAttemptsPerWindow, EmergencyResetWindow)
	t.budget = mustNewKeyedLimiter(ratelimit.Config{Rate: rate, Burst: burst, Now: t.clock})
	return t
}

// mustNewKeyedLimiter builds the per-address budget from the constant settings
// above. An error here means a constant is invalid, a programming error caught
// at startup, so it panics instead of threading an unreachable error through
// every caller.
func mustNewKeyedLimiter(cfg ratelimit.Config) *ratelimit.KeyedLimiter {
	budget, err := ratelimit.NewKeyedLimiter(cfg)
	if err != nil {
		panic(fmt.Errorf("create emergency reset budget: %w", err))
	}
	return budget
}

// Middleware runs before any token comparison or database read in the reset
// handler. It peeks at the caller's per-address budget first (exhausted answers
// 429 at once), then takes a check slot, waiting up to the slot wait before
// answering 429. It then takes the address's gate, so attempts from one address
// run one at a time. Under the gate it peeks again, because an earlier attempt
// from the same address may have spent the last token while this one waited.
// The handler runs only when the budget admits the request. An attempt the
// handler refuses as unauthorized (401, wrong or missing token) then consumes
// one token before the gate is released; every other response, including other
// 4xx statuses such as 400 or 403 and any success, consumes nothing. Because the
// peek and the charge both happen under the gate, concurrent attempts from one
// address cannot overshoot the budget. A refused request is audited and never
// reaches the handler.
func (t *EmergencyResetThrottle) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := ratelimit.ClientKey(c.ClientIP())
		if !t.budgetAdmits(c, key) {
			return
		}

		if !t.acquireSlot(c.Request.Context()) {
			t.refuseBusy(c)
			return
		}
		defer t.releaseSlot()

		release, ok := t.gates.acquire(c.Request.Context(), key)
		if !ok {
			t.refuseBusy(c)
			return
		}
		defer release()

		if !t.budgetAdmits(c, key) {
			return
		}

		c.Next()

		if isFailedAttempt(c.Writer.Status()) {
			t.budget.Allow(key)
		}
	}
}

// refuseBusy answers a request that could not get a check slot or its address
// gate in time with 429, and audits the refusal.
func (t *EmergencyResetThrottle) refuseBusy(c *gin.Context) {
	t.audit(c, "concurrent attempts busy")
	ratelimit.Reject(c, ratelimit.Decision{RetryAfter: t.wait})
}

// budgetAdmits reports whether the address may make an attempt now. Over budget
// it audits the refusal and answers 429, consuming nothing either way.
func (t *EmergencyResetThrottle) budgetAdmits(c *gin.Context, key string) bool {
	decision := t.budget.Peek(key)
	if decision.Allowed {
		return true
	}
	t.audit(c, "per-address attempt budget exhausted")
	ratelimit.Reject(c, decision)
	return false
}

// isFailedAttempt reports whether a handler status counts against the budget.
// Only 401 does: it means the token was wrong or missing. Other error statuses
// (for example 403 from a management network check or 400 for a malformed body)
// say nothing about the token, so they do not spend the budget.
func isFailedAttempt(status int) bool {
	return status == http.StatusUnauthorized
}

// acquireSlot takes a check slot, waiting up to t.wait. It returns false when
// the wait expires or the request context ends first.
func (t *EmergencyResetThrottle) acquireSlot(ctx context.Context) bool {
	select {
	case t.slots <- struct{}{}:
		return true
	default:
	}

	timer := time.NewTimer(t.wait)
	defer timer.Stop()
	select {
	case t.slots <- struct{}{}:
		return true
	case <-timer.C:
		return false
	case <-ctx.Done():
		return false
	}
}

func (t *EmergencyResetThrottle) releaseSlot() {
	<-t.slots
}

// addressGates serializes the budget check and the handler for one client
// address, so an attempt's charge is visible to the next attempt from that
// address. Gates exist only while a request holds or waits for one, so the map
// is bounded by in-flight requests rather than by addresses seen.
type addressGates struct {
	mu    sync.Mutex
	gates map[string]*addressGate
}

// addressGate is a one-holder lock. users counts holders and waiters and is
// guarded by addressGates.mu; busy holds a token while a request owns the gate.
type addressGate struct {
	busy  chan struct{}
	users int
}

func newAddressGates() *addressGates {
	return &addressGates{gates: make(map[string]*addressGate)}
}

// acquire takes the gate for key, waiting until the holder releases it or ctx
// ends. It returns the release func, or false when ctx ended first. The wait
// lasts at most as long as the holder's handler call, which holds a check slot.
func (g *addressGates) acquire(ctx context.Context, key string) (release func(), ok bool) {
	g.mu.Lock()
	gate, found := g.gates[key]
	if !found {
		gate = &addressGate{busy: make(chan struct{}, 1)}
		g.gates[key] = gate
	}
	gate.users++
	g.mu.Unlock()

	select {
	case gate.busy <- struct{}{}:
		return func() {
			<-gate.busy
			g.drop(key, gate)
		}, true
	case <-ctx.Done():
		g.drop(key, gate)
		return nil, false
	}
}

// drop forgets one user of gate, deleting the gate once nobody holds or waits.
func (g *addressGates) drop(key string, gate *addressGate) {
	g.mu.Lock()
	defer g.mu.Unlock()
	gate.users--
	if gate.users == 0 {
		delete(g.gates, key)
	}
}

// audit writes one emergency_reset_throttled row for the first refusal of a
// client address in each window. Later refusals in the same window are counted
// and logged at Debug; the count rides on the next audited row for that address.
// A failed write is logged and never changes the response.
func (t *EmergencyResetThrottle) audit(c *gin.Context, reason string) {
	actor := util.CanonicalizeIPForSecurity(c.ClientIP())
	record, suppressed := t.takeAuditSlot(ratelimit.ClientKey(c.ClientIP()))
	fields := map[string]any{
		"ip":       util.SanitizeForLog(actor),
		"listener": util.SanitizeForLog(t.listener),
	}
	if !record {
		logger.Log().WithFields(fields).Debug("Emergency reset request refused by throttle (audit suppressed)")
		return
	}
	logger.Log().WithFields(fields).Warn("Emergency reset request refused by throttle")

	if t.auditor == nil {
		return
	}
	details := fmt.Sprintf("Emergency reset throttled: %s | listener=%s | result=failure", reason, t.listener)
	if suppressed > 0 {
		details += fmt.Sprintf(" | suppressed_refusals=%d", suppressed)
	}
	row := &models.SecurityAudit{
		Actor:   actor,
		Action:  EmergencyResetThrottledAction,
		Details: details,
	}
	if err := t.auditor.LogAudit(row); err != nil {
		logger.Log().WithError(fmt.Errorf("write emergency throttle audit: %w", err)).Error("Failed to log emergency throttle audit event")
	}
}

// takeAuditSlot reports whether a refusal for key should write an audit row. It
// does so for the first refusal of a window, and returns the refusals suppressed
// since the previous audited row for key.
func (t *EmergencyResetThrottle) takeAuditSlot(key string) (record bool, suppressed uint64) {
	t.auditMu.Lock()
	defer t.auditMu.Unlock()

	now := t.clock()
	// Pruning runs after this address's own window is handled, so an expired
	// window still carries its suppressed count onto the row written now.
	defer t.pruneAuditStateLocked(now)

	st, ok := t.auditState[key]
	if !ok {
		t.auditState[key] = &emergencyAuditWindow{start: now}
		return true, 0
	}
	if now.Sub(st.start) >= EmergencyResetWindow {
		suppressed = st.suppressed
		st.start = now
		st.suppressed = 0
		return true, suppressed
	}
	st.suppressed++
	return false, 0
}

// pruneAuditStateLocked drops audit windows that have expired, at most once per
// window, so the map is bounded by the addresses refused in one window. Counts
// still pending on a dropped window cannot ride on a row any more, so they are
// logged at Info. The caller must hold auditMu.
func (t *EmergencyResetThrottle) pruneAuditStateLocked(now time.Time) {
	if now.Sub(t.lastPrune) < EmergencyResetWindow {
		return
	}
	t.lastPrune = now

	var dropped uint64
	for key, st := range t.auditState {
		if now.Sub(st.start) >= EmergencyResetWindow {
			dropped += st.suppressed
			delete(t.auditState, key)
		}
	}
	if dropped > 0 {
		logger.Log().WithFields(map[string]any{
			"listener":   util.SanitizeForLog(t.listener),
			"suppressed": dropped,
		}).Info("Emergency reset refusals expired without an audit row")
	}
}
