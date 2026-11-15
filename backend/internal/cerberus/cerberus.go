// Package cerberus provides lightweight security checks (WAF, ACL, CrowdSec) with notification support.
package cerberus

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/metrics"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/ratelimit"
	securitypkg "github.com/Wikid82/charon/backend/internal/security"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/internal/util"
)

// Cerberus provides a lightweight facade for security checks (WAF, CrowdSec, ACL).
type Cerberus struct {
	cfg               config.SecurityConfig
	db                *gorm.DB
	accessSvc         *services.AccessListService
	securityNotifySvc *services.SecurityNotificationService
	enhancedNotifySvc *services.EnhancedSecurityNotificationService

	// Settings cache for performance - avoids DB queries on every request
	settingsCache     map[string]string
	settingsCacheMu   sync.RWMutex
	settingsCacheTime time.Time
	settingsCacheTTL  time.Duration

	// now is the clock used by the API limiter (injectable for tests).
	now func() time.Time
	// rateLimitWarn caps per-episode WARN lines from the API limiter.
	rateLimitWarn *ratelimit.WarnBudget
	// managementWarn caps the WARN lines about management access lists that cannot be used.
	managementWarn *ratelimit.WarnBudget
}

// New creates a new Cerberus instance
func New(cfg config.SecurityConfig, db *gorm.DB) *Cerberus {
	c := &Cerberus{
		cfg:               cfg,
		db:                db,
		accessSvc:         services.NewAccessListService(db),
		securityNotifySvc: services.NewSecurityNotificationService(db),
		enhancedNotifySvc: services.NewEnhancedSecurityNotificationService(db),
		settingsCache:     make(map[string]string),
		settingsCacheTTL:  60 * time.Second,
		now:               time.Now,
	}
	c.rateLimitWarn = ratelimit.NewWarnBudget(rateLimitWarnBurst, rateLimitWarnInterval, func() time.Time { return c.now() })
	c.managementWarn = ratelimit.NewWarnBudget(rateLimitWarnBurst, rateLimitWarnInterval, func() time.Time { return c.now() })
	return c
}

// getSetting retrieves a setting with in-memory caching.
// Returns the value and a boolean indicating if the key was found.
func (c *Cerberus) getSetting(key string) (string, bool) {
	if c.db == nil {
		return "", false
	}

	// Fast path: check cache with read lock
	c.settingsCacheMu.RLock()
	if time.Since(c.settingsCacheTime) < c.settingsCacheTTL {
		val, ok := c.settingsCache[key]
		c.settingsCacheMu.RUnlock()
		return val, ok
	}
	c.settingsCacheMu.RUnlock()

	// Slow path: refresh cache with write lock
	c.settingsCacheMu.Lock()
	defer c.settingsCacheMu.Unlock()

	// Double-check: another goroutine might have refreshed cache
	if time.Since(c.settingsCacheTime) < c.settingsCacheTTL {
		val, ok := c.settingsCache[key]
		return val, ok
	}

	// Refresh entire cache from DB (batch query is faster than individual queries)
	var settings []models.Setting
	if err := c.db.Where("key LIKE ?", "security.%").Find(&settings).Error; err != nil {
		logger.Log().WithError(err).Debug("Failed to refresh settings cache")
		return "", false
	}

	// Update cache
	c.settingsCache = make(map[string]string)
	for _, s := range settings {
		c.settingsCache[s.Key] = s.Value
	}
	c.settingsCacheTime = time.Now()

	val, ok := c.settingsCache[key]
	return val, ok
}

// InvalidateCache forces cache refresh on next access.
// Call this after updating security settings.
func (c *Cerberus) InvalidateCache() {
	c.settingsCacheMu.Lock()
	c.settingsCacheTime = time.Time{} // Zero time forces refresh
	c.settingsCacheMu.Unlock()
}

