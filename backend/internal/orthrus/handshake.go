package orthrus

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/time/rate"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/ratelimit"
	"github.com/Wikid82/charon/backend/internal/util"
)

// BcryptCost is the work factor used for every agent key hash.
const BcryptCost = 12

const (
	// handshakeFailuresPerWindow failed handshakes are allowed per client
	// address and budget key within handshakeFailureWindow.
	handshakeFailuresPerWindow = 10
	handshakeFailureWindow     = time.Minute

	// handshakeVerifySlots bounds concurrent bcrypt verifications.
	handshakeVerifySlots = 2
	// handshakeVerifyWait bounds how long a handshake waits for a slot.
	handshakeVerifyWait = 5 * time.Second

	// legacyScanEvery and legacyScanBurst cap the uuid-less scan globally.
	legacyScanEvery = 30 * time.Second
	legacyScanBurst = 2

	// unknownAgentRef is what agents send when no agent ID is configured.
	unknownAgentRef = "unknown"
	// anyAgentBudget is the budget-key suffix for handshakes that do not claim
	// an agent by UUID: name, legacy scan and unknown-UUID attempts.
	anyAgentBudget = "*"
	// legacyScanKey is the single key of the global legacy-scan budget.
	legacyScanKey = "global"
)

// BcryptInput returns the bytes fed to bcrypt for a key. Keys of 72 bytes or
// more are truncated so the stored hash and the verification use the same
// input.
func BcryptInput(token string) []byte {
	b := []byte(token)
	if len(b) >= 72 {
		b = b[:71]
	}
	return b
}

// dummyHash returns a cost-12 bcrypt hash of a random secret that nobody
// holds. Failure paths compare against it so that they cost the same as a real
// verification. Generated once per process; a nil result (only possible if the
// system random source fails) makes the compare fail fast, and the handshake
// still fails closed.
var dummyHash = sync.OnceValue(func() []byte {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword(secret, BcryptCost)
	if err != nil {
		return nil
	}
	return hash
})

// handshakeGuard admits agent WebSocket handshakes. Failures are throttled
// per (client address, target agent) before any key is verified, and key
// verifications are bounded by a small slot pool.
type handshakeGuard struct {
	// failures holds one budget per client address and target. Only failed
	// handshakes are charged.
	failures *ratelimit.KeyedLimiter
	// legacyScan is the global budget for the uuid-less scan over all agents.
	legacyScan *ratelimit.KeyedLimiter
	// slots holds one token per verification in progress.
	slots      chan struct{}
	verifyWait time.Duration
	// dummy is compared on failure paths that have no stored hash to check.
	// It is computed when the guard is built so the first failure costs the
	// same as later ones.
	dummy []byte
	// compare verifies a key against a stored hash. Tests replace it to count
	// comparisons.
	compare func(hashedPassword, password []byte) error
	// beforeSlot, when set, runs once an attempt has passed the first budget
	// check and before it waits for a slot. Tests use it to order events.
	beforeSlot func()
	// warnedLegacy records agents already warned about a missing agent ID.
	warnedLegacy sync.Map
}

// newHandshakeGuard builds a guard with the production budgets. now is the
// clock used by the budgets.
func newHandshakeGuard(now func() time.Time) (*handshakeGuard, error) {
	failureRate, failureBurst := ratelimit.PerWindow(handshakeFailuresPerWindow, handshakeFailureWindow)
	failures, err := ratelimit.NewKeyedLimiter(ratelimit.Config{Rate: failureRate, Burst: failureBurst, Now: now})
	if err != nil {
		return nil, fmt.Errorf("orthrus: create handshake failure budget: %w", err)
	}
	legacy, err := ratelimit.NewKeyedLimiter(ratelimit.Config{
		Rate:    rate.Every(legacyScanEvery),
		Burst:   legacyScanBurst,
		MaxKeys: 1,
		Now:     now,
	})
	if err != nil {
		return nil, fmt.Errorf("orthrus: create legacy scan budget: %w", err)
	}
	return &handshakeGuard{
		failures:   failures,
		legacyScan: legacy,
		slots:      make(chan struct{}, handshakeVerifySlots),
		verifyWait: handshakeVerifyWait,
		dummy:      dummyHash(),
		compare:    bcrypt.CompareHashAndPassword,
	}, nil
}

