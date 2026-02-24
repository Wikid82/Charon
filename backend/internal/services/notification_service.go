package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	neturl "net/url"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/network"
	"github.com/Wikid82/charon/backend/internal/notifications"
	"github.com/Wikid82/charon/backend/internal/security"
	"github.com/Wikid82/charon/backend/internal/trace"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/util"
	"gorm.io/gorm"
)

type NotificationService struct {
	DB          *gorm.DB
	httpWrapper *notifications.HTTPWrapper
}

func NewNotificationService(db *gorm.DB) *NotificationService {
	return &NotificationService{
		DB:          db,
		httpWrapper: notifications.NewNotifyHTTPWrapper(),
	}
}

var discordWebhookRegex = regexp.MustCompile(`^https://discord(?:app)?\.com/api/webhooks/(\d+)/([a-zA-Z0-9_-]+)`)

var allowedDiscordWebhookHosts = map[string]struct{}{
	"discord.com":        {},
	"canary.discord.com": {},
}

func normalizeURL(serviceType, rawURL string) string {
	if serviceType == "discord" {
		matches := discordWebhookRegex.FindStringSubmatch(rawURL)
		if len(matches) == 3 {
			id := matches[1]
			token := matches[2]
			return fmt.Sprintf("discord://%s@%s", token, id)
		}
	}
	return rawURL
}

var ErrLegacyFallbackDisabled = errors.New("legacy fallback is retired and disabled")

func legacyFallbackInvocationError(providerType string) error {
	return fmt.Errorf("%w: provider type %q is not supported by notify-only runtime", ErrLegacyFallbackDisabled, providerType)
}

func validateDiscordWebhookURL(rawURL string) error {
	parsedURL, err := neturl.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid Discord webhook URL: failed to parse URL; use the HTTPS webhook URL provided by Discord")
	}

	if strings.EqualFold(parsedURL.Scheme, "discord") {
		return nil
	}

	if !strings.EqualFold(parsedURL.Scheme, "https") {
		return fmt.Errorf("invalid Discord webhook URL: URL must use HTTPS and the hostname URL provided by Discord")
	}

	hostname := strings.ToLower(parsedURL.Hostname())
	if hostname == "" {
		return fmt.Errorf("invalid Discord webhook URL: missing hostname; use the HTTPS webhook URL provided by Discord")
	}

	if net.ParseIP(hostname) != nil {
		return fmt.Errorf("invalid Discord webhook URL: IP address hosts are not allowed; use the hostname URL provided by Discord (discord.com or canary.discord.com)")
	}

	if _, ok := allowedDiscordWebhookHosts[hostname]; !ok {
		return fmt.Errorf("invalid Discord webhook URL: host must be discord.com or canary.discord.com; use the hostname URL provided by Discord")
	}

	return nil
}

func validateDiscordProviderURL(providerType, rawURL string) error {
	if !strings.EqualFold(providerType, "discord") {
		return nil
	}

	return validateDiscordWebhookURL(rawURL)
}

// supportsJSONTemplates returns true if the provider type can use JSON templates
func supportsJSONTemplates(providerType string) bool {
	switch strings.ToLower(providerType) {
	case "webhook", "discord", "gotify", "slack", "generic":
		return true
	default:
		return false
	}
}

func isSupportedNotificationProviderType(providerType string) bool {
	switch strings.ToLower(strings.TrimSpace(providerType)) {
	case "discord", "gotify", "webhook":
		return true
	default:
		return false
	}
}

func (s *NotificationService) isDispatchEnabled(providerType string) bool {
	switch strings.ToLower(strings.TrimSpace(providerType)) {
	case "discord":
		return true
	case "gotify":
		return s.getFeatureFlagValue(notifications.FlagGotifyServiceEnabled, false)
	case "webhook":
		return s.getFeatureFlagValue(notifications.FlagWebhookServiceEnabled, false)
	default:
		return false
	}
}

func (s *NotificationService) getFeatureFlagValue(key string, fallback bool) bool {
	var setting models.Setting
	err := s.DB.Where("key = ?", key).First(&setting).Error
	if err != nil {
		return fallback
	}

	v := strings.ToLower(strings.TrimSpace(setting.Value))
	return v == "1" || v == "true" || v == "yes"
}

// Internal Notifications (DB)

