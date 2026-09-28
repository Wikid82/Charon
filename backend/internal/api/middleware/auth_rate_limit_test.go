package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/metrics"
	"github.com/Wikid82/charon/backend/internal/ratelimit"
)

type throttleClock struct {
	mu  sync.Mutex
	now time.Time
}

func newThrottleClock() *throttleClock {
	return &throttleClock{now: time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)}
}

func (tc *throttleClock) Now() time.Time {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	return tc.now
}

func (tc *throttleClock) Advance(d time.Duration) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.now = tc.now.Add(d)
}

type throttleHarness struct {
	t       *testing.T
	limiter *AuthRateLimiter
	router  *gin.Engine
	clock   *throttleClock
	hook    *logtest.Hook
	bypass  bool
}

// newThrottleHarness builds a real gin.Engine whose trusted proxies match the
// limiter's, with every classified /auth route, an unclassified /auth route,
// and a password-guarded route outside the group.
func newThrottleHarness(t *testing.T, cfg config.AuthRateLimitConfig, trusted []string) *throttleHarness {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := &throttleHarness{t: t, clock: newThrottleClock()}
	lim, err := NewAuthRateLimiter(cfg, trusted, WithAuthRateLimitClock(h.clock.Now))
	require.NoError(t, err)
	h.limiter = lim

	base, hook := logtest.NewNullLogger()
	base.SetLevel(logrus.DebugLevel)
	h.hook = hook

	r := gin.New()
	require.NoError(t, r.SetTrustedProxies(trusted))
	r.Use(func(c *gin.Context) {
		c.Set("logger", logrus.NewEntry(base))
		if h.bypass {
			c.Set(EmergencyBypassContextKey, true)
		}
		c.Next()
	})
	ok := func(c *gin.Context) { c.Status(http.StatusOK) }
	auth := r.Group("/api/v1/auth")
	auth.Use(lim.Middleware())
	for route := range AuthRouteClasses() {
		method, path, _ := strings.Cut(route, " ")
		auth.Handle(method, strings.TrimPrefix(path, "/api/v1/auth"), ok)
	}
	auth.POST("/unclassified", ok)
	r.POST("/api/v1/user/profile", func(c *gin.Context) {
		if !lim.AllowPasswordAttempt(c) {
			return
		}
		c.Status(http.StatusOK)
	})
	h.router = r
	return h
}

