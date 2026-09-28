package cerberus

import (
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/ratelimit"
	"github.com/Wikid82/charon/backend/internal/util"
)

// Built-in API limiter budget, used unless config or settings override it.
const (
	defaultRateLimitRequests  = 100
	defaultRateLimitWindowSec = 60
	defaultRateLimitBurst     = 20
)

// Cap on per-episode WARN lines emitted by the Cerberus limiter.
const (
	rateLimitWarnBurst    = 10
	rateLimitWarnInterval = time.Minute
)

// controlPlanePrefixes are the admin configuration paths exempt from the API
// limiter for validated admins, so an admin can always manage security settings.
var controlPlanePrefixes = []string{"/api/v1/security", "/api/v1/settings", "/api/v1/config"}

// isAdminSecurityControlPlaneRequest reports whether the request comes from an
// admin validated by OptionalAuth and targets a control-plane path (segment-aware).
func (c *Cerberus) isAdminSecurityControlPlaneRequest(ctx *gin.Context) bool {
	if !c.isAuthenticatedAdmin(ctx) {
		return false
	}

	parsedPath := ctx.Request.URL.Path
	if rawPath := ctx.Request.URL.RawPath; rawPath != "" {
		if decoded, err := url.PathUnescape(rawPath); err == nil {
			parsedPath = decoded
		}
	}

	for _, prefix := range controlPlanePrefixes {
		if parsedPath == prefix || strings.HasPrefix(parsedPath, prefix+"/") {
			return true
		}
	}
	return false
}

// rateLimitEnabled applies the static mode, then lets the runtime setting
// override it in either direction.
func (c *Cerberus) rateLimitEnabled() bool {
	enabled := c.cfg.RateLimitMode == "enabled"
	if v, ok := c.getSetting("security.rate_limit.enabled"); ok {
		enabled = strings.EqualFold(v, "true")
	}
	return enabled
}

// positiveSetting returns the setting as a positive int, if present and valid.
func (c *Cerberus) positiveSetting(key string) (int, bool) {
	val, ok := c.getSetting(key)
	if !ok {
		return 0, false
	}
	v, err := strconv.Atoi(val)
	if err != nil || v <= 0 {
		return 0, false
	}
	return v, true
}

// effectiveRateLimit resolves the budget: built-in defaults, then positive
// static config values, then positive runtime settings.
func (c *Cerberus) effectiveRateLimit() (requests, windowSec, burst int) {
	requests, windowSec, burst = defaultRateLimitRequests, defaultRateLimitWindowSec, defaultRateLimitBurst

	if c.cfg.RateLimitRequests > 0 {
		requests = c.cfg.RateLimitRequests
	}
	if c.cfg.RateLimitWindowSec > 0 {
		windowSec = c.cfg.RateLimitWindowSec
	}
	if c.cfg.RateLimitBurst > 0 {
		burst = c.cfg.RateLimitBurst
	}

	if v, ok := c.positiveSetting("security.rate_limit.requests"); ok {
		requests = v
	}
	if v, ok := c.positiveSetting("security.rate_limit.window"); ok {
		windowSec = v
	}
	if v, ok := c.positiveSetting("security.rate_limit.burst"); ok {
		burst = v
	}
	return requests, windowSec, burst
}

// RateLimitMiddleware enforces the opt-in per-client API limit configured by
// security config and settings. Each call returns an independent limiter.
func (c *Cerberus) RateLimitMiddleware() gin.HandlerFunc {
	clock := func() time.Time { return c.now() }
	r, b := ratelimit.PerWindow(defaultRateLimitRequests, defaultRateLimitWindowSec*time.Second)
	limiter := ratelimit.MustNewKeyedLimiter(ratelimit.Config{Rate: r, Burst: b, Now: clock})

	return func(ctx *gin.Context) {
		if middleware.IsEmergencyBypass(ctx) || c.isAdminSecurityControlPlaneRequest(ctx) || !c.rateLimitEnabled() {
			ctx.Next()
			return
		}

		requests, windowSec, burst := c.effectiveRateLimit()
		// float64 division: requests and windowSec are ints.
		if err := limiter.Reconfigure(rate.Limit(float64(requests)/float64(windowSec)), burst); err != nil {
			logger.Log().WithError(err).Error("Cerberus rate limiter: invalid budget; request not limited")
			ctx.Next()
			return
		}

		key := ratelimit.ClientKey(ctx.ClientIP())
		decision := limiter.Allow(key)
		if decision.Allowed {
			ctx.Next()
			return
		}

		c.logRateLimitDenial(ctx, key, decision)
		ratelimit.Reject(ctx, decision)
	}
}

// logRateLimitDenial logs the first denial of an episode at WARN (globally
// capped) and everything else at DEBUG. No request data beyond the sanitized
// client key and route template is logged.
func (c *Cerberus) logRateLimitDenial(ctx *gin.Context, key string, d ratelimit.Decision) {
	entry := middleware.GetRequestLogger(ctx).WithFields(map[string]any{
		"client": util.SanitizeForLog(key),
		"route":  util.SanitizeForLog(ctx.FullPath()),
	})
	c.rateLimitWarn.LogDenial(entry, d, "API rate limit exceeded")
}
