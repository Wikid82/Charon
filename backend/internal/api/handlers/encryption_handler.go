// Package handlers provides HTTP request handlers for the API.
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
)

// EncryptionHandler manages encryption key operations and rotation.
type EncryptionHandler struct {
	rotationService *crypto.RotationService
	securityService *services.SecurityService
}

// NewEncryptionHandler creates a new encryption handler.
func NewEncryptionHandler(rotationService *crypto.RotationService, securityService *services.SecurityService) *EncryptionHandler {
	return &EncryptionHandler{
		rotationService: rotationService,
		securityService: securityService,
	}
}

// GetStatus returns the current encryption key rotation status.
// GET /api/v1/admin/encryption/status
func (h *EncryptionHandler) GetStatus(c *gin.Context) {
	// Admin-only check (via middleware or direct check)
	if !isAdmin(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
		return
	}

	status, err := h.rotationService.GetStatus()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, status)
}

// Rotate triggers re-encryption of all credentials with the next key.
// POST /api/v1/admin/encryption/rotate
func (h *EncryptionHandler) Rotate(c *gin.Context) {
	// Admin-only check
	if !isAdmin(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
		return
	}

	// Log rotation start
	if err := h.securityService.LogAudit(&models.SecurityAudit{
		Actor:         getActorFromGinContext(c),
		Action:        "encryption_key_rotation_started",
		EventCategory: "encryption",
		Details:       "{}",
		IPAddress:     c.ClientIP(),
		UserAgent:     c.Request.UserAgent(),
	}); err != nil {
		logger.Log().WithError(err).Warn("Failed to log audit event")
	}

	// Perform rotation
	result, err := h.rotationService.RotateAllCredentials(c.Request.Context())
	if err != nil {
		// Log failure
		detailsJSON, _ := json.Marshal(map[string]interface{}{
			"error": err.Error(),
		})
		_ = h.securityService.LogAudit(&models.SecurityAudit{
			Actor:         getActorFromGinContext(c),
			Action:        "encryption_key_rotation_failed",
			EventCategory: "encryption",
			Details:       string(detailsJSON),
			IPAddress:     c.ClientIP(),
			UserAgent:     c.Request.UserAgent(),
		})

		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Log rotation completion
	detailsJSON, _ := json.Marshal(map[string]interface{}{
		"total_providers":  result.TotalProviders,
		"success_count":    result.SuccessCount,
		"failure_count":    result.FailureCount,
		"failed_providers": result.FailedProviders,
		"duration":         result.Duration,
		"new_key_version":  result.NewKeyVersion,
	})
	_ = h.securityService.LogAudit(&models.SecurityAudit{
		Actor:         getActorFromGinContext(c),
		Action:        "encryption_key_rotation_completed",
		EventCategory: "encryption",
		Details:       string(detailsJSON),
		IPAddress:     c.ClientIP(),
		UserAgent:     c.Request.UserAgent(),
	})

	c.JSON(http.StatusOK, result)
}

// GetHistory returns audit logs related to encryption key operations.
// GET /api/v1/admin/encryption/history
func (h *EncryptionHandler) GetHistory(c *gin.Context) {
	// Admin-only check
	if !isAdmin(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
		return
	}

	// Parse pagination parameters
	page := 1
	limit := 50
	if pageParam := c.Query("page"); pageParam != "" {
		if p, err := strconv.Atoi(pageParam); err == nil && p > 0 {
			page = p
		}
	}
	if limitParam := c.Query("limit"); limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	// Query audit logs for encryption category
	filter := services.AuditLogFilter{
		EventCategory: "encryption",
	}

	audits, total, err := h.securityService.ListAuditLogs(filter, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"audits": audits,
		"total":  total,
		"page":   page,
		"limit":  limit,
	})
}

// Validate checks the current encryption key configuration.
// POST /api/v1/admin/encryption/validate
func (h *EncryptionHandler) Validate(c *gin.Context) {
	// Admin-only check
	if !isAdmin(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
		return
	}

	if err := h.rotationService.ValidateKeyConfiguration(); err != nil {
		// Log validation failure
		detailsJSON, _ := json.Marshal(map[string]interface{}{
			"error": err.Error(),
		})
		_ = h.securityService.LogAudit(&models.SecurityAudit{
			Actor:         getActorFromGinContext(c),
			Action:        "encryption_key_validation_failed",
			EventCategory: "encryption",
			Details:       string(detailsJSON),
			IPAddress:     c.ClientIP(),
			UserAgent:     c.Request.UserAgent(),
		})

		c.JSON(http.StatusBadRequest, gin.H{
			"valid": false,
			"error": err.Error(),
		})
		return
	}

	// Log validation success
	_ = h.securityService.LogAudit(&models.SecurityAudit{
		Actor:         getActorFromGinContext(c),
		Action:        "encryption_key_validation_success",
		EventCategory: "encryption",
		Details:       "{}",
		IPAddress:     c.ClientIP(),
		UserAgent:     c.Request.UserAgent(),
	})

	c.JSON(http.StatusOK, gin.H{
		"valid":   true,
		"message": "All encryption keys are valid",
	})
}

// isAdmin checks if the current user has admin privileges.
// This should ideally use the existing auth middleware context.
func isAdmin(c *gin.Context) bool {
	// Check if user is authenticated and is admin
	// Auth middleware sets "role" context key (not "user_role")
	userRole, exists := c.Get("role")
	if !exists {
		return false
	}

	role, ok := userRole.(string)
	if !ok {
		return false
	}

	return role == "admin"
}

// getActorFromGinContext extracts the user ID from Gin context for audit logging.
func getActorFromGinContext(c *gin.Context) string {
	// Auth middleware sets "userID" (not "user_id")
	if userID, exists := c.Get("userID"); exists {
		if id, ok := userID.(uint); ok {
			return strconv.FormatUint(uint64(id), 10)
		}
		if id, ok := userID.(string); ok {
			return id
		}
	}
	return "system"
}