// IsEnabled returns whether Cerberus features are enabled via config or settings.
func (c *Cerberus) IsEnabled() bool {
	// DB-backed break-glass disable must take effect even when static config defaults to enabled.
	// This keeps the API reachable and prevents accidental lockouts when Cerberus/ACL is disabled via /security/disable.
	if c.db != nil {
		if sc, err := models.FindDefaultSecurityConfig(c.db); err == nil {
			if !sc.Enabled {
				return false
			}
		}

		// Runtime feature flag (highest priority after break-glass disable), then the legacy key for backward
		// compatibility. A stored switch decides, whatever its value.
		if sw, err := models.ResolveCerberusSwitchKeys(c.db); err == nil && sw != models.CerberusSwitchAbsent {
			return sw == models.CerberusSwitchOn
		}
	}

	if c.cfg.CerberusEnabled {
		return true
	}

	// If any of the security modes are explicitly enabled, consider Cerberus enabled.
	// Treat empty values as disabled to avoid treating zero-values ("") as enabled.
	if c.cfg.CrowdSecMode == "local" {
		return true
	}
	if (c.cfg.WAFMode != "" && c.cfg.WAFMode != "disabled") || c.cfg.RateLimitMode == "enabled" || c.cfg.ACLMode == "enabled" {
		return true
	}

	// Back-compat: check if all config fields are their zero values (implies defaults = enabled)
	// Note: cannot use == for struct comparison when it contains slices
	if c.cfg.CrowdSecMode == "" && c.cfg.CrowdSecAPIURL == "" && c.cfg.CrowdSecAPIKey == "" &&
		c.cfg.CrowdSecConfigDir == "" && c.cfg.WAFMode == "" && c.cfg.RateLimitMode == "" &&
		c.cfg.ACLMode == "" && !c.cfg.CerberusEnabled && len(c.cfg.ManagementCIDRs) == 0 {
		return true
	}

	return false
}

// orthrusConnectRoute is the agent WebSocket route. It is exempt from the management access lists.
const orthrusConnectRoute = "/api/v1/ws/orthrus/connect"

// warnManagement logs a management access problem, within the WARN budget.
func (c *Cerberus) warnManagement(msg string, err error) {
	if ok, suppressed := c.managementWarn.Take(); ok {
		logger.Log().WithError(err).WithField("suppressed", suppressed).Warn(msg)
	}
}

// Middleware returns a Gin middleware that enforces Cerberus checks when enabled.
func (c *Cerberus) Middleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// Check for emergency bypass flag (set by EmergencyBypass middleware)
		if middleware.IsEmergencyBypass(ctx) {
			logger.Log().WithField("path", util.SanitizeForLog(ctx.Request.URL.Path)).Debug("Cerberus: Skipping security checks (emergency bypass)")
			ctx.Next()
			return
		}

		if !c.IsEnabled() {
			ctx.Next()
			return
		}

		// WAF: The actual WAF protection is handled by the Coraza plugin at the Caddy layer.
		// This middleware just tracks metrics for requests when WAF is enabled.
		// Resolved the same way as the proxy config: settings key, then the default row, then static config.
		// Read per request, so a failed read is logged at debug level to keep the request path quiet.
		row, rowErr := models.DefaultSecurityConfigOrZero(c.db)
		wafEnabled, wafErr := models.WAFEnabled(c.db, row, models.StaticWAFEnabled(c.cfg.WAFMode))
		if readErr := errors.Join(rowErr, wafErr); readErr != nil {
			logger.Log().WithError(readErr).Debug("Failed to read WAF setting for metrics, using the default SecurityConfig row or static configuration")
		}
		if wafEnabled {
			metrics.IncWAFRequest()
			// Note: Actual blocking is done by Coraza in Caddy. This middleware
			// provides defense-in-depth tracking and ACL enforcement only.
		}

		// Rate Limit: Actual rate limiting is done by Caddy middleware.
		// Notifications are sent when Caddy middleware detects limit exceeded via webhook.
		// No per-request tracking needed here (Blocker 1: Production runtime dispatch for rate limit hits).

		// Access lists protecting Charon's own interface. Proxy-host lists never apply here. The Orthrus agent route is
		// exempt: agents authenticate with their own key and connect from remote hosts by design.
		if services.ACLEnabled(c.getSetting, c.cfg) && ctx.FullPath() != orthrusConnectRoute {
			if !c.managementAllows(ctx) {
				return
			}
		}

		// CrowdSec integration: The actual IP blocking is handled by the caddy-crowdsec-bouncer
		// plugin at the Caddy layer. This middleware provides defense-in-depth tracking.
		// When CrowdSec mode is "local", the bouncer communicates directly with the LAPI
		// to receive ban decisions and block malicious IPs before they reach the application.
		if c.cfg.CrowdSecMode == "local" {
			// Track that this request passed through CrowdSec evaluation
			// Note: Blocking decisions are made by Caddy bouncer, not here
			metrics.IncCrowdSecRequest()
			logger.Log().WithField("client_ip", util.SanitizeForLog(ctx.ClientIP())).WithField("path", util.SanitizeForLog(ctx.Request.URL.Path)).Debug("Request evaluated by CrowdSec bouncer at Caddy layer")
			// Blocker 1: Production runtime dispatch for CrowdSec decisions
			// CrowdSec decisions trigger notifications when bouncer blocks at Caddy layer
			// The actual notification is sent by Caddy via webhook callback to /api/v1/security/events
		}

		ctx.Next()
	}
}