// handshakeRejection carries the HTTP status for a refused handshake.
type handshakeRejection struct {
	status     int
	retryAfter time.Duration // set for 429 and 503
}

func tooManyHandshakes(retryAfter time.Duration) *handshakeRejection {
	return &handshakeRejection{status: http.StatusTooManyRequests, retryAfter: retryAfter}
}

// handshakeTarget is the agent a handshake claims to be, resolved from the
// X-Orthrus-Name header without any key verification.
type handshakeTarget struct {
	// agent is set when the header names exactly one known agent.
	agent *models.OrthrusAgent
	// scan is set when the key must be checked against every agent.
	scan bool
	// legacy is set for name and scan resolutions, which are supported only
	// for agents that do not send their ID.
	legacy bool
}

// budgetRef is the per-agent part of the failure budget key. Only a UUID-form
// claim that resolves to a stored agent gets its own budget. Name and scan
// resolutions share anyAgentBudget, so name-path failures never spend an
// agent's UUID budget and a name hit is budgeted the same as a miss.
func (t handshakeTarget) budgetRef() string {
	if t.agent != nil && !t.legacy {
		return t.agent.UUID
	}
	return anyAgentBudget
}

// authenticate decides whether a handshake may proceed. On success it returns
// the agent; on refusal it returns the status to send. The failure budget is
// checked before any key is verified, and every failure path costs exactly one
// bcrypt comparison.
func (s *OrthrusServer) authenticate(ctx context.Context, clientIP, agentRef, token string) (*models.OrthrusAgent, *handshakeRejection) {
	g := s.handshake

	target, err := s.resolveHandshakeTarget(strings.TrimSpace(agentRef))
	if err != nil {
		logger.Log().WithError(err).Error("orthrus: resolve agent for handshake failed")
		return nil, &handshakeRejection{status: http.StatusInternalServerError}
	}

	budgetKey := ratelimit.ClientKey(clientIP) + "|" + target.budgetRef()
	if d := g.failures.Peek(budgetKey); !d.Allowed {
		return nil, tooManyHandshakes(d.RetryAfter)
	}

	if g.beforeSlot != nil {
		g.beforeSlot()
	}
	if !g.acquireSlot(ctx) {
		return nil, &handshakeRejection{status: http.StatusServiceUnavailable, retryAfter: g.verifyWait}
	}
	defer g.releaseSlot()

	// Failures recorded while this attempt waited for a slot count here.
	if d := g.failures.Peek(budgetKey); !d.Allowed {
		return nil, tooManyHandshakes(d.RetryAfter)
	}

	agent, err := s.verifyHandshake(target, BcryptInput(token))
	if err != nil {
		logger.Log().WithError(err).Error("orthrus: verify agent key failed")
		return nil, &handshakeRejection{status: http.StatusInternalServerError}
	}
	if agent == nil {
		g.failures.Allow(budgetKey)
		return nil, &handshakeRejection{status: http.StatusUnauthorized}
	}
	if target.legacy {
		s.warnLegacyMatch(agent)
	}
	return agent, nil
}

