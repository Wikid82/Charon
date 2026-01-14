// Package custom provides custom DNS provider plugins for non-built-in integrations.
package custom

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Wikid82/charon/backend/pkg/dnsprovider"
)

// Webhook provider constants.
const (
	WebhookDefaultTimeoutSeconds     = 30
	WebhookDefaultRetryCount         = 3
	WebhookDefaultPropagationTimeout = 120 * time.Second
	WebhookDefaultPollingInterval    = 5 * time.Second
	WebhookMinTimeoutSeconds         = 5
	WebhookMaxTimeoutSeconds         = 300
	WebhookMinRetryCount             = 0
	WebhookMaxRetryCount             = 10
)

// WebhookProvider implements the ProviderPlugin interface for generic HTTP webhook DNS challenges.
// This provider calls external HTTP endpoints to create/delete DNS TXT records,
// enabling integration with custom or proprietary DNS systems.
type WebhookProvider struct {
	propagationTimeout time.Duration
	pollingInterval    time.Duration
}

// NewWebhookProvider creates a new WebhookProvider with default settings.
func NewWebhookProvider() *WebhookProvider {
	return &WebhookProvider{
		propagationTimeout: WebhookDefaultPropagationTimeout,
		pollingInterval:    WebhookDefaultPollingInterval,
	}
}

// Type returns the unique provider type identifier.
func (p *WebhookProvider) Type() string {
	return "webhook"
}

// Metadata returns descriptive information about the provider.
func (p *WebhookProvider) Metadata() dnsprovider.ProviderMetadata {
	return dnsprovider.ProviderMetadata{
		Type:             "webhook",
		Name:             "Webhook (HTTP)",
		Description:      "Generic HTTP webhook for DNS challenges. Calls external endpoints to create and delete TXT records. Useful for custom DNS APIs or proprietary systems.",
		DocumentationURL: "https://charon.dev/docs/features/webhook-dns",
		IsBuiltIn:        false,
		Version:          "1.0.0",
		InterfaceVersion: dnsprovider.InterfaceVersion,
	}
}

// Init is called after the plugin is registered.
func (p *WebhookProvider) Init() error {
	return nil
}

// Cleanup is called before the plugin is unregistered.
func (p *WebhookProvider) Cleanup() error {
	return nil
}

// RequiredCredentialFields returns credential fields that must be provided.
func (p *WebhookProvider) RequiredCredentialFields() []dnsprovider.CredentialFieldSpec {
	return []dnsprovider.CredentialFieldSpec{
		{
			Name:        "create_url",
			Label:       "Create URL",
			Type:        "text",
			Placeholder: "https://dns-api.example.com/txt/create",
			Hint:        "POST endpoint for creating DNS TXT records. Must be HTTPS (HTTP allowed for localhost in development).",
		},
		{
			Name:        "delete_url",
			Label:       "Delete URL",
			Type:        "text",
			Placeholder: "https://dns-api.example.com/txt/delete",
			Hint:        "POST/DELETE endpoint for removing DNS TXT records. Must be HTTPS (HTTP allowed for localhost in development).",
		},
	}
}

// OptionalCredentialFields returns credential fields that may be provided.
func (p *WebhookProvider) OptionalCredentialFields() []dnsprovider.CredentialFieldSpec {
	return []dnsprovider.CredentialFieldSpec{
		{
			Name:        "auth_header",
			Label:       "Authorization Header Name",
			Type:        "text",
			Placeholder: "Authorization",
			Hint:        "Custom header name for authentication (e.g., Authorization, X-API-Key)",
		},
		{
			Name:        "auth_value",
			Label:       "Authorization Header Value",
			Type:        "password",
			Placeholder: "",
			Hint:        "Value for the authorization header (e.g., Bearer token, API key)",
		},
		{
			Name:        "timeout_seconds",
			Label:       "Request Timeout (seconds)",
			Type:        "text",
			Placeholder: strconv.Itoa(WebhookDefaultTimeoutSeconds),
			Hint:        fmt.Sprintf("HTTP request timeout (%d-%d seconds, default: %d)", WebhookMinTimeoutSeconds, WebhookMaxTimeoutSeconds, WebhookDefaultTimeoutSeconds),
		},
		{
			Name:        "retry_count",
			Label:       "Retry Count",
			Type:        "text",
			Placeholder: strconv.Itoa(WebhookDefaultRetryCount),
			Hint:        fmt.Sprintf("Number of retries on failure (%d-%d, default: %d)", WebhookMinRetryCount, WebhookMaxRetryCount, WebhookDefaultRetryCount),
		},
		{
			Name:        "insecure_skip_verify",
			Label:       "Skip TLS Verification",
			Type:        "select",
			Placeholder: "",
			Hint:        "⚠️ DEVELOPMENT ONLY: Skip TLS certificate verification. Never enable in production!",
			Options: []dnsprovider.SelectOption{
				{Value: "false", Label: "No (Recommended)"},
				{Value: "true", Label: "Yes (Insecure - Dev Only)"},
			},
		},
	}
}

