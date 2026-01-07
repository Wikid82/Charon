// Package dnsprovider defines the plugin interface and types for DNS provider plugins.
// Both built-in providers and external plugins implement this interface.
package dnsprovider

import "time"

// InterfaceVersion is the current plugin interface version.
// Plugins built against a different version may not be compatible.
const InterfaceVersion = "v1"

// ProviderPlugin defines the interface that all DNS provider plugins must implement.
// Both built-in providers and external plugins implement this interface.
type ProviderPlugin interface {
	// Type returns the unique provider type identifier (e.g., "cloudflare", "powerdns").
	// This must be lowercase, alphanumeric with optional underscores.
	Type() string

	// Metadata returns descriptive information about the provider for UI display.
	Metadata() ProviderMetadata

	// Init is called after the plugin is registered. Use for startup initialization
	// (loading config files, validating environment, establishing connections).
	// Return an error to prevent the plugin from being registered.
	Init() error

	// Cleanup is called before the plugin is unregistered. Use for resource cleanup
	// (closing connections, flushing caches). Note: Go plugins cannot be unloaded
	// from memory - this is only called during graceful shutdown.
	Cleanup() error

	// RequiredCredentialFields returns the credential fields that must be provided.
	RequiredCredentialFields() []CredentialFieldSpec

	// OptionalCredentialFields returns credential fields that may be provided.
	OptionalCredentialFields() []CredentialFieldSpec

	// ValidateCredentials checks if the provided credentials are valid.
	// Returns nil if valid, error describing the issue otherwise.
	ValidateCredentials(creds map[string]string) error

	// TestCredentials attempts to verify credentials work with the provider API.
	// This may make network calls to the provider.
	TestCredentials(creds map[string]string) error

	// SupportsMultiCredential indicates if the provider can handle zone-specific credentials.
	// If true, BuildCaddyConfigForZone will be called instead of BuildCaddyConfig when
	// multi-credential mode is enabled (Phase 3 feature).
	SupportsMultiCredential() bool

	// BuildCaddyConfig constructs the Caddy DNS challenge configuration.
	// The returned map is embedded into Caddy's TLS automation policy.
	// Used when multi-credential mode is disabled.
	BuildCaddyConfig(creds map[string]string) map[string]any

	// BuildCaddyConfigForZone constructs config for a specific zone (multi-credential mode).
	// Only called if SupportsMultiCredential() returns true.
	// baseDomain is the zone being configured (e.g., "example.com").
	BuildCaddyConfigForZone(baseDomain string, creds map[string]string) map[string]any

	// PropagationTimeout returns the recommended DNS propagation wait time.
	PropagationTimeout() time.Duration

	// PollingInterval returns the recommended polling interval for DNS verification.
	PollingInterval() time.Duration
}

// ProviderMetadata contains descriptive information about a DNS provider.
type ProviderMetadata struct {
	Type             string `json:"type"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	DocumentationURL string `json:"documentation_url,omitempty"`
	Author           string `json:"author,omitempty"`
	Version          string `json:"version,omitempty"`
	IsBuiltIn        bool   `json:"is_built_in"`

	// Version compatibility (required for external plugins)
	GoVersion        string `json:"go_version,omitempty"`        // Go version used to build (e.g., "1.23.4")
	InterfaceVersion string `json:"interface_version,omitempty"` // Plugin interface version (e.g., "v1")
}

// CredentialFieldSpec defines a credential field for UI rendering.
type CredentialFieldSpec struct {
	Name        string         `json:"name"`                  // Field key (e.g., "api_token")
	Label       string         `json:"label"`                 // Display label (e.g., "API Token")
	Type        string         `json:"type"`                  // "text", "password", "textarea", "select"
	Placeholder string         `json:"placeholder,omitempty"` // Input placeholder text
	Hint        string         `json:"hint,omitempty"`        // Help text shown below field
	Options     []SelectOption `json:"options,omitempty"`     // For "select" type
}

// SelectOption represents an option in a select dropdown.
type SelectOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}
