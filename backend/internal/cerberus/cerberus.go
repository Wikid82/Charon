// Package cerberus provides lightweight security checks (WAF, ACL, CrowdSec) with notification support.
package cerberus

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/metrics"
	"github.com/Wikid82/charon/backend/internal/models"
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
}

// New creates a new Cerberus instance
func New(cfg config.SecurityConfig, db *gorm.DB) *Cerberus {
	return &Cerberus{
		cfg:               cfg,
		db:                db,
		accessSvc:         services.NewAccessListService(db),
		securityNotifySvc: services.NewSecurityNotificationService(db),
		enhancedNotifySvc: services.NewEnhancedSecurityNotificationService(db),
		settingsCache:     make(map[string]string),
		settingsCacheTTL:  60 * time.Second,
	}
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
		var sc models.SecurityConfig
		if err := c.db.Where("name = ?", "default").First(&sc).Error; err == nil {
			if !sc.Enabled {
				return false
			}
		}

		var s models.Setting
		// Runtime feature flag (highest priority after break-glass disable)
		if err := c.db.Where("key = ?", "feature.cerberus.enabled").First(&s).Error; err == nil {
			return strings.EqualFold(s.Value, "true")
		}
		// Fallback to legacy setting for backward compatibility
		s = models.Setting{} // Reset to prevent ID leakage from previous query
		if err := c.db.Where("key = ?", "security.cerberus.enabled").First(&s).Error; err == nil {
			return strings.EqualFold(s.Value, "true")
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

// Middleware returns a Gin middleware that enforces Cerberus checks when enabled.
func (c *Cerberus) Middleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// Check for emergency bypass flag (set by EmergencyBypass middleware)
		if bypass, exists := ctx.Get("emergency_bypass"); exists && bypass.(bool) {
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
		// Check runtime setting first (from cache), then fall back to static config.
		wafEnabled := c.cfg.WAFMode != "" && c.cfg.WAFMode != "disabled"
		if val, ok := c.getSetting("security.waf.enabled"); ok {
			wafEnabled = strings.EqualFold(val, "true")
		}
		if wafEnabled {
			metrics.IncWAFRequest()
			// Note: Actual blocking is done by Coraza in Caddy. This middleware
			// provides defense-in-depth tracking and ACL enforcement only.
		}

		// ACL: simple per-request evaluation against all access lists if enabled
		// Check runtime setting first (from cache), then fall back to static config.
		aclEnabled := c.cfg.ACLMode == "enabled"
		if val, ok := c.getSetting("security.acl.enabled"); ok {
			aclEnabled = strings.EqualFold(val, "true")
		}

		if aclEnabled {
			clientIP := util.CanonicalizeIPForSecurity(ctx.ClientIP())
			isAdmin := c.isAuthenticatedAdmin(ctx)
			adminWhitelistConfigured := false
			if isAdmin {
				whitelisted, hasWhitelist := c.adminWhitelistStatus(clientIP)
				adminWhitelistConfigured = hasWhitelist
				if whitelisted {
					ctx.Next()
					return
				}
			}

			acls, err := c.accessSvc.List()
			if err == nil {
				activeCount := 0
				for _, acl := range acls {
					if !acl.Enabled {
						continue
					}
					activeCount++
					allowed, _, err := c.accessSvc.TestIP(acl.ID, clientIP)
					if err == nil && !allowed {
						// Send security notification via appropriate dispatch path
						_ = c.sendSecurityNotification(context.Background(), models.SecurityEvent{
							EventType: "acl_deny",
							Severity:  "warn",
							Message:   "Access control list blocked request",
							ClientIP:  clientIP,
							Path:      ctx.Request.URL.Path,
							Timestamp: time.Now(),
							Metadata: map[string]any{
								"acl_name": acl.Name,
								"acl_id":   acl.ID,
							},
						})

						ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Blocked by access control list"})
						return
					}
				}
				if activeCount == 0 {
					if isAdmin && !adminWhitelistConfigured {
						ctx.Next()
						return
					}
					ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Blocked by access control list"})
					return
				}
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
		}

		ctx.Next()
	}
}

func (c *Cerberus) isAuthenticatedAdmin(ctx *gin.Context) bool {
	role, exists := ctx.Get("role")
	if !exists {
		return false
	}
	roleStr, ok := role.(string)
	if !ok || roleStr != "admin" {
		return false
	}
	userID, exists := ctx.Get("userID")
	if !exists {
		return false
	}
	switch id := userID.(type) {
	case uint:
		return id > 0
	case int:
		return id > 0
	case int64:
		return id > 0
	default:
		return false
	}
}

func (c *Cerberus) adminWhitelistStatus(clientIP string) (bool, bool) {
	if c.db == nil {
		return false, false
	}

	var sc models.SecurityConfig
	if err := c.db.Where("name = ?", "default").First(&sc).Error; err != nil {
		return false, false
	}
	if strings.TrimSpace(sc.AdminWhitelist) == "" {
		return false, false
	}

	return securitypkg.IsIPInCIDRList(clientIP, sc.AdminWhitelist), true
}

// sendSecurityNotification dispatches a security event notification.
// Blocker 1: Wires runtime dispatch to provider-event authority under feature flag semantics.
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

	// Feature flag disabled or not found: use legacy dispatch (fail-closed)
	return c.securityNotifySvc.Send(ctx, event)
}
