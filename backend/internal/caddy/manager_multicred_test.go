package caddy

import (
	"testing"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestExtractBaseDomain tests the domain extraction logic
func TestExtractBaseDomain(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "wildcard domain",
			input:    "*.example.com",
			expected: "example.com",
		},
		{
			name:     "normal domain",
			input:    "example.com",
			expected: "example.com",
		},
		{
			name:     "multiple domains",
			input:    "*.example.com,example.com",
			expected: "example.com",
		},
		{
			name:     "empty",
			input:    "",
			expected: "",
		},
		{
			name:     "with spaces",
			input:    "  *.example.com  ",
			expected: "example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractBaseDomain(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestMatchesZoneFilter tests the zone matching logic
func TestMatchesZoneFilter(t *testing.T) {
	tests := []struct {
		name       string
		zoneFilter string
		domain     string
		exactOnly  bool
		expected   bool
	}{
		{
			name:       "exact match",
			zoneFilter: "example.com",
			domain:     "example.com",
			exactOnly:  true,
			expected:   true,
		},
		{
			name:       "exact match (not exact only)",
			zoneFilter: "example.com",
			domain:     "example.com",
			exactOnly:  false,
			expected:   true,
		},
		{
			name:       "wildcard match",
			zoneFilter: "*.example.com",
			domain:     "app.example.com",
			exactOnly:  false,
			expected:   true,
		},
		{
			name:       "wildcard no match (exact only)",
			zoneFilter: "*.example.com",
			domain:     "app.example.com",
			exactOnly:  true,
			expected:   false,
		},
		{
			name:       "wildcard base domain match",
			zoneFilter: "*.example.com",
			domain:     "example.com",
			exactOnly:  false,
			expected:   true,
		},
		{
			name:       "no match",
			zoneFilter: "example.com",
			domain:     "other.com",
			exactOnly:  false,
			expected:   false,
		},
		{
			name:       "comma-separated zones",
			zoneFilter: "example.com,example.org",
			domain:     "example.org",
			exactOnly:  true,
			expected:   true,
		},
		{
			name:       "empty filter",
			zoneFilter: "",
			domain:     "example.com",
			exactOnly:  false,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesZoneFilter(tt.zoneFilter, tt.domain, tt.exactOnly)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Note: The getCredentialForDomain helper function is comprehensively tested
// via the integration tests in manager_multicred_integration_test.go which
// cover all scenarios: single-credential, exact match, wildcard match, and catch-all
// with proper encryption setup and end-to-end validation.

// TestManager_GetCredentialForDomain_NoMatch tests error case
func TestManager_GetCredentialForDomain_NoMatch(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.DNSProvider{}, &models.DNSProviderCredential{})
	require.NoError(t, err)

	// Create a multi-credential provider with no catch-all
	provider := models.DNSProvider{
		ID:                  1,
		ProviderType:        "cloudflare",
		UseMultiCredentials: true,
		Credentials: []models.DNSProviderCredential{
			{
				ID:                   1,
				DNSProviderID:        1,
				ZoneFilter:           "example.com",
				CredentialsEncrypted: "encrypted-example-com",
				Enabled:              true,
			},
		},
	}
	require.NoError(t, db.Create(&provider).Error)

	manager := NewManager(nil, db, t.TempDir(), "", false, config.SecurityConfig{})

	_, err = manager.getCredentialForDomain(provider.ID, "other.com", &provider)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no matching credential found")
}
