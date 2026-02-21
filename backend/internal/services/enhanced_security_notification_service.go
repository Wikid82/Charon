package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/security"
	"github.com/Wikid82/charon/backend/internal/util"
	"gorm.io/gorm"
)

// EnhancedSecurityNotificationService provides provider-based security notifications with compatibility layer.
type EnhancedSecurityNotificationService struct {
	db *gorm.DB
}

// NewEnhancedSecurityNotificationService creates a new enhanced service instance.
func NewEnhancedSecurityNotificationService(db *gorm.DB) *EnhancedSecurityNotificationService {
	return &EnhancedSecurityNotificationService{db: db}
}

// CompatibilitySettings represents the compatibility GET/PUT structure.
type CompatibilitySettings struct {
	SecurityWAFEnabled       bool   `json:"security_waf_enabled"`
	SecurityACLEnabled       bool   `json:"security_acl_enabled"`
	SecurityRateLimitEnabled bool   `json:"security_rate_limit_enabled"`
	Destination              string `json:"destination,omitempty"`
	DestinationAmbiguous     bool   `json:"destination_ambiguous,omitempty"`
	WebhookURL               string `json:"webhook_url,omitempty"`
	DiscordWebhookURL        string `json:"discord_webhook_url,omitempty"`
	SlackWebhookURL          string `json:"slack_webhook_url,omitempty"`
	GotifyURL                string `json:"gotify_url,omitempty"`
	GotifyToken              string `json:"-"` // Security: Never expose token in JSON (OWASP A02)
}

// MigrationMarker represents the migration state stored in settings table.
type MigrationMarker struct {
	Version         string `json:"version"`
	Checksum        string `json:"checksum"`
	LastCompletedAt string `json:"last_completed_at"`
	Result          string `json:"result"` // completed | completed_with_warnings
}

// GetSettings retrieves compatibility settings via provider aggregation (Spec Section 2).
func (s *EnhancedSecurityNotificationService) GetSettings() (*models.NotificationConfig, error) {
	// Check feature flag
	enabled, err := s.isFeatureEnabled()
	if err != nil {
		return nil, fmt.Errorf("check feature flag: %w", err)
	}

	if !enabled {
		// Feature disabled: return legacy config
		return s.getLegacyConfig()
	}

	// Feature enabled: aggregate from providers
	return s.getProviderAggregatedConfig()
}

// getProviderAggregatedConfig aggregates settings from active providers using OR semantics.
// Blocker 2: Returns proper compatibility contract with security_* fields.
// Blocker 3: Filters enabled=true AND supported notify-only provider types.
// Note: This is the GET aggregation path, not the dispatch path. All provider types are included
// for configuration visibility. Discord-only enforcement applies to SendViaProviders (dispatch path).
func (s *EnhancedSecurityNotificationService) getProviderAggregatedConfig() (*models.NotificationConfig, error) {
	var providers []models.NotificationProvider
	err := s.db.Where("enabled = ?", true).Find(&providers).Error
	if err != nil {
		return nil, fmt.Errorf("query providers: %w", err)
	}

	// Blocker 3: Filter for supported notify-only provider types (PR-1 scope)
	// All supported types are included in GET aggregation for configuration visibility
	supportedTypes := map[string]bool{
		"webhook": true,
		"discord": true,
		"slack":   true,
		"gotify":  true,
	}
	filteredProviders := []models.NotificationProvider{}
	for _, p := range providers {
		if supportedTypes[p.Type] {
			filteredProviders = append(filteredProviders, p)
		}
	}

	// OR aggregation: if ANY provider has true, result is true
	config := &models.NotificationConfig{
		NotifyWAFBlocks:         false,
		NotifyACLDenies:         false,
		NotifyRateLimitHits:     false,
		NotifyCrowdSecDecisions: false,
	}

	for _, p := range filteredProviders {
		if p.NotifySecurityWAFBlocks {
			config.NotifyWAFBlocks = true
		}
		if p.NotifySecurityACLDenies {
			config.NotifyACLDenies = true
		}
		if p.NotifySecurityRateLimitHits {
			config.NotifyRateLimitHits = true
		}
		if p.NotifySecurityCrowdSecDecisions {
			config.NotifyCrowdSecDecisions = true
		}
	}

	// Destination reporting: only if exactly one managed provider exists
	managedProviders := []models.NotificationProvider{}
	for _, p := range filteredProviders {
		if p.ManagedLegacySecurity {
			managedProviders = append(managedProviders, p)
		}
	}

	if len(managedProviders) == 1 {
		// Exactly one managed provider - report destination based on type
		p := managedProviders[0]
		switch p.Type {
		case "webhook":
			config.WebhookURL = p.URL
		case "discord":
			config.DiscordWebhookURL = p.URL
		case "slack":
			config.SlackWebhookURL = p.URL
		case "gotify":
			config.GotifyURL = p.URL
			// Blocker 2: Never expose gotify token in compatibility GET responses
			// Token remains in DB but is not returned to client
		}
		config.DestinationAmbiguous = false
	} else {
		// Zero or multiple managed providers = ambiguous
		config.DestinationAmbiguous = true
	}

	return config, nil
}

