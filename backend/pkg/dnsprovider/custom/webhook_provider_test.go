package custom

import (
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/pkg/dnsprovider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWebhookProvider(t *testing.T) {
	provider := NewWebhookProvider()

	require.NotNil(t, provider)
	assert.Equal(t, WebhookDefaultPropagationTimeout, provider.propagationTimeout)
	assert.Equal(t, WebhookDefaultPollingInterval, provider.pollingInterval)
}

func TestWebhookProvider_Type(t *testing.T) {
	provider := NewWebhookProvider()
	assert.Equal(t, "webhook", provider.Type())
}

func TestWebhookProvider_Metadata(t *testing.T) {
	provider := NewWebhookProvider()
	metadata := provider.Metadata()

	assert.Equal(t, "webhook", metadata.Type)
	assert.Equal(t, "Webhook (HTTP)", metadata.Name)
	assert.Contains(t, metadata.Description, "HTTP webhook")
	assert.NotEmpty(t, metadata.DocumentationURL)
	assert.False(t, metadata.IsBuiltIn)
	assert.Equal(t, "1.0.0", metadata.Version)
	assert.Equal(t, dnsprovider.InterfaceVersion, metadata.InterfaceVersion)
}

func TestWebhookProvider_Init(t *testing.T) {
	provider := NewWebhookProvider()
	err := provider.Init()
	assert.NoError(t, err)
}

func TestWebhookProvider_Cleanup(t *testing.T) {
	provider := NewWebhookProvider()
	err := provider.Cleanup()
	assert.NoError(t, err)
}

func TestWebhookProvider_RequiredCredentialFields(t *testing.T) {
	provider := NewWebhookProvider()
	fields := provider.RequiredCredentialFields()

	expectedFields := map[string]bool{
		"create_url": false,
		"delete_url": false,
	}

	assert.Len(t, fields, len(expectedFields))

	for _, field := range fields {
		if _, ok := expectedFields[field.Name]; !ok {
			t.Errorf("Unexpected required field: %q", field.Name)
		}
		expectedFields[field.Name] = true

		assert.NotEmpty(t, field.Label, "Field %q has empty label", field.Name)
		assert.NotEmpty(t, field.Type, "Field %q has empty type", field.Name)
		assert.NotEmpty(t, field.Hint, "Field %q has empty hint", field.Name)
	}

	for name, found := range expectedFields {
		if !found {
			t.Errorf("Missing required field: %q", name)
		}
	}
}

func TestWebhookProvider_OptionalCredentialFields(t *testing.T) {
	provider := NewWebhookProvider()
	fields := provider.OptionalCredentialFields()

	expectedFields := map[string]bool{
		"auth_header":          false,
		"auth_value":           false,
		"timeout_seconds":      false,
		"retry_count":          false,
		"insecure_skip_verify": false,
	}

	assert.Len(t, fields, len(expectedFields))

	for _, field := range fields {
		if _, ok := expectedFields[field.Name]; !ok {
			t.Errorf("Unexpected optional field: %q", field.Name)
		}
		expectedFields[field.Name] = true

		assert.NotEmpty(t, field.Label, "Field %q has empty label", field.Name)

		// Verify auth_value is password type
		if field.Name == "auth_value" {
			assert.Equal(t, "password", field.Type, "auth_value should be password type")
		}

		// Verify insecure_skip_verify has select options
		if field.Name == "insecure_skip_verify" {
			assert.Equal(t, "select", field.Type, "insecure_skip_verify should be select type")
			assert.Len(t, field.Options, 2, "insecure_skip_verify should have 2 options")
			assert.Contains(t, field.Hint, "DEVELOPMENT ONLY", "insecure_skip_verify should warn about dev-only usage")
		}
	}

	for name, found := range expectedFields {
		if !found {
			t.Errorf("Missing optional field: %q", name)
		}
	}
}

func TestWebhookProvider_ValidateCredentials(t *testing.T) {
	provider := NewWebhookProvider()

	tests := []struct {
		name    string
		creds   map[string]string
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid credentials with HTTPS URLs",
			creds: map[string]string{
				"create_url": "https://dns-api.example.com/create",
				"delete_url": "https://dns-api.example.com/delete",
			},
			wantErr: false,
		},
		{
			name: "valid credentials with localhost HTTP",
			creds: map[string]string{
				"create_url": "http://localhost:8080/create",
				"delete_url": "http://localhost:8080/delete",
			},
			wantErr: false,
		},
		{
			name: "valid credentials with 127.0.0.1 HTTP",
			creds: map[string]string{
				"create_url": "http://127.0.0.1:8080/create",
				"delete_url": "http://127.0.0.1:8080/delete",
			},
			wantErr: false,
		},
		{
			name: "valid credentials with all optional fields",
			creds: map[string]string{
				"create_url":           "https://dns-api.example.com/create",
				"delete_url":           "https://dns-api.example.com/delete",
				"auth_header":          "Authorization",
				"auth_value":           "Bearer token123",
				"timeout_seconds":      "60",
				"retry_count":          "5",
				"insecure_skip_verify": "false",
			},
			wantErr: false,
		},
		{
			name: "valid credentials with whitespace trimming",
			creds: map[string]string{
				"create_url": "  https://dns-api.example.com/create  ",
				"delete_url": "  https://dns-api.example.com/delete  ",
			},
			wantErr: false,
		},
		{
			name: "missing create_url",
			creds: map[string]string{
				"delete_url": "https://dns-api.example.com/delete",
			},
			wantErr: true,
			errMsg:  "create_url is required",
		},
		{
			name: "empty create_url",
			creds: map[string]string{
				"create_url": "",
				"delete_url": "https://dns-api.example.com/delete",
			},
			wantErr: true,
			errMsg:  "create_url is required",
		},
		{
			name: "whitespace-only create_url",
			creds: map[string]string{
				"create_url": "   ",
				"delete_url": "https://dns-api.example.com/delete",
			},
			wantErr: true,
			errMsg:  "create_url is required",
		},
		{
			name: "missing delete_url",
			creds: map[string]string{
				"create_url": "https://dns-api.example.com/create",
			},
			wantErr: true,
			errMsg:  "delete_url is required",
		},
		{
			name: "empty delete_url",
			creds: map[string]string{
				"create_url": "https://dns-api.example.com/create",
				"delete_url": "",
			},
			wantErr: true,
			errMsg:  "delete_url is required",
		},
		{
			name: "invalid create_url format",
			creds: map[string]string{
				"create_url": "not-a-valid-url",
				"delete_url": "https://dns-api.example.com/delete",
			},
			wantErr: true,
			errMsg:  "create_url",
		},
		{
			name: "create_url with ftp scheme",
			creds: map[string]string{
				"create_url": "ftp://dns-api.example.com/create",
				"delete_url": "https://dns-api.example.com/delete",
			},
			wantErr: true,
			errMsg:  "must use http or https scheme",
		},
		{
			name: "HTTP scheme for non-localhost",
			creds: map[string]string{
				"create_url": "http://dns-api.example.com/create",
				"delete_url": "https://dns-api.example.com/delete",
			},
			wantErr: true,
			errMsg:  "must use HTTPS for non-localhost",
		},
		{
			name: "timeout_seconds not a number",
			creds: map[string]string{
				"create_url":      "https://dns-api.example.com/create",
				"delete_url":      "https://dns-api.example.com/delete",
				"timeout_seconds": "abc",
			},
			wantErr: true,
			errMsg:  "timeout_seconds must be a number",
		},
		{
			name: "timeout_seconds too low",
			creds: map[string]string{
				"create_url":      "https://dns-api.example.com/create",
				"delete_url":      "https://dns-api.example.com/delete",
				"timeout_seconds": "1",
			},
			wantErr: true,
			errMsg:  "timeout_seconds must be between",
		},
		{
			name: "timeout_seconds too high",
			creds: map[string]string{
				"create_url":      "https://dns-api.example.com/create",
				"delete_url":      "https://dns-api.example.com/delete",
				"timeout_seconds": "500",
			},
			wantErr: true,
			errMsg:  "timeout_seconds must be between",
		},
		{
			name: "retry_count not a number",
			creds: map[string]string{
				"create_url":  "https://dns-api.example.com/create",
				"delete_url":  "https://dns-api.example.com/delete",
				"retry_count": "abc",
			},
			wantErr: true,
			errMsg:  "retry_count must be a number",
		},
		{
			name: "retry_count negative",
			creds: map[string]string{
				"create_url":  "https://dns-api.example.com/create",
				"delete_url":  "https://dns-api.example.com/delete",
				"retry_count": "-1",
			},
			wantErr: true,
			errMsg:  "retry_count must be between",
		},
		{
			name: "retry_count too high",
			creds: map[string]string{
				"create_url":  "https://dns-api.example.com/create",
				"delete_url":  "https://dns-api.example.com/delete",
				"retry_count": "20",
			},
			wantErr: true,
			errMsg:  "retry_count must be between",
		},
		{
			name: "insecure_skip_verify invalid value",
			creds: map[string]string{
				"create_url":           "https://dns-api.example.com/create",
				"delete_url":           "https://dns-api.example.com/delete",
				"insecure_skip_verify": "maybe",
			},
			wantErr: true,
			errMsg:  "insecure_skip_verify must be 'true' or 'false'",
		},
		{
			name: "auth_header without auth_value",
			creds: map[string]string{
				"create_url":  "https://dns-api.example.com/create",
				"delete_url":  "https://dns-api.example.com/delete",
				"auth_header": "Authorization",
			},
			wantErr: true,
			errMsg:  "both auth_header and auth_value must be provided together",
		},
		{
			name: "auth_value without auth_header",
			creds: map[string]string{
				"create_url": "https://dns-api.example.com/create",
				"delete_url": "https://dns-api.example.com/delete",
				"auth_value": "Bearer token",
			},
			wantErr: true,
			errMsg:  "both auth_header and auth_value must be provided together",
		},
		{
			name: "valid min timeout",
			creds: map[string]string{
				"create_url":      "https://dns-api.example.com/create",
				"delete_url":      "https://dns-api.example.com/delete",
				"timeout_seconds": "5",
			},
			wantErr: false,
		},
		{
			name: "valid max timeout",
			creds: map[string]string{
				"create_url":      "https://dns-api.example.com/create",
				"delete_url":      "https://dns-api.example.com/delete",
				"timeout_seconds": "300",
			},
			wantErr: false,
		},
		{
			name: "valid min retry count",
			creds: map[string]string{
				"create_url":  "https://dns-api.example.com/create",
				"delete_url":  "https://dns-api.example.com/delete",
				"retry_count": "0",
			},
			wantErr: false,
		},
		{
			name: "valid max retry count",
			creds: map[string]string{
				"create_url":  "https://dns-api.example.com/create",
				"delete_url":  "https://dns-api.example.com/delete",
				"retry_count": "10",
			},
			wantErr: false,
		},
		{
			name: "insecure_skip_verify true",
			creds: map[string]string{
				"create_url":           "https://dns-api.example.com/create",
				"delete_url":           "https://dns-api.example.com/delete",
				"insecure_skip_verify": "true",
			},
			wantErr: false,
		},
		{
			name: "insecure_skip_verify TRUE (case insensitive)",
			creds: map[string]string{
				"create_url":           "https://dns-api.example.com/create",
				"delete_url":           "https://dns-api.example.com/delete",
				"insecure_skip_verify": "TRUE",
			},
			wantErr: false,
		},
		{
			name:    "all empty credentials",
			creds:   map[string]string{},
			wantErr: true,
			errMsg:  "create_url is required",
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

func TestWebhookProvider_ValidateWebhookURL(t *testing.T) {
	provider := NewWebhookProvider()

	tests := []struct {
		name      string
		url       string
		fieldName string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid HTTPS URL",
			url:       "https://api.example.com/webhook",
			fieldName: "test_url",
			wantErr:   false,
		},
		{
			name:      "valid localhost HTTP",
			url:       "http://localhost:8080/webhook",
			fieldName: "test_url",
			wantErr:   false,
		},
		{
			name:      "valid 127.0.0.1 HTTP",
			url:       "http://127.0.0.1:8080/webhook",
			fieldName: "test_url",
			wantErr:   false,
		},
		{
			name:      "invalid scheme ftp",
			url:       "ftp://example.com/webhook",
			fieldName: "test_url",
			wantErr:   true,
			errMsg:    "must use http or https scheme",
		},
		{
			name:      "HTTP for non-localhost rejected",
			url:       "http://api.example.com/webhook",
			fieldName: "test_url",
			wantErr:   true,
			errMsg:    "must use HTTPS for non-localhost",
		},
		{
			name:      "missing hostname",
			url:       "https:///path",
			fieldName: "test_url",
			wantErr:   true,
			errMsg:    "is missing hostname",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := provider.validateWebhookURL(tt.url, tt.fieldName)
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

func TestWebhookProvider_TestCredentials(t *testing.T) {
	provider := NewWebhookProvider()

	// TestCredentials should behave the same as ValidateCredentials
	validCreds := map[string]string{
		"create_url": "https://dns-api.example.com/create",
		"delete_url": "https://dns-api.example.com/delete",
	}

	err := provider.TestCredentials(validCreds)
	assert.NoError(t, err)

	invalidCreds := map[string]string{
		"create_url": "https://dns-api.example.com/create",
	}

	err = provider.TestCredentials(invalidCreds)
	assert.Error(t, err)
}

func TestWebhookProvider_SupportsMultiCredential(t *testing.T) {
	provider := NewWebhookProvider()
	assert.False(t, provider.SupportsMultiCredential())
}

func TestWebhookProvider_BuildCaddyConfig(t *testing.T) {
	provider := NewWebhookProvider()

	tests := []struct {
		name     string
		creds    map[string]string
		expected map[string]any
	}{
		{
			name: "minimal config with defaults",
			creds: map[string]string{
				"create_url": "https://dns-api.example.com/create",
				"delete_url": "https://dns-api.example.com/delete",
			},
			expected: map[string]any{
				"name":                 "webhook",
				"create_url":           "https://dns-api.example.com/create",
				"delete_url":           "https://dns-api.example.com/delete",
				"timeout_seconds":      WebhookDefaultTimeoutSeconds,
				"retry_count":          WebhookDefaultRetryCount,
				"insecure_skip_verify": false,
			},
		},
		{
			name: "full config with all options",
			creds: map[string]string{
				"create_url":           "https://dns-api.example.com/create",
				"delete_url":           "https://dns-api.example.com/delete",
				"auth_header":          "X-API-Key",
				"auth_value":           "secret123",
				"timeout_seconds":      "60",
				"retry_count":          "5",
				"insecure_skip_verify": "true",
			},
			expected: map[string]any{
				"name":                 "webhook",
				"create_url":           "https://dns-api.example.com/create",
				"delete_url":           "https://dns-api.example.com/delete",
				"auth_header":          "X-API-Key",
				"auth_value":           "secret123",
				"timeout_seconds":      60,
				"retry_count":          5,
				"insecure_skip_verify": true,
			},
		},
		{
			name: "whitespace trimming",
			creds: map[string]string{
				"create_url":      "  https://dns-api.example.com/create  ",
				"delete_url":      "  https://dns-api.example.com/delete  ",
				"auth_header":     "  Authorization  ",
				"auth_value":      "  Bearer token  ",
				"timeout_seconds": "  45  ",
				"retry_count":     "  2  ",
			},
			expected: map[string]any{
				"name":                 "webhook",
				"create_url":           "https://dns-api.example.com/create",
				"delete_url":           "https://dns-api.example.com/delete",
				"auth_header":          "Authorization",
				"auth_value":           "Bearer token",
				"timeout_seconds":      45,
				"retry_count":          2,
				"insecure_skip_verify": false,
			},
		},
		{
			name: "invalid timeout falls back to default",
			creds: map[string]string{
				"create_url":      "https://dns-api.example.com/create",
				"delete_url":      "https://dns-api.example.com/delete",
				"timeout_seconds": "invalid",
			},
			expected: map[string]any{
				"name":                 "webhook",
				"create_url":           "https://dns-api.example.com/create",
				"delete_url":           "https://dns-api.example.com/delete",
				"timeout_seconds":      WebhookDefaultTimeoutSeconds,
				"retry_count":          WebhookDefaultRetryCount,
				"insecure_skip_verify": false,
			},
		},
		{
			name: "out-of-range timeout falls back to default",
			creds: map[string]string{
				"create_url":      "https://dns-api.example.com/create",
				"delete_url":      "https://dns-api.example.com/delete",
				"timeout_seconds": "1000",
			},
			expected: map[string]any{
				"name":                 "webhook",
				"create_url":           "https://dns-api.example.com/create",
				"delete_url":           "https://dns-api.example.com/delete",
				"timeout_seconds":      WebhookDefaultTimeoutSeconds,
				"retry_count":          WebhookDefaultRetryCount,
				"insecure_skip_verify": false,
			},
		},
		{
			name: "out-of-range retry falls back to default",
			creds: map[string]string{
				"create_url":  "https://dns-api.example.com/create",
				"delete_url":  "https://dns-api.example.com/delete",
				"retry_count": "100",
			},
			expected: map[string]any{
				"name":                 "webhook",
				"create_url":           "https://dns-api.example.com/create",
				"delete_url":           "https://dns-api.example.com/delete",
				"timeout_seconds":      WebhookDefaultTimeoutSeconds,
				"retry_count":          WebhookDefaultRetryCount,
				"insecure_skip_verify": false,
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

func TestWebhookProvider_BuildCaddyConfigForZone(t *testing.T) {
	provider := NewWebhookProvider()

	creds := map[string]string{
		"create_url": "https://dns-api.example.com/create",
		"delete_url": "https://dns-api.example.com/delete",
	}

	config := provider.BuildCaddyConfigForZone("example.org", creds)

	// Should return same as BuildCaddyConfig since multi-credential is not supported
	assert.Equal(t, "webhook", config["name"])
	assert.Equal(t, "https://dns-api.example.com/create", config["create_url"])
	assert.Equal(t, "https://dns-api.example.com/delete", config["delete_url"])
}

func TestWebhookProvider_PropagationTimeout(t *testing.T) {
	provider := NewWebhookProvider()
	timeout := provider.PropagationTimeout()

	assert.Equal(t, 120*time.Second, timeout)
}

func TestWebhookProvider_PollingInterval(t *testing.T) {
	provider := NewWebhookProvider()
	interval := provider.PollingInterval()

	assert.Equal(t, 5*time.Second, interval)
}

func TestWebhookProvider_GetTimeoutSeconds(t *testing.T) {
	provider := NewWebhookProvider()

	tests := []struct {
		name     string
		creds    map[string]string
		expected int
	}{
		{
			name:     "empty creds returns default",
			creds:    map[string]string{},
			expected: WebhookDefaultTimeoutSeconds,
		},
		{
			name:     "valid timeout returns value",
			creds:    map[string]string{"timeout_seconds": "60"},
			expected: 60,
		},
		{
			name:     "invalid number returns default",
			creds:    map[string]string{"timeout_seconds": "abc"},
			expected: WebhookDefaultTimeoutSeconds,
		},
		{
			name:     "out of range low returns default",
			creds:    map[string]string{"timeout_seconds": "1"},
			expected: WebhookDefaultTimeoutSeconds,
		},
		{
			name:     "out of range high returns default",
			creds:    map[string]string{"timeout_seconds": "500"},
			expected: WebhookDefaultTimeoutSeconds,
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.GetTimeoutSeconds(tt.creds)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWebhookProvider_GetRetryCount(t *testing.T) {
	provider := NewWebhookProvider()

	tests := []struct {
		name     string
		creds    map[string]string
		expected int
	}{
		{
			name:     "empty creds returns default",
			creds:    map[string]string{},
			expected: WebhookDefaultRetryCount,
		},
		{
			name:     "valid retry count returns value",
			creds:    map[string]string{"retry_count": "5"},
			expected: 5,
		},
		{
			name:     "invalid number returns default",
			creds:    map[string]string{"retry_count": "abc"},
			expected: WebhookDefaultRetryCount,
		},
		{
			name:     "negative returns default",
			creds:    map[string]string{"retry_count": "-1"},
			expected: WebhookDefaultRetryCount,
		},
		{
			name:     "out of range high returns default",
			creds:    map[string]string{"retry_count": "100"},
			expected: WebhookDefaultRetryCount,
		},
		{
			name:     "min value returns value",
			creds:    map[string]string{"retry_count": "0"},
			expected: 0,
		},
		{
			name:     "max value returns value",
			creds:    map[string]string{"retry_count": "10"},
			expected: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.GetRetryCount(tt.creds)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWebhookProvider_IsInsecureSkipVerify(t *testing.T) {
	provider := NewWebhookProvider()

	tests := []struct {
		name     string
		creds    map[string]string
		expected bool
	}{
		{
			name:     "empty creds returns false",
			creds:    map[string]string{},
			expected: false,
		},
		{
			name:     "false returns false",
			creds:    map[string]string{"insecure_skip_verify": "false"},
			expected: false,
		},
		{
			name:     "true returns true",
			creds:    map[string]string{"insecure_skip_verify": "true"},
			expected: true,
		},
		{
			name:     "TRUE returns true",
			creds:    map[string]string{"insecure_skip_verify": "TRUE"},
			expected: true,
		},
		{
			name:     "False returns false",
			creds:    map[string]string{"insecure_skip_verify": "False"},
			expected: false,
		},
		{
			name:     "invalid value returns false",
			creds:    map[string]string{"insecure_skip_verify": "invalid"},
			expected: false,
		},
		{
			name:     "with whitespace",
			creds:    map[string]string{"insecure_skip_verify": "  true  "},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.IsInsecureSkipVerify(tt.creds)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWebhookProvider_Constants(t *testing.T) {
	// Verify constant values are sensible
	assert.Equal(t, 30, WebhookDefaultTimeoutSeconds)
	assert.Equal(t, 3, WebhookDefaultRetryCount)
	assert.Equal(t, 120*time.Second, WebhookDefaultPropagationTimeout)
	assert.Equal(t, 5*time.Second, WebhookDefaultPollingInterval)
	assert.Equal(t, 5, WebhookMinTimeoutSeconds)
	assert.Equal(t, 300, WebhookMaxTimeoutSeconds)
	assert.Equal(t, 0, WebhookMinRetryCount)
	assert.Equal(t, 10, WebhookMaxRetryCount)

	// Ensure min < default < max for timeout
	assert.Less(t, WebhookMinTimeoutSeconds, WebhookDefaultTimeoutSeconds)
	assert.Less(t, WebhookDefaultTimeoutSeconds, WebhookMaxTimeoutSeconds)

	// Ensure min <= default <= max for retry
	assert.LessOrEqual(t, WebhookMinRetryCount, WebhookDefaultRetryCount)
	assert.LessOrEqual(t, WebhookDefaultRetryCount, WebhookMaxRetryCount)
}

func TestWebhookProvider_ImplementsInterface(t *testing.T) {
	provider := NewWebhookProvider()

	// Compile-time check that WebhookProvider implements ProviderPlugin
	var _ dnsprovider.ProviderPlugin = provider
}
