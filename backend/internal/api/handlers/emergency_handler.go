package handlers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

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
	tokenService    *services.EmergencyTokenService
	caddyManager    CaddyConfigManager
	cerberus        CacheInvalidator
}

// NewEmergencyHandler creates a new EmergencyHandler
func NewEmergencyHandler(db *gorm.DB) *EmergencyHandler {
	return &EmergencyHandler{
		db:              db,
		securityService: services.NewSecurityService(db),
		tokenService:    services.NewEmergencyTokenService(db),
	}
}

// NewEmergencyHandlerWithDeps creates a new EmergencyHandler with optional cache invalidation and config reload.
func NewEmergencyHandlerWithDeps(db *gorm.DB, caddyManager CaddyConfigManager, cerberus CacheInvalidator) *EmergencyHandler {
	return &EmergencyHandler{
		db:              db,
		securityService: services.NewSecurityService(db),
		tokenService:    services.NewEmergencyTokenService(db),
		caddyManager:    caddyManager,
		cerberus:        cerberus,
	}
}

// NewEmergencyTokenHandler creates a handler for emergency token management endpoints
// This is an alias for NewEmergencyHandler, provided for semantic clarity in route registration
func NewEmergencyTokenHandler(tokenService *services.EmergencyTokenService) *EmergencyHandler {
	return &EmergencyHandler{
		db:              tokenService.DB(),
		securityService: nil, // Not needed for token management endpoints
		tokenService:    tokenService,
	}
}

// Close shuts down the handler's resources (e.g., SecurityService).
func (h *EmergencyHandler) Close() {
	if h.securityService != nil {
		h.securityService.Close()
	}
}

// SecurityReset disables all security modules for emergency lockout recovery.
// This endpoint works in conjunction with the EmergencyBypass middleware which
// validates the token and IP restrictions, then sets the emergency_bypass flag.
//
// Security measures:
// - EmergencyBypass middleware validates token and IP (timing-safe comparison)
// - All attempts (success and failure) are logged to audit trail with timestamp and IP
func (h *EmergencyHandler) SecurityReset(c *gin.Context) {
	clientIP := util.CanonicalizeIPForSecurity(c.ClientIP())
	startTime := time.Now()

	// Check if request has been pre-validated by EmergencyBypass middleware
	bypassActive, exists := c.Get("emergency_bypass")
	if exists && bypassActive.(bool) {
		// Request already validated by middleware - proceed directly to reset
		log.WithFields(log.Fields{
			"ip":     clientIP,
			"action": "emergency_reset_via_middleware",
		}).Debug("Emergency reset validated by middleware")

		// Proceed with security reset
		h.performSecurityReset(c, clientIP, startTime)
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
		h.logEnhancedAudit(clientIP, "emergency_reset_not_configured", "Emergency token not configured", false, time.Since(startTime))
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
		h.logEnhancedAudit(clientIP, "emergency_reset_invalid_config", "Configured token too short", false, time.Since(startTime))
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
		h.logEnhancedAudit(clientIP, "emergency_reset_missing_token", "No token provided in header", false, time.Since(startTime))
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

	// Validate token using service (checks database first, then env var)
	_, err := h.tokenService.Validate(providedToken)
	if err != nil {
		h.logEnhancedAudit(clientIP, "emergency_reset_invalid_token", fmt.Sprintf("Token validation failed: %v", err), false, time.Since(startTime))
		log.WithFields(log.Fields{
			"ip":     clientIP,
			"action": "emergency_reset_invalid_token",
			"error":  err.Error(),
		}).Warn("Emergency reset attempted with invalid token")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "unauthorized",
			"message": "Invalid or expired emergency token.",
		})
		return
	}

	// Token is valid - disable all security modules
	h.performSecurityReset(c, clientIP, startTime)
}