// getLegacyConfig retrieves settings from the legacy notification_configs table.
func (s *EnhancedSecurityNotificationService) getLegacyConfig() (*models.NotificationConfig, error) {
	var config models.NotificationConfig
	err := s.db.First(&config).Error
	if err == gorm.ErrRecordNotFound {
		return &models.NotificationConfig{
			NotifyWAFBlocks:     true,
			NotifyACLDenies:     true,
			NotifyRateLimitHits: true,
		}, nil
	}
	return &config, err
}

// UpdateSettings updates security notification settings via managed provider set (Spec Section 3).
func (s *EnhancedSecurityNotificationService) UpdateSettings(req *models.NotificationConfig) error {
	// Check feature flag
	enabled, err := s.isFeatureEnabled()
	if err != nil {
		return fmt.Errorf("check feature flag: %w", err)
	}

	if !enabled {
		// Feature disabled: update legacy config
		return s.updateLegacyConfig(req)
	}

	// Feature enabled: update via managed provider set
	return s.updateManagedProviders(req)
}

// updateManagedProviders updates the managed provider set with replace semantics.
// Blocker 4: Complete gotify validation - requires both URL and token, reject incomplete with 422.
func (s *EnhancedSecurityNotificationService) updateManagedProviders(req *models.NotificationConfig) error {
	// Validate destination mapping (Spec Section 5: fail-safe handling)
	destCount := 0
	var destType string

	if req.WebhookURL != "" {
		destCount++
		destType = "webhook"
	}
	if req.DiscordWebhookURL != "" {
		destCount++
		destType = "discord"
	}
	if req.SlackWebhookURL != "" {
		destCount++
		destType = "slack"
	}
	// Blocker 4: Validate gotify requires BOTH url and token
	if req.GotifyURL != "" || req.GotifyToken != "" {
		destCount++
		destType = "gotify"
		// Reject incomplete gotify payload with 422 and no mutation
		if req.GotifyURL == "" || req.GotifyToken == "" {
			return fmt.Errorf("incomplete gotify configuration: both gotify_url and gotify_token are required")
		}
	}

	if destCount > 1 {
		return fmt.Errorf("ambiguous destination: multiple destination types provided")
	}

	// Resolve deterministic target set (Spec Section 3: deterministic conflict behavior)
	return s.db.Transaction(func(tx *gorm.DB) error {
		var managedProviders []models.NotificationProvider
		err := tx.Where("managed_legacy_security = ?", true).Find(&managedProviders).Error
		if err != nil {
			return fmt.Errorf("query managed providers: %w", err)
		}

		// Blocker 4: Deterministic target set allows one-or-more managed providers
		// Update full managed set; only 409 on true non-resolvable identity corruption
		// Multiple managed providers ARE the valid target set (not corruption)

		if len(managedProviders) == 0 {
			// Create managed provider
			provider := &models.NotificationProvider{
				Name:                        "Migrated Security Notifications (Legacy)",
				Type:                        destType,
				Enabled:                     true,
				ManagedLegacySecurity:       true,
				NotifySecurityWAFBlocks:     req.NotifyWAFBlocks,
				NotifySecurityACLDenies:     req.NotifyACLDenies,
				NotifySecurityRateLimitHits: req.NotifyRateLimitHits,
				URL:                         s.extractDestinationURL(req),
				Token:                       s.extractDestinationToken(req),
			}
			return tx.Create(provider).Error
		}

		// Blocker 3: Enforce PUT idempotency - only save if values actually changed
		// Update all managed providers with replace semantics
		for i := range managedProviders {
			changed := false

			// Check if security event flags changed
			if managedProviders[i].NotifySecurityWAFBlocks != req.NotifyWAFBlocks {
				managedProviders[i].NotifySecurityWAFBlocks = req.NotifyWAFBlocks
				changed = true
			}
			if managedProviders[i].NotifySecurityACLDenies != req.NotifyACLDenies {
				managedProviders[i].NotifySecurityACLDenies = req.NotifyACLDenies
				changed = true
			}
			if managedProviders[i].NotifySecurityRateLimitHits != req.NotifyRateLimitHits {
				managedProviders[i].NotifySecurityRateLimitHits = req.NotifyRateLimitHits
				changed = true
			}

			// Update destination if provided
			if destURL := s.extractDestinationURL(req); destURL != "" {
				if managedProviders[i].URL != destURL {
					managedProviders[i].URL = destURL
					changed = true
				}
				if managedProviders[i].Type != destType {
					managedProviders[i].Type = destType
					changed = true
				}
				if managedProviders[i].Token != s.extractDestinationToken(req) {
					managedProviders[i].Token = s.extractDestinationToken(req)
					changed = true
				}
			}

			// Blocker 3: Only save (update timestamps) if values actually changed
			if changed {
				if err := tx.Save(&managedProviders[i]).Error; err != nil {
					return fmt.Errorf("update provider %s: %w", managedProviders[i].ID, err)
				}
			}
		}

		return nil
	})
}

