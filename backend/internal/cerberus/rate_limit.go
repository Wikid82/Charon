package cerberus

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/util"
)

func isAdminSecurityControlPlaneRequest(ctx *gin.Context) bool {
	parsedPath := ctx.Request.URL.Path
	if rawPath := ctx.Request.URL.RawPath; rawPath != "" {
		if decoded, err := url.PathUnescape(rawPath); err == nil {
			parsedPath = decoded
		}
	}

	isControlPlanePath := strings.HasPrefix(parsedPath, "/api/v1/security/") ||
		strings.HasPrefix(parsedPath, "/api/v1/settings") ||
		strings.HasPrefix(parsedPath, "/api/v1/config")

	if !isControlPlanePath {
		return false
	}

	role, exists := ctx.Get("role")
	if exists {
		if roleStr, ok := role.(string); ok && strings.EqualFold(roleStr, "admin") {
			return true
		}
	}

	authHeader := strings.TrimSpace(ctx.GetHeader("Authorization"))
	return strings.HasPrefix(strings.ToLower(authHeader), "bearer ")
}

// rateLimitManager manages per-IP rate limiters.
type rateLimitManager struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	lastSeen map[string]time.Time
}

func newRateLimitManager() *rateLimitManager {
	rl := &rateLimitManager{
		limiters: make(map[string]*rate.Limiter),
		lastSeen: make(map[string]time.Time),
	}
	// Start cleanup goroutine
	go rl.cleanupLoop()
	return rl
}

func (rl *rateLimitManager) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.cleanup()
	}
}

func (rl *rateLimitManager) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	cutoff := time.Now().Add(-10 * time.Minute)
	for ip, seen := range rl.lastSeen {
		if seen.Before(cutoff) {
			delete(rl.limiters, ip)
			delete(rl.lastSeen, ip)
		}
	}
}

func (rl *rateLimitManager) getLimiter(ip string, r rate.Limit, b int) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	lim, exists := rl.limiters[ip]
	if !exists {
		lim = rate.NewLimiter(r, b)
		rl.limiters[ip] = lim
	}
	rl.lastSeen[ip] = time.Now()

	// Check if limit changed (re-config)
	if lim.Limit() != r || lim.Burst() != b {
		lim = rate.NewLimiter(r, b)
		rl.limiters[ip] = lim
	}

	return lim
}

// NewRateLimitMiddleware creates a new rate limit middleware with fixed parameters.
// Useful for testing or when Cerberus context is not available.
func NewRateLimitMiddleware(requests int, windowSec int, burst int) gin.HandlerFunc {
	mgr := newRateLimitManager()

	if windowSec <= 0 {
		windowSec = 1
	}
	limit := rate.Limit(float64(requests) / float64(windowSec))

	return func(ctx *gin.Context) {
		// Check for emergency bypass flag
		if bypass, exists := ctx.Get("emergency_bypass"); exists && bypass.(bool) {
			ctx.Next()
			return
		}

		if isAdminSecurityControlPlaneRequest(ctx) {
			ctx.Next()
			return
		}

		clientIP := util.CanonicalizeIPForSecurity(ctx.ClientIP())
		limiter := mgr.getLimiter(clientIP, limit, burst)

		if !limiter.Allow() {
			logger.Log().WithField("ip", clientIP).Warn("Rate limit exceeded (Go middleware)")
			ctx.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Too many requests"})
			return
		}

		ctx.Next()
	}
}

// RateLimitMiddleware enforces rate limiting based on security config.
func (c *Cerberus) RateLimitMiddleware() gin.HandlerFunc {
	mgr := newRateLimitManager()

	return func(ctx *gin.Context) {
		// Check for emergency bypass flag
		if bypass, exists := ctx.Get("emergency_bypass"); exists && bypass.(bool) {
			ctx.Next()
			return
		}

		if isAdminSecurityControlPlaneRequest(ctx) {
			ctx.Next()
			return
		}

		// Check config enabled status, then let dynamic setting override both true and false.
		enabled := c.cfg.RateLimitMode == "enabled"
		if v, ok := c.getSetting("security.rate_limit.enabled"); ok {
			enabled = strings.EqualFold(v, "true")
		}

		if !enabled {
			ctx.Next()
			return
		}

		// Determine limits
		requests := 100 // per window
		window := 60    // seconds
		burst := 20

		if c.cfg.RateLimitRequests > 0 {
			requests = c.cfg.RateLimitRequests
		}
		if c.cfg.RateLimitWindowSec > 0 {
			window = c.cfg.RateLimitWindowSec
		}
		if c.cfg.RateLimitBurst > 0 {
			burst = c.cfg.RateLimitBurst
		}

		// Check for dynamic overrides from settings (Issue #3 fix)
		if val, ok := c.getSetting("security.rate_limit.requests"); ok {
			if v, err := strconv.Atoi(val); err == nil && v > 0 {
				requests = v
			}
		}
		if val, ok := c.getSetting("security.rate_limit.window"); ok {
			if v, err := strconv.Atoi(val); err == nil && v > 0 {
				window = v
			}
		}
		if val, ok := c.getSetting("security.rate_limit.burst"); ok {
			if v, err := strconv.Atoi(val); err == nil && v > 0 {
				burst = v
			}
		}

		if window == 0 {
			window = 60
		}
		limit := rate.Limit(float64(requests) / float64(window))

		clientIP := util.CanonicalizeIPForSecurity(ctx.ClientIP())
		limiter := mgr.getLimiter(clientIP, limit, burst)

		if !limiter.Allow() {
			logger.Log().WithField("ip", clientIP).Warn("Rate limit exceeded (Go middleware)")
			ctx.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Too many requests"})
			return
		}

		ctx.Next()
	}
}
