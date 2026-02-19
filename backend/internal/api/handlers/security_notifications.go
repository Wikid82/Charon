package handlers

import (
	"fmt"
	"net/http"
	"net/mail"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/security"
	"github.com/Wikid82/charon/backend/internal/services"
)

// SecurityNotificationServiceInterface defines the interface for security notification service.
type SecurityNotificationServiceInterface interface {
	GetSettings() (*models.NotificationConfig, error)
	UpdateSettings(*models.NotificationConfig) error
}

// SecurityNotificationHandler handles notification settings endpoints.
type SecurityNotificationHandler struct {
	service         SecurityNotificationServiceInterface
	securityService *services.SecurityService
	dataRoot        string
}

// NewSecurityNotificationHandler creates a new handler instance.
func NewSecurityNotificationHandler(service SecurityNotificationServiceInterface) *SecurityNotificationHandler {
	return NewSecurityNotificationHandlerWithDeps(service, nil, "")
}

func NewSecurityNotificationHandlerWithDeps(service SecurityNotificationServiceInterface, securityService *services.SecurityService, dataRoot string) *SecurityNotificationHandler {
	return &SecurityNotificationHandler{service: service, securityService: securityService, dataRoot: dataRoot}
}

// GetSettings retrieves the current notification settings.
func (h *SecurityNotificationHandler) GetSettings(c *gin.Context) {
	settings, err := h.service.GetSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve settings"})
		return
	}
	c.JSON(http.StatusOK, settings)
}

func (h *SecurityNotificationHandler) DeprecatedGetSettings(c *gin.Context) {
	c.Header("X-Charon-Deprecated", "true")
	c.Header("X-Charon-Canonical-Endpoint", "/api/v1/notifications/settings/security")
	h.GetSettings(c)
}

// UpdateSettings updates the notification settings.
func (h *SecurityNotificationHandler) UpdateSettings(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	var config models.NotificationConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Validate min_log_level
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if config.MinLogLevel != "" && !validLevels[config.MinLogLevel] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid min_log_level. Must be one of: debug, info, warn, error"})
		return
	}

	// CRITICAL FIX: Validate webhook URL immediately (fail-fast principle)
	// This prevents invalid/malicious URLs from being saved to the database
	if config.WebhookURL != "" {
		if _, err := security.ValidateExternalURL(config.WebhookURL,
			security.WithAllowLocalhost(),
			security.WithAllowHTTP(),
		); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("Invalid webhook URL: %v", err),
				"help":  "URL must be publicly accessible and cannot point to private networks or cloud metadata endpoints",
			})
			return
		}
	}

	if normalized, err := normalizeEmailRecipients(config.EmailRecipients); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	} else {
		config.EmailRecipients = normalized
	}

	if err := h.service.UpdateSettings(&config); err != nil {
		if respondPermissionError(c, h.securityService, "security_notifications_save_failed", err, h.dataRoot) {
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update settings"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Settings updated successfully"})
}

func (h *SecurityNotificationHandler) DeprecatedUpdateSettings(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	c.Header("X-Charon-Deprecated", "true")
	c.Header("X-Charon-Canonical-Endpoint", "/api/v1/notifications/settings/security")
	c.JSON(http.StatusGone, gin.H{
		"error":              "This endpoint is deprecated and no longer accepts updates",
		"canonical_endpoint": "/api/v1/notifications/settings/security",
	})
}

func normalizeEmailRecipients(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", nil
	}

	parts := strings.Split(trimmed, ",")
	valid := make([]string, 0, len(parts))
	invalid := make([]string, 0)
	for _, part := range parts {
		candidate := strings.TrimSpace(part)
		if candidate == "" {
			continue
		}
		if _, err := mail.ParseAddress(candidate); err != nil {
			invalid = append(invalid, candidate)
			continue
		}
		valid = append(valid, candidate)
	}

	if len(invalid) > 0 {
		return "", fmt.Errorf("invalid email recipients: %s", strings.Join(invalid, ", "))
	}

	return strings.Join(valid, ", "), nil
}
