package models_test

import (
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestDNSProviderCredential_TableName(t *testing.T) {
	cred := &models.DNSProviderCredential{}
	assert.Equal(t, "dns_provider_credentials", cred.TableName())
}

func TestDNSProviderCredential_Struct(t *testing.T) {
	now := time.Now()
	cred := &models.DNSProviderCredential{
		ID:                   1,
		UUID:                 "test-uuid",
		DNSProviderID:        1,
		Label:                "Test Credential",
		ZoneFilter:           "example.com,*.example.org",
		CredentialsEncrypted: "encrypted_data",
		Enabled:              true,
		KeyVersion:           1,
		PropagationTimeout:   120,
		PollingInterval:      5,
		SuccessCount:         10,
		FailureCount:         2,
		LastError:            "",
		LastUsedAt:           &now,
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	assert.Equal(t, uint(1), cred.ID)
	assert.Equal(t, "test-uuid", cred.UUID)
	assert.Equal(t, uint(1), cred.DNSProviderID)
	assert.Equal(t, "Test Credential", cred.Label)
	assert.Equal(t, "example.com,*.example.org", cred.ZoneFilter)
	assert.Equal(t, "encrypted_data", cred.CredentialsEncrypted)
	assert.True(t, cred.Enabled)
	assert.Equal(t, 1, cred.KeyVersion)
	assert.Equal(t, 120, cred.PropagationTimeout)
	assert.Equal(t, 5, cred.PollingInterval)
	assert.Equal(t, 10, cred.SuccessCount)
	assert.Equal(t, 2, cred.FailureCount)
	assert.Equal(t, "", cred.LastError)
	assert.NotNil(t, cred.LastUsedAt)
}
