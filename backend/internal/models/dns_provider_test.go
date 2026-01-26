package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDNSProvider_TableName(t *testing.T) {
	provider := DNSProvider{}
	assert.Equal(t, "dns_providers", provider.TableName())
}

func TestDNSProvider_Fields(t *testing.T) {
	provider := DNSProvider{
		UUID:               "test-uuid",
		Name:               "Test Provider",
		ProviderType:       "cloudflare",
		Enabled:            true,
		IsDefault:          false,
		PropagationTimeout: 120,
		PollingInterval:    5,
		SuccessCount:       0,
		FailureCount:       0,
	}

	assert.Equal(t, "test-uuid", provider.UUID)
	assert.Equal(t, "Test Provider", provider.Name)
	assert.Equal(t, "cloudflare", provider.ProviderType)
	assert.True(t, provider.Enabled)
	assert.False(t, provider.IsDefault)
	assert.Equal(t, 120, provider.PropagationTimeout)
	assert.Equal(t, 5, provider.PollingInterval)
	assert.Equal(t, 0, provider.SuccessCount)
	assert.Equal(t, 0, provider.FailureCount)
}

func TestDNSProvider_CredentialsEncrypted_NotSerialized(t *testing.T) {
	// This test verifies that the CredentialsEncrypted field has the json:"-" tag
	// by checking that it's not included in JSON serialization
	provider := DNSProvider{
		Name:                 "Test",
		ProviderType:         "cloudflare",
		CredentialsEncrypted: "encrypted-data-should-not-appear-in-json",
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(provider)
	assert.NoError(t, err)

	// Verify credentials are not in JSON
	jsonString := string(jsonData)
	assert.NotContains(t, jsonString, "credentials_encrypted")
	assert.NotContains(t, jsonString, "encrypted-data-should-not-appear-in-json")
	assert.Contains(t, jsonString, "Test")
	assert.Contains(t, jsonString, "cloudflare")
}
