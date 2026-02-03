package caddy

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/crypto"
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

// TestManager_GetCredentialForDomain_NoEncryptionKey tests missing encryption key error
func TestManager_GetCredentialForDomain_NoEncryptionKey(t *testing.T) {
	// Clear all encryption key environment variables
	oldKeys := map[string]string{
		"CHARON_ENCRYPTION_KEY":   os.Getenv("CHARON_ENCRYPTION_KEY"),
		"ENCRYPTION_KEY":          os.Getenv("ENCRYPTION_KEY"),
		"CERBERUS_ENCRYPTION_KEY": os.Getenv("CERBERUS_ENCRYPTION_KEY"),
	}
	defer func() {
		for k, v := range oldKeys {
			if v != "" {
				require.NoError(t, os.Setenv(k, v))
			} else {
				require.NoError(t, os.Unsetenv(k))
			}
		}
	}()

	_ = os.Unsetenv("CHARON_ENCRYPTION_KEY")
	_ = os.Unsetenv("ENCRYPTION_KEY")
	_ = os.Unsetenv("CERBERUS_ENCRYPTION_KEY")

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.DNSProvider{}, &models.DNSProviderCredential{})
	require.NoError(t, err)

	// Create a single-credential provider
	provider := models.DNSProvider{
		ID:                   1,
		ProviderType:         "cloudflare",
		UseMultiCredentials:  false,
		CredentialsEncrypted: "some-encrypted-data",
	}
	require.NoError(t, db.Create(&provider).Error)

	manager := NewManager(nil, db, t.TempDir(), "", false, config.SecurityConfig{})

	_, err = manager.getCredentialForDomain(provider.ID, "example.com", &provider)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no encryption key available")
}

// TestManager_GetCredentialForDomain_DecryptionFailure tests decryption error handling
func TestManager_GetCredentialForDomain_DecryptionFailure(t *testing.T) {
	// Set up a valid encryption key
	encryptionKey := "MTIzNDU2Nzg5MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTI="
	_ = os.Setenv("CHARON_ENCRYPTION_KEY", encryptionKey)
	defer func() { _ = os.Unsetenv("CHARON_ENCRYPTION_KEY") }()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.DNSProvider{}, &models.DNSProviderCredential{})
	require.NoError(t, err)

	// Create a provider with invalid encrypted data
	provider := models.DNSProvider{
		ID:                   1,
		ProviderType:         "cloudflare",
		UseMultiCredentials:  false,
		CredentialsEncrypted: "invalid-base64-data-!@#$%",
	}
	require.NoError(t, db.Create(&provider).Error)

	manager := NewManager(nil, db, t.TempDir(), "", false, config.SecurityConfig{})

	_, err = manager.getCredentialForDomain(provider.ID, "example.com", &provider)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decrypt")
}

// TestManager_GetCredentialForDomain_InvalidJSON tests JSON unmarshal error
func TestManager_GetCredentialForDomain_InvalidJSON(t *testing.T) {
	// Set up valid encryption
	encryptionKey := "MTIzNDU2Nzg5MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTI="
	require.NoError(t, os.Setenv("CHARON_ENCRYPTION_KEY", encryptionKey))
	defer func() { require.NoError(t, os.Unsetenv("CHARON_ENCRYPTION_KEY")) }()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.DNSProvider{}, &models.DNSProviderCredential{})
	require.NoError(t, err)

	// Encrypt invalid JSON (not a map[string]string)
	encryptor, err := crypto.NewEncryptionService(encryptionKey)
	require.NoError(t, err)

	invalidJSON := []byte("not-valid-json-{{{")
	encrypted, err := encryptor.Encrypt(invalidJSON)
	require.NoError(t, err)

	provider := models.DNSProvider{
		ID:                   1,
		ProviderType:         "cloudflare",
		UseMultiCredentials:  false,
		CredentialsEncrypted: encrypted,
	}
	require.NoError(t, db.Create(&provider).Error)

	manager := NewManager(nil, db, t.TempDir(), "", false, config.SecurityConfig{})

	_, err = manager.getCredentialForDomain(provider.ID, "example.com", &provider)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse credentials")
}

