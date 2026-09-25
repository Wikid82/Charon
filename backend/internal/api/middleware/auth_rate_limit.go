package middleware

import (
	"fmt"
	"maps"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/metrics"
	"github.com/Wikid82/charon/backend/internal/ratelimit"
	"github.com/Wikid82/charon/backend/internal/security"
)

// AuthRateLimitClass selects which per-client budget a route draws on.
type AuthRateLimitClass string

// Sign-in throttle classes (issue #1317).
const (
	// AuthRateLimitLogin is the strict budget shared by every password verification.
	AuthRateLimitLogin AuthRateLimitClass = "login"
	// AuthRateLimitSession is the looser budget for token and status routes.
	AuthRateLimitSession AuthRateLimitClass = "session"
	// AuthRateLimitExempt routes are never throttled.
	AuthRateLimitExempt AuthRateLimitClass = "exempt"
)

// Global cap on first-denial WARN lines, shared by all classes.
const (
	authThrottleWarnBurst    = 10
	authThrottleWarnInterval = time.Minute
)

// detectorWarnInterval is the minimum spacing of forwarded-header WARNs per scope.
const detectorWarnInterval = 15 * time.Minute

const trustedProxiesDoc = "docs/configuration/trusted-proxies.md"

// authRouteClasses classifies every /api/v1/auth route by "METHOD full-path
// template". Routes missing from the table are throttled as session; the route
// inventory test fails until they are classified here.
var authRouteClasses = map[string]AuthRateLimitClass{
	"POST /api/v1/auth/login":             AuthRateLimitLogin,
	"POST /api/v1/auth/change-password":   AuthRateLimitLogin,
	"POST /api/v1/auth/refresh":           AuthRateLimitSession,
	"GET /api/v1/auth/status":             AuthRateLimitSession,
	"GET /api/v1/auth/verify":             AuthRateLimitExempt,
	"GET /api/v1/auth/me":                 AuthRateLimitExempt,
	"POST /api/v1/auth/logout":            AuthRateLimitExempt,
	"GET /api/v1/auth/accessible-hosts":   AuthRateLimitExempt,
	"GET /api/v1/auth/check-host/:hostId": AuthRateLimitExempt,
}

// AuthRouteClasses returns a copy of the route classification table.
func AuthRouteClasses() map[string]AuthRateLimitClass {
	return maps.Clone(authRouteClasses)
}

// AuthRouteClass returns the class for a route; unknown routes are session.
func AuthRouteClass(method, fullPath string) AuthRateLimitClass {
	if class, ok := authRouteClasses[method+" "+fullPath]; ok {
		return class
	}
	return AuthRateLimitSession
}

// AuthRateLimitBudget is one class's effective budget.
type AuthRateLimitBudget struct {
	Requests      int `json:"requests"`
	WindowSeconds int `json:"window_seconds"`
}

// ForwardedHeaderObservation records forwarded client-address headers sent by
// untrusted peers of one scope.
type ForwardedHeaderObservation struct {
	Count         uint64     `json:"count"`
	LastSeen      *time.Time `json:"last_seen"`
	LastPeer      string     `json:"last_peer"`
	LastPeerScope string     `json:"last_peer_scope"`
}

// ForwardedHeaderObservations splits observations by peer scope so public
// noise never masks a local (loopback/private) misconfiguration signal.
type ForwardedHeaderObservations struct {
	Local  ForwardedHeaderObservation `json:"local"`
	Public ForwardedHeaderObservation `json:"public"`
}

// AuthRateLimitStatus is the admin-visible state of the sign-in throttle.
type AuthRateLimitStatus struct {
	Enabled                   bool                        `json:"enabled"`
	Login                     AuthRateLimitBudget         `json:"login"`
	Session                   AuthRateLimitBudget         `json:"session"`
	TrustedProxyCount         int                         `json:"trusted_proxy_count"`
	UntrustedForwardedHeaders ForwardedHeaderObservations `json:"untrusted_forwarded_headers"`
}

// AuthRateLimiterOption customizes NewAuthRateLimiter.
type AuthRateLimiterOption func(*AuthRateLimiter)

// WithAuthRateLimitClock injects the clock used by buckets and log caps (tests).
func WithAuthRateLimitClock(now func() time.Time) AuthRateLimiterOption {
	return func(a *AuthRateLimiter) { a.now = now }
}

