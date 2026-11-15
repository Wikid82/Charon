package handlers

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/ratelimit"
)

// Each user may run this many connection tests per window. The limit is shared by
// the proxy host test and both remote server tests, so none of them can be used to
// scan destinations faster than this.
const (
	connectionTestsPerWindow = 10
	connectionTestWindow     = time.Minute
)

// defaultConnectionTestLimiter is the package limiter. A handler without a setter
// uses it, so the limit never depends on wiring.
var defaultConnectionTestLimiter = newConnectionTestLimiter()

func newConnectionTestLimiter() *ratelimit.KeyedLimiter {
	rate, burst := ratelimit.PerWindow(connectionTestsPerWindow, connectionTestWindow)
	return ratelimit.MustNewKeyedLimiter(ratelimit.Config{Rate: rate, Burst: burst})
}

// connectionTestGate limits connection tests per caller. Handlers embed it; the
// zero value uses the package limiter.
type connectionTestGate struct {
	limiter *ratelimit.KeyedLimiter
}

// SetConnectionTestLimiter replaces the limiter of this handler. Tests use it to
// set a small or fake limit; production code keeps the default.
func (g *connectionTestGate) SetConnectionTestLimiter(l *ratelimit.KeyedLimiter) {
	g.limiter = l
}

// allowConnectionTest reports whether the caller may run a connection test now.
// When it may not, it writes the 429 and returns false. It runs before any name
// lookup or dial.
func (g *connectionTestGate) allowConnectionTest(c *gin.Context) bool {
	limiter := g.limiter
	if limiter == nil {
		limiter = defaultConnectionTestLimiter
	}
	decision := limiter.Allow(connectionTestKey(c))
	if !decision.Allowed {
		ratelimit.Reject(c, decision)
		return false
	}
	return true
}

// connectionTestKey keys the limit by user, and by client address when no user is known.
func connectionTestKey(c *gin.Context) string {
	if id, ok := middleware.CallerID(c); ok && id > 0 {
		return fmt.Sprintf("user:%d", id)
	}
	clientIP := ""
	if c.Request != nil {
		clientIP = c.ClientIP()
	}
	return ratelimit.ClientKey(clientIP)
}

// logConnectionTest writes one debug line for a connection test: the caller and the
// outcome only. The server log is readable by standard users, so the address, the
// port and the cause are never written here.
func logConnectionTest(c *gin.Context, outcome string) {
	entry := logger.Log().WithField("outcome", outcome)
	if id, ok := middleware.CallerID(c); ok {
		entry = entry.WithField("caller_id", id)
	}
	entry.Debug("connection test finished")
}