// extractDestinationURL extracts the destination URL from the request.
func (s *EnhancedSecurityNotificationService) extractDestinationURL(req *models.NotificationConfig) string {
	if req.WebhookURL != "" {
		return req.WebhookURL
	}
	if req.DiscordWebhookURL != "" {
		return req.DiscordWebhookURL
	}
	if req.SlackWebhookURL != "" {
		return req.SlackWebhookURL
	}
	if req.GotifyURL != "" {
		return req.GotifyURL
	}
	return ""
}

// extractDestinationToken extracts the auth token from the request (currently only gotify).
func (s *EnhancedSecurityNotificationService) extractDestinationToken(req *models.NotificationConfig) string {
	if req.GotifyToken != "" {
		return req.GotifyToken
	}
	return ""
}

// updateLegacyConfig updates the legacy notification_configs table.
func (s *EnhancedSecurityNotificationService) updateLegacyConfig(req *models.NotificationConfig) error {
	var existing models.NotificationConfig
	err := s.db.First(&existing).Error

	if err == gorm.ErrRecordNotFound {
		return s.db.Create(req).Error
	}

	if err != nil {
		return fmt.Errorf("fetch existing config: %w", err)
	}

	req.ID = existing.ID
	return s.db.Save(req).Error
}

// MigrateFromLegacyConfig performs deterministic migration from legacy config to managed provider (Spec Section 4).
// Blocker 2: Respects feature flag - does NOT mutate providers when flag=false.
func (s *EnhancedSecurityNotificationService) MigrateFromLegacyConfig() error {
	// Check feature flag first
	enabled, err := s.isFeatureEnabled()
	if err != nil {
		return fmt.Errorf("check feature flag: %w", err)
	}

	// Read legacy config
	var legacyConfig models.NotificationConfig
	err = s.db.First(&legacyConfig).Error
	if err == gorm.ErrRecordNotFound {
		// No legacy config to migrate
		return nil
	}
	if err != nil {
		return fmt.Errorf("read legacy config: %w", err)
	}

	// Compute checksum
	checksum := computeConfigChecksum(legacyConfig)

	// Read migration marker
	var markerSetting models.Setting
	err = s.db.Where("key = ?", "notifications.security_provider_events.migration.v1").First(&markerSetting).Error

	if err == nil {
		// Marker exists - check if checksum matches
		var marker MigrationMarker
		if err := json.Unmarshal([]byte(markerSetting.Value), &marker); err != nil {
			logger.Log().WithError(err).Warn("Failed to unmarshal migration marker")
		} else if marker.Checksum == checksum {
			// Checksum matches - no-op
			return nil
		}
	}

	// If feature flag is disabled, perform dry-evaluate only (no mutation)
	if !enabled {
		logger.Log().Info("Feature flag disabled - migration runs in read-only mode (no provider mutation)")
		return nil
	}

	// Perform migration in transaction
	return s.db.Transaction(func(tx *gorm.DB) error {
		// Upsert managed provider
		var provider models.NotificationProvider
		err := tx.Where("managed_legacy_security = ?", true).First(&provider).Error

		if err == gorm.ErrRecordNotFound {
			// Create new managed provider
			provider = models.NotificationProvider{
				Name:                        "Migrated Security Notifications (Legacy)",
				Type:                        "webhook",
				Enabled:                     true,
				ManagedLegacySecurity:       true,
				NotifySecurityWAFBlocks:     legacyConfig.NotifyWAFBlocks,
				NotifySecurityACLDenies:     legacyConfig.NotifyACLDenies,
				NotifySecurityRateLimitHits: legacyConfig.NotifyRateLimitHits,
				URL:                         legacyConfig.WebhookURL,
			}
			if err := tx.Create(&provider).Error; err != nil {
				return fmt.Errorf("create managed provider: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("query managed provider: %w", err)
		} else {
			// Update existing managed provider
			provider.NotifySecurityWAFBlocks = legacyConfig.NotifyWAFBlocks
			provider.NotifySecurityACLDenies = legacyConfig.NotifyACLDenies
			provider.NotifySecurityRateLimitHits = legacyConfig.NotifyRateLimitHits
			provider.URL = legacyConfig.WebhookURL
			if err := tx.Save(&provider).Error; err != nil {
				return fmt.Errorf("update managed provider: %w", err)
			}
		}

		// Write migration marker
		marker := MigrationMarker{
			Version:         "v1",
			Checksum:        checksum,
			LastCompletedAt: time.Now().UTC().Format(time.RFC3339),
			Result:          "completed",
		}
		markerJSON, err := json.Marshal(marker)
		if err != nil {
			return fmt.Errorf("marshal marker: %w", err)
		}

		newMarkerSetting := models.Setting{
			Key:      "notifications.security_provider_events.migration.v1",
			Value:    string(markerJSON),
			Type:     "json",
			Category: "notifications",
		}

		// Upsert marker
		if err := tx.Where("key = ?", newMarkerSetting.Key).First(&markerSetting).Error; err == gorm.ErrRecordNotFound {
			return tx.Create(&newMarkerSetting).Error
		}
		newMarkerSetting.ID = markerSetting.ID
		return tx.Save(&newMarkerSetting).Error
	})
}

// computeConfigChecksum computes a deterministic checksum from legacy config fields.
func computeConfigChecksum(config models.NotificationConfig) string {
	// Create deterministic string representation
	fields := []string{
		fmt.Sprintf("waf:%t", config.NotifyWAFBlocks),
		fmt.Sprintf("acl:%t", config.NotifyACLDenies),
		fmt.Sprintf("rate:%t", config.NotifyRateLimitHits),
		fmt.Sprintf("url:%s", config.WebhookURL),
	}
	sort.Strings(fields) // Ensure field order doesn't affect checksum

	data := ""
	for _, f := range fields {
		data += f + "|"
	}

	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// isFeatureEnabled checks the feature flag in settings table (Spec Section 6).
func (s *EnhancedSecurityNotificationService) isFeatureEnabled() (bool, error) {
	var setting models.Setting
	err := s.db.Where("key = ?", "feature.notifications.security_provider_events.enabled").First(&setting).Error

	if err == gorm.ErrRecordNotFound {
		// Blocker 5: Implement feature flag defaults exactly as per spec
		// Initialize based on environment detection
		defaultValue := s.getDefaultFeatureFlagValue()

		// Create the setting with appropriate default
		newSetting := models.Setting{
			Key:      "feature.notifications.security_provider_events.enabled",
			Value:    defaultValue,
			Type:     "bool",
			Category: "feature",
		}

		if createErr := s.db.Create(&newSetting).Error; createErr != nil {
			// If creation fails (e.g., race condition), re-query
			if queryErr := s.db.Where("key = ?", newSetting.Key).First(&setting).Error; queryErr != nil {
				return defaultValue == "true", fmt.Errorf("create and requery feature flag: %w", queryErr)
			}
			return setting.Value == "true", nil
		}

		return defaultValue == "true", nil
	}

	if err != nil {
		return false, fmt.Errorf("query feature flag: %w", err)
	}

	return setting.Value == "true", nil
}

// SendViaProviders dispatches security events to active providers.
// When feature flag is enabled, this is the authoritative dispatch path.
// Blocker 3: Discord-only enforcement for rollout - only Discord providers receive security events.
// Server-side guarantee holds for existing rows and all dispatch paths.
func (s *EnhancedSecurityNotificationService) SendViaProviders(ctx context.Context, event models.SecurityEvent) error {
	// Query active providers that have the relevant event type enabled
	var providers []models.NotificationProvider
	err := s.db.Where("enabled = ?", true).Find(&providers).Error
	if err != nil {
		return fmt.Errorf("query providers: %w", err)
	}

	// Blocker 3: Discord-only enforcement for rollout stage
	// ONLY Discord providers are allowed to receive security events
	// This is a server-side guarantee that prevents any non-Discord provider
	// from receiving security notifications, even if flags are enabled in DB
	supportedTypes := map[string]bool{
		"discord": true,
		// webhook, slack, gotify explicitly excluded for rollout
	}

	// Filter providers based on event type AND supported type
	var targetProviders []models.NotificationProvider
	for _, p := range providers {
		if !supportedTypes[p.Type] {
			continue
		}
		shouldNotify := false
		// Normalize event type to handle variations
		normalizedEventType := normalizeSecurityEventType(event.EventType)
		switch normalizedEventType {
		case "waf_block":
			shouldNotify = p.NotifySecurityWAFBlocks
		case "acl_deny":
			shouldNotify = p.NotifySecurityACLDenies
		case "rate_limit":
			shouldNotify = p.NotifySecurityRateLimitHits
		case "crowdsec_decision":
			shouldNotify = p.NotifySecurityCrowdSecDecisions
		}
		if shouldNotify {
			targetProviders = append(targetProviders, p)
		}
	}

	if len(targetProviders) == 0 {
		// No providers configured for this event type - fail closed (no notification)
		logger.Log().WithField("event_type", util.SanitizeForLog(event.EventType)).Debug("No providers configured for security event")
		return nil
	}

	// Dispatch to all target providers (best-effort, log failures but don't block)
	for _, p := range targetProviders {
		if err := s.dispatchToProvider(ctx, p, event); err != nil {
			logger.Log().WithError(err).WithField("provider_id", p.ID).Error("Failed to dispatch to provider")
			// Continue to next provider (best-effort)
		}
	}

	return nil
}

// dispatchToProvider sends the event to a single provider.
// Discord-only enforcement: rejects all non-Discord providers in this rollout.
func (s *EnhancedSecurityNotificationService) dispatchToProvider(ctx context.Context, provider models.NotificationProvider, event models.SecurityEvent) error {
	// Discord-only enforcement for rollout: reject non-Discord types explicitly
	if provider.Type != "discord" {
		return fmt.Errorf("discord-only rollout: provider type %q is not supported; only discord is enabled", provider.Type)
	}
	// Discord dispatch via webhook
	return s.sendWebhook(ctx, provider.URL, event)
}

// sendWebhook sends a security event to a webhook URL (shared with legacy service).
// Blocker 4: SSRF-safe URL validation before outbound requests.
func (s *EnhancedSecurityNotificationService) sendWebhook(ctx context.Context, webhookURL string, event models.SecurityEvent) error {
	// Blocker 4: Validate URL before making outbound request (SSRF protection)
	validatedURL, err := security.ValidateExternalURL(webhookURL,
		security.WithAllowHTTP(),      // Allow HTTP for backwards compatibility
		security.WithAllowLocalhost(), // Allow localhost for testing
	)
	if err != nil {
		return fmt.Errorf("ssrf validation failed: %w", err)
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

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// normalizeSecurityEventType normalizes event type variations to canonical forms.
func normalizeSecurityEventType(eventType string) string {
	normalized := strings.ToLower(strings.TrimSpace(eventType))

	// Map variations to canonical forms
	switch {
	case strings.Contains(normalized, "waf"):
		return "waf_block"
	case strings.Contains(normalized, "acl"):
		return "acl_deny"
	case strings.Contains(normalized, "rate") && strings.Contains(normalized, "limit"):
		return "rate_limit"
	case strings.Contains(normalized, "crowdsec"):
		return "crowdsec_decision"
	default:
		return normalized
	}
}

// getDefaultFeatureFlagValue returns default based on environment (Spec Section 6).
// Blocker 1: Reliable prod=false, dev/test=true without fragile markers.
// Production detection: CHARON_ENV=production OR (CHARON_ENV unset AND GIN_MODE unset)
func (s *EnhancedSecurityNotificationService) getDefaultFeatureFlagValue() string {
	// Explicit production declaration
	charonEnv := os.Getenv("CHARON_ENV")
	if charonEnv == "production" || charonEnv == "prod" {
		return "false" // Production: default disabled
	}

	// Check if we're in a test environment via test marker (inserted by test setup)
	var testMarker models.Setting
	err := s.db.Where("key = ?", "_test_mode_marker").First(&testMarker).Error
	if err == nil && testMarker.Value == "true" {
		return "true" // Test environment
	}

	// Check GIN_MODE for dev/test detection
	ginMode := os.Getenv("GIN_MODE")
	if ginMode == "debug" || ginMode == "test" {
		return "true" // Development/test
	}

	// Blocker 1 Fix: When both CHARON_ENV and GIN_MODE are unset, assume production
	// Production systems should be explicit with CHARON_ENV=production, but default to safe (disabled)
	if charonEnv == "" && ginMode == "" {
		return "false" // Unset env vars = production default
	}

	// All other cases: enable for dev/test safety
	return "true"
}