func (h *throttleHarness) do(method, path, remote string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(`{"email":"victim@example.com","password":"hunter2-secret"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = remote
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, req)
	return w
}

func (h *throttleHarness) login(remote string, headers map[string]string) int {
	return h.do(http.MethodPost, "/api/v1/auth/login", remote, headers).Code
}

func (h *throttleHarness) entries(level logrus.Level) []*logrus.Entry {
	var out []*logrus.Entry
	for _, e := range h.hook.AllEntries() {
		if e.Level == level {
			out = append(out, e)
		}
	}
	return out
}

func smallBudget(login, session int) config.AuthRateLimitConfig {
	return config.AuthRateLimitConfig{LoginRequests: login, LoginWindowSec: 600, SessionRequests: session, SessionWindowSec: 60}
}

const peerA = "203.0.113.10:5555"

func TestAuthRouteClass(t *testing.T) {
	assert.Equal(t, AuthRateLimitLogin, AuthRouteClass(http.MethodPost, "/api/v1/auth/login"))
	assert.Equal(t, AuthRateLimitLogin, AuthRouteClass(http.MethodPost, "/api/v1/auth/change-password"))
	assert.Equal(t, AuthRateLimitSession, AuthRouteClass(http.MethodPost, "/api/v1/auth/refresh"))
	assert.Equal(t, AuthRateLimitSession, AuthRouteClass(http.MethodGet, "/api/v1/auth/status"))
	for _, r := range [][2]string{
		{http.MethodGet, "/api/v1/auth/verify"},
		{http.MethodGet, "/api/v1/auth/me"},
		{http.MethodPost, "/api/v1/auth/logout"},
		{http.MethodGet, "/api/v1/auth/accessible-hosts"},
		{http.MethodGet, "/api/v1/auth/check-host/:hostId"},
	} {
		assert.Equal(t, AuthRateLimitExempt, AuthRouteClass(r[0], r[1]), r[1])
	}
	assert.Equal(t, AuthRateLimitSession, AuthRouteClass(http.MethodPost, "/api/v1/auth/new-thing"), "unknown routes fail safe to session")
	assert.Equal(t, AuthRateLimitSession, AuthRouteClass(http.MethodGet, ""), "empty route template fails safe to session")
	assert.Equal(t, AuthRateLimitSession, AuthRouteClass(http.MethodPost, "/api/v1/auth/me"), "method is part of the classification")

	table := AuthRouteClasses()
	table["POST /api/v1/auth/login"] = AuthRateLimitExempt
	assert.Equal(t, AuthRateLimitLogin, AuthRouteClass(http.MethodPost, "/api/v1/auth/login"), "AuthRouteClasses returns a copy")
}

func TestNewAuthRateLimiter_NormalizesHandBuiltConfig(t *testing.T) {
	lim, err := NewAuthRateLimiter(config.AuthRateLimitConfig{LoginRequests: -1}, nil)
	require.NoError(t, err)
	st := lim.Status()
	assert.True(t, st.Enabled)
	assert.Equal(t, AuthRateLimitBudget{Requests: 10, WindowSeconds: 600}, st.Login)
	assert.Equal(t, AuthRateLimitBudget{Requests: 60, WindowSeconds: 60}, st.Session)
	assert.Equal(t, 0, st.TrustedProxyCount)
}

func TestAuthRateLimiter_LoginAndChangePasswordShareBucket(t *testing.T) {
	h := newThrottleHarness(t, smallBudget(4, 60), nil)
	assert.Equal(t, http.StatusOK, h.login(peerA, nil))
	assert.Equal(t, http.StatusOK, h.do(http.MethodPost, "/api/v1/auth/change-password", peerA, nil).Code)
	assert.Equal(t, http.StatusOK, h.login(peerA, nil))
	assert.Equal(t, http.StatusOK, h.do(http.MethodPost, "/api/v1/auth/change-password", peerA, nil).Code)

	w := h.do(http.MethodPost, "/api/v1/auth/change-password", peerA, nil)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, "150", w.Header().Get("Retry-After"), "4 per 600s refills one token every 150s")
	assert.Equal(t, http.StatusTooManyRequests, h.login(peerA, nil))

	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]string{"error": ratelimit.TooManyRequestsMessage}, body)
	assert.NotContains(t, w.Body.String(), "victim")

	h.clock.Advance(150 * time.Second)
	assert.Equal(t, http.StatusOK, h.login(peerA, nil))
}

func TestAuthRateLimiter_SessionClassSeparateBudget(t *testing.T) {
	h := newThrottleHarness(t, smallBudget(1, 3), nil)
	for i := 0; i < 3; i++ {
		assert.Equal(t, http.StatusOK, h.do(http.MethodPost, "/api/v1/auth/refresh", peerA, nil).Code)
	}
	w := h.do(http.MethodGet, "/api/v1/auth/status", peerA, nil)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, "20", w.Header().Get("Retry-After"))

	assert.Equal(t, http.StatusOK, h.login(peerA, nil), "session exhaustion does not touch the login budget")
	assert.Equal(t, http.StatusTooManyRequests, h.login(peerA, nil))
	h.clock.Advance(20 * time.Second)
	assert.Equal(t, http.StatusOK, h.do(http.MethodGet, "/api/v1/auth/status", peerA, nil).Code)
}

func TestAuthRateLimiter_ExemptRoutesNeverThrottled(t *testing.T) {
	h := newThrottleHarness(t, smallBudget(1, 1), nil)
	require.Equal(t, http.StatusOK, h.login(peerA, nil))
	require.Equal(t, http.StatusTooManyRequests, h.login(peerA, nil))
	require.Equal(t, http.StatusOK, h.do(http.MethodPost, "/api/v1/auth/refresh", peerA, nil).Code)
	require.Equal(t, http.StatusTooManyRequests, h.do(http.MethodPost, "/api/v1/auth/refresh", peerA, nil).Code)

	for _, r := range [][2]string{
		{http.MethodGet, "/api/v1/auth/verify"},
		{http.MethodGet, "/api/v1/auth/me"},
		{http.MethodPost, "/api/v1/auth/logout"},
		{http.MethodGet, "/api/v1/auth/accessible-hosts"},
		{http.MethodGet, "/api/v1/auth/check-host/42"},
	} {
		for i := 0; i < 20; i++ {
			assert.Equal(t, http.StatusOK, h.do(r[0], r[1], peerA, nil).Code, r[1])
		}
	}
	assert.Equal(t, http.StatusOK, h.login("198.51.100.20:1", nil), "other clients are unaffected")
}

func TestAuthRateLimiter_UnknownRouteDefaultsToSession(t *testing.T) {
	h := newThrottleHarness(t, smallBudget(10, 2), nil)
	assert.Equal(t, http.StatusOK, h.do(http.MethodPost, "/api/v1/auth/unclassified", peerA, nil).Code)
	assert.Equal(t, http.StatusOK, h.do(http.MethodPost, "/api/v1/auth/refresh", peerA, nil).Code)
	assert.Equal(t, http.StatusTooManyRequests, h.do(http.MethodPost, "/api/v1/auth/unclassified", peerA, nil).Code)
}

func TestAuthRateLimiter_EmergencyBypassSkips(t *testing.T) {
	h := newThrottleHarness(t, smallBudget(1, 1), nil)
	h.bypass = true
	for i := 0; i < 5; i++ {
		assert.Equal(t, http.StatusOK, h.login(peerA, nil))
		assert.Equal(t, http.StatusOK, h.do(http.MethodPost, "/api/v1/user/profile", peerA, nil).Code)
	}
	h.bypass = false
	assert.Equal(t, http.StatusOK, h.login(peerA, nil), "bypassed requests consumed nothing")
	assert.Equal(t, http.StatusTooManyRequests, h.login(peerA, nil))
}

func TestAuthRateLimiter_DisabledPassesThrough(t *testing.T) {
	cfg := smallBudget(1, 1)
	cfg.Disabled = true
	h := newThrottleHarness(t, cfg, nil)
	for i := 0; i < 5; i++ {
		assert.Equal(t, http.StatusOK, h.login(peerA, nil))
		assert.Equal(t, http.StatusOK, h.do(http.MethodPost, "/api/v1/auth/refresh", peerA, nil).Code)
		assert.Equal(t, http.StatusOK, h.do(http.MethodPost, "/api/v1/user/profile", peerA, nil).Code)
	}
	assert.False(t, h.limiter.Status().Enabled)

	// The detector still records while the throttle is off.
	h.do(http.MethodGet, "/api/v1/auth/me", "10.0.0.9:1", map[string]string{"X-Forwarded-For": "1.2.3.4"})
	assert.Equal(t, uint64(1), h.limiter.Status().UntrustedForwardedHeaders.Local.Count)
}

func TestAuthRateLimiter_AllowPasswordAttemptSharesLoginBucket(t *testing.T) {
	h := newThrottleHarness(t, smallBudget(2, 60), nil)
	assert.Equal(t, http.StatusOK, h.do(http.MethodPost, "/api/v1/user/profile", peerA, nil).Code)
	assert.Equal(t, http.StatusOK, h.login(peerA, nil))
	w := h.do(http.MethodPost, "/api/v1/user/profile", peerA, nil)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, "300", w.Header().Get("Retry-After"))
	assert.Equal(t, http.StatusTooManyRequests, h.login(peerA, nil))
}

func TestAuthRateLimiter_EpisodeLoggingOncePerEpisode(t *testing.T) {
	h := newThrottleHarness(t, smallBudget(1, 60), nil)
	h.login(peerA, nil)
	for i := 0; i < 4; i++ {
		h.login(peerA, nil)
	}
	warns := h.entries(logrus.WarnLevel)
	require.Len(t, warns, 1)
	assert.Equal(t, "login", warns[0].Data["class"])
	assert.Equal(t, "203.0.113.10", warns[0].Data["client"])
	assert.Equal(t, "/api/v1/auth/login", warns[0].Data["route"])
	assert.Equal(t, 600, warns[0].Data["retry_after_seconds"])
	assert.Equal(t, uint64(0), warns[0].Data["suppressed"])
	assert.Len(t, h.entries(logrus.DebugLevel), 3)

	// A new episode after the client is allowed again logs one more WARN.
	h.clock.Advance(600 * time.Second)
	h.login(peerA, nil)
	h.login(peerA, nil)
	assert.Len(t, h.entries(logrus.WarnLevel), 2)
}

func TestAuthRateLimiter_GlobalWarnCapWithSuppressedCount(t *testing.T) {
	h := newThrottleHarness(t, smallBudget(1, 1), nil)
	for i := 0; i < 15; i++ {
		addr := fmt.Sprintf("198.51.100.%d:1", i)
		h.login(addr, nil)
		h.login(addr, nil)
	}
	// Session-class episodes share the same global cap.
	h.do(http.MethodPost, "/api/v1/auth/refresh", "198.51.100.200:1", nil)
	h.do(http.MethodPost, "/api/v1/auth/refresh", "198.51.100.200:1", nil)
	assert.Len(t, h.entries(logrus.WarnLevel), authThrottleWarnBurst)

	h.hook.Reset()
	h.clock.Advance(time.Minute)
	h.login("198.51.100.250:1", nil)
	h.login("198.51.100.250:1", nil)
	warns := h.entries(logrus.WarnLevel)
	require.Len(t, warns, 1)
	assert.Equal(t, uint64(16-authThrottleWarnBurst), warns[0].Data["suppressed"])
}

func TestAuthRateLimiter_LogsContainNoCredentials(t *testing.T) {
	h := newThrottleHarness(t, smallBudget(1, 1), nil)
	headers := map[string]string{
		"Authorization":   "Bearer secret-token-value",
		"Cookie":          "auth_token=secret-cookie-value",
		"X-Forwarded-For": "evil\nnewline",
	}
	for i := 0; i < 3; i++ {
		h.login(peerA, headers)
		h.do(http.MethodPost, "/api/v1/user/profile", peerA, headers)
	}
	require.NotEmpty(t, h.hook.AllEntries())
	for _, e := range h.hook.AllEntries() {
		line, err := e.String()
		require.NoError(t, err)
		for _, secret := range []string{"victim@example.com", "hunter2-secret", "secret-token-value", "secret-cookie-value", "evil"} {
			assert.NotContains(t, line, secret)
		}
	}
}

func TestAuthRateLimiter_MetricDeltaPerClass(t *testing.T) {
	loginBefore := testutil.ToFloat64(metrics.AuthRateLimitedCounter("login"))
	sessionBefore := testutil.ToFloat64(metrics.AuthRateLimitedCounter("session"))

	h := newThrottleHarness(t, smallBudget(1, 1), nil)
	h.login(peerA, nil)
	h.login(peerA, nil)
	h.login(peerA, nil)
	h.do(http.MethodPost, "/api/v1/user/profile", peerA, nil)
	h.do(http.MethodGet, "/api/v1/auth/status", peerA, nil)
	h.do(http.MethodGet, "/api/v1/auth/status", peerA, nil)

	assert.InDelta(t, loginBefore+3, testutil.ToFloat64(metrics.AuthRateLimitedCounter("login")), 0)
	assert.InDelta(t, sessionBefore+1, testutil.ToFloat64(metrics.AuthRateLimitedCounter("session")), 0)
}

// --- Trusted proxy keying -------------------------------------------------------

func TestAuthRateLimiter_UntrustedPeerIgnoresForwardedHeaders(t *testing.T) {
	h := newThrottleHarness(t, smallBudget(1, 60), nil)
	assert.Equal(t, http.StatusOK, h.login(peerA, map[string]string{"X-Forwarded-For": "198.18.0.1"}))
	assert.Equal(t, http.StatusTooManyRequests, h.login(peerA, map[string]string{"X-Forwarded-For": "198.18.0.2"}),
		"rotating forwarded headers from an untrusted peer does not grant a new bucket")
	assert.Equal(t, http.StatusTooManyRequests, h.login(peerA, map[string]string{"X-Real-IP": "198.18.0.3"}))
}

func TestAuthRateLimiter_TrustedPeerKeysOnRealClient(t *testing.T) {
	h := newThrottleHarness(t, smallBudget(1, 60), []string{"172.18.0.5"})
	proxy := "172.18.0.5:40000"
	assert.Equal(t, http.StatusOK, h.login(proxy, map[string]string{"X-Forwarded-For": "198.18.0.1"}))
	assert.Equal(t, http.StatusTooManyRequests, h.login(proxy, map[string]string{"X-Forwarded-For": "198.18.0.1"}))
	assert.Equal(t, http.StatusOK, h.login(proxy, map[string]string{"X-Forwarded-For": "198.18.0.2"}), "distinct real clients get distinct buckets")
	assert.Equal(t, http.StatusOK, h.login(proxy, nil), "the proxy's own requests use the proxy address")
	assert.Zero(t, h.limiter.Status().UntrustedForwardedHeaders.Local.Count, "trusted peers are not recorded")
}

func TestAuthRateLimiter_TrustedPeerUsesRightmostUntrustedHop(t *testing.T) {
	h := newThrottleHarness(t, smallBudget(1, 60), []string{"172.18.0.0/16"})
	proxy := "172.18.0.5:40000"
	// Client-supplied left-most entries are ignored; the right-most untrusted hop is the key.
	assert.Equal(t, http.StatusOK, h.login(proxy, map[string]string{"X-Forwarded-For": "1.1.1.1, 198.18.0.9, 172.18.0.7"}))
	assert.Equal(t, http.StatusTooManyRequests, h.login(proxy, map[string]string{"X-Forwarded-For": "2.2.2.2, 198.18.0.9"}))
}

func TestAuthRateLimiter_IPv6SlashSixtyFourAggregation(t *testing.T) {
	h := newThrottleHarness(t, smallBudget(1, 60), nil)
	assert.Equal(t, http.StatusOK, h.login("[2001:db8:1:2::1]:1", nil))
	assert.Equal(t, http.StatusTooManyRequests, h.login("[2001:db8:1:2:ffff::9]:1", nil))
	assert.Equal(t, http.StatusOK, h.login("[2001:db8:1:3::1]:1", nil))
}

// --- Forwarded-header detector ---------------------------------------------------

func TestAuthRateLimiter_UntrustedPeerCountsAndRecordsScope(t *testing.T) {
	h := newThrottleHarness(t, smallBudget(10, 60), nil)
	assert.Zero(t, h.limiter.Status().UntrustedForwardedHeaders.Local.Count)
	assert.Nil(t, h.limiter.Status().UntrustedForwardedHeaders.Local.LastSeen)

	h.do(http.MethodGet, "/api/v1/auth/me", "172.18.0.5:1", map[string]string{"X-Forwarded-For": "198.18.0.1"})
	h.clock.Advance(time.Second)
	h.do(http.MethodPost, "/api/v1/auth/login", "127.0.0.1:1", map[string]string{"X-Real-IP": "198.18.0.1"})
	h.do(http.MethodPost, "/api/v1/auth/login", "172.18.0.6:1", nil) // no header: not recorded

	st := h.limiter.Status().UntrustedForwardedHeaders
	assert.Equal(t, uint64(2), st.Local.Count)
	require.NotNil(t, st.Local.LastSeen)
	assert.Equal(t, h.clock.Now(), *st.Local.LastSeen)
	assert.Equal(t, "127.0.0.1", st.Local.LastPeer)
	assert.Equal(t, "loopback", st.Local.LastPeerScope)
	assert.Equal(t, ForwardedHeaderObservation{}, st.Public)

	data, err := json.Marshal(h.limiter.Status())
	require.NoError(t, err)
	assert.Contains(t, string(data), `"public":{"count":0,"last_seen":null,"last_peer":"","last_peer_scope":""}`)
}

func TestAuthRateLimiter_TrustedPeerMalformedHeaderNotCounted(t *testing.T) {
	h := newThrottleHarness(t, smallBudget(10, 60), []string{"172.18.0.5/32"})
	h.do(http.MethodGet, "/api/v1/auth/me", "172.18.0.5:1", map[string]string{"X-Forwarded-For": "not-an-ip"})
	h.do(http.MethodGet, "/api/v1/auth/me", "172.18.0.5:1", map[string]string{"X-Real-IP": "also bad"})
	st := h.limiter.Status().UntrustedForwardedHeaders
	assert.Zero(t, st.Local.Count)
	assert.Zero(t, st.Public.Count)
	assert.Empty(t, h.entries(logrus.WarnLevel))
	assert.Len(t, h.entries(logrus.DebugLevel), 2)

	h.hook.Reset()
	h.do(http.MethodGet, "/api/v1/auth/me", "172.18.0.5:1", map[string]string{"X-Forwarded-For": "198.18.0.1, 172.18.0.5"})
	assert.Empty(t, h.hook.AllEntries(), "well-formed headers from a trusted peer are silent")
}

func TestAuthRateLimiter_PrivatePeerWarnSuggestsSlash32Or128(t *testing.T) {
	h := newThrottleHarness(t, smallBudget(10, 60), nil)
	h.do(http.MethodGet, "/api/v1/auth/me", "172.18.0.5:1", map[string]string{"X-Forwarded-For": "198.18.0.1"})
	h.do(http.MethodGet, "/api/v1/auth/me", "[fd00::5]:1", map[string]string{"X-Forwarded-For": "198.18.0.1"})
	warns := h.entries(logrus.WarnLevel)
	require.Len(t, warns, 1, "one WARN per scope per interval")
	assert.Contains(t, warns[0].Message, "172.18.0.5/32")
	assert.Contains(t, warns[0].Message, "CHARON_TRUSTED_PROXIES")
	assert.Contains(t, warns[0].Message, "docs/configuration/trusted-proxies.md")
	assert.Equal(t, "private", warns[0].Data["scope"])

	h.hook.Reset()
	h.clock.Advance(16 * time.Minute)
	h.do(http.MethodGet, "/api/v1/auth/me", "[fd00::5]:1", map[string]string{"X-Forwarded-For": "198.18.0.1"})
	warns = h.entries(logrus.WarnLevel)
	require.Len(t, warns, 1)
	assert.Contains(t, warns[0].Message, "fd00::5/128")
	assert.Equal(t, uint64(3), warns[0].Data["count"])
}

func TestAuthRateLimiter_PublicPeerWarnHasNoTrustSuggestion(t *testing.T) {
	h := newThrottleHarness(t, smallBudget(10, 60), nil)
	h.do(http.MethodPost, "/api/v1/auth/login", "203.0.113.9:1", map[string]string{"X-Forwarded-For": "198.18.0.1"})
	warns := h.entries(logrus.WarnLevel)
	require.Len(t, warns, 1)
	assert.NotContains(t, warns[0].Message, "CHARON_TRUSTED_PROXIES")
	assert.NotContains(t, warns[0].Message, "/32")
	assert.NotContains(t, warns[0].Message, "203.0.113.9")
	assert.Contains(t, warns[0].Message, "needs no action")
	assert.Equal(t, "public", warns[0].Data["scope"])
	assert.Equal(t, "203.0.113.9", warns[0].Data["peer"])

	st := h.limiter.Status().UntrustedForwardedHeaders.Public
	assert.Equal(t, uint64(1), st.Count)
	assert.Equal(t, "203.0.113.9", st.LastPeer)
	assert.Equal(t, "public", st.LastPeerScope)
}

func TestAuthRateLimiter_DetectorWarnRateLimited(t *testing.T) {
	h := newThrottleHarness(t, smallBudget(10, 60), nil)
	hdr := map[string]string{"X-Forwarded-For": "198.18.0.1"}
	for i := 0; i < 20; i++ {
		h.do(http.MethodGet, "/api/v1/auth/me", "10.1.1.1:1", hdr)
		h.clock.Advance(30 * time.Second) // 10 minutes total
	}
	assert.Len(t, h.entries(logrus.WarnLevel), 1)
	h.clock.Advance(6 * time.Minute)
	h.do(http.MethodGet, "/api/v1/auth/me", "10.1.1.1:1", hdr)
	assert.Len(t, h.entries(logrus.WarnLevel), 2)
	assert.Equal(t, uint64(21), h.limiter.Status().UntrustedForwardedHeaders.Local.Count)
}

func TestAuthRateLimiter_PerScopeStateIndependent(t *testing.T) {
	h := newThrottleHarness(t, smallBudget(10, 60), nil)
	hdr := map[string]string{"X-Forwarded-For": "198.18.0.1"}
	h.do(http.MethodGet, "/api/v1/auth/me", "192.168.1.2:1", hdr)
	h.do(http.MethodGet, "/api/v1/auth/me", "203.0.113.9:1", hdr)
	assert.Len(t, h.entries(logrus.WarnLevel), 2, "each scope has its own WARN budget")

	st := h.limiter.Status().UntrustedForwardedHeaders
	assert.Equal(t, uint64(1), st.Local.Count)
	assert.Equal(t, "192.168.1.2", st.Local.LastPeer)
	assert.Equal(t, "private", st.Local.LastPeerScope)
	assert.Equal(t, uint64(1), st.Public.Count)
}

func TestAuthRateLimiter_PublicNoiseDoesNotMaskPrivateWarning(t *testing.T) {
	h := newThrottleHarness(t, smallBudget(1000, 1000), nil)
	hdr := map[string]string{"X-Forwarded-For": "198.18.0.1"}
	for i := 0; i < 50; i++ {
		h.do(http.MethodGet, "/api/v1/auth/me", fmt.Sprintf("203.0.113.%d:1", i), hdr)
	}
	h.do(http.MethodGet, "/api/v1/auth/me", "172.18.0.5:1", hdr)
	for i := 0; i < 50; i++ {
		h.do(http.MethodGet, "/api/v1/auth/me", fmt.Sprintf("198.51.100.%d:1", i), hdr)
	}

	var privateWarns int
	for _, e := range h.entries(logrus.WarnLevel) {
		if e.Data["scope"] == "private" {
			privateWarns++
			assert.Contains(t, e.Message, "172.18.0.5/32")
		}
	}
	assert.Equal(t, 1, privateWarns)
	st := h.limiter.Status().UntrustedForwardedHeaders
	assert.Equal(t, "172.18.0.5", st.Local.LastPeer)
	assert.Equal(t, uint64(1), st.Local.Count)
	assert.Equal(t, uint64(100), st.Public.Count)
}

func TestAuthRateLimiter_StatusReportsEffectiveConfig(t *testing.T) {
	lim, err := NewAuthRateLimiter(
		config.AuthRateLimitConfig{LoginRequests: 300, LoginWindowSec: 60, SessionRequests: 600, SessionWindowSec: 60},
		[]string{"10.0.0.0/8", "::1"},
	)
	require.NoError(t, err)
	st := lim.Status()
	assert.True(t, st.Enabled)
	assert.Equal(t, AuthRateLimitBudget{Requests: 300, WindowSeconds: 60}, st.Login)
	assert.Equal(t, AuthRateLimitBudget{Requests: 600, WindowSeconds: 60}, st.Session)
	assert.Equal(t, 2, st.TrustedProxyCount)

	data, err := json.Marshal(st)
	require.NoError(t, err)
	for _, key := range []string{`"enabled":true`, `"login":{"requests":300,"window_seconds":60}`, `"trusted_proxy_count":2`, `"untrusted_forwarded_headers":{"local":`} {
		assert.Contains(t, string(data), key)
	}
}