func (s *NotificationService) Create(nType models.NotificationType, title, message string) (*models.Notification, error) {
	notification := &models.Notification{
		Type:    nType,
		Title:   title,
		Message: message,
		Read:    false,
	}
	result := s.DB.Create(notification)
	return notification, result.Error
}

func (s *NotificationService) List(unreadOnly bool) ([]models.Notification, error) {
	var notifications []models.Notification
	query := s.DB.Order("created_at desc")
	if unreadOnly {
		query = query.Where("read = ?", false)
	}
	result := query.Find(&notifications)
	return notifications, result.Error
}

func (s *NotificationService) MarkAsRead(id string) error {
	return s.DB.Model(&models.Notification{}).Where("id = ?", id).Update("read", true).Error
}

func (s *NotificationService) MarkAllAsRead() error {
	return s.DB.Model(&models.Notification{}).Where("read = ?", false).Update("read", true).Error
}

// External Notifications (Custom Webhooks)

func (s *NotificationService) SendExternal(ctx context.Context, eventType, title, message string, data map[string]any) {
	var providers []models.NotificationProvider
	if err := s.DB.Where("enabled = ?", true).Find(&providers).Error; err != nil {
		logger.Log().WithError(err).Error("Failed to fetch notification providers")
		return
	}

	// Prepare data for templates
	if data == nil {
		data = make(map[string]any)
	}
	data["Title"] = title
	data["Message"] = message
	data["Time"] = time.Now().Format(time.RFC3339)
	data["EventType"] = eventType

	for _, provider := range providers {
		// Filter based on preferences
		shouldSend := false
		switch eventType {
		case "proxy_host":
			shouldSend = provider.NotifyProxyHosts
		case "remote_server":
			shouldSend = provider.NotifyRemoteServers
		case "domain":
			shouldSend = provider.NotifyDomains
		case "cert":
			shouldSend = provider.NotifyCerts
		case "uptime":
			shouldSend = provider.NotifyUptime
		case "security_waf":
			shouldSend = provider.NotifySecurityWAFBlocks
		case "security_acl":
			shouldSend = provider.NotifySecurityACLDenies
		case "security_rate_limit":
			shouldSend = provider.NotifySecurityRateLimitHits
		case "security_crowdsec":
			shouldSend = provider.NotifySecurityCrowdSecDecisions
		case "test":
			shouldSend = true
		default:
			// Unknown event types default to false for security
			shouldSend = false
		}

		if !shouldSend {
			continue
		}
		if !s.isDispatchEnabled(provider.Type) {
			logger.Log().WithField("provider", util.SanitizeForLog(provider.Name)).
				WithField("type", provider.Type).
				Warn("Skipping dispatch because provider type is disabled for notify dispatch")
			continue
		}
		go func(p models.NotificationProvider) {
			if !supportsJSONTemplates(p.Type) {
				err := legacyFallbackInvocationError(p.Type)
				logger.Log().WithError(err).WithField("provider", util.SanitizeForLog(p.Name)).Error("Notify-only runtime blocked legacy fallback invocation")
				return
			}

			if err := s.sendJSONPayload(ctx, p, data); err != nil {
				logger.Log().WithError(err).WithField("provider", util.SanitizeForLog(p.Name)).Error("Failed to send JSON notification")
			}
		}(provider)
	}
}

// legacySendFunc is a test hook for outbound sends.
// In notify-only mode this path is retired and always fails closed.
var legacySendFunc = func(_ string, _ string) error {
	return ErrLegacyFallbackDisabled
}

// webhookDoRequestFunc is a test hook for outbound JSON webhook requests.
// In production it defaults to (*http.Client).Do.
var webhookDoRequestFunc = func(client *http.Client, req *http.Request) (*http.Response, error) {
	return client.Do(req)
}

// validateDiscordProviderURLFunc is a test hook for Discord webhook URL validation.
// In tests, you can override this to bypass strict hostname checks for localhost testing.
var validateDiscordProviderURLFunc = validateDiscordProviderURL

