package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/gin-gonic/gin"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/util"
)

// TrustedOriginSource returns the origin ("scheme://host[:port]") of the configured public address, and false when none
// is configured. It is read only by BrowserOriginGuard.Refresh, never per request.
type TrustedOriginSource func() (origin string, ok bool)

// BrowserOriginGuard runs the standard library's cross-origin check with the configured public address added as a
// trusted origin. The public address is what a browser sends as Origin when Charon sits behind another proxy that
// rewrites Host, so a page served from it is not refused. Refresh rebuilds the check from the source. Each rebuild
// replaces the previous one atomically, so requests never read the database and a changed setting takes effect at the
// next Refresh.
type BrowserOriginGuard struct {
	source  TrustedOriginSource
	mu      sync.Mutex // serialises Refresh so the last rebuild reflects the newest setting
	checker atomic.Pointer[http.CrossOriginProtection]
}

// NewBrowserOriginGuard builds a guard from the current value of source. A nil source trusts no extra origin.
func NewBrowserOriginGuard(source TrustedOriginSource) *BrowserOriginGuard {
	g := &BrowserOriginGuard{source: source}
	if err := g.Refresh(); err != nil {
		logger.Log().WithError(err).Warn("Browser origin guard started without the configured public origin")
	}
	return g
}

// Refresh rebuilds the check from the current source. Call it after the public address setting is written. A value
// the standard library rejects leaves the guard trusting no extra origin (fail closed) and returns the error.
func (g *BrowserOriginGuard) Refresh() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	checker := http.NewCrossOriginProtection()
	var err error
	if g.source != nil {
		if origin, ok := g.source(); ok {
			err = checker.AddTrustedOrigin(origin)
		}
	}
	// AddTrustedOrigin rejects a value before it is stored, so a failed checker trusts no extra origin.
	g.checker.Store(checker)
	if err != nil {
		return fmt.Errorf("trust configured public origin: %w", err)
	}
	return nil
}

// RequireSameOriginForBrowserAuth refuses cross-origin requests that a browser would send with ambient credentials. A
// request that carries an Authorization header is header-authenticated, so it passes unchanged: a page on another
// origin cannot add that header without a CORS preflight, which Charon does not grant. Any other request is checked
// with the guard's standard library Sec-Fetch-Site and Origin rules, where the configured public address is trusted.
// Safe methods and non-browser clients pass, except that a WebSocket upgrade (a GET) is checked like a mutation so that
// another site cannot open an authenticated socket. A refused request gets 403 cross_origin_refused before the handler
// runs.
//
// SameSite cookies alone do not cover same-site sibling origins, so the protected route group applies this check to
// every unsafe method, not only to the routes that need it.
func RequireSameOriginForBrowserAuth(guard *BrowserOriginGuard) gin.HandlerFunc {
	return func(c *gin.Context) {
		if len(c.Request.Header.Values("Authorization")) > 0 {
			c.Next()
			return
		}
		if err := guard.check(c.Request); err != nil {
			logger.Log().
				WithField("sec_fetch_site", util.SanitizeForLog(c.Request.Header.Get("Sec-Fetch-Site"))).
				WithField("origin", util.SanitizeForLog(c.Request.Header.Get("Origin"))).
				Debug("Refused cross-origin request to a cookie-authenticated route")
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":      "Cross-origin request refused",
				"error_code": "cross_origin_refused",
			})
			return
		}
		c.Next()
	}
}

// check applies the standard library rules to r. The standard library skips safe methods, and a WebSocket handshake
// is a GET, so an upgrade request is checked as if it were a POST. The probe is a shallow copy and r is not changed.
func (g *BrowserOriginGuard) check(r *http.Request) error {
	checker := g.checker.Load()
	if isWebSocketUpgrade(r) {
		probe := r.WithContext(r.Context())
		probe.Method = http.MethodPost
		return checker.Check(probe)
	}
	return checker.Check(r)
}

// isWebSocketUpgrade reports whether r asks to switch to the WebSocket protocol.
func isWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}