// NotifySecurityEvent provides external notification hook for security events from Caddy/Coraza.
// Blocker 1: Production runtime dispatch path for WAF blocks, Rate limit hits, and CrowdSec decisions.
func (c *Cerberus) NotifySecurityEvent(ctx *gin.Context, event models.SecurityEvent) error {
	if !c.IsEnabled() {
		return nil
	}
	return c.sendSecurityNotification(ctx.Request.Context(), event)
}

// isAuthenticatedAdmin reports whether the caller is an authenticated administrator with an ID.
func (c *Cerberus) isAuthenticatedAdmin(ctx *gin.Context) bool {
	if middleware.CallerRole(ctx) != string(models.RoleAdmin) {
		return false
	}
	id, ok := middleware.CallerID(ctx)
	return ok && id > 0
}

// managementAllows judges the caller against the access lists chosen to protect Charon's own interface. It answers
// 403 and returns false when a listed, enabled list refuses the caller, or when the stored choice cannot be read or
// used: a set that cannot be read must not leave the interface open.
func (c *Cerberus) managementAllows(ctx *gin.Context) bool {
	uuids, err := services.ParseManagementListUUIDs(c.settingValue(securitypkg.ManagementListsSettingKey))
	if err != nil {
		c.warnManagement("Management access list choice cannot be read; refusing the request", err)
		ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Blocked by access control list"})
		return false
	}
	if len(uuids) == 0 {
		return true
	}

	clientIP := util.CanonicalizeIPForSecurity(ctx.ClientIP())
	verdict, err := c.accessSvc.EvaluateManagement(uuids, clientIP, c.accessSvc.AdminWhitelistRanges(), nil)
	if err != nil {
		c.warnManagement("Management access list cannot be used; refusing the request", err)
		ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Blocked by access control list"})
		return false
	}
	for _, name := range verdict.Skipped {
		c.warnManagement("Management access list not applied", errors.New(name))
	}
	if verdict.Allowed {
		return true
	}

	_ = c.sendSecurityNotification(context.Background(), models.SecurityEvent{
		EventType: "acl_deny",
		Severity:  "warn",
		Message:   "Access control list blocked request",
		ClientIP:  clientIP,
		Path:      ctx.Request.URL.Path,
		Timestamp: time.Now(),
		Metadata: map[string]any{
			"acl_name": verdict.Refused.Name,
			"acl_id":   verdict.Refused.ID,
		},
	})
	ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Blocked by access control list"})
	return false
}

// settingValue returns the cached value of a security setting, or "" when it is absent.
func (c *Cerberus) settingValue(key string) string {
	val, _ := c.getSetting(key)
	return val
}

// sendSecurityNotification dispatches a security event notification.
// Blocker 1: Wires runtime dispatch to provider-event authority under feature flag semantics.
// Blocker 2: Enforces notify-only fail-closed behavior - no legacy fallback when flag absent/false.
func (c *Cerberus) sendSecurityNotification(ctx context.Context, event models.SecurityEvent) error {
	if c.db == nil {
		return nil
	}

	// Check feature flag
	var setting models.Setting
	err := c.db.Where("key = ?", "feature.notifications.security_provider_events.enabled").First(&setting).Error

	// If feature flag is enabled, use provider-based dispatch
	if err == nil && strings.EqualFold(setting.Value, "true") {
		return c.enhancedNotifySvc.SendViaProviders(ctx, event)
	}

	// Blocker 2: Feature flag disabled or not found - fail closed (no notification, no legacy fallback)
	logger.Log().WithField("event_type", event.EventType).Debug("Security notification suppressed: feature flag disabled or absent")
	return nil
}