// ValidateCredentials checks if the provided credentials are valid.
func (p *WebhookProvider) ValidateCredentials(creds map[string]string) error {
	// Validate required fields
	createURL := strings.TrimSpace(creds["create_url"])
	if createURL == "" {
		return fmt.Errorf("create_url is required")
	}

	deleteURL := strings.TrimSpace(creds["delete_url"])
	if deleteURL == "" {
		return fmt.Errorf("delete_url is required")
	}

	// Validate create URL format and security
	if err := p.validateWebhookURL(createURL, "create_url"); err != nil {
		return err
	}

	// Validate delete URL format and security
	if err := p.validateWebhookURL(deleteURL, "delete_url"); err != nil {
		return err
	}

	// Validate timeout if provided
	if timeoutStr := strings.TrimSpace(creds["timeout_seconds"]); timeoutStr != "" {
		timeout, err := strconv.Atoi(timeoutStr)
		if err != nil {
			return fmt.Errorf("timeout_seconds must be a number: %w", err)
		}
		if timeout < WebhookMinTimeoutSeconds || timeout > WebhookMaxTimeoutSeconds {
			return fmt.Errorf("timeout_seconds must be between %d and %d", WebhookMinTimeoutSeconds, WebhookMaxTimeoutSeconds)
		}
	}

	// Validate retry count if provided
	if retryStr := strings.TrimSpace(creds["retry_count"]); retryStr != "" {
		retry, err := strconv.Atoi(retryStr)
		if err != nil {
			return fmt.Errorf("retry_count must be a number: %w", err)
		}
		if retry < WebhookMinRetryCount || retry > WebhookMaxRetryCount {
			return fmt.Errorf("retry_count must be between %d and %d", WebhookMinRetryCount, WebhookMaxRetryCount)
		}
	}

	// Validate insecure_skip_verify if provided
	if insecureStr := strings.TrimSpace(creds["insecure_skip_verify"]); insecureStr != "" {
		insecureStr = strings.ToLower(insecureStr)
		if insecureStr != "true" && insecureStr != "false" {
			return fmt.Errorf("insecure_skip_verify must be 'true' or 'false'")
		}
	}

	// Validate auth header/value consistency
	authHeader := strings.TrimSpace(creds["auth_header"])
	authValue := strings.TrimSpace(creds["auth_value"])
	if (authHeader != "" && authValue == "") || (authHeader == "" && authValue != "") {
		return fmt.Errorf("both auth_header and auth_value must be provided together, or neither")
	}

	return nil
}

// validateWebhookURL validates a webhook URL for format and SSRF protection.
// Note: During validation, we only check format and basic security constraints.
// Full SSRF validation with DNS resolution happens at runtime when the webhook is called.
func (p *WebhookProvider) validateWebhookURL(rawURL, fieldName string) error {
	// Parse URL first for basic validation
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("%s has invalid URL format: %w", fieldName, err)
	}

	// Validate scheme
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%s must use http or https scheme", fieldName)
	}

	// Validate hostname exists
	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("%s is missing hostname", fieldName)
	}

	// Check if this is a localhost URL (allowed for development)
	isLocalhost := host == "localhost" || host == "127.0.0.1" || host == "::1"

	// Require HTTPS for non-localhost URLs
	if !isLocalhost && parsed.Scheme != "https" {
		return fmt.Errorf("%s must use HTTPS for non-localhost URLs (security requirement)", fieldName)
	}

	// For external URLs (non-localhost), we skip DNS-based SSRF validation during
	// credential validation as the target might not be reachable from the validation
	// environment. Runtime SSRF protection will be enforced when actually calling the webhook.
	// This matches the pattern used by RFC2136Provider which also validates format only.

	return nil
}

