package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	neturl "net/url"
	"regexp"
	"strings"
	"text/template"
	"time"

	notify "github.com/Wikid82/go_notify_yourself"
	"github.com/Wikid82/go_notify_yourself/providers/email"
	"github.com/Wikid82/go_notify_yourself/providers/webhook"
	"github.com/Wikid82/go_notify_yourself/transport"

	"github.com/Wikid82/charon/backend/internal/logger"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/util"
	"gorm.io/gorm"
)

type NotificationService struct {
	DB                 *gorm.DB
	notifyWrapper      *transport.Wrapper
	mailService        MailServiceInterface
	telegramAPIBaseURL string
	pushoverAPIBaseURL string
	validateSlackURL   func(string) error
}

// NotificationServiceOption configures a NotificationService at construction time.
type NotificationServiceOption func(*NotificationService)

// WithSlackURLValidator overrides the Slack webhook URL validator. Intended for use
// in tests that need to bypass real URL validation without mutating shared state.
func WithSlackURLValidator(fn func(string) error) NotificationServiceOption {
	return func(s *NotificationService) {
		s.validateSlackURL = fn
	}
}

// WithNotifyTransportWrapper overrides the *transport.Wrapper used to
// dispatch notifications through the extracted notify module's provider
// packages (buildNotifySender, notify_provider_adapter.go). Intended for
// tests that need to intercept outbound requests (e.g. a fake
// http.RoundTripper) without hitting a real network destination —
// production code always uses the wrapper built by NewNotifyTransportWrapper.
func WithNotifyTransportWrapper(w *transport.Wrapper) NotificationServiceOption {
	return func(s *NotificationService) {
		s.notifyWrapper = w
	}
}

