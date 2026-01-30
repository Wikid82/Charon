package custom

import (
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/pkg/dnsprovider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewScriptProvider(t *testing.T) {
	provider := NewScriptProvider()

	require.NotNil(t, provider)
	assert.Equal(t, ScriptDefaultPropagationTimeout, provider.propagationTimeout)
	assert.Equal(t, ScriptDefaultPollingInterval, provider.pollingInterval)
}

func TestScriptProvider_Type(t *testing.T) {
	provider := NewScriptProvider()
	assert.Equal(t, "script", provider.Type())
}

func TestScriptProvider_Metadata(t *testing.T) {
	provider := NewScriptProvider()
	metadata := provider.Metadata()

	assert.Equal(t, "script", metadata.Type)
	assert.Equal(t, "Script (Shell)", metadata.Name)
	assert.Contains(t, metadata.Description, "ADVANCED")
	assert.Contains(t, metadata.Description, "HIGH-RISK")
	assert.Contains(t, metadata.Description, "/scripts/")
	assert.NotEmpty(t, metadata.DocumentationURL)
	assert.False(t, metadata.IsBuiltIn)
	assert.Equal(t, "1.0.0", metadata.Version)
	assert.Equal(t, dnsprovider.InterfaceVersion, metadata.InterfaceVersion)
}

func TestScriptProvider_Init(t *testing.T) {
	provider := NewScriptProvider()
	err := provider.Init()
	assert.NoError(t, err)
}

func TestScriptProvider_Cleanup(t *testing.T) {
	provider := NewScriptProvider()
	err := provider.Cleanup()
	assert.NoError(t, err)
}

func TestScriptProvider_RequiredCredentialFields(t *testing.T) {
	provider := NewScriptProvider()
	fields := provider.RequiredCredentialFields()

	require.Len(t, fields, 1)

	field := fields[0]
	assert.Equal(t, "script_path", field.Name)
	assert.Equal(t, "Script Path", field.Label)
	assert.Equal(t, "text", field.Type)
	assert.NotEmpty(t, field.Placeholder)
	assert.Contains(t, field.Hint, "/scripts/")
}

func TestScriptProvider_OptionalCredentialFields(t *testing.T) {
	provider := NewScriptProvider()
	fields := provider.OptionalCredentialFields()

	expectedFields := map[string]bool{
		"timeout_seconds": false,
		"env_vars":        false,
	}

	assert.Len(t, fields, len(expectedFields))

	for _, field := range fields {
		if _, ok := expectedFields[field.Name]; !ok {
			t.Errorf("Unexpected optional field: %q", field.Name)
		}
		expectedFields[field.Name] = true

		assert.NotEmpty(t, field.Label, "Field %q has empty label", field.Name)
		assert.NotEmpty(t, field.Type, "Field %q has empty type", field.Name)

		// Verify env_vars is textarea type
		if field.Name == "env_vars" {
			assert.Equal(t, "textarea", field.Type, "env_vars should be textarea type")
		}
	}

	for name, found := range expectedFields {
		if !found {
			t.Errorf("Missing optional field: %q", name)
		}
	}
}

func TestScriptProvider_ValidateCredentials(t *testing.T) {
	provider := NewScriptProvider()

	tests := []struct {
		name    string
		creds   map[string]string
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid credentials minimal",
			creds: map[string]string{
				"script_path": "/scripts/dns-challenge.sh",
			},
			wantErr: false,
		},
		{
			name: "valid credentials with all options",
			creds: map[string]string{
				"script_path":     "/scripts/dns-challenge.sh",
				"timeout_seconds": "120",
				"env_vars":        "API_KEY=secret123\nAPI_URL=https://api.example.com",
			},
			wantErr: false,
		},
		{
			name: "valid credentials with nested directory",
			creds: map[string]string{
				"script_path": "/scripts/dns/acme/challenge.sh",
			},
			wantErr: false,
		},
		{
			name: "valid credentials with whitespace trimming",
			creds: map[string]string{
				"script_path": "  /scripts/dns-challenge.sh  ",
			},
			wantErr: false,
		},
		{
			name:    "missing script_path",
			creds:   map[string]string{},
			wantErr: true,
			errMsg:  "script_path is required",
		},
		{
			name: "empty script_path",
			creds: map[string]string{
				"script_path": "",
			},
			wantErr: true,
			errMsg:  "script_path is required",
		},
		{
			name: "whitespace-only script_path",
			creds: map[string]string{
				"script_path": "   ",
			},
			wantErr: true,
			errMsg:  "script_path is required",
		},
		// Path traversal attacks - caught via directory prefix check after path normalization
		{
			name: "path traversal with ..",
			creds: map[string]string{
				"script_path": "/scripts/../etc/passwd",
			},
			wantErr: true,
			errMsg:  "script must be in /scripts/ directory",
		},
		{
			name: "path traversal at end",
			creds: map[string]string{
				"script_path": "/scripts/dns/../../../etc/passwd",
			},
			wantErr: true,
			errMsg:  "script must be in /scripts/ directory",
		},
		{
			name: "script outside allowed directory",
			creds: map[string]string{
				"script_path": "/etc/passwd",
			},
			wantErr: true,
			errMsg:  "script must be in /scripts/ directory",
		},
		{
			name: "script in home directory",
			creds: map[string]string{
				"script_path": "/home/user/script.sh",
			},
			wantErr: true,
			errMsg:  "script must be in /scripts/ directory",
		},
		{
			name: "script at root",
			creds: map[string]string{
				"script_path": "/script.sh",
			},
			wantErr: true,
			errMsg:  "script must be in /scripts/ directory",
		},
		{
			name: "relative path",
			creds: map[string]string{
				"script_path": "scripts/dns-challenge.sh",
			},
			wantErr: true,
			errMsg:  "script must be in /scripts/ directory",
		},
		{
			name: "script filename with hyphen prefix",
			creds: map[string]string{
				"script_path": "/scripts/-rf",
			},
			wantErr: true,
			errMsg:  "script filename cannot start with hyphen",
		},
		{
			name: "script filename with special characters",
			creds: map[string]string{
				"script_path": "/scripts/script;rm -rf /",
			},
			wantErr: true,
			errMsg:  "script filename contains invalid characters",
		},
		{
			name: "script filename with spaces",
			creds: map[string]string{
				"script_path": "/scripts/my script.sh",
			},
			wantErr: true,
			errMsg:  "script filename contains invalid characters",
		},
		// Timeout validation
		{
			name: "timeout_seconds not a number",
			creds: map[string]string{
				"script_path":     "/scripts/dns-challenge.sh",
				"timeout_seconds": "abc",
			},
			wantErr: true,
			errMsg:  "timeout_seconds must be a number",
		},
		{
			name: "timeout_seconds too low",
			creds: map[string]string{
				"script_path":     "/scripts/dns-challenge.sh",
				"timeout_seconds": "1",
			},
			wantErr: true,
			errMsg:  "timeout_seconds must be between",
		},
		{
			name: "timeout_seconds too high",
			creds: map[string]string{
				"script_path":     "/scripts/dns-challenge.sh",
				"timeout_seconds": "500",
			},
			wantErr: true,
			errMsg:  "timeout_seconds must be between",
		},
		{
			name: "valid min timeout",
			creds: map[string]string{
				"script_path":     "/scripts/dns-challenge.sh",
				"timeout_seconds": "5",
			},
			wantErr: false,
		},
		{
			name: "valid max timeout",
			creds: map[string]string{
				"script_path":     "/scripts/dns-challenge.sh",
				"timeout_seconds": "300",
			},
			wantErr: false,
		},
		// Environment variable validation
		{
			name: "env_vars invalid format no equals",
			creds: map[string]string{
				"script_path": "/scripts/dns-challenge.sh",
				"env_vars":    "INVALID_VAR",
			},
			wantErr: true,
			errMsg:  "invalid format, expected KEY=VALUE",
		},
		{
			name: "env_vars key starting with number",
			creds: map[string]string{
				"script_path": "/scripts/dns-challenge.sh",
				"env_vars":    "1INVALID=value",
			},
			wantErr: true,
			errMsg:  "invalid environment variable format",
		},
		{
			name: "env_vars override PATH",
			creds: map[string]string{
				"script_path": "/scripts/dns-challenge.sh",
				"env_vars":    "PATH=/malicious/path",
			},
			wantErr: true,
			errMsg:  "cannot override critical environment variable",
		},
		{
			name: "env_vars override LD_PRELOAD",
			creds: map[string]string{
				"script_path": "/scripts/dns-challenge.sh",
				"env_vars":    "LD_PRELOAD=/malicious/lib.so",
			},
			wantErr: true,
			errMsg:  "cannot override critical environment variable",
		},
		{
			name: "env_vars override HOME (case insensitive)",
			creds: map[string]string{
				"script_path": "/scripts/dns-challenge.sh",
				"env_vars":    "home=/tmp",
			},
			wantErr: true,
			errMsg:  "cannot override critical environment variable",
		},
		{
			name: "env_vars with empty lines",
			creds: map[string]string{
				"script_path": "/scripts/dns-challenge.sh",
				"env_vars":    "API_KEY=secret\n\nAPI_URL=https://example.com\n",
			},
			wantErr: false,
		},
		{
			name: "env_vars with underscore prefix",
			creds: map[string]string{
				"script_path": "/scripts/dns-challenge.sh",
				"env_vars":    "_PRIVATE_VAR=value",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := provider.ValidateCredentials(tt.creds)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestScriptProvider_ValidateScriptPath(t *testing.T) {
	tests := []struct {
		name       string
		scriptPath string
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "valid path",
			scriptPath: "/scripts/dns-challenge.sh",
			wantErr:    false,
		},
		{
			name:       "valid nested path",
			scriptPath: "/scripts/acme/dns/challenge.sh",
			wantErr:    false,
		},
		{
			name:       "valid path with dots in filename",
			scriptPath: "/scripts/dns.challenge.v1.sh",
			wantErr:    false,
		},
		{
			name:       "valid path with underscore",
			scriptPath: "/scripts/dns_challenge.sh",
			wantErr:    false,
		},
		{
			name:       "valid path with hyphen",
			scriptPath: "/scripts/dns-challenge.sh",
			wantErr:    false,
		},
		{
			name:       "path traversal basic",
			scriptPath: "/scripts/../etc/passwd",
			wantErr:    true,
			errMsg:     "script must be in /scripts/ directory",
		},
		{
			name:       "path traversal complex",
			scriptPath: "/scripts/subdir/../../../etc/passwd",
			wantErr:    true,
			errMsg:     "script must be in /scripts/ directory",
		},
		{
			name:       "outside allowed directory",
			scriptPath: "/etc/passwd",
			wantErr:    true,
			errMsg:     "script must be in /scripts/ directory",
		},
		{
			name:       "script with hyphen prefix filename",
			scriptPath: "/scripts/-help",
			wantErr:    true,
			errMsg:     "script filename cannot start with hyphen",
		},
		{
			name:       "script with shell metacharacters",
			scriptPath: "/scripts/script$(whoami).sh",
			wantErr:    true,
			errMsg:     "script filename contains invalid characters",
		},
		{
			name:       "script with backtick",
			scriptPath: "/scripts/script`id`.sh",
			wantErr:    true,
			errMsg:     "script filename contains invalid characters",
		},
		{
			name:       "script with pipe",
			scriptPath: "/scripts/script|cat.sh",
			wantErr:    true,
			errMsg:     "script filename contains invalid characters",
		},
		{
			name:       "script with semicolon",
			scriptPath: "/scripts/script;ls.sh",
			wantErr:    true,
			errMsg:     "script filename contains invalid characters",
		},
		{
			name:       "script with ampersand",
			scriptPath: "/scripts/script&bg.sh",
			wantErr:    true,
			errMsg:     "script filename contains invalid characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateScriptPath(tt.scriptPath)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestScriptProvider_ValidateEnvVars(t *testing.T) {
	tests := []struct {
		name    string
		envVars string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty string",
			envVars: "",
			wantErr: false,
		},
		{
			name:    "single valid variable",
			envVars: "API_KEY=secret123",
			wantErr: false,
		},
		{
			name:    "multiple valid variables",
			envVars: "API_KEY=secret123\nAPI_URL=https://api.example.com",
			wantErr: false,
		},
		{
			name:    "variable with empty value",
			envVars: "EMPTY_VAR=",
			wantErr: false,
		},
		{
			name:    "variable starting with underscore",
			envVars: "_PRIVATE=value",
			wantErr: false,
		},
		{
			name:    "empty lines are ignored",
			envVars: "VAR1=val1\n\n\nVAR2=val2",
			wantErr: false,
		},
		{
			name:    "whitespace lines are ignored",
			envVars: "VAR1=val1\n   \nVAR2=val2",
			wantErr: false,
		},
		{
			name:    "missing equals sign",
			envVars: "INVALID_VAR",
			wantErr: true,
			errMsg:  "invalid format, expected KEY=VALUE",
		},
		{
			name:    "key starting with number",
			envVars: "1INVALID=value",
			wantErr: true,
			errMsg:  "invalid environment variable format",
		},
		{
			name:    "key with special characters",
			envVars: "INVALID-KEY=value",
			wantErr: true,
			errMsg:  "invalid environment variable format",
		},
		{
			name:    "override PATH",
			envVars: "PATH=/malicious",
			wantErr: true,
			errMsg:  "cannot override critical environment variable",
		},
		{
			name:    "override LD_PRELOAD",
			envVars: "LD_PRELOAD=/lib.so",
			wantErr: true,
			errMsg:  "cannot override critical environment variable",
		},
		{
			name:    "override LD_LIBRARY_PATH",
			envVars: "LD_LIBRARY_PATH=/lib",
			wantErr: true,
			errMsg:  "cannot override critical environment variable",
		},
		{
			name:    "override HOME",
			envVars: "HOME=/tmp",
			wantErr: true,
			errMsg:  "cannot override critical environment variable",
		},
		{
			name:    "override USER",
			envVars: "USER=root",
			wantErr: true,
			errMsg:  "cannot override critical environment variable",
		},
		{
			name:    "override SHELL",
			envVars: "SHELL=/bin/bash",
			wantErr: true,
			errMsg:  "cannot override critical environment variable",
		},
		{
			name:    "case insensitive critical var check",
			envVars: "path=/malicious",
			wantErr: true,
			errMsg:  "cannot override critical environment variable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEnvVars(tt.envVars)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestScriptProvider_ParseEnvVars(t *testing.T) {
	tests := []struct {
		name     string
		envVars  string
		expected map[string]string
	}{
		{
			name:     "empty string",
			envVars:  "",
			expected: map[string]string{},
		},
		{
			name:    "single variable",
			envVars: "API_KEY=secret123",
			expected: map[string]string{
				"API_KEY": "secret123",
			},
		},
		{
			name:    "multiple variables",
			envVars: "API_KEY=secret123\nAPI_URL=https://api.example.com",
			expected: map[string]string{
				"API_KEY": "secret123",
				"API_URL": "https://api.example.com",
			},
		},
		{
			name:    "variable with empty value",
			envVars: "EMPTY_VAR=",
			expected: map[string]string{
				"EMPTY_VAR": "",
			},
		},
		{
			name:    "variable with equals in value",
			envVars: "CONNECTION=host=localhost;port=5432",
			expected: map[string]string{
				"CONNECTION": "host=localhost;port=5432",
			},
		},
		{
			name:    "skip empty lines",
			envVars: "VAR1=val1\n\nVAR2=val2",
			expected: map[string]string{
				"VAR1": "val1",
				"VAR2": "val2",
			},
		},
		{
			name:    "trim whitespace",
			envVars: "  VAR1=val1  \n  VAR2=val2  ",
			expected: map[string]string{
				"VAR1": "val1",
				"VAR2": "val2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseEnvVars(tt.envVars)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestScriptProvider_TestCredentials(t *testing.T) {
	provider := NewScriptProvider()

	// TestCredentials should behave the same as ValidateCredentials
	validCreds := map[string]string{
		"script_path": "/scripts/dns-challenge.sh",
	}

	err := provider.TestCredentials(validCreds)
	assert.NoError(t, err)

	invalidCreds := map[string]string{
		"script_path": "/etc/passwd",
	}

	err = provider.TestCredentials(invalidCreds)
	assert.Error(t, err)
}

func TestScriptProvider_SupportsMultiCredential(t *testing.T) {
	provider := NewScriptProvider()
	assert.False(t, provider.SupportsMultiCredential())
}

func TestScriptProvider_BuildCaddyConfig(t *testing.T) {
	provider := NewScriptProvider()

	tests := []struct {
		name     string
		creds    map[string]string
		expected map[string]any
	}{
		{
			name: "minimal config with defaults",
			creds: map[string]string{
				"script_path": "/scripts/dns-challenge.sh",
			},
			expected: map[string]any{
				"name":            "script",
				"script_path":     "/scripts/dns-challenge.sh",
				"timeout_seconds": ScriptDefaultTimeoutSeconds,
				"env_vars":        map[string]string{},
			},
		},
		{
			name: "full config with all options",
			creds: map[string]string{
				"script_path":     "/scripts/dns-challenge.sh",
				"timeout_seconds": "120",
				"env_vars":        "API_KEY=secret\nAPI_URL=https://example.com",
			},
			expected: map[string]any{
				"name":            "script",
				"script_path":     "/scripts/dns-challenge.sh",
				"timeout_seconds": 120,
				"env_vars": map[string]string{
					"API_KEY": "secret",
					"API_URL": "https://example.com",
				},
			},
		},
		{
			name: "whitespace trimming",
			creds: map[string]string{
				"script_path":     "  /scripts/dns-challenge.sh  ",
				"timeout_seconds": "  90  ",
			},
			expected: map[string]any{
				"name":            "script",
				"script_path":     "/scripts/dns-challenge.sh",
				"timeout_seconds": 90,
				"env_vars":        map[string]string{},
			},
		},
		{
			name: "invalid timeout falls back to default",
			creds: map[string]string{
				"script_path":     "/scripts/dns-challenge.sh",
				"timeout_seconds": "invalid",
			},
			expected: map[string]any{
				"name":            "script",
				"script_path":     "/scripts/dns-challenge.sh",
				"timeout_seconds": ScriptDefaultTimeoutSeconds,
				"env_vars":        map[string]string{},
			},
		},
		{
			name: "out-of-range timeout falls back to default",
			creds: map[string]string{
				"script_path":     "/scripts/dns-challenge.sh",
				"timeout_seconds": "1000",
			},
			expected: map[string]any{
				"name":            "script",
				"script_path":     "/scripts/dns-challenge.sh",
				"timeout_seconds": ScriptDefaultTimeoutSeconds,
				"env_vars":        map[string]string{},
			},
		},
		{
			name: "path normalization",
			creds: map[string]string{
				"script_path": "/scripts/./subdir/../dns-challenge.sh",
			},
			expected: map[string]any{
				"name":            "script",
				"script_path":     "/scripts/dns-challenge.sh",
				"timeout_seconds": ScriptDefaultTimeoutSeconds,
				"env_vars":        map[string]string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := provider.BuildCaddyConfig(tt.creds)

			for key, expectedValue := range tt.expected {
				actualValue, ok := config[key]
				if !ok {
					t.Errorf("BuildCaddyConfig() missing key %q", key)
					continue
				}
				assert.Equal(t, expectedValue, actualValue, "BuildCaddyConfig()[%q] mismatch", key)
			}

			// Check no unexpected keys
			for key := range config {
				if _, ok := tt.expected[key]; !ok {
					t.Errorf("BuildCaddyConfig() unexpected key %q", key)
				}
			}
		})
	}
}

func TestScriptProvider_BuildCaddyConfigForZone(t *testing.T) {
	provider := NewScriptProvider()

	creds := map[string]string{
		"script_path": "/scripts/dns-challenge.sh",
	}

	config := provider.BuildCaddyConfigForZone("example.org", creds)

	// Should return same as BuildCaddyConfig since multi-credential is not supported
	assert.Equal(t, "script", config["name"])
	assert.Equal(t, "/scripts/dns-challenge.sh", config["script_path"])
}

func TestScriptProvider_PropagationTimeout(t *testing.T) {
	provider := NewScriptProvider()
	timeout := provider.PropagationTimeout()

	assert.Equal(t, 120*time.Second, timeout)
}

func TestScriptProvider_PollingInterval(t *testing.T) {
	provider := NewScriptProvider()
	interval := provider.PollingInterval()

	assert.Equal(t, 5*time.Second, interval)
}

func TestScriptProvider_GetTimeoutSeconds(t *testing.T) {
	provider := NewScriptProvider()

	tests := []struct {
		name     string
		creds    map[string]string
		expected int
	}{
		{
			name:     "empty creds returns default",
			creds:    map[string]string{},
			expected: ScriptDefaultTimeoutSeconds,
		},
		{
			name:     "valid timeout returns value",
			creds:    map[string]string{"timeout_seconds": "90"},
			expected: 90,
		},
		{
			name:     "invalid number returns default",
			creds:    map[string]string{"timeout_seconds": "abc"},
			expected: ScriptDefaultTimeoutSeconds,
		},
		{
			name:     "out of range low returns default",
			creds:    map[string]string{"timeout_seconds": "1"},
			expected: ScriptDefaultTimeoutSeconds,
		},
		{
			name:     "out of range high returns default",
			creds:    map[string]string{"timeout_seconds": "500"},
			expected: ScriptDefaultTimeoutSeconds,
		},
		{
			name:     "min value returns value",
			creds:    map[string]string{"timeout_seconds": "5"},
			expected: 5,
		},
		{
			name:     "max value returns value",
			creds:    map[string]string{"timeout_seconds": "300"},
			expected: 300,
		},
		{
			name:     "with whitespace",
			creds:    map[string]string{"timeout_seconds": "  60  "},
			expected: 60,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.GetTimeoutSeconds(tt.creds)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestScriptProvider_GetEnvVars(t *testing.T) {
	provider := NewScriptProvider()

	tests := []struct {
		name     string
		creds    map[string]string
		expected map[string]string
	}{
		{
			name:     "empty creds returns empty map",
			creds:    map[string]string{},
			expected: map[string]string{},
		},
		{
			name:     "empty env_vars returns empty map",
			creds:    map[string]string{"env_vars": ""},
			expected: map[string]string{},
		},
		{
			name:  "single variable",
			creds: map[string]string{"env_vars": "API_KEY=secret"},
			expected: map[string]string{
				"API_KEY": "secret",
			},
		},
		{
			name:  "multiple variables",
			creds: map[string]string{"env_vars": "KEY1=val1\nKEY2=val2"},
			expected: map[string]string{
				"KEY1": "val1",
				"KEY2": "val2",
			},
		},
		{
			name:     "whitespace only returns empty map",
			creds:    map[string]string{"env_vars": "   \n   "},
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.GetEnvVars(tt.creds)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestScriptProvider_Constants(t *testing.T) {
	// Verify constant values are sensible
	assert.Equal(t, 60, ScriptDefaultTimeoutSeconds)
	assert.Equal(t, 120*time.Second, ScriptDefaultPropagationTimeout)
	assert.Equal(t, 5*time.Second, ScriptDefaultPollingInterval)
	assert.Equal(t, 5, ScriptMinTimeoutSeconds)
	assert.Equal(t, 300, ScriptMaxTimeoutSeconds)
	assert.Equal(t, "/scripts/", ScriptAllowedDirectory)

	// Ensure min < default < max for timeout
	assert.Less(t, ScriptMinTimeoutSeconds, ScriptDefaultTimeoutSeconds)
	assert.Less(t, ScriptDefaultTimeoutSeconds, ScriptMaxTimeoutSeconds)
}

func TestScriptProvider_ImplementsInterface(t *testing.T) {
	provider := NewScriptProvider()

	// Compile-time check that ScriptProvider implements ProviderPlugin
	var _ dnsprovider.ProviderPlugin = provider
}

func TestScriptProvider_SecurityPatterns(t *testing.T) {
	// Test the regex patterns used for security validation

	t.Run("scriptArgPattern", func(t *testing.T) {
		validNames := []string{
			"script.sh",
			"dns-challenge.sh",
			"dns_challenge_v1.sh",
			"CHALLENGE.SH",
			"script123.sh",
			"a.b.c.d.sh",
		}

		for _, name := range validNames {
			assert.True(t, scriptArgPattern.MatchString(name), "Expected %q to match", name)
		}

		invalidNames := []string{
			"script$(id).sh",
			"script`id`.sh",
			"script;rm.sh",
			"script|cat.sh",
			"script&bg.sh",
			"script>out.sh",
			"script<in.sh",
			"script\\n.sh",
			"script .sh",
			"script\t.sh",
			"script'quote.sh",
			"script\"quote.sh",
		}

		for _, name := range invalidNames {
			assert.False(t, scriptArgPattern.MatchString(name), "Expected %q to NOT match", name)
		}
	})

	t.Run("envVarLinePattern", func(t *testing.T) {
		validLines := []string{
			"VAR=value",
			"_VAR=value",
			"VAR123=value",
			"_VAR_123=value",
			"VAR=",
			"VAR=value=with=equals",
			"var=lowercase",
		}

		for _, line := range validLines {
			assert.True(t, envVarLinePattern.MatchString(line), "Expected %q to match", line)
		}

		invalidLines := []string{
			"123VAR=value",
			"-VAR=value",
			"VAR-NAME=value",
			"VAR.NAME=value",
			"VAR NAME=value",
		}

		for _, line := range invalidLines {
			assert.False(t, envVarLinePattern.MatchString(line), "Expected %q to NOT match", line)
		}
	})
}
