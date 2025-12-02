package cerberus

import (
	"net/http"
	"strings"

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
	cfg       config.SecurityConfig
	db        *gorm.DB
	accessSvc *services.AccessListService
}

// New creates a new Cerberus instance
func New(cfg config.SecurityConfig, db *gorm.DB) *Cerberus {
	return &Cerberus{
		cfg:       cfg,
		db:        db,
		accessSvc: services.NewAccessListService(db),
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
		if err := c.db.Where("key = ?", "security.cerberus.enabled").First(&s).Error; err == nil {
			return strings.EqualFold(s.Value, "true")
		}
	}

	return false
}

// Middleware returns a Gin middleware that enforces Cerberus checks when enabled.
func (c *Cerberus) Middleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if !c.IsEnabled() {
			ctx.Next()
			return
		}

		// WAF: naive example check - block requests containing <script> in URL
		if c.cfg.WAFMode != "" && c.cfg.WAFMode != "disabled" {
			metrics.IncWAFRequest()
			if strings.Contains(ctx.Request.RequestURI, "<script>") {
				logger.Log().WithFields(map[string]interface{}{
					"source":   "waf",
					"decision": "block",
					"mode":     c.cfg.WAFMode,
					"path":     ctx.Request.URL.Path,
					"query":    ctx.Request.URL.RawQuery,
				}).Warn("WAF blocked request")
				metrics.IncWAFBlocked()
				ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "WAF: suspicious payload detected"})
				return
			}
			// Monitoring mode logs but does not block
			if c.cfg.WAFMode == "monitor" {
				logger.Log().WithFields(map[string]interface{}{
					"source":   "waf",
					"decision": "monitor",
					"mode":     c.cfg.WAFMode,
					"path":     ctx.Request.URL.Path,
					"query":    ctx.Request.URL.RawQuery,
				}).Info("WAF monitored request")
				metrics.IncWAFMonitored()
			}
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
						ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Blocked by access control list"})
						return
					}
				}
			}
		}

		// CrowdSec placeholder: integration would check CrowdSec API and apply blocks
		// (no-op for the moment)

		// Rate limiting placeholder (no-op for the moment)

		ctx.Next()
	}
}
