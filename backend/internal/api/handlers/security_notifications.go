package handlers

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/internal/util"
)

// SecurityNotificationServiceInterface defines the interface for security notification service.
type SecurityNotificationServiceInterface interface {
	GetSettings() (*models.NotificationConfig, error)
	UpdateSettings(*models.NotificationConfig) error
	SendViaProviders(ctx context.Context, event models.SecurityEvent) error
}

// SecurityNotificationHandler handles notification settings endpoints.
type SecurityNotificationHandler struct {
	service             SecurityNotificationServiceInterface
	securityService     *services.SecurityService
	dataRoot            string
	notificationService *services.NotificationService
	managementCIDRs     []string
}

// NewSecurityNotificationHandler creates a new handler instance.
func NewSecurityNotificationHandler(service SecurityNotificationServiceInterface) *SecurityNotificationHandler {
	return NewSecurityNotificationHandlerWithDeps(service, nil, "", nil, nil)
}

func NewSecurityNotificationHandlerWithDeps(
	service SecurityNotificationServiceInterface,
	securityService *services.SecurityService,
	dataRoot string,
	notificationService *services.NotificationService,
	managementCIDRs []string,
) *SecurityNotificationHandler {
	return &SecurityNotificationHandler{
		service:             service,
		securityService:     securityService,
		dataRoot:            dataRoot,
		notificationService: notificationService,
		managementCIDRs:     managementCIDRs,
	}
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

// UpdateSettings is deprecated and returns 410 Gone (R6).
// Security settings must now be managed via provider Notification Events.
func (h *SecurityNotificationHandler) UpdateSettings(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	c.JSON(http.StatusGone, gin.H{
		"error":   "legacy_security_settings_deprecated",
		"message": "Use provider Notification Events.",
		"code":    "LEGACY_SECURITY_SETTINGS_DEPRECATED",
	})
}

func (h *SecurityNotificationHandler) DeprecatedUpdateSettings(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	c.JSON(http.StatusGone, gin.H{
		"error":   "legacy_security_settings_deprecated",
		"message": "Use provider Notification Events.",
		"code":    "LEGACY_SECURITY_SETTINGS_DEPRECATED",
	})
}

// HandleSecurityEvent receives runtime security events from Caddy/Cerberus (Blocker 1: Production dispatch path).
// This endpoint is called by Caddy bouncer/middleware when security events occur (WAF blocks, CrowdSec decisions, etc.).
func (h *SecurityNotificationHandler) HandleSecurityEvent(c *gin.Context) {
	// Blocker 2: Source validation - verify request originates from localhost or management CIDRs
	clientIPStr := util.CanonicalizeIPForSecurity(c.ClientIP())
	clientIP := net.ParseIP(clientIPStr)
	if clientIP == nil {
		logger.Log().WithField("ip", util.SanitizeForLog(clientIPStr)).Warn("Security event intake: invalid client IP")
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "invalid_source",
			"message": "Request source could not be validated",
		})
		return
	}

	// Check if IP is localhost (IPv4 or IPv6)
	isLocalhost := clientIP.IsLoopback()

	// Check if IP is in management CIDRs
	isInManagementNetwork := false
	for _, cidrStr := range h.managementCIDRs {
		_, ipnet, err := net.ParseCIDR(cidrStr)
		if err != nil {
			logger.Log().WithError(err).WithField("cidr", util.SanitizeForLog(cidrStr)).Warn("Security event intake: invalid CIDR")
			continue
		}
		if ipnet.Contains(clientIP) {
			isInManagementNetwork = true
			break
		}
	}

	// Reject if not from localhost or management network
	if !isLocalhost && !isInManagementNetwork {
		logger.Log().WithField("ip", util.SanitizeForLog(clientIP.String())).Warn("Security event intake: IP not authorized")
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "unauthorized_source",
			"message": "Request must originate from localhost or management network",
		})
		return
	}

	var event models.SecurityEvent
	if err := c.ShouldBindJSON(&event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid security event payload"})
		return
	}

	// Set timestamp if not provided
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Log the event for audit trail
	logger.Log().WithFields(map[string]interface{}{
		"event_type": util.SanitizeForLog(event.EventType),
		"severity":   util.SanitizeForLog(event.Severity),
		"client_ip":  util.SanitizeForLog(event.ClientIP),
		"path":       util.SanitizeForLog(event.Path),
	}).Info("Security event received")

	c.Set("security_event_type", event.EventType)
	c.Set("security_event_severity", event.Severity)

	// Dispatch through provider-security-event authoritative path
	// This enforces Discord-only rollout guarantee and proper event filtering
	if err := h.service.SendViaProviders(c.Request.Context(), event); err != nil {
		logger.Log().WithError(err).WithField("event_type", util.SanitizeForLog(event.EventType)).Error("Failed to dispatch security event")
		// Continue - dispatch failure shouldn't prevent intake acknowledgment
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message":    "Security event recorded",
		"event_type": event.EventType,
		"timestamp":  event.Timestamp.Format(time.RFC3339),
	})
}
