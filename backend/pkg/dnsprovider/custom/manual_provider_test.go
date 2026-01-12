package custom

import (
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/pkg/dnsprovider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManualProvider(t *testing.T) {
	provider := NewManualProvider()

	require.NotNil(t, provider)
	assert.Equal(t, DefaultTimeoutMinutes, provider.timeoutMinutes)
	assert.Equal(t, DefaultPollingIntervalSeconds, provider.pollingIntervalSeconds)
}

func TestManualProvider_Type(t *testing.T) {
	provider := NewManualProvider()
	assert.Equal(t, "manual", provider.Type())
}

func TestManualProvider_Metadata(t *testing.T) {
	provider := NewManualProvider()
	metadata := provider.Metadata()

	assert.Equal(t, "manual", metadata.Type)
	assert.Equal(t, "Manual (No Automation)", metadata.Name)
	assert.Contains(t, metadata.Description, "Manually create DNS TXT records")
	assert.NotEmpty(t, metadata.DocumentationURL)
	assert.False(t, metadata.IsBuiltIn)
	assert.Equal(t, "1.0.0", metadata.Version)
	assert.Equal(t, dnsprovider.InterfaceVersion, metadata.InterfaceVersion)
}

func TestManualProvider_Init(t *testing.T) {
	provider := NewManualProvider()
	err := provider.Init()
	assert.NoError(t, err)
}

func TestManualProvider_Cleanup(t *testing.T) {
	provider := NewManualProvider()
	err := provider.Cleanup()
	assert.NoError(t, err)
}

func TestManualProvider_RequiredCredentialFields(t *testing.T) {
	provider := NewManualProvider()
	fields := provider.RequiredCredentialFields()

	// Manual provider has no required credentials
	assert.Empty(t, fields)
}

func TestManualProvider_OptionalCredentialFields(t *testing.T) {
	provider := NewManualProvider()
	fields := provider.OptionalCredentialFields()

	assert.Len(t, fields, 2)

	// Find timeout field
	var timeoutField *dnsprovider.CredentialFieldSpec
	var intervalField *dnsprovider.CredentialFieldSpec

	for i := range fields {
		if fields[i].Name == "timeout_minutes" {
			timeoutField = &fields[i]
		}
		if fields[i].Name == "polling_interval_seconds" {
			intervalField = &fields[i]
		}
	}

	require.NotNil(t, timeoutField, "timeout_minutes field should exist")
	assert.Equal(t, "Challenge Timeout (minutes)", timeoutField.Label)
	assert.Equal(t, "text", timeoutField.Type)
	assert.Equal(t, "10", timeoutField.Placeholder)

	require.NotNil(t, intervalField, "polling_interval_seconds field should exist")
	assert.Equal(t, "DNS Check Interval (seconds)", intervalField.Label)
	assert.Equal(t, "text", intervalField.Type)
	assert.Equal(t, "30", intervalField.Placeholder)
}

func TestManualProvider_ValidateCredentials(t *testing.T) {
	provider := NewManualProvider()

	tests := []struct {
		name    string
		creds   map[string]string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty credentials valid",
			creds:   map[string]string{},
			wantErr: false,
		},
		{
			name:    "valid timeout",
			creds:   map[string]string{"timeout_minutes": "5"},
			wantErr: false,
		},
		{
			name:    "valid polling interval",
			creds:   map[string]string{"polling_interval_seconds": "60"},
			wantErr: false,
		},
		{
			name:    "valid both values",
			creds:   map[string]string{"timeout_minutes": "30", "polling_interval_seconds": "15"},
			wantErr: false,
		},
		{
			name:    "timeout too low",
			creds:   map[string]string{"timeout_minutes": "0"},
			wantErr: true,
			errMsg:  "timeout_minutes must be between",
		},
		{
			name:    "timeout too high",
			creds:   map[string]string{"timeout_minutes": "100"},
			wantErr: true,
			errMsg:  "timeout_minutes must be between",
		},
		{
			name:    "timeout not a number",
			creds:   map[string]string{"timeout_minutes": "abc"},
			wantErr: true,
			errMsg:  "timeout_minutes must be a number",
		},
		{
			name:    "polling interval too low",
			creds:   map[string]string{"polling_interval_seconds": "2"},
			wantErr: true,
			errMsg:  "polling_interval_seconds must be between",
		},
		{
			name:    "polling interval too high",
			creds:   map[string]string{"polling_interval_seconds": "200"},
			wantErr: true,
			errMsg:  "polling_interval_seconds must be between",
		},
		{
			name:    "polling interval not a number",
			creds:   map[string]string{"polling_interval_seconds": "fast"},
			wantErr: true,
			errMsg:  "polling_interval_seconds must be a number",
		},
		{
			name:    "min timeout valid",
			creds:   map[string]string{"timeout_minutes": "1"},
			wantErr: false,
		},
		{
			name:    "max timeout valid",
			creds:   map[string]string{"timeout_minutes": "60"},
			wantErr: false,
		},
		{
			name:    "min polling interval valid",
			creds:   map[string]string{"polling_interval_seconds": "5"},
			wantErr: false,
		},
		{
			name:    "max polling interval valid",
			creds:   map[string]string{"polling_interval_seconds": "120"},
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

func TestManualProvider_TestCredentials(t *testing.T) {
	provider := NewManualProvider()

	// TestCredentials should succeed for valid credentials
	err := provider.TestCredentials(map[string]string{})
	assert.NoError(t, err)

	// TestCredentials should fail for invalid credentials
	err = provider.TestCredentials(map[string]string{"timeout_minutes": "abc"})
	assert.Error(t, err)
}

func TestManualProvider_SupportsMultiCredential(t *testing.T) {
	provider := NewManualProvider()
	assert.False(t, provider.SupportsMultiCredential())
}

func TestManualProvider_BuildCaddyConfig(t *testing.T) {
	provider := NewManualProvider()
	config := provider.BuildCaddyConfig(map[string]string{})

	assert.Equal(t, "manual", config["name"])
	assert.Equal(t, true, config["manual"])
}

func TestManualProvider_BuildCaddyConfigForZone(t *testing.T) {
	provider := NewManualProvider()
	config := provider.BuildCaddyConfigForZone("example.com", map[string]string{})

	// Should return same as BuildCaddyConfig
	assert.Equal(t, "manual", config["name"])
	assert.Equal(t, true, config["manual"])
}

func TestManualProvider_PropagationTimeout(t *testing.T) {
	provider := NewManualProvider()
	timeout := provider.PropagationTimeout()

	expected := time.Duration(DefaultTimeoutMinutes) * time.Minute
	assert.Equal(t, expected, timeout)
}

func TestManualProvider_PollingInterval(t *testing.T) {
	provider := NewManualProvider()
	interval := provider.PollingInterval()

	expected := time.Duration(DefaultPollingIntervalSeconds) * time.Second
	assert.Equal(t, expected, interval)
}

func TestManualProvider_GetTimeoutMinutes(t *testing.T) {
	provider := NewManualProvider()

	tests := []struct {
		name     string
		creds    map[string]string
		expected int
	}{
		{
			name:     "empty creds returns default",
			creds:    map[string]string{},
			expected: DefaultTimeoutMinutes,
		},
		{
			name:     "valid timeout returns value",
			creds:    map[string]string{"timeout_minutes": "30"},
			expected: 30,
		},
		{
			name:     "invalid number returns default",
			creds:    map[string]string{"timeout_minutes": "abc"},
			expected: DefaultTimeoutMinutes,
		},
		{
			name:     "out of range low returns default",
			creds:    map[string]string{"timeout_minutes": "0"},
			expected: DefaultTimeoutMinutes,
		},
		{
			name:     "out of range high returns default",
			creds:    map[string]string{"timeout_minutes": "100"},
			expected: DefaultTimeoutMinutes,
		},
		{
			name:     "min value returns value",
			creds:    map[string]string{"timeout_minutes": "1"},
			expected: 1,
		},
		{
			name:     "max value returns value",
			creds:    map[string]string{"timeout_minutes": "60"},
			expected: 60,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.GetTimeoutMinutes(tt.creds)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestManualProvider_GetPollingIntervalSeconds(t *testing.T) {
	provider := NewManualProvider()

	tests := []struct {
		name     string
		creds    map[string]string
		expected int
	}{
		{
			name:     "empty creds returns default",
			creds:    map[string]string{},
			expected: DefaultPollingIntervalSeconds,
		},
		{
			name:     "valid interval returns value",
			creds:    map[string]string{"polling_interval_seconds": "60"},
			expected: 60,
		},
		{
			name:     "invalid number returns default",
			creds:    map[string]string{"polling_interval_seconds": "abc"},
			expected: DefaultPollingIntervalSeconds,
		},
		{
			name:     "out of range low returns default",
			creds:    map[string]string{"polling_interval_seconds": "2"},
			expected: DefaultPollingIntervalSeconds,
		},
		{
			name:     "out of range high returns default",
			creds:    map[string]string{"polling_interval_seconds": "200"},
			expected: DefaultPollingIntervalSeconds,
		},
		{
			name:     "min value returns value",
			creds:    map[string]string{"polling_interval_seconds": "5"},
			expected: 5,
		},
		{
			name:     "max value returns value",
			creds:    map[string]string{"polling_interval_seconds": "120"},
			expected: 120,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.GetPollingIntervalSeconds(tt.creds)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestManualProvider_Constants(t *testing.T) {
	// Verify constant values are sensible
	assert.Equal(t, 10, DefaultTimeoutMinutes)
	assert.Equal(t, 30, DefaultPollingIntervalSeconds)
	assert.Equal(t, 1, MinTimeoutMinutes)
	assert.Equal(t, 60, MaxTimeoutMinutes)
	assert.Equal(t, 5, MinPollingIntervalSeconds)
	assert.Equal(t, 120, MaxPollingIntervalSeconds)

	// Ensure min < default < max
	assert.Less(t, MinTimeoutMinutes, DefaultTimeoutMinutes)
	assert.Less(t, DefaultTimeoutMinutes, MaxTimeoutMinutes)
	assert.Less(t, MinPollingIntervalSeconds, DefaultPollingIntervalSeconds)
	assert.Less(t, DefaultPollingIntervalSeconds, MaxPollingIntervalSeconds)
}

func TestManualProvider_ImplementsInterface(t *testing.T) {
	provider := NewManualProvider()

	// Compile-time check that ManualProvider implements ProviderPlugin
	var _ dnsprovider.ProviderPlugin = provider
}