func (s *NotificationService) sendJSONPayload(ctx context.Context, p models.NotificationProvider, data map[string]any) error {
	// Built-in templates
	const minimalTemplate = `{"message": {{toJSON .Message}}, "title": {{toJSON .Title}}, "time": {{toJSON .Time}}, "event": {{toJSON .EventType}}}`
	const detailedTemplate = `{"title": {{toJSON .Title}}, "message": {{toJSON .Message}}, "time": {{toJSON .Time}}, "event": {{toJSON .EventType}}, "host": {{toJSON .HostName}}, "host_ip": {{toJSON .HostIP}}, "service_count": {{toJSON .ServiceCount}}, "services": {{toJSON .Services}}, "data": {{toJSON .}}}`

	// Select template based on provider.Template; if 'custom' use Config; else builtin.
	tmplStr := p.Config
	switch strings.ToLower(strings.TrimSpace(p.Template)) {
	case "detailed":
		tmplStr = detailedTemplate
	case "minimal":
		tmplStr = minimalTemplate
	case "custom":
		if tmplStr == "" {
			tmplStr = minimalTemplate
		}
	default:
		if tmplStr == "" {
			tmplStr = minimalTemplate
		}
	}

	// Template size limit validation (10KB max)
	const maxTemplateSize = 10 * 1024
	if len(tmplStr) > maxTemplateSize {
		return fmt.Errorf("template size exceeds maximum limit of %d bytes", maxTemplateSize)
	}

	providerType := strings.ToLower(strings.TrimSpace(p.Type))
	if providerType == "discord" {
		if err := validateDiscordProviderURLFunc(p.Type, p.URL); err != nil {
			return err
		}

		if !isValidRedirectURL(p.URL) {
			return fmt.Errorf("invalid webhook url")
		}
	}

	// Parse template and add helper funcs
	tmpl, err := template.New("webhook").Funcs(template.FuncMap{
		"toJSON": func(v any) string {
			b, _ := json.Marshal(v)
			return string(b)
		},
	}).Parse(tmplStr)
	if err != nil {
		return fmt.Errorf("failed to parse webhook template: %w", err)
	}

	// Template execution with timeout (5 seconds)
	var body bytes.Buffer
	execDone := make(chan error, 1)
	go func() {
		execDone <- tmpl.Execute(&body, data)
	}()

	select {
	case execErr := <-execDone:
		if execErr != nil {
			return fmt.Errorf("failed to execute webhook template: %w", execErr)
		}
	case <-time.After(5 * time.Second):
		return fmt.Errorf("template execution timeout after 5 seconds")
	}

	// Service-specific JSON validation
	var jsonPayload map[string]any
	if unmarshalErr := json.Unmarshal(body.Bytes(), &jsonPayload); unmarshalErr != nil {
		return fmt.Errorf("invalid JSON payload: %w", unmarshalErr)
	}

	// Validate service-specific requirements
	switch strings.ToLower(p.Type) {
	case "discord":
		// Discord requires either 'content' or 'embeds'
		if _, hasContent := jsonPayload["content"]; !hasContent {
			if _, hasEmbeds := jsonPayload["embeds"]; !hasEmbeds {
				if messageValue, hasMessage := jsonPayload["message"]; hasMessage {
					jsonPayload["content"] = messageValue
					normalizedBody, marshalErr := json.Marshal(jsonPayload)
					if marshalErr != nil {
						return fmt.Errorf("failed to normalize discord payload: %w", marshalErr)
					}
					body.Reset()
					if _, writeErr := body.Write(normalizedBody); writeErr != nil {
						return fmt.Errorf("failed to write normalized discord payload: %w", writeErr)
					}
				} else {
					return fmt.Errorf("discord payload requires 'content' or 'embeds' field")
				}
			}
		}
	case "slack":
		// Slack requires either 'text' or 'blocks'
		if _, hasText := jsonPayload["text"]; !hasText {
			if _, hasBlocks := jsonPayload["blocks"]; !hasBlocks {
				return fmt.Errorf("slack payload requires 'text' or 'blocks' field")
			}
		}
	case "gotify":
		// Gotify requires 'message' field
		if _, hasMessage := jsonPayload["message"]; !hasMessage {
			return fmt.Errorf("gotify payload requires 'message' field")
		}
	}

	if providerType == "gotify" || providerType == "webhook" {
		headers := map[string]string{
			"Content-Type": "application/json",
			"User-Agent":   "Charon-Notify/1.0",
		}
		if rid := ctx.Value(trace.RequestIDKey); rid != nil {
			if ridStr, ok := rid.(string); ok {
				headers["X-Request-ID"] = ridStr
			}
		}
		if providerType == "gotify" {
			if strings.TrimSpace(p.Token) != "" {
				headers["X-Gotify-Key"] = strings.TrimSpace(p.Token)
			}
		}

		if _, sendErr := s.httpWrapper.Send(ctx, notifications.HTTPWrapperRequest{
			URL:     p.URL,
			Headers: headers,
			Body:    body.Bytes(),
		}); sendErr != nil {
			return fmt.Errorf("failed to send webhook: %w", sendErr)
		}
		return nil
	}

	validatedURLStr, err := security.ValidateExternalURL(p.URL,
		security.WithAllowHTTP(),
		security.WithAllowLocalhost(),
	)
	if err != nil {
		return fmt.Errorf("invalid webhook url: %w", err)
	}

	client := network.NewSafeHTTPClient(
		network.WithTimeout(10*time.Second),
		network.WithAllowLocalhost(),
	)

	req, err := http.NewRequestWithContext(ctx, "POST", validatedURLStr, &body)
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if rid := ctx.Value(trace.RequestIDKey); rid != nil {
		if ridStr, ok := rid.(string); ok {
			req.Header.Set("X-Request-ID", ridStr)
		}
	}

	resp, err := webhookDoRequestFunc(client, req)
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Log().WithError(err).Warn("failed to close webhook response body")
		}
	}()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status: %d", resp.StatusCode)
	}
	return nil
}

