package handlers

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/internal/util"
)

const (
	// EmergencyTokenEnvVar is the environment variable name for the emergency token
	EmergencyTokenEnvVar = "CHARON_EMERGENCY_TOKEN"

	// EmergencyTokenHeader is the HTTP header name for the emergency token
	EmergencyTokenHeader = "X-Emergency-Token"

	// MinTokenLength is the minimum required length for the emergency token
	MinTokenLength = 32
)

// EmergencyHandler handles emergency security reset operations
type EmergencyHandler struct {
	db              *gorm.DB
	securityService *services.SecurityService
}

// NewEmergencyHandler creates a new EmergencyHandler
func NewEmergencyHandler(db *gorm.DB) *EmergencyHandler {
	return &EmergencyHandler{
		db:              db,
		securityService: services.NewSecurityService(db),
	}
}

// SecurityReset disables all security modules for emergency lockout recovery.
// This endpoint works in conjunction with the EmergencyBypass middleware which
// validates the token and IP restrictions, then sets the emergency_bypass flag.
//
// Security measures:
// - EmergencyBypass middleware validates token and IP (timing-safe comparison)
// - No rate limiting (break-glass mechanism must work when normal APIs are blocked)
// - All attempts (success and failure) are logged to audit trail
func (h *EmergencyHandler) SecurityReset(c *gin.Context) {
	clientIP := util.CanonicalizeIPForSecurity(c.ClientIP())

	// Check if request has been pre-validated by EmergencyBypass middleware
	bypassActive, exists := c.Get("emergency_bypass")
	if exists && bypassActive.(bool) {
		// Request already validated by middleware - proceed directly to reset
		log.WithFields(log.Fields{
			"ip":     clientIP,
			"action": "emergency_reset_via_middleware",
		}).Debug("Emergency reset validated by middleware")

		// Proceed with security reset
		h.performSecurityReset(c, clientIP)
		return
	}

	// Fallback: Legacy direct token validation (deprecated - use middleware)
	// This path is kept for backward compatibility but will be removed in future versions
	log.WithFields(log.Fields{
		"ip":     clientIP,
		"action": "emergency_reset_legacy_path",
	}).Debug("Emergency reset using legacy direct validation")

	// Check if emergency token is configured
	configuredToken := os.Getenv(EmergencyTokenEnvVar)
	if configuredToken == "" {
		h.logAudit(clientIP, "emergency_reset_not_configured", "Emergency token not configured")
		log.WithFields(log.Fields{
			"ip":     clientIP,
			"action": "emergency_reset_not_configured",
		}).Warn("Emergency reset attempted but token not configured")

		c.JSON(http.StatusNotImplemented, gin.H{
			"error":   "not configured",
			"message": "Emergency reset is not configured. Set CHARON_EMERGENCY_TOKEN environment variable.",
		})
		return
	}

	// Validate token length
	if len(configuredToken) < MinTokenLength {
		h.logAudit(clientIP, "emergency_reset_invalid_config", "Configured token too short")
		log.WithFields(log.Fields{
			"ip":     clientIP,
			"action": "emergency_reset_invalid_config",
		}).Error("Emergency token configured but too short")

		c.JSON(http.StatusNotImplemented, gin.H{
			"error":   "not configured",
			"message": "Emergency token is configured but does not meet minimum length requirements.",
		})
		return
	}

	// Get token from header
	providedToken := c.GetHeader(EmergencyTokenHeader)
	if providedToken == "" {
		// No rate limiting on emergency endpoint - this is a "break-glass" mechanism
		// that must work when normal APIs are blocked. Security is provided by:
		// - Strong token requirement (32+ chars minimum)
		// - IP restrictions (ManagementCIDRs)
		// - Constant-time token comparison (timing attack protection)
		// - Comprehensive audit logging

		h.logAudit(clientIP, "emergency_reset_missing_token", "No token provided in header")
		log.WithFields(log.Fields{
			"ip":     clientIP,
			"action": "emergency_reset_missing_token",
		}).Warn("Emergency reset attempted without token")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "unauthorized",
			"message": "Emergency token required in X-Emergency-Token header.",
		})
		return
	}

	// Timing-safe token comparison to prevent timing attacks
	if !constantTimeCompare(configuredToken, providedToken) {
		h.logAudit(clientIP, "emergency_reset_invalid_token", "Invalid token provided")
		log.WithFields(log.Fields{
			"ip":     clientIP,
			"action": "emergency_reset_invalid_token",
		}).Warn("Emergency reset attempted with invalid token")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "unauthorized",
			"message": "Invalid emergency token.",
		})
		return
	}

	// Token is valid - disable all security modules
	h.performSecurityReset(c, clientIP)
}