func NewNotificationService(db *gorm.DB, mailService MailServiceInterface, opts ...NotificationServiceOption) *NotificationService {
	s := &NotificationService{
		DB:                 db,
		notifyWrapper:      NewNotifyTransportWrapper(),
		mailService:        mailService,
		telegramAPIBaseURL: "https://api.telegram.org",
		pushoverAPIBaseURL: "https://api.pushover.net",
		validateSlackURL:   validateSlackWebhookURL,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

var allowedDiscordWebhookHosts = map[string]struct{}{
	"discord.com":        {},
	"canary.discord.com": {},
}

var slackWebhookRegex = regexp.MustCompile(`^https://hooks\.slack\.com/services/T[A-Za-z0-9_-]+/B[A-Za-z0-9_-]+/[A-Za-z0-9_-]+$`)

func validateSlackWebhookURL(rawURL string) error {
	if !slackWebhookRegex.MatchString(rawURL) {
		return fmt.Errorf("invalid Slack webhook URL: must match https://hooks.slack.com/services/T.../B.../xxx")
	}
	return nil
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
	case "webhook", "discord", "gotify", "slack", "generic", "telegram", "pushover", "ntfy":
		return true
	default:
		return false
	}
}

func isSupportedNotificationProviderType(providerType string) bool {
	switch strings.ToLower(strings.TrimSpace(providerType)) {
	case "discord", "email", "gotify", "webhook", "telegram", "slack", "pushover", "ntfy":
		return true
	default:
		return false
	}
}

func (s *NotificationService) isDispatchEnabled(providerType string) bool {
	switch strings.ToLower(strings.TrimSpace(providerType)) {
	case "discord":
		return true
	case "email":
		return s.getFeatureFlagValue(FlagEmailServiceEnabled, false)
	case "gotify":
		return s.getFeatureFlagValue(FlagGotifyServiceEnabled, true)
	case "webhook":
		return s.getFeatureFlagValue(FlagWebhookServiceEnabled, true)
	case "telegram":
		return s.getFeatureFlagValue(FlagTelegramServiceEnabled, true)
	case "slack":
		return s.getFeatureFlagValue(FlagSlackServiceEnabled, true)
	case "pushover":
		return s.getFeatureFlagValue(FlagPushoverServiceEnabled, true)
	case "ntfy":
		return s.getFeatureFlagValue(FlagNtfyServiceEnabled, true)
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
	var notifList []models.Notification
	query := s.DB.Order("created_at desc")
	if unreadOnly {
		query = query.Where("read = ?", false)
	}
	result := query.Find(&notifList)
	return notifList, result.Error
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
		if strings.ToLower(strings.TrimSpace(provider.Type)) == "email" {
			go s.dispatchEmailViaNotify(ctx, provider, eventType, title, message)
			continue
		}
		go func(p models.NotificationProvider) {
			if !supportsJSONTemplates(p.Type) {
				logger.Log().WithField("provider", util.SanitizeForLog(p.Name)).WithField("type", p.Type).Warn("Provider type is not supported by notify-only runtime")
				return
			}
			s.dispatchViaNotify(ctx, p, eventType, title, message, data)
		}(provider)
	}
}

// notifyMessageDataFromLegacyFlatMap extracts the host/service extras that
// legacyDetailedTemplate (notify_provider_adapter.go) reads via
// {{index .Data "HostName"}}/{{index .Data "HostIP"}}/
// {{index .Data "ServiceCount"}}/{{index .Data "Services"}} from the flat
// data map SendExternal callers (e.g. uptime_service.go's
// sendHostDownNotification) pass in, so a provider configured with the old
// "detailed" template keeps rendering the same host/IP/service-count/
// services values after cutover to the extracted notify module. Reading a
// missing key from a nil or incomplete map yields nil (renders as JSON
// null), matching the old flat-map template's behavior for callers that
// don't supply these optional fields (e.g. proxy_host/domain/cert/
// remote_server events).
func notifyMessageDataFromLegacyFlatMap(data map[string]any) map[string]any {
	return map[string]any{
		"HostName":     data["HostName"],
		"HostIP":       data["HostIP"],
		"ServiceCount": data["ServiceCount"],
		"Services":     data["Services"],
	}
}

// dispatchViaNotify sends a notification through the extracted notify
// module (buildNotifySender, notify_provider_adapter.go). It builds
// a notify.Message from the caller-supplied source data
// (title/message/eventType plus the HostName/HostIP/ServiceCount/Services
// extras a caller may have supplied), then dispatches it through the
// provider-specific Sender, which routes through the shared
// *transport.Wrapper (s.notifyWrapper) — gaining that wrapper's
// retry/backoff behavior for every dispatch that goes through this path.
func (s *NotificationService) dispatchViaNotify(ctx context.Context, p models.NotificationProvider, eventType, title, message string, data map[string]any) {
	sender, err := buildNotifySender(p, s.notifyWrapper)
	if err != nil {
		logger.Log().WithError(err).WithField("provider", util.SanitizeForLog(p.Name)).Error("Failed to build notify sender")
		return
	}

	msg := notify.Message{
		Title:     title,
		Body:      message,
		EventType: eventType,
		Data:      notifyMessageDataFromLegacyFlatMap(data),
	}

	if err := sender.Send(ctx, msg); err != nil {
		logger.Log().WithError(err).WithField("provider", util.SanitizeForLog(p.Name)).Error("Failed to send notification via notify module")
	}
}

// dispatchEmailViaNotify sends an email notification through the extracted
// notify module's email package (NewNotifyEmailConfig, notify_email_adapter.go).
// It runs in a goroutine; all errors are logged rather than returned.
//
// Behavior note: a template-rendering failure still
// results in the notification being sent, using a manually built plain
// HTML body — see mailServiceTemplateRendererAdapter.Render's doc comment
// (notify_email_adapter.go) for where that fallback now lives. Only a real
// Mailer/SMTP transport failure (mailServiceMailerAdapter.Send) causes this
// function's error branch to fire.
func (s *NotificationService) dispatchEmailViaNotify(ctx context.Context, p models.NotificationProvider, eventType, title, message string) {
	if s.mailService == nil || !s.mailService.IsConfigured() {
		logger.Log().WithField("provider", util.SanitizeForLog(p.Name)).Warn("Email provider is not configured, skipping dispatch")
		return
	}

	recipients := parseEmailRecipients(p.URL)
	if len(recipients) == 0 {
		logger.Log().WithField("provider", util.SanitizeForLog(p.Name)).Warn("Email provider has no recipients configured")
		return
	}

	client := email.New(NewNotifyEmailConfig(s.mailService, recipients))

	msg := notify.Message{
		Title:     title,
		Body:      message,
		EventType: eventType,
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := client.Send(timeoutCtx, msg); err != nil {
		logger.Log().WithError(err).WithField("provider", util.SanitizeForLog(p.Name)).Error("Failed to send email notification")
	}
}

// parseEmailRecipients splits a NotificationProvider's comma-separated URL
// field into a trimmed, non-empty recipient list. Shared by
// dispatchEmailViaNotify and TestEmailProvider's notify-path counterpart.
func parseEmailRecipients(rawURL string) []string {
	rawRecipients := strings.Split(rawURL, ",")
	recipients := make([]string, 0, len(rawRecipients))
	for _, r := range rawRecipients {
		if trimmed := strings.TrimSpace(r); trimmed != "" {
			recipients = append(recipients, trimmed)
		}
	}
	return recipients
}

func emailTemplateForEventType(eventType string) string {
	switch strings.ToLower(strings.TrimSpace(eventType)) {
	case "security_waf", "security_acl", "security_rate_limit", "security_crowdsec":
		return "email_security_alert.html"
	case "cert":
		return "email_ssl_event.html"
	case "uptime":
		return "email_uptime_event.html"
	default:
		return "email_system_event.html"
	}
}

// validateDiscordProviderURLFunc is a test hook for Discord webhook URL validation.
// In tests, you can override this to bypass strict hostname checks for localhost testing.
var validateDiscordProviderURLFunc = validateDiscordProviderURL

func (s *NotificationService) TestProvider(provider models.NotificationProvider) error {
	providerType := strings.ToLower(strings.TrimSpace(provider.Type))
	if !isSupportedNotificationProviderType(providerType) {
		return fmt.Errorf("unsupported provider type: %s", providerType)
	}

	if err := validateDiscordProviderURLFunc(providerType, provider.URL); err != nil {
		return err
	}

	if !supportsJSONTemplates(providerType) {
		return fmt.Errorf("provider type %q does not support JSON templates", providerType)
	}

	return s.testProviderViaNotify(provider)
}

// testProviderViaNotify sends a test notification through the extracted
// notify module (buildNotifySender, notify_provider_adapter.go).
func (s *NotificationService) testProviderViaNotify(provider models.NotificationProvider) error {
	sender, err := buildNotifySender(provider, s.notifyWrapper)
	if err != nil {
		return fmt.Errorf("build notify sender: %w", err)
	}

	msg := notify.Message{
		Title:     "Test Notification",
		Body:      "This is a test notification from Charon",
		EventType: "test",
	}
	return sender.Send(context.Background(), msg)
}

// TestEmailProvider sends a test email to the recipients configured in
// provider.URL, dispatched through the extracted notify module's email
// package (providers/email) the same way TestEmailProvider's real-dispatch
// counterpart (dispatchEmailViaNotify) is. It bypasses the JSON-template
// path used by TestProvider.
//
// This uses its own inline email.Config, rather than NewNotifyEmailConfig
// (notify_email_adapter.go), because the test-send subject prefix
// ("[Charon Test] ") and forced "email_system_event.html" template differ
// from NewNotifyEmailConfig's production values ("[Charon Alert] " and
// emailTemplateForEventType's event-type-based mapping) — matching the old
// TestEmailProvider's hardcoded subject/template exactly.
//
// Behavior note: see dispatchEmailViaNotify's comment — like that path,
// this still falls back to a manually built plain HTML body (via
// mailServiceTemplateRendererAdapter.Render) when template rendering fails,
// and still sends/succeeds. Only a real Mailer/SMTP transport failure
// causes this function to return an error.
func (s *NotificationService) TestEmailProvider(provider models.NotificationProvider) error {
	if s.mailService == nil || !s.mailService.IsConfigured() {
		return fmt.Errorf("email service is not configured; configure SMTP settings before testing email providers")
	}

	recipients := parseEmailRecipients(provider.URL)
	if len(recipients) == 0 {
		return fmt.Errorf("no recipients configured; add at least one recipient email address")
	}

	cfg := email.Config{
		Recipients:    recipients,
		SubjectPrefix: "[Charon Test] ",
		TemplateName: func(notify.Message) string {
			return "email_system_event.html"
		},
		Renderer: &mailServiceTemplateRendererAdapter{mailService: s.mailService},
		Mailer:   &mailServiceMailerAdapter{mailService: s.mailService},
	}
	client := email.New(cfg)

	msg := notify.Message{
		Title:     "Test Notification",
		Body:      "This is a test notification from Charon. If you received this email, your email notification provider is configured correctly.",
		EventType: "test",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return client.Send(ctx, msg)
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

	if provider.Type == "slack" {
		token := strings.TrimSpace(provider.Token)
		if token == "" {
			return fmt.Errorf("slack webhook URL is required")
		}
		if err := s.validateSlackURL(token); err != nil {
			return err
		}
	}

	if provider.Type != "gotify" && provider.Type != "telegram" && provider.Type != "slack" && provider.Type != "ntfy" && provider.Type != "pushover" {
		provider.Token = ""
	}

	// Validate custom template before creating. Uses providers/webhook.RenderPreview
	// (the extracted notify module's template-preview function) rather than the old
	// RenderTemplate, so preview validation exercises the same TemplateData shape
	// (Title/Message/Time/EventType/Data) that dispatchViaNotify's actual dispatch
	// uses — a custom template referencing {{index .Data "..."}} now validates
	// correctly instead of failing preview with a flat map that had no Data field.
	if strings.ToLower(strings.TrimSpace(provider.Template)) == "custom" && strings.TrimSpace(provider.Config) != "" {
		previewMsg := notify.Message{Title: "Preview", Body: "Preview", EventType: "preview"}
		if _, _, err := webhook.RenderPreview(provider.Config, previewMsg); err != nil {
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

	if provider.Type == "gotify" || provider.Type == "telegram" || provider.Type == "slack" || provider.Type == "ntfy" || provider.Type == "pushover" {
		if strings.TrimSpace(provider.Token) == "" {
			provider.Token = existing.Token
		}
	} else {
		provider.Token = ""
	}

	if provider.Type == "slack" && provider.Token != existing.Token {
		if err := s.validateSlackURL(strings.TrimSpace(provider.Token)); err != nil {
			return err
		}
	}

	// Validate custom template before saving — see the matching comment in
	// CreateProvider for why this uses providers/webhook.RenderPreview.
	if strings.ToLower(strings.TrimSpace(provider.Template)) == "custom" && strings.TrimSpace(provider.Config) != "" {
		previewMsg := notify.Message{Title: "Preview", Body: "Preview", EventType: "preview"}
		if _, _, err := webhook.RenderPreview(provider.Config, previewMsg); err != nil {
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