// TestManager_GetCredentialForDomain_SkipsDisabledCredentials tests that disabled credentials are skipped
func TestManager_GetCredentialForDomain_SkipsDisabledCredentials(t *testing.T) {
	encryptionKey := "MTIzNDU2Nzg5MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTI="
	require.NoError(t, os.Setenv("CHARON_ENCRYPTION_KEY", encryptionKey))
	defer func() { require.NoError(t, os.Unsetenv("CHARON_ENCRYPTION_KEY")) }()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.DNSProvider{}, &models.DNSProviderCredential{})
	require.NoError(t, err)

	// Create encrypted credentials
	encryptor, err := crypto.NewEncryptionService(encryptionKey)
	require.NoError(t, err)

	disabledCred, err := json.Marshal(map[string]string{"api_token": "disabled-token"})
	require.NoError(t, err)
	disabledEncrypted, err := encryptor.Encrypt(disabledCred)
	require.NoError(t, err)

	enabledCred, err := json.Marshal(map[string]string{"api_token": "enabled-token"})
	require.NoError(t, err)
	enabledEncrypted, err := encryptor.Encrypt(enabledCred)
	require.NoError(t, err)

	// Create provider first
	provider := models.DNSProvider{
		ProviderType:        "cloudflare",
		UseMultiCredentials: true,
	}
	require.NoError(t, db.Create(&provider).Error)

	// Create credentials separately to ensure proper DB state
	disabledCredential := models.DNSProviderCredential{
		DNSProviderID:        provider.ID,
		UUID:                 "disabled-credential-uuid",
		ZoneFilter:           "example.com",
		CredentialsEncrypted: disabledEncrypted,
	}
	require.NoError(t, db.Create(&disabledCredential).Error)

	// Explicitly update Enabled field to false (GORM default override)
	require.NoError(t, db.Model(&disabledCredential).Update("enabled", false).Error)

	enabledCredential := models.DNSProviderCredential{
		DNSProviderID:        provider.ID,
		UUID:                 "enabled-credential-uuid",
		ZoneFilter:           "example.com",
		CredentialsEncrypted: enabledEncrypted,
		Enabled:              true,
	}
	require.NoError(t, db.Create(&enabledCredential).Error)

	// Reload provider with credentials
	err = db.Preload("Credentials").First(&provider, provider.ID).Error
	require.NoError(t, err)

	manager := NewManager(nil, db, t.TempDir(), "", false, config.SecurityConfig{})

	creds, err := manager.getCredentialForDomain(provider.ID, "example.com", &provider)
	require.NoError(t, err)
	assert.Equal(t, "enabled-token", creds["api_token"], "Should use enabled credential, not disabled one")
}

// TestManager_GetCredentialForDomain_MultiCredential_DecryptionFailure tests decryption error in multi-credential mode
func TestManager_GetCredentialForDomain_MultiCredential_DecryptionFailure(t *testing.T) {
	encryptionKey := "MTIzNDU2Nzg5MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTI="
	require.NoError(t, os.Setenv("CHARON_ENCRYPTION_KEY", encryptionKey))
	defer func() { require.NoError(t, os.Unsetenv("CHARON_ENCRYPTION_KEY")) }()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.DNSProvider{}, &models.DNSProviderCredential{})
	require.NoError(t, err)

	// Create a multi-credential provider with corrupt encrypted data
	provider := models.DNSProvider{
		ID:                  1,
		ProviderType:        "cloudflare",
		UseMultiCredentials: true,
		Credentials: []models.DNSProviderCredential{
			{
				ID:                   1,
				DNSProviderID:        1,
				UUID:                 "test-uuid-1",
				ZoneFilter:           "example.com",
				CredentialsEncrypted: "corrupt-data-!@#$",
				Enabled:              true,
			},
		},
	}
	require.NoError(t, db.Create(&provider).Error)

	manager := NewManager(nil, db, t.TempDir(), "", false, config.SecurityConfig{})

	_, err = manager.getCredentialForDomain(provider.ID, "example.com", &provider)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decrypt credential test-uuid-1")
}

// TestManager_GetCredentialForDomain_MultiCredential_InvalidJSON tests JSON parse error in multi-credential mode
func TestManager_GetCredentialForDomain_MultiCredential_InvalidJSON(t *testing.T) {
	encryptionKey := "MTIzNDU2Nzg5MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTI="
	require.NoError(t, os.Setenv("CHARON_ENCRYPTION_KEY", encryptionKey))
	defer func() { require.NoError(t, os.Unsetenv("CHARON_ENCRYPTION_KEY")) }()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.DNSProvider{}, &models.DNSProviderCredential{})
	require.NoError(t, err)

	// Encrypt invalid JSON
	encryptor, err := crypto.NewEncryptionService(encryptionKey)
	require.NoError(t, err)

	invalidJSON := []byte("not-valid-json-{{{")
	encrypted, err := encryptor.Encrypt(invalidJSON)
	require.NoError(t, err)

	provider := models.DNSProvider{
		ID:                  1,
		ProviderType:        "cloudflare",
		UseMultiCredentials: true,
		Credentials: []models.DNSProviderCredential{
			{
				ID:                   1,
				DNSProviderID:        1,
				UUID:                 "test-uuid-2",
				ZoneFilter:           "example.com",
				CredentialsEncrypted: encrypted,
				Enabled:              true,
			},
		},
	}
	require.NoError(t, db.Create(&provider).Error)

	manager := NewManager(nil, db, t.TempDir(), "", false, config.SecurityConfig{})

	_, err = manager.getCredentialForDomain(provider.ID, "example.com", &provider)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse credential test-uuid-2")
}

// TestExtractBaseDomain_EmptyAfterSplit tests edge case where split results in empty string
func TestExtractBaseDomain_EmptyAfterSplit(t *testing.T) {
	result := extractBaseDomain("  ,  ,  ")
	assert.Equal(t, "", result, "Should return empty string for whitespace-only comma-separated input")
}

// TestMatchesZoneFilter_WhitespaceInFilter tests zone filter with extra whitespace
func TestMatchesZoneFilter_WhitespaceInFilter(t *testing.T) {
	assert.True(t, matchesZoneFilter("  example.com  ,  example.org  ", "example.org", true))
	assert.False(t, matchesZoneFilter("  example.com  ", "other.com", true))
}

// TestMatchesZoneFilter_CaseInsensitive tests case-insensitive matching
func TestMatchesZoneFilter_CaseInsensitive(t *testing.T) {
	assert.True(t, matchesZoneFilter("Example.COM", "example.com", true))
	assert.True(t, matchesZoneFilter("*.Example.COM", "app.example.com", false))
}