// AuthRateLimiter is the always-on per-client throttle for sign-in and
// password-verifying routes, plus the forwarded-header detector.
type AuthRateLimiter struct {
	cfg      config.AuthRateLimitConfig
	now      func() time.Time
	login    *ratelimit.KeyedLimiter
	session  *ratelimit.KeyedLimiter
	trusted  security.TrustedProxyMatcher
	warn     *ratelimit.WarnBudget
	detector *forwardedHeaderDetector
}

// NewAuthRateLimiter builds the throttle from cfg (normalized again here so
// hand-built configs get secure defaults) and the effective trusted-proxy list.
func NewAuthRateLimiter(cfg config.AuthRateLimitConfig, trustedProxies []string, opts ...AuthRateLimiterOption) (*AuthRateLimiter, error) {
	cfg, _ = cfg.Normalize()
	a := &AuthRateLimiter{cfg: cfg, now: time.Now, trusted: security.NewTrustedProxyMatcher(trustedProxies)}
	for _, opt := range opts {
		opt(a)
	}

	var err error
	if a.login, err = newClassLimiter(cfg.LoginRequests, cfg.LoginWindowSec, a.now); err != nil {
		return nil, fmt.Errorf("create login throttle: %w", err)
	}
	if a.session, err = newClassLimiter(cfg.SessionRequests, cfg.SessionWindowSec, a.now); err != nil {
		return nil, fmt.Errorf("create session throttle: %w", err)
	}
	a.warn = ratelimit.NewWarnBudget(authThrottleWarnBurst, authThrottleWarnInterval, a.now)
	a.detector = newForwardedHeaderDetector(a.now)
	return a, nil
}

func newClassLimiter(requests, windowSec int, now func() time.Time) (*ratelimit.KeyedLimiter, error) {
	r, b := ratelimit.PerWindow(requests, time.Duration(windowSec)*time.Second)
	return ratelimit.NewKeyedLimiter(ratelimit.Config{Rate: r, Burst: b, Now: now})
}

// Middleware throttles the /api/v1/auth group per route class. The detector
// runs on every request, including exempt and bypassed ones.
func (a *AuthRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		a.detector.observe(c, a.trusted)
		if a.skip(c) {
			c.Next()
			return
		}
		class := AuthRouteClass(c.Request.Method, c.FullPath())
		if class == AuthRateLimitExempt || a.allow(c, class) {
			c.Next()
		}
	}
}

// AllowPasswordAttempt charges one password verification outside the /auth
// group against the caller's login budget. It returns false after writing the
// standard 429 response.
func (a *AuthRateLimiter) AllowPasswordAttempt(c *gin.Context) bool {
	a.detector.observe(c, a.trusted)
	if a.skip(c) {
		return true
	}
	return a.allow(c, AuthRateLimitLogin)
}

// Status reports the effective budgets, trust list size and detector state.
func (a *AuthRateLimiter) Status() AuthRateLimitStatus {
	return AuthRateLimitStatus{
		Enabled:                   !a.cfg.Disabled,
		Login:                     AuthRateLimitBudget{Requests: a.cfg.LoginRequests, WindowSeconds: a.cfg.LoginWindowSec},
		Session:                   AuthRateLimitBudget{Requests: a.cfg.SessionRequests, WindowSeconds: a.cfg.SessionWindowSec},
		TrustedProxyCount:         a.trusted.Len(),
		UntrustedForwardedHeaders: a.detector.snapshot(),
	}
}

func (a *AuthRateLimiter) skip(c *gin.Context) bool {
	return a.cfg.Disabled || IsEmergencyBypass(c)
}

// allow consumes one token from the class bucket, or rejects with 429.
func (a *AuthRateLimiter) allow(c *gin.Context, class AuthRateLimitClass) bool {
	limiter := a.session
	if class == AuthRateLimitLogin {
		limiter = a.login
	}
	key := ratelimit.ClientKey(c.ClientIP())
	decision := limiter.Allow(key)
	if decision.Allowed {
		return true
	}

	metrics.IncAuthRateLimited(string(class))
	entry := GetRequestLogger(c).WithFields(map[string]any{
		"class":  string(class),
		"client": key,
		"route":  c.FullPath(),
	})
	a.warn.LogDenial(entry, decision, "Sign-in throttle limited a client")
	ratelimit.Reject(c, decision)
	return false
}

// --- forwarded-header detector ----------------------------------------------------

