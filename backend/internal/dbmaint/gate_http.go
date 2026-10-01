package dbmaint

import (
	"crypto/sha256"
	_ "embed" // maintenance.html is embedded below.
	"encoding/base64"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

// Paths the gate answers itself, by exact match. StatusPath is answered in
// every phase; HealthPath only while the gate is active.
const (
	StatusPath = "/api/v1/maintenance/status"
	HealthPath = "/api/v1/health"

	// retryAfterSeconds is the polling hint sent with every 503.
	retryAfterSeconds = 15
	maintenanceError  = "Database optimization in progress"
)

// maintenancePage is the self-contained page served for unmatched paths while
// the gate is active. It has one inline script and one inline style, both
// authorized by hash in maintenanceCSP.
//
//go:embed maintenance.html
var maintenancePage []byte

// maintenanceCSP is computed once from the embedded page. The gate precedes the
// SecurityHeaders middleware, so its responses carry their own policy.
var maintenanceCSP = buildMaintenanceCSP(maintenancePage)

var (
	inlineScriptRe = regexp.MustCompile(`(?s)<script>(.*?)</script>`)
	inlineStyleRe  = regexp.MustCompile(`(?s)<style>(.*?)</style>`)
)

func buildMaintenanceCSP(page []byte) string {
	hashOf := func(re *regexp.Regexp) string {
		m := re.FindSubmatch(page)
		if m == nil {
			// The page is embedded at build time; a page without its inline
			// script or style is a programming error caught by the tests.
			return "'none'"
		}
		sum := sha256.Sum256(m[1])
		return "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
	}
	return fmt.Sprintf("default-src 'none'; script-src %s; style-src %s; connect-src 'self'; "+
		"base-uri 'none'; form-action 'none'; frame-ancestors 'none'",
		hashOf(inlineScriptRe), hashOf(inlineStyleRe))
}

// setGateHeaders sets the headers every gate-written response carries.
func setGateHeaders(c *gin.Context) {
	h := c.Writer.Header()
	h.Set("Content-Security-Policy", maintenanceCSP)
	h.Set("Cache-Control", "no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
}

func setRetryAfter(c *gin.Context) {
	c.Writer.Header().Set("Retry-After", fmt.Sprint(retryAfterSeconds))
}

// write sends a complete response and aborts the chain. HEAD gets the status
// and headers without a body.
func write(c *gin.Context, status int, contentType string, body []byte) {
	if c.Request.Method == http.MethodHead {
		c.Header("Content-Type", contentType)
		c.Status(status)
		c.Writer.WriteHeaderNow()
		c.Abort()
		return
	}
	c.Data(status, contentType, body)
	c.Abort()
}

func isReadMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead
}

// wantsJSON selects the JSON 503: API paths and clients that ask for JSON.
func wantsJSON(c *gin.Context) bool {
	path := c.Request.URL.Path
	return path == "/api" || strings.HasPrefix(path, "/api/") ||
		strings.Contains(c.GetHeader("Accept"), "application/json")
}

// Middleware returns the gate middleware; install it before every other
// middleware that touches the database (EmergencyBypass, RateLimit).
//
// In EVERY phase it answers GET/HEAD status itself with a static body and
// aborts. While the gate is active (checking or converting) it also answers
// GET/HEAD health itself, because nothing behind it, which may wait on the
// pinned pool connection, can be allowed to make the healthcheck hang; health
// is the DB-free health handler. Every other active-phase request gets a 503:
// JSON for API paths and clients that ask for it, the embedded page otherwise.
// In all other phases requests, health included, pass through to the normal
// chain so rate limiting and the other middleware keep applying. A nil gate is
// a pass-through.
func (g *Gate) Middleware(health gin.HandlerFunc) gin.HandlerFunc {
	if g == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if isReadMethod(c.Request.Method) {
			switch path {
			case StatusPath:
				g.writeStatus(c)
				return
			case HealthPath:
				if g.Active() {
					setGateHeaders(c)
					health(c)
					c.Abort()
					return
				}
			}
		}

		if !g.Active() {
			c.Next()
			return
		}
		setGateHeaders(c)
		setRetryAfter(c)
		if wantsJSON(c) {
			write(c, http.StatusServiceUnavailable, gin.MIMEJSON, maintenanceJSON)
			return
		}
		write(c, http.StatusServiceUnavailable, gin.MIMEHTML, maintenancePage)
	}
}

// maintenanceJSON is the static 503 body for API clients.
var maintenanceJSON = []byte(fmt.Sprintf(`{"error":%q,"maintenance":true,"retry_after_seconds":%d}`,
	maintenanceError, retryAfterSeconds))

// emergencyJSON is the static 503 body of the emergency server.
var emergencyJSON = []byte(fmt.Sprintf(`{"error":%q,"maintenance":true}`, maintenanceError))

func (g *Gate) writeStatus(c *gin.Context) {
	snap := g.Snapshot()
	setGateHeaders(c)
	if snap.Phase.active() {
		setRetryAfter(c)
	}
	body := fmt.Sprintf(`{"active":%t,"phase":%q,"elapsed_seconds":%d}`,
		snap.Phase.active(), snap.Phase, snap.ElapsedSeconds)
	write(c, http.StatusOK, gin.MIMEJSON, []byte(body))
}

// EmergencyMiddleware makes the emergency server answer a fast 503 while the
// gate is active, without touching the database pool. Register the server's
// DB-free /health route before it so that stays 200 in every phase.
func (g *Gate) EmergencyMiddleware() gin.HandlerFunc {
	if g == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		if !g.Active() {
			c.Next()
			return
		}
		setGateHeaders(c)
		setRetryAfter(c)
		write(c, http.StatusServiceUnavailable, gin.MIMEJSON, emergencyJSON)
	}
}
