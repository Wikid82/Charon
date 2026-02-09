package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/security"
)

// SecurityNotificationServiceInterface defines the interface for security notification service.
type SecurityNotificationServiceInterface interface {
	GetSettings() (*models.NotificationConfig, error)
	UpdateSettings(*models.NotificationConfig) error
}

// SecurityNotificationHandler handles notification settings endpoints.
type SecurityNotificationHandler struct {
	service SecurityNotificationServiceInterface
}

// NewSecurityNotificationHandler creates a new handler instance.
func NewSecurityNotificationHandler(service SecurityNotificationServiceInterface) *SecurityNotificationHandler {
	return &SecurityNotificationHandler{service: service}
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

// UpdateSettings updates the notification settings.
func (h *SecurityNotificationHandler) UpdateSettings(c *gin.Context) {
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

	if err := h.service.UpdateSettings(&config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update settings"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Settings updated successfully"})
}