type observationRecord struct {
	count    uint64
	lastSeen time.Time
	lastPeer netip.Addr
	scope    ratelimit.AddrScope
	warn     *ratelimit.WarnBudget
}

func (r *observationRecord) snapshot() ForwardedHeaderObservation {
	if r.count == 0 {
		return ForwardedHeaderObservation{}
	}
	seen := r.lastSeen
	peer := ""
	if r.lastPeer.IsValid() {
		peer = r.lastPeer.String()
	}
	return ForwardedHeaderObservation{Count: r.count, LastSeen: &seen, LastPeer: peer, LastPeerScope: string(r.scope)}
}

// forwardedHeaderDetector records requests whose TCP peer is not a trusted
// proxy but which carry X-Forwarded-For or X-Real-IP (headers Gin ignores).
type forwardedHeaderDetector struct {
	mu     sync.Mutex
	now    func() time.Time
	local  observationRecord
	public observationRecord
}

func newForwardedHeaderDetector(now func() time.Time) *forwardedHeaderDetector {
	return &forwardedHeaderDetector{
		now:    now,
		local:  observationRecord{warn: ratelimit.NewWarnBudget(1, detectorWarnInterval, now)},
		public: observationRecord{warn: ratelimit.NewWarnBudget(1, detectorWarnInterval, now)},
	}
}

func (d *forwardedHeaderDetector) observe(c *gin.Context, trusted security.TrustedProxyMatcher) {
	xff := c.GetHeader("X-Forwarded-For")
	realIP := c.GetHeader("X-Real-IP")
	if xff == "" && realIP == "" {
		return
	}

	peer, err := netip.ParseAddr(c.RemoteIP())
	if err == nil {
		peer = peer.WithZone("").Unmap()
	}
	if trusted.Contains(peer) {
		if !forwardedHeadersWellFormed(xff, realIP) {
			GetRequestLogger(c).WithField("peer", peer.String()).
				Debug("Trusted proxy sent a malformed forwarded client-address header; using the proxy address")
		}
		return
	}

	scope := ratelimit.ClassifyAddr(peer)
	d.mu.Lock()
	rec := &d.public
	if scope != ratelimit.ScopePublic {
		rec = &d.local
	}
	rec.count++
	rec.lastSeen = d.now().UTC()
	rec.lastPeer = peer
	rec.scope = scope
	count := rec.count
	emit, _ := rec.warn.Take()
	d.mu.Unlock()

	if emit {
		logUntrustedForwardedHeaders(c, peer, scope, count)
	}
}

func (d *forwardedHeaderDetector) snapshot() ForwardedHeaderObservations {
	d.mu.Lock()
	defer d.mu.Unlock()
	return ForwardedHeaderObservations{Local: d.local.snapshot(), Public: d.public.snapshot()}
}

// logUntrustedForwardedHeaders emits the scope-aware WARN. For public peers it
// never suggests trusting the address: anyone can send these headers.
func logUntrustedForwardedHeaders(c *gin.Context, peer netip.Addr, scope ratelimit.AddrScope, count uint64) {
	peerStr := ""
	if peer.IsValid() {
		peerStr = peer.String()
	}
	entry := GetRequestLogger(c).WithFields(map[string]any{"peer": peerStr, "scope": string(scope), "count": count})
	if scope == ratelimit.ScopePublic {
		entry.Warn("Ignored forwarded client-address headers sent from a public address that is not a trusted proxy. " +
			"This is usually a client sending forged headers and needs no action. See " + trustedProxiesDoc + ".")
		return
	}
	suffix := "/128"
	if peer.Is4() {
		suffix = "/32"
	}
	entry.Warn(fmt.Sprintf("Sign-in requests carry forwarded client-address headers from %s, which is not a trusted proxy, "+
		"so every visitor behind it shares one sign-in budget. If %s is your reverse proxy, add %s%s to "+
		"CHARON_TRUSTED_PROXIES. See %s.", peerStr, peerStr, peerStr, suffix, trustedProxiesDoc))
}

// forwardedHeadersWellFormed reports whether every X-Forwarded-For hop and the
// X-Real-IP value (when present) parse as IP addresses, as Gin requires.
func forwardedHeadersWellFormed(xff, realIP string) bool {
	if xff != "" {
		for _, hop := range strings.Split(xff, ",") {
			if net.ParseIP(strings.TrimSpace(hop)) == nil {
				return false
			}
		}
	}
	return realIP == "" || net.ParseIP(strings.TrimSpace(realIP)) != nil
}