// performSecurityReset executes the actual security module disable operation
func (h *EmergencyHandler) performSecurityReset(c *gin.Context, clientIP string) {
	disabledModules, err := h.disableAllSecurityModules()
	if err != nil {
		h.logAudit(clientIP, "emergency_reset_failed", fmt.Sprintf("Failed to disable modules: %v", err))
		log.WithFields(log.Fields{
			"ip":     clientIP,
			"action": "emergency_reset_failed",
			"error":  err.Error(),
		}).Error("Emergency reset failed to disable security modules")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal error",
			"message": "Failed to disable security modules. Check server logs.",
		})
		return
	}

	// Log successful reset
	h.logAudit(clientIP, "emergency_reset_success", fmt.Sprintf("Disabled modules: %v", disabledModules))
	log.WithFields(log.Fields{
		"ip":               clientIP,
		"action":           "emergency_reset_success",
		"disabled_modules": disabledModules,
	}).Warn("EMERGENCY SECURITY RESET: All security modules disabled")

	c.JSON(http.StatusOK, gin.H{
		"success":          true,
		"message":          "All security modules have been disabled. Please reconfigure security settings.",
		"disabled_modules": disabledModules,
	})
}

// disableAllSecurityModules disables Cerberus, ACL, WAF, Rate Limit, and CrowdSec
func (h *EmergencyHandler) disableAllSecurityModules() ([]string, error) {
	disabledModules := []string{}

	// Settings to disable
	securitySettings := map[string]string{
		"feature.cerberus.enabled":    "false",
		"security.acl.enabled":        "false",
		"security.waf.enabled":        "false",
		"security.rate_limit.enabled": "false",
		"security.crowdsec.enabled":   "false",
	}

	// Disable each module via settings
	for key, value := range securitySettings {
		setting := models.Setting{
			Key:      key,
			Value:    value,
			Category: "security",
			Type:     "bool",
		}

		if err := h.db.Where(models.Setting{Key: key}).Assign(setting).FirstOrCreate(&setting).Error; err != nil {
			return disabledModules, fmt.Errorf("failed to disable %s: %w", key, err)
		}
		disabledModules = append(disabledModules, key)
	}

	// Also update the SecurityConfig record if it exists
	var securityConfig models.SecurityConfig
	if err := h.db.Where("name = ?", "default").First(&securityConfig).Error; err == nil {
		securityConfig.Enabled = false
		securityConfig.WAFMode = "disabled"
		securityConfig.RateLimitMode = "disabled"
		securityConfig.RateLimitEnable = false
		securityConfig.CrowdSecMode = "disabled"

		if err := h.db.Save(&securityConfig).Error; err != nil {
			log.WithError(err).Warn("Failed to update SecurityConfig record during emergency reset")
		}
	}

	return disabledModules, nil
}

// logAudit logs an emergency action to the security audit trail
func (h *EmergencyHandler) logAudit(actor, action, details string) {
	if h.securityService == nil {
		return
	}

	audit := &models.SecurityAudit{
		Actor:   actor,
		Action:  action,
		Details: details,
	}

	if err := h.securityService.LogAudit(audit); err != nil {
		log.WithError(err).Error("Failed to log emergency audit event")
	}
}

// constantTimeCompare performs a timing-safe string comparison
func constantTimeCompare(a, b string) bool {
	// Use crypto/subtle for timing-safe comparison
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
