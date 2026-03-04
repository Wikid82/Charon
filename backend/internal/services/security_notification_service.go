package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/network"
	"github.com/Wikid82/charon/backend/internal/security"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// SecurityNotificationService handles dispatching security event notifications.
type SecurityNotificationService struct {
	db *gorm.DB
}

// NewSecurityNotificationService creates a new SecurityNotificationService instance.
func NewSecurityNotificationService(db *gorm.DB) *SecurityNotificationService {
	return &SecurityNotificationService{db: db}
}

// GetSettings retrieves the notification configuration.
func (s *SecurityNotificationService) GetSettings() (*models.NotificationConfig, error) {
	var config models.NotificationConfig
	err := s.db.First(&config).Error
	if err == gorm.ErrRecordNotFound {
		// Return default config if none exists
		return &models.NotificationConfig{
			Enabled:             false,
			MinLogLevel:         "error",
			NotifyWAFBlocks:     true,
			NotifyACLDenies:     true,
			NotifyRateLimitHits: true,
			EmailRecipients:     "",
		}, nil
	}
	return &config, err
}

// UpdateSettings updates the notification configuration.
func (s *SecurityNotificationService) UpdateSettings(config *models.NotificationConfig) error {
	var existing models.NotificationConfig
	err := s.db.First(&existing).Error

	if err == gorm.ErrRecordNotFound {
		// Create new config
		return s.db.Create(config).Error
	}

	if err != nil {
		return fmt.Errorf("fetch existing config: %w", err)
	}

	// Update existing config
	config.ID = existing.ID
	return s.db.Save(config).Error
}

// Send dispatches a security event to configured channels.
func (s *SecurityNotificationService) Send(ctx context.Context, event models.SecurityEvent) error {
	config, err := s.GetSettings()
	if err != nil {
		return fmt.Errorf("get settings: %w", err)
	}

	if !config.Enabled {
		return nil
	}

	// Check if event type should be notified
	if event.EventType == "waf_block" && !config.NotifyWAFBlocks {
		return nil
	}
	if event.EventType == "acl_deny" && !config.NotifyACLDenies {
		return nil
	}

	// Check severity against minimum log level
	if !shouldNotify(event.Severity, config.MinLogLevel) {
		return nil
	}

	// Dispatch to webhook if configured
	if config.WebhookURL != "" {
		if err := s.sendWebhook(ctx, config.WebhookURL, event); err != nil {
			logger.Log().WithError(err).Error("Failed to send webhook notification")
			return fmt.Errorf("send webhook: %w", err)
		}
	}

	return nil
}

// sendWebhook sends the event to a webhook URL.
func (s *SecurityNotificationService) sendWebhook(ctx context.Context, webhookURL string, event models.SecurityEvent) error {
	// CRITICAL FIX: Validate webhook URL before making request (SSRF protection)
	validatedURL, err := security.ValidateExternalURL(webhookURL,
		security.WithAllowLocalhost(), // Allow localhost for testing
		security.WithAllowHTTP(),      // Some webhooks use HTTP
	)
	if err != nil {
		// Log SSRF attempt with high severity
		logger.Log().WithFields(logrus.Fields{
			"url":        webhookURL,
			"error":      err.Error(),
			"event_type": "ssrf_blocked",
			"severity":   "HIGH",
		}).Warn("Blocked SSRF attempt in security notification webhook")

		return fmt.Errorf("invalid webhook URL: %w", err)
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", validatedURL, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Charon-Cerberus/1.0")

	// Use SSRF-safe HTTP client for defense-in-depth
	client := network.NewSafeHTTPClient(
		network.WithTimeout(10*time.Second),
		network.WithAllowLocalhost(), // Allow localhost for testing
	)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Log().WithError(err).Warn("Failed to close response body")
		}
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// shouldNotify determines if an event should trigger a notification based on severity.
func shouldNotify(eventSeverity, minLevel string) bool {
	levels := map[string]int{
		"debug": 0,
		"info":  1,
		"warn":  2,
		"error": 3,
	}

	eventLevel := levels[eventSeverity]
	minLevelValue := levels[minLevel]

	return eventLevel >= minLevelValue
}