// isPrivateIP returns true for RFC1918, loopback and link-local addresses.
// This wraps network.IsPrivateIP for backward compatibility and local use.
func isPrivateIP(ip net.IP) bool {
	return network.IsPrivateIP(ip)
}

func isValidRedirectURL(rawURL string) bool {
	u, err := neturl.Parse(rawURL)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	if u.Hostname() == "" {
		return false
	}
	return true
}

func (s *NotificationService) TestProvider(provider models.NotificationProvider) error {
	providerType := strings.ToLower(strings.TrimSpace(provider.Type))
	if !isSupportedNotificationProviderType(providerType) {
		return fmt.Errorf("only discord provider type is supported in this release")
	}

	if !s.isDispatchEnabled(providerType) {
		return fmt.Errorf("only discord provider type is supported in this release")
	}

	if err := validateDiscordProviderURLFunc(providerType, provider.URL); err != nil {
		return err
	}

	if !supportsJSONTemplates(providerType) {
		return legacyFallbackInvocationError(providerType)
	}

	data := map[string]any{
		"Title":   "Test Notification",
		"Message": "This is a test notification from Charon",
		"Status":  "TEST",
		"Name":    "Test Monitor",
		"Latency": 123,
		"Time":    time.Now().Format(time.RFC3339),
	}
	return s.sendJSONPayload(context.Background(), provider, data)
}