// resolveHandshakeTarget finds the claimed agent with database lookups only.
// A UUID-shaped header never falls back to a name lookup or a scan.
func (s *OrthrusServer) resolveHandshakeTarget(ref string) (handshakeTarget, error) {
	if ref == "" || ref == unknownAgentRef {
		return handshakeTarget{scan: true, legacy: true}, nil
	}

	if _, err := uuid.Parse(ref); err == nil {
		var agent models.OrthrusAgent
		err := s.db.Where("uuid = ?", ref).First(&agent).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return handshakeTarget{}, nil
		}
		if err != nil {
			return handshakeTarget{}, fmt.Errorf("orthrus: look up agent by uuid: %w", err)
		}
		return handshakeTarget{agent: &agent}, nil
	}

	var byName []models.OrthrusAgent
	if err := s.db.Where("name = ?", ref).Limit(2).Find(&byName).Error; err != nil {
		return handshakeTarget{}, fmt.Errorf("orthrus: look up agent by name: %w", err)
	}
	if len(byName) == 1 {
		return handshakeTarget{agent: &byName[0], legacy: true}, nil
	}
	return handshakeTarget{scan: true, legacy: true}, nil
}

// verifyHandshake returns the matched agent, or nil when the key does not
// match. Each branch performs exactly one bcrypt comparison per candidate and
// one for the dummy hash when there is no candidate.
func (s *OrthrusServer) verifyHandshake(target handshakeTarget, password []byte) (*models.OrthrusAgent, error) {
	g := s.handshake
	switch {
	case target.agent != nil:
		if g.compare([]byte(target.agent.AuthKeyHash), password) != nil {
			return nil, nil
		}
		return target.agent, nil
	case target.scan && g.legacyScan.Allow(legacyScanKey).Allowed:
		return s.scanAgentsByToken(password)
	default:
		_ = g.compare(g.dummy, password)
		return nil, nil
	}
}

// scanAgentsByToken compares the key against every agent. It performs one
// dummy comparison when no agent exists, so an empty table costs the same as
// a miss.
func (s *OrthrusServer) scanAgentsByToken(password []byte) (*models.OrthrusAgent, error) {
	var agents []models.OrthrusAgent
	if err := s.db.Find(&agents).Error; err != nil {
		return nil, fmt.Errorf("orthrus: query agents: %w", err)
	}
	if len(agents) == 0 {
		_ = s.handshake.compare(s.handshake.dummy, password)
		return nil, nil
	}
	for i := range agents {
		if s.handshake.compare([]byte(agents[i].AuthKeyHash), password) == nil {
			return &agents[i], nil
		}
	}
	return nil, nil
}

// warnLegacyMatch logs once per agent that it connected without its agent ID.
func (s *OrthrusServer) warnLegacyMatch(agent *models.OrthrusAgent) {
	if _, seen := s.handshake.warnedLegacy.LoadOrStore(agent.UUID, struct{}{}); seen {
		return
	}
	logger.Log().WithFields(logrus.Fields{
		"uuid": util.SanitizeForLog(agent.UUID),
		"name": util.SanitizeForLog(agent.Name),
	}).Warn("orthrus: agent connected without its agent ID; set ORTHRUS_AGENT_ID to the value shown in Charon")
}

// acquireSlot waits for a verification slot, up to verifyWait or until ctx is
// done. It reports whether a slot was taken.
func (g *handshakeGuard) acquireSlot(ctx context.Context) bool {
	timer := time.NewTimer(g.verifyWait)
	defer timer.Stop()
	select {
	case g.slots <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	case <-timer.C:
		return false
	}
}

func (g *handshakeGuard) releaseSlot() {
	<-g.slots
}

// abortHandshake writes the refusal for a rejected handshake.
func abortHandshake(c *gin.Context, rej *handshakeRejection) {
	switch rej.status {
	case http.StatusTooManyRequests:
		ratelimit.Reject(c, ratelimit.Decision{RetryAfter: rej.retryAfter})
	case http.StatusServiceUnavailable:
		c.Header("Retry-After", strconv.Itoa(ratelimit.RetryAfterSeconds(rej.retryAfter)))
		c.AbortWithStatus(http.StatusServiceUnavailable)
	default:
		c.AbortWithStatus(rej.status)
	}
}