// TestCredentials attempts to verify credentials work.
// For webhook, we validate the format but cannot test without making actual HTTP calls.
func (p *WebhookProvider) TestCredentials(creds map[string]string) error {
	return p.ValidateCredentials(creds)
}

// SupportsMultiCredential indicates if the provider can handle zone-specific credentials.
func (p *WebhookProvider) SupportsMultiCredential() bool {
	return false
}

// BuildCaddyConfig constructs the Caddy DNS challenge configuration.
// For webhook, this returns a config that Charon's internal webhook handler will use.
func (p *WebhookProvider) BuildCaddyConfig(creds map[string]string) map[string]any {
	config := map[string]any{
		"name":       "webhook",
		"create_url": strings.TrimSpace(creds["create_url"]),
		"delete_url": strings.TrimSpace(creds["delete_url"]),
	}

	// Add auth header if provided
	if authHeader := strings.TrimSpace(creds["auth_header"]); authHeader != "" {
		config["auth_header"] = authHeader
	}

	// Add auth value if provided
	if authValue := strings.TrimSpace(creds["auth_value"]); authValue != "" {
		config["auth_value"] = authValue
	}

	// Add timeout with default
	timeoutSeconds := WebhookDefaultTimeoutSeconds
	if timeoutStr := strings.TrimSpace(creds["timeout_seconds"]); timeoutStr != "" {
		if t, err := strconv.Atoi(timeoutStr); err == nil && t >= WebhookMinTimeoutSeconds && t <= WebhookMaxTimeoutSeconds {
			timeoutSeconds = t
		}
	}
	config["timeout_seconds"] = timeoutSeconds

	// Add retry count with default
	retryCount := WebhookDefaultRetryCount
	if retryStr := strings.TrimSpace(creds["retry_count"]); retryStr != "" {
		if r, err := strconv.Atoi(retryStr); err == nil && r >= WebhookMinRetryCount && r <= WebhookMaxRetryCount {
			retryCount = r
		}
	}
	config["retry_count"] = retryCount

	// Add insecure skip verify with default (false)
	insecureSkipVerify := false
	if insecureStr := strings.TrimSpace(creds["insecure_skip_verify"]); insecureStr != "" {
		insecureSkipVerify = strings.ToLower(insecureStr) == "true"
	}
	config["insecure_skip_verify"] = insecureSkipVerify

	return config
}

// BuildCaddyConfigForZone constructs config for a specific zone.
func (p *WebhookProvider) BuildCaddyConfigForZone(baseDomain string, creds map[string]string) map[string]any {
	return p.BuildCaddyConfig(creds)
}

// PropagationTimeout returns the recommended DNS propagation wait time.
func (p *WebhookProvider) PropagationTimeout() time.Duration {
	return p.propagationTimeout
}

// PollingInterval returns the recommended polling interval for DNS verification.
func (p *WebhookProvider) PollingInterval() time.Duration {
	return p.pollingInterval
}

// GetTimeoutSeconds returns the configured timeout in seconds or the default.
func (p *WebhookProvider) GetTimeoutSeconds(creds map[string]string) int {
	if timeoutStr := strings.TrimSpace(creds["timeout_seconds"]); timeoutStr != "" {
		if timeout, err := strconv.Atoi(timeoutStr); err == nil {
			if timeout >= WebhookMinTimeoutSeconds && timeout <= WebhookMaxTimeoutSeconds {
				return timeout
			}
		}
	}
	return WebhookDefaultTimeoutSeconds
}

// GetRetryCount returns the configured retry count or the default.
func (p *WebhookProvider) GetRetryCount(creds map[string]string) int {
	if retryStr := strings.TrimSpace(creds["retry_count"]); retryStr != "" {
		if retry, err := strconv.Atoi(retryStr); err == nil {
			if retry >= WebhookMinRetryCount && retry <= WebhookMaxRetryCount {
				return retry
			}
		}
	}
	return WebhookDefaultRetryCount
}

// IsInsecureSkipVerify returns whether TLS verification should be skipped.
func (p *WebhookProvider) IsInsecureSkipVerify(creds map[string]string) bool {
	if insecureStr := strings.TrimSpace(creds["insecure_skip_verify"]); insecureStr != "" {
		return strings.ToLower(insecureStr) == "true"
	}
	return false
}