// ListTemplates returns all external notification templates stored in the database.
func (s *NotificationService) ListTemplates() ([]models.NotificationTemplate, error) {
	var list []models.NotificationTemplate
	if err := s.DB.Order("created_at desc").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// GetTemplate returns a single notification template by its ID.
func (s *NotificationService) GetTemplate(id string) (*models.NotificationTemplate, error) {
	var t models.NotificationTemplate
	if err := s.DB.Where("id = ?", id).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

// CreateTemplate stores a new notification template in the database.
func (s *NotificationService) CreateTemplate(t *models.NotificationTemplate) error {
	return s.DB.Create(t).Error
}

// UpdateTemplate saves updates to an existing notification template.
func (s *NotificationService) UpdateTemplate(t *models.NotificationTemplate) error {
	return s.DB.Save(t).Error
}

// DeleteTemplate removes a notification template by its ID.
func (s *NotificationService) DeleteTemplate(id string) error {
	return s.DB.Delete(&models.NotificationTemplate{}, "id = ?", id).Error
}

// RenderTemplate renders a provider template with provided data and returns
// the rendered JSON string and the parsed object for previewing/validation.
func (s *NotificationService) RenderTemplate(p models.NotificationProvider, data map[string]any) (resp string, parsed any, err error) {
	// Built-in templates
	const minimalTemplate = `{"message": {{toJSON .Message}}, "title": {{toJSON .Title}}, "time": {{toJSON .Time}}, "event": {{toJSON .EventType}}}`
	const detailedTemplate = `{"title": {{toJSON .Title}}, "message": {{toJSON .Message}}, "time": {{toJSON .Time}}, "event": {{toJSON .EventType}}, "host": {{toJSON .HostName}}, "host_ip": {{toJSON .HostIP}}, "service_count": {{toJSON .ServiceCount}}, "services": {{toJSON .Services}}, "data": {{toJSON .}}}`

	tmplStr := p.Config
	switch strings.ToLower(strings.TrimSpace(p.Template)) {
	case "detailed":
		tmplStr = detailedTemplate
	case "minimal":
		tmplStr = minimalTemplate
	case "custom":
		if tmplStr == "" {
			tmplStr = minimalTemplate
		}
	default:
		if tmplStr == "" {
			tmplStr = minimalTemplate
		}
	}

	// Parse and execute template with helper funcs
	tmpl, err := template.New("webhook").Funcs(template.FuncMap{
		"toJSON": func(v any) string {
			b, _ := json.Marshal(v)
			return string(b)
		},
	}).Parse(tmplStr)
	if err != nil {
		return "", nil, fmt.Errorf("failed to parse webhook template: %w", err)
	}

	var body bytes.Buffer
	if err := tmpl.Execute(&body, data); err != nil {
		return "", nil, fmt.Errorf("failed to execute webhook template: %w", err)
	}

	// Validate produced JSON
	if err := json.Unmarshal(body.Bytes(), &parsed); err != nil {
		return body.String(), nil, fmt.Errorf("failed to parse rendered template: %w", err)
	}
	return body.String(), parsed, nil
}

// Provider Management

func (s *NotificationService) ListProviders() ([]models.NotificationProvider, error) {
	var providers []models.NotificationProvider
	result := s.DB.Find(&providers)
	return providers, result.Error
}

func (s *NotificationService) CreateProvider(provider *models.NotificationProvider) error {
	provider.Type = strings.ToLower(strings.TrimSpace(provider.Type))
	if !isSupportedNotificationProviderType(provider.Type) {
		return fmt.Errorf("unsupported provider type")
	}

	if err := validateDiscordProviderURLFunc(provider.Type, provider.URL); err != nil {
		return err
	}

	if provider.Type != "gotify" {
		provider.Token = ""
	}

	// Validate custom template before creating
	if strings.ToLower(strings.TrimSpace(provider.Template)) == "custom" && strings.TrimSpace(provider.Config) != "" {
		// Provide a minimal preview payload
		payload := map[string]any{"Title": "Preview", "Message": "Preview", "Time": time.Now().Format(time.RFC3339), "EventType": "preview"}
		if _, _, err := s.RenderTemplate(*provider, payload); err != nil {
			return fmt.Errorf("invalid custom template: %w", err)
		}
	}
	return s.DB.Create(provider).Error
}

func (s *NotificationService) UpdateProvider(provider *models.NotificationProvider) error {
	// Fetch existing provider to check type
	var existing models.NotificationProvider
	if err := s.DB.Where("id = ?", provider.ID).First(&existing).Error; err != nil {
		return err
	}

	// Block type mutation for existing providers to avoid cross-provider token/schema confusion
	if strings.TrimSpace(provider.Type) != "" && provider.Type != existing.Type {
		return fmt.Errorf("cannot change provider type for existing providers")
	}
	provider.Type = existing.Type

	if !isSupportedNotificationProviderType(provider.Type) {
		return fmt.Errorf("unsupported provider type")
	}

	if err := validateDiscordProviderURLFunc(provider.Type, provider.URL); err != nil {
		return err
	}

	if provider.Type == "gotify" {
		if strings.TrimSpace(provider.Token) == "" {
			provider.Token = existing.Token
		}
	} else {
		provider.Token = ""
	}

	// Validate custom template before saving
	if strings.ToLower(strings.TrimSpace(provider.Template)) == "custom" && strings.TrimSpace(provider.Config) != "" {
		payload := map[string]any{"Title": "Preview", "Message": "Preview", "Time": time.Now().Format(time.RFC3339), "EventType": "preview"}
		if _, _, err := s.RenderTemplate(*provider, payload); err != nil {
			return fmt.Errorf("invalid custom template: %w", err)
		}
	}

	updates := map[string]any{
		"name":                               provider.Name,
		"type":                               provider.Type,
		"url":                                provider.URL,
		"token":                              provider.Token,
		"config":                             provider.Config,
		"template":                           provider.Template,
		"enabled":                            provider.Enabled,
		"notify_proxy_hosts":                 provider.NotifyProxyHosts,
		"notify_remote_servers":              provider.NotifyRemoteServers,
		"notify_domains":                     provider.NotifyDomains,
		"notify_certs":                       provider.NotifyCerts,
		"notify_uptime":                      provider.NotifyUptime,
		"notify_security_waf_blocks":         provider.NotifySecurityWAFBlocks,
		"notify_security_acl_denies":         provider.NotifySecurityACLDenies,
		"notify_security_rate_limit_hits":    provider.NotifySecurityRateLimitHits,
		"notify_security_crowdsec_decisions": provider.NotifySecurityCrowdSecDecisions,
	}

	return s.DB.Model(&models.NotificationProvider{}).
		Where("id = ?", provider.ID).
		Updates(updates).Error
}

func (s *NotificationService) DeleteProvider(id string) error {
	return s.DB.Delete(&models.NotificationProvider{}, "id = ?", id).Error
}

// EnsureNotifyOnlyProviderMigration reconciles notification_providers rows to terminal state
// for Discord-only rollout. This migration is:
// - Idempotent: safe to run multiple times
// - Transactional: all updates succeed or all fail
// - Audited: logs all mutations with provider details
//
// Migration Policy:
// - Discord providers: marked as "migrated" with engine "notify_v1"
// - Non-Discord providers: marked as "deprecated" and disabled (non-dispatch, non-enable)
//
// Rollback Procedure:
// To rollback this migration:
// 1. Restore database from pre-migration backup (see data/backups/)
// 2. OR manually update providers: UPDATE notification_providers SET migration_state='pending', enabled=true WHERE type != 'discord'
// 3. Restart application with previous version
//
// This is invoked once at server boot.
func (s *NotificationService) EnsureNotifyOnlyProviderMigration(ctx context.Context) error {
	// Begin transaction for atomicity
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var providers []models.NotificationProvider
		if err := tx.Find(&providers).Error; err != nil {
			return fmt.Errorf("failed to fetch notification providers for migration: %w", err)
		}

		// Pre-migration audit log
		logger.Log().WithField("provider_count", len(providers)).
			Info("Starting Discord-only provider migration")

		now := time.Now()
		for _, provider := range providers {
			// Skip if already in terminal state (idempotency)
			if provider.MigrationState == "migrated" || provider.MigrationState == "deprecated" {
				continue
			}

			var updates map[string]any

			if provider.Type == "discord" {
				// Discord provider: mark as migrated
				updates = map[string]any{
					"engine":           "notify_v1",
					"migration_state":  "migrated",
					"migration_error":  "",
					"last_migrated_at": now,
				}
			} else {
				// Non-Discord provider: mark as deprecated and disable
				updates = map[string]any{
					"migration_state":  "deprecated",
					"migration_error":  "provider type not supported in discord-only rollout; delete and recreate as discord provider",
					"enabled":          false,
					"last_migrated_at": now,
				}
			}

			// Preserve legacy_url if URL is being set but legacy_url is empty (audit field)
			if provider.LegacyURL == "" && provider.URL != "" {
				updates["legacy_url"] = provider.URL
			}

			if err := tx.Model(&models.NotificationProvider{}).
				Where("id = ?", provider.ID).
				Updates(updates).Error; err != nil {
				return fmt.Errorf("failed to migrate notification provider (id=%s, name=%q, type=%q): %w",
					provider.ID, util.SanitizeForLog(provider.Name), provider.Type, err)
			}

			// Audit log for each mutated row
			logger.Log().WithField("provider_id", provider.ID).
				WithField("provider_name", util.SanitizeForLog(provider.Name)).
				WithField("provider_type", provider.Type).
				WithField("migration_state", updates["migration_state"]).
				WithField("enabled", updates["enabled"]).
				WithField("migration_timestamp", now.Format(time.RFC3339)).
				Info("Migrated notification provider")
		}

		logger.Log().Info("Discord-only provider migration completed successfully")
		return nil
	})
}