// performSecurityReset executes the actual security module disable operation
func (h *EmergencyHandler) performSecurityReset(c *gin.Context, clientIP string, startTime time.Time) {
	disabledModules, err := h.disableAllSecurityModules()
	if err != nil {
		h.logEnhancedAudit(clientIP, "emergency_reset_failed", fmt.Sprintf("Failed to disable modules: %v", err), false, time.Since(startTime))
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

	h.syncSecurityState(c.Request.Context())

	// Log successful reset
	h.logEnhancedAudit(clientIP, "emergency_reset_success", fmt.Sprintf("Disabled modules: %v", disabledModules), true, time.Since(startTime))
	log.WithFields(log.Fields{
		"ip":               clientIP,
		"action":           "emergency_reset_success",
		"disabled_modules": disabledModules,
		"duration_ms":      time.Since(startTime).Milliseconds(),
	}).Warn("EMERGENCY SECURITY RESET: All security modules disabled")

	c.JSON(http.StatusOK, gin.H{
		"success":          true,
		"message":          "All security modules have been disabled. Please reconfigure security settings.",
		"disabled_modules": disabledModules,
	})
}

// disableAllSecurityModules disables ACL, WAF, Rate Limit, and CrowdSec modules
// while keeping the Cerberus framework enabled for break glass testing.
func (h *EmergencyHandler) disableAllSecurityModules() ([]string, error) {
	disabledModules := []string{}

	// Settings to disable - NOTE: We keep feature.cerberus.enabled = true
	// so E2E tests can validate break glass functionality.
	// Only individual security modules are disabled for clean test state.
	securitySettings := map[string]string{
		// Feature framework stays ENABLED (removed from this map)
		// "feature.cerberus.enabled": "false",  ← BUG FIX: Keep framework enabled
		// "security.cerberus.enabled": "false", ← BUG FIX: Keep framework enabled

		// Individual security modules disabled for clean slate
		"security.acl.enabled":        "false",
		"security.waf.enabled":        "false",
		"security.rate_limit.enabled": "false",
		"security.crowdsec.enabled":   "false",
		"security.crowdsec.mode":      "disabled",
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

	// Clear admin whitelist to prevent bypass persistence after reset
	adminWhitelistSetting := models.Setting{
		Key:      "security.admin_whitelist",
		Value:    "",
		Category: "security",
		Type:     "string",
	}
	if err := h.db.Where(models.Setting{Key: adminWhitelistSetting.Key}).Assign(adminWhitelistSetting).FirstOrCreate(&adminWhitelistSetting).Error; err != nil {
		return disabledModules, fmt.Errorf("failed to clear admin whitelist: %w", err)
	}

	// Also update the SecurityConfig record if it exists
	var securityConfig models.SecurityConfig
	if err := h.db.Where("name = ?", "default").First(&securityConfig).Error; err == nil {
		securityConfig.Enabled = false
		securityConfig.AdminWhitelist = ""
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

// logEnhancedAudit logs an emergency action with enhanced metadata (timestamp, result, duration)
func (h *EmergencyHandler) logEnhancedAudit(actor, action, details string, success bool, duration time.Duration) {
	if h.securityService == nil {
		return
	}

	result := "failure"
	if success {
		result = "success"
	}

	enhancedDetails := fmt.Sprintf("%s | result=%s | duration=%dms | timestamp=%s",
		details,
		result,
		duration.Milliseconds(),
		time.Now().UTC().Format(time.RFC3339))

	audit := &models.SecurityAudit{
		Actor:   actor,
		Action:  action,
		Details: enhancedDetails,
	}

	if err := h.securityService.LogAudit(audit); err != nil {
		log.WithError(err).Error("Failed to log emergency audit event")
	}
}

func (h *EmergencyHandler) syncSecurityState(ctx context.Context) {
	if h.cerberus != nil {
		h.cerberus.InvalidateCache()
	}
	if h.caddyManager == nil {
		return
	}

	applyCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := h.caddyManager.ApplyConfig(applyCtx); err != nil {
		log.WithError(err).Warn("Failed to reload Caddy config after emergency reset")
	}
}

// GenerateToken generates a new emergency token with expiration policy
// POST /api/v1/emergency/token/generate
// Requires admin authentication
func (h *EmergencyHandler) GenerateToken(c *gin.Context) {
	// Check admin role
	role, exists := c.Get("role")
	if !exists || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	// Get user ID from context
	userID, _ := c.Get("userID")
	var userIDPtr *uint
	if id, ok := userID.(uint); ok {
		userIDPtr = &id
	}

	// Parse request body
	type GenerateTokenRequest struct {
		ExpirationDays int `json:"expiration_days"` // 0 = never, 30/60/90 = preset, 1-365 = custom
	}

	var req GenerateTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate expiration days
	if req.ExpirationDays < 0 || req.ExpirationDays > 365 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Expiration days must be between 0 and 365"})
		return
	}

	// Generate token
	response, err := h.tokenService.Generate(services.GenerateRequest{
		ExpirationDays: req.ExpirationDays,
		UserID:         userIDPtr,
	})
	if err != nil {
		log.WithError(err).Error("Failed to generate emergency token")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Audit log
	clientIP := util.CanonicalizeIPForSecurity(c.ClientIP())
	h.logAudit(clientIP, "emergency_token_generated", fmt.Sprintf("Policy: %s, Expires: %v", response.ExpirationPolicy, response.ExpiresAt))

	c.JSON(http.StatusOK, response)
}

// GetTokenStatus returns token metadata (not the token itself)
// GET /api/v1/emergency/token/status
// Requires admin authentication
func (h *EmergencyHandler) GetTokenStatus(c *gin.Context) {
	// Check admin role
	role, exists := c.Get("role")
	if !exists || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	status, err := h.tokenService.GetStatus()
	if err != nil {
		log.WithError(err).Error("Failed to get token status")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get token status"})
		return
	}

	c.JSON(http.StatusOK, status)
}

// RevokeToken revokes the current emergency token
// DELETE /api/v1/emergency/token
// Requires admin authentication
func (h *EmergencyHandler) RevokeToken(c *gin.Context) {
	// Check admin role
	role, exists := c.Get("role")
	if !exists || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	if err := h.tokenService.Revoke(); err != nil {
		log.WithError(err).Error("Failed to revoke emergency token")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Audit log
	clientIP := util.CanonicalizeIPForSecurity(c.ClientIP())
	h.logAudit(clientIP, "emergency_token_revoked", "Token revoked by admin")

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Emergency token revoked",
	})
}

// UpdateTokenExpiration updates the expiration policy for the current token
// PATCH /api/v1/emergency/token/expiration
// Requires admin authentication
func (h *EmergencyHandler) UpdateTokenExpiration(c *gin.Context) {
	// Check admin role
	role, exists := c.Get("role")
	if !exists || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	// Parse request body
	type UpdateExpirationRequest struct {
		ExpirationDays int `json:"expiration_days"` // 0 = never, 30/60/90 = preset, 1-365 = custom
	}

	var req UpdateExpirationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate expiration days
	if req.ExpirationDays < 0 || req.ExpirationDays > 365 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Expiration days must be between 0 and 365"})
		return
	}

	// Update expiration
	expiresAt, err := h.tokenService.UpdateExpiration(req.ExpirationDays)
	if err != nil {
		log.WithError(err).Error("Failed to update token expiration")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Audit log
	clientIP := util.CanonicalizeIPForSecurity(c.ClientIP())
	h.logAudit(clientIP, "emergency_token_expiration_updated", fmt.Sprintf("New expiration: %v", expiresAt))

	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"new_expires_at": expiresAt,
	})
}
