// Package custom provides custom DNS provider plugins for non-built-in integrations.
package custom

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Wikid82/charon/backend/pkg/dnsprovider"
)

// Default configuration values for the manual provider.
const (
	DefaultTimeoutMinutes         = 10
	DefaultPollingIntervalSeconds = 30
	MinTimeoutMinutes             = 1
	MaxTimeoutMinutes             = 60
	MinPollingIntervalSeconds     = 5
	MaxPollingIntervalSeconds     = 120
)

// ManualProvider implements the ProviderPlugin interface for manual DNS challenges.
// Users manually create TXT records at their DNS provider and click verify.
type ManualProvider struct {
	timeoutMinutes         int
	pollingIntervalSeconds int
}

// NewManualProvider creates a new ManualProvider with default settings.
func NewManualProvider() *ManualProvider {
	return &ManualProvider{
		timeoutMinutes:         DefaultTimeoutMinutes,
		pollingIntervalSeconds: DefaultPollingIntervalSeconds,
	}
}

// Type returns the unique provider type identifier.
func (p *ManualProvider) Type() string {
	return "manual"
}

// Metadata returns descriptive information about the provider.
func (p *ManualProvider) Metadata() dnsprovider.ProviderMetadata {
	return dnsprovider.ProviderMetadata{
		Type:             "manual",
		Name:             "Manual (No Automation)",
		Description:      "Manually create DNS TXT records for ACME challenges. Suitable for testing or providers without API access.",
		DocumentationURL: "https://charon.dev/docs/features/manual-dns-challenge",
		IsBuiltIn:        false,
		Version:          "1.0.0",
		InterfaceVersion: dnsprovider.InterfaceVersion,
	}
}

// Init is called after the plugin is registered.
func (p *ManualProvider) Init() error {
	return nil
}

// Cleanup is called before the plugin is unregistered.
func (p *ManualProvider) Cleanup() error {
	return nil
}

// RequiredCredentialFields returns credential fields that must be provided.
// Manual provider has no required credentials.
func (p *ManualProvider) RequiredCredentialFields() []dnsprovider.CredentialFieldSpec {
	return []dnsprovider.CredentialFieldSpec{}
}

// OptionalCredentialFields returns credential fields that may be provided.
func (p *ManualProvider) OptionalCredentialFields() []dnsprovider.CredentialFieldSpec {
	return []dnsprovider.CredentialFieldSpec{
		{
			Name:        "timeout_minutes",
			Label:       "Challenge Timeout (minutes)",
			Type:        "text",
			Placeholder: "10",
			Hint:        fmt.Sprintf("Time before challenge expires (%d-%d minutes, default: %d)", MinTimeoutMinutes, MaxTimeoutMinutes, DefaultTimeoutMinutes),
		},
		{
			Name:        "polling_interval_seconds",
			Label:       "DNS Check Interval (seconds)",
			Type:        "text",
			Placeholder: "30",
			Hint:        fmt.Sprintf("How often to check DNS propagation (%d-%d seconds, default: %d)", MinPollingIntervalSeconds, MaxPollingIntervalSeconds, DefaultPollingIntervalSeconds),
		},
	}
}

// ValidateCredentials checks if the provided credentials are valid.
func (p *ManualProvider) ValidateCredentials(creds map[string]string) error {
	// Validate timeout if provided
	if timeoutStr := creds["timeout_minutes"]; timeoutStr != "" {
		timeout, err := strconv.Atoi(timeoutStr)
		if err != nil {
			return fmt.Errorf("timeout_minutes must be a number: %w", err)
		}
		if timeout < MinTimeoutMinutes || timeout > MaxTimeoutMinutes {
			return fmt.Errorf("timeout_minutes must be between %d and %d", MinTimeoutMinutes, MaxTimeoutMinutes)
		}
	}

	// Validate polling interval if provided
	if intervalStr := creds["polling_interval_seconds"]; intervalStr != "" {
		interval, err := strconv.Atoi(intervalStr)
		if err != nil {
			return fmt.Errorf("polling_interval_seconds must be a number: %w", err)
		}
		if interval < MinPollingIntervalSeconds || interval > MaxPollingIntervalSeconds {
			return fmt.Errorf("polling_interval_seconds must be between %d and %d", MinPollingIntervalSeconds, MaxPollingIntervalSeconds)
		}
	}

	return nil
}

// TestCredentials attempts to verify credentials work.
// For manual provider, this always succeeds since there's no external API.
func (p *ManualProvider) TestCredentials(creds map[string]string) error {
	return p.ValidateCredentials(creds)
}

// SupportsMultiCredential indicates if the provider can handle zone-specific credentials.
func (p *ManualProvider) SupportsMultiCredential() bool {
	return false
}

// BuildCaddyConfig constructs the Caddy DNS challenge configuration.
// For manual provider, this returns a marker that tells Caddy to use manual mode.
func (p *ManualProvider) BuildCaddyConfig(creds map[string]string) map[string]any {
	// Manual provider doesn't integrate with Caddy's DNS challenge directly.
	// Instead, Charon handles the challenge flow and signals completion.
	return map[string]any{
		"name":   "manual",
		"manual": true,
	}
}

// BuildCaddyConfigForZone constructs config for a specific zone.
func (p *ManualProvider) BuildCaddyConfigForZone(baseDomain string, creds map[string]string) map[string]any {
	return p.BuildCaddyConfig(creds)
}

// PropagationTimeout returns the recommended DNS propagation wait time.
func (p *ManualProvider) PropagationTimeout() time.Duration {
	return time.Duration(p.timeoutMinutes) * time.Minute
}

// PollingInterval returns the recommended polling interval for DNS verification.
func (p *ManualProvider) PollingInterval() time.Duration {
	return time.Duration(p.pollingIntervalSeconds) * time.Second
}

// GetTimeoutMinutes returns the configured timeout in minutes.
func (p *ManualProvider) GetTimeoutMinutes(creds map[string]string) int {
	if timeoutStr := creds["timeout_minutes"]; timeoutStr != "" {
		if timeout, err := strconv.Atoi(timeoutStr); err == nil {
			if timeout >= MinTimeoutMinutes && timeout <= MaxTimeoutMinutes {
				return timeout
			}
		}
	}
	return DefaultTimeoutMinutes
}

// GetPollingIntervalSeconds returns the configured polling interval in seconds.
func (p *ManualProvider) GetPollingIntervalSeconds(creds map[string]string) int {
	if intervalStr := creds["polling_interval_seconds"]; intervalStr != "" {
		if interval, err := strconv.Atoi(intervalStr); err == nil {
			if interval >= MinPollingIntervalSeconds && interval <= MaxPollingIntervalSeconds {
				return interval
			}
		}
	}
	return DefaultPollingIntervalSeconds
}
