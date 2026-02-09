// Package custom provides custom DNS provider plugins for non-built-in integrations.
package custom

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Wikid82/charon/backend/pkg/dnsprovider"
)

// Script provider constants.
const (
	ScriptDefaultTimeoutSeconds     = 60
	ScriptDefaultPropagationTimeout = 120 * time.Second
	ScriptDefaultPollingInterval    = 5 * time.Second
	ScriptMinTimeoutSeconds         = 5
	ScriptMaxTimeoutSeconds         = 300
	ScriptAllowedDirectory          = "/scripts/"
)

// scriptArgPattern validates script arguments to prevent injection attacks.
// Only allows alphanumeric characters, dots, underscores, equals, and hyphens.
var scriptArgPattern = regexp.MustCompile(`^[a-zA-Z0-9._=-]+$`)

// envVarLinePattern validates environment variable format (KEY=VALUE).
var envVarLinePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=.*$`)

// ScriptProvider implements the ProviderPlugin interface for shell script DNS challenges.
// This provider executes local scripts to create/delete DNS TXT records.
//
// SECURITY WARNING: This is a HIGH-RISK feature. Scripts are executed on the server
// with the same privileges as the Charon process. Only administrators should configure
// this provider, and scripts must be carefully reviewed before deployment.
type ScriptProvider struct {
	propagationTimeout time.Duration
	pollingInterval    time.Duration
}

// NewScriptProvider creates a new ScriptProvider with default settings.
func NewScriptProvider() *ScriptProvider {
	return &ScriptProvider{
		propagationTimeout: ScriptDefaultPropagationTimeout,
		pollingInterval:    ScriptDefaultPollingInterval,
	}
}

// Type returns the unique provider type identifier.
func (p *ScriptProvider) Type() string {
	return "script"
}

// Metadata returns descriptive information about the provider.
func (p *ScriptProvider) Metadata() dnsprovider.ProviderMetadata {
	return dnsprovider.ProviderMetadata{
		Type:             "script",
		Name:             "Script (Shell)",
		Description:      "⚠️ ADVANCED: Execute shell scripts for DNS challenges. Scripts must be located in /scripts/. HIGH-RISK feature - scripts run with server privileges. Only for administrators with custom DNS infrastructure.",
		DocumentationURL: "https://charon.dev/docs/features/script-dns",
		IsBuiltIn:        false,
		Version:          "1.0.0",
		InterfaceVersion: dnsprovider.InterfaceVersion,
	}
}

// Init is called after the plugin is registered.
func (p *ScriptProvider) Init() error {
	return nil
}

// Cleanup is called before the plugin is unregistered.
func (p *ScriptProvider) Cleanup() error {
	return nil
}

// RequiredCredentialFields returns credential fields that must be provided.
func (p *ScriptProvider) RequiredCredentialFields() []dnsprovider.CredentialFieldSpec {
	return []dnsprovider.CredentialFieldSpec{
		{
			Name:        "script_path",
			Label:       "Script Path",
			Type:        "text",
			Placeholder: "/scripts/dns-challenge.sh",
			Hint:        "Path to the DNS challenge script. Must be located in /scripts/ directory. Script receives: action (create/delete), domain, token, and key_auth as arguments.",
		},
	}
}

// OptionalCredentialFields returns credential fields that may be provided.
func (p *ScriptProvider) OptionalCredentialFields() []dnsprovider.CredentialFieldSpec {
	return []dnsprovider.CredentialFieldSpec{
		{
			Name:        "timeout_seconds",
			Label:       "Script Timeout (seconds)",
			Type:        "text",
			Placeholder: strconv.Itoa(ScriptDefaultTimeoutSeconds),
			Hint:        fmt.Sprintf("Maximum execution time for the script (%d-%d seconds, default: %d)", ScriptMinTimeoutSeconds, ScriptMaxTimeoutSeconds, ScriptDefaultTimeoutSeconds),
		},
		{
			Name:        "env_vars",
			Label:       "Environment Variables",
			Type:        "textarea",
			Placeholder: "DNS_API_KEY=your-key\nDNS_API_URL=https://api.example.com",
			Hint:        "Optional environment variables passed to the script. One KEY=VALUE pair per line. Keys must start with a letter or underscore.",
		},
	}
}

// ValidateCredentials checks if the provided credentials are valid.
func (p *ScriptProvider) ValidateCredentials(creds map[string]string) error {
	// Validate required script path
	scriptPath := strings.TrimSpace(creds["script_path"])
	if scriptPath == "" {
		return fmt.Errorf("script_path is required")
	}

	// Validate script path for security
	if err := validateScriptPath(scriptPath); err != nil {
		return fmt.Errorf("script_path validation failed: %w", err)
	}

	// Validate timeout if provided
	if timeoutStr := strings.TrimSpace(creds["timeout_seconds"]); timeoutStr != "" {
		timeout, err := strconv.Atoi(timeoutStr)
		if err != nil {
			return fmt.Errorf("timeout_seconds must be a number: %w", err)
		}
		if timeout < ScriptMinTimeoutSeconds || timeout > ScriptMaxTimeoutSeconds {
			return fmt.Errorf("timeout_seconds must be between %d and %d", ScriptMinTimeoutSeconds, ScriptMaxTimeoutSeconds)
		}
	}

	// Validate environment variables if provided
	if envVars := strings.TrimSpace(creds["env_vars"]); envVars != "" {
		if err := validateEnvVars(envVars); err != nil {
			return fmt.Errorf("env_vars validation failed: %w", err)
		}
	}

	return nil
}

// validateScriptPath validates a script path for security.
// SECURITY: This function is critical for preventing path traversal attacks.
func validateScriptPath(scriptPath string) error {
	// Clean the path first to normalize it
	// SECURITY: filepath.Clean resolves ".." sequences, so "/scripts/../etc/passwd"
	// becomes "/etc/passwd" - the directory check below will then reject it.
	cleaned := filepath.Clean(scriptPath)

	// SECURITY: Must start with the allowed directory
	// This check catches path traversal because filepath.Clean already resolved ".."
	if !strings.HasPrefix(cleaned, ScriptAllowedDirectory) {
		return fmt.Errorf("script must be in %s directory, got: %s", ScriptAllowedDirectory, cleaned)
	}

	// SECURITY: Validate the path doesn't contain null bytes (common injection vector)
	if strings.ContainsRune(scriptPath, '\x00') {
		return fmt.Errorf("path contains invalid characters")
	}

	// SECURITY: Validate the filename portion doesn't start with a hyphen
	// (to prevent argument injection in shell commands)
	base := filepath.Base(cleaned)
	if strings.HasPrefix(base, "-") {
		return fmt.Errorf("script filename cannot start with hyphen")
	}

	// SECURITY: Validate the script name matches safe pattern
	if !scriptArgPattern.MatchString(base) {
		return fmt.Errorf("script filename contains invalid characters: only alphanumeric, dots, underscores, equals, and hyphens allowed")
	}

	return nil
}

// validateEnvVars validates environment variable format.
func validateEnvVars(envVars string) error {
	lines := strings.Split(envVars, "\n")

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue // Skip empty lines
		}

		// Validate format: KEY=VALUE
		if !strings.Contains(line, "=") {
			return fmt.Errorf("line %d: invalid format, expected KEY=VALUE", i+1)
		}

		// Validate the line matches the pattern
		if !envVarLinePattern.MatchString(line) {
			return fmt.Errorf("line %d: invalid environment variable format, key must start with letter or underscore", i+1)
		}

		// Extract and validate key
		parts := strings.SplitN(line, "=", 2)
		key := parts[0]

		// SECURITY: Prevent overriding critical environment variables
		criticalVars := []string{"PATH", "LD_PRELOAD", "LD_LIBRARY_PATH", "HOME", "USER", "SHELL"}
		for _, critical := range criticalVars {
			if strings.EqualFold(key, critical) {
				return fmt.Errorf("line %d: cannot override critical environment variable %q", i+1, key)
			}
		}
	}

	return nil
}

// parseEnvVars parses environment variable string into a map.
func parseEnvVars(envVars string) map[string]string {
	result := make(map[string]string)

	if envVars == "" {
		return result
	}

	lines := strings.Split(envVars, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}

	return result
}

// TestCredentials attempts to verify credentials work.
// For script provider, we validate the format but cannot test without executing the script.
func (p *ScriptProvider) TestCredentials(creds map[string]string) error {
	return p.ValidateCredentials(creds)
}

// SupportsMultiCredential indicates if the provider can handle zone-specific credentials.
func (p *ScriptProvider) SupportsMultiCredential() bool {
	return false
}

// BuildCaddyConfig constructs the Caddy DNS challenge configuration.
// For script, this returns a config that Charon's internal script handler will use.
func (p *ScriptProvider) BuildCaddyConfig(creds map[string]string) map[string]any {
	scriptPath := strings.TrimSpace(creds["script_path"])

	config := map[string]any{
		"name":        "script",
		"script_path": filepath.Clean(scriptPath),
	}

	// Add timeout with default
	timeoutSeconds := ScriptDefaultTimeoutSeconds
	if timeoutStr := strings.TrimSpace(creds["timeout_seconds"]); timeoutStr != "" {
		if t, err := strconv.Atoi(timeoutStr); err == nil && t >= ScriptMinTimeoutSeconds && t <= ScriptMaxTimeoutSeconds {
			timeoutSeconds = t
		}
	}
	config["timeout_seconds"] = timeoutSeconds

	// Add environment variables if provided
	if envVars := strings.TrimSpace(creds["env_vars"]); envVars != "" {
		config["env_vars"] = parseEnvVars(envVars)
	} else {
		config["env_vars"] = map[string]string{}
	}

	return config
}

// BuildCaddyConfigForZone constructs config for a specific zone.
func (p *ScriptProvider) BuildCaddyConfigForZone(baseDomain string, creds map[string]string) map[string]any {
	return p.BuildCaddyConfig(creds)
}

// PropagationTimeout returns the recommended DNS propagation wait time.
func (p *ScriptProvider) PropagationTimeout() time.Duration {
	return p.propagationTimeout
}

// PollingInterval returns the recommended polling interval for DNS verification.
func (p *ScriptProvider) PollingInterval() time.Duration {
	return p.pollingInterval
}

// GetTimeoutSeconds returns the configured timeout in seconds or the default.
func (p *ScriptProvider) GetTimeoutSeconds(creds map[string]string) int {
	if timeoutStr := strings.TrimSpace(creds["timeout_seconds"]); timeoutStr != "" {
		if timeout, err := strconv.Atoi(timeoutStr); err == nil {
			if timeout >= ScriptMinTimeoutSeconds && timeout <= ScriptMaxTimeoutSeconds {
				return timeout
			}
		}
	}
	return ScriptDefaultTimeoutSeconds
}

// GetEnvVars returns the parsed environment variables from credentials.
func (p *ScriptProvider) GetEnvVars(creds map[string]string) map[string]string {
	envVars := strings.TrimSpace(creds["env_vars"])
	return parseEnvVars(envVars)
}
