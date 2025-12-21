// Package cerberus provides lightweight security checks (WAF, ACL, CrowdSec) with notification support.
package cerberus

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/metrics"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// Cerberus provides a lightweight facade for security checks (WAF, CrowdSec, ACL).
type Cerberus struct {
	cfg               config.SecurityConfig
	db                *gorm.DB
	accessSvc         *services.AccessListService
	securityNotifySvc *services.SecurityNotificationService
}

// New creates a new Cerberus instance
func New(cfg config.SecurityConfig, db *gorm.DB) *Cerberus {
	return &Cerberus{
		cfg:               cfg,
		db:                db,
		accessSvc:         services.NewAccessListService(db),
		securityNotifySvc: services.NewSecurityNotificationService(db),
	}
}

// IsEnabled returns whether Cerberus features are enabled via config or settings.
func (c *Cerberus) IsEnabled() bool {
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

	// Check database setting (runtime toggle) only if db is provided
	if c.db != nil {
		var s models.Setting
		// Check feature flag
		if err := c.db.Where("key = ?", "feature.cerberus.enabled").First(&s).Error; err == nil {
			return strings.EqualFold(s.Value, "true")
		}
		// Fallback to legacy setting for backward compatibility
		if err := c.db.Where("key = ?", "security.cerberus.enabled").First(&s).Error; err == nil {
			return strings.EqualFold(s.Value, "true")
		}
	}

	// Default to true (Optional Features spec)
	return true
}

// Middleware returns a Gin middleware that enforces Cerberus checks when enabled.
func (c *Cerberus) Middleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if !c.IsEnabled() {
			ctx.Next()
			return
		}

		// WAF: The actual WAF protection is handled by the Coraza plugin at the Caddy layer.
		// This middleware just tracks metrics for requests when WAF is enabled.
		// The naive <script> check has been removed as it's trivially bypassed and
		// proper WAF protection is now provided by Coraza at the reverse proxy level.
		if c.cfg.WAFMode != "" && c.cfg.WAFMode != "disabled" {
			metrics.IncWAFRequest()
			// Note: Actual blocking is done by Coraza in Caddy. This middleware
			// provides defense-in-depth tracking and ACL enforcement only.
		}

		// ACL: simple per-request evaluation against all access lists if enabled
		if c.cfg.ACLMode == "enabled" {
			acls, err := c.accessSvc.List()
			if err == nil {
				clientIP := ctx.ClientIP()
				for _, acl := range acls {
					if !acl.Enabled {
						continue
					}
					allowed, _, err := c.accessSvc.TestIP(acl.ID, clientIP)
					if err == nil && !allowed {
						// Send security notification
						_ = c.securityNotifySvc.Send(context.Background(), models.SecurityEvent{
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
			logger.Log().WithField("client_ip", ctx.ClientIP()).WithField("path", ctx.Request.URL.Path).Debug("Request evaluated by CrowdSec bouncer at Caddy layer")
		}

		// Rate limiting placeholder (no-op for the moment)

		ctx.Next()
	}
}
