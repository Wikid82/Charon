package services_test

import (
	"context"
	"testing"

	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/crypto/cryptotest"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// otherKeyCipher returns a cipher built from a key that differs from the one
// setupCredentialTestDB uses, so it cannot open rows written by that key.
func otherKeyCipher(t *testing.T) crypto.VersionedCipher {
	t.Helper()
	svc, err := crypto.NewEncryptionService("ZmVkY2JhOTg3NjU0MzIxMGZlZGNiYTk4NzY1NDMyMTA=") // "fedcba9876543210fedcba9876543210"
	require.NoError(t, err)
	return cryptotest.SingleKey(svc)
}

func createCredentialWithKey(t *testing.T, ctx context.Context, svc services.CredentialService, providerID uint) *models.DNSProviderCredential {
	t.Helper()
	cred, err := svc.Create(ctx, providerID, services.CreateCredentialRequest{
		Label:       "Zone credential",
		ZoneFilter:  "example.com",
		Credentials: map[string]string{"api_token": "zone-token"},
		Enabled:     true,
	})
	require.NoError(t, err)
	return cred
}

func TestCredentialService_CreateWithoutCipherStoresNothing(t *testing.T) {
	db, encryptor := setupCredentialTestDB(t)
	ctx := context.Background()
	provider := createTestProvider(t, db, encryptor, true)

	svc := services.NewCredentialService(db, nil)
	_, err := svc.Create(ctx, provider.ID, services.CreateCredentialRequest{
		Label:       "No key",
		Credentials: map[string]string{"api_token": "token"},
	})
	require.ErrorIs(t, err, services.ErrEncryptionFailed)

	var count int64
	require.NoError(t, db.Model(&models.DNSProviderCredential{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestCredentialService_UpdateCredentialsWithoutCipherKeepsStoredSecret(t *testing.T) {
	db, encryptor := setupCredentialTestDB(t)
	ctx := context.Background()
	provider := createTestProvider(t, db, encryptor, true)
	cred := createCredentialWithKey(t, ctx, services.NewCredentialService(db, cryptotest.SingleKey(encryptor)), provider.ID)

	label := "Renamed"
	svc := services.NewCredentialService(db, nil)
	_, err := svc.Update(ctx, provider.ID, cred.ID, services.UpdateCredentialRequest{
		Label:       &label,
		Credentials: map[string]string{"api_token": "new-token"},
	})
	require.ErrorIs(t, err, services.ErrEncryptionFailed)

	var stored models.DNSProviderCredential
	require.NoError(t, db.First(&stored, cred.ID).Error)
	assert.Equal(t, cred.CredentialsEncrypted, stored.CredentialsEncrypted)
	assert.Equal(t, cred.KeyVersion, stored.KeyVersion)
	assert.Equal(t, "Zone credential", stored.Label)
}

func TestCredentialService_UpdateCredentialsStoresNewSecret(t *testing.T) {
	db, encryptor := setupCredentialTestDB(t)
	ctx := context.Background()
	provider := createTestProvider(t, db, encryptor, true)
	svc := services.NewCredentialService(db, cryptotest.SingleKey(encryptor))
	cred := createCredentialWithKey(t, ctx, svc, provider.ID)

	_, err := svc.Update(ctx, provider.ID, cred.ID, services.UpdateCredentialRequest{
		Credentials: map[string]string{"api_token": "replacement-token"},
	})
	require.NoError(t, err)

	var stored models.DNSProviderCredential
	require.NoError(t, db.First(&stored, cred.ID).Error)
	assert.NotEqual(t, cred.CredentialsEncrypted, stored.CredentialsEncrypted)
	assert.Equal(t, 1, stored.KeyVersion)
	plaintext, err := encryptor.Decrypt(stored.CredentialsEncrypted)
	require.NoError(t, err)
	assert.JSONEq(t, `{"api_token":"replacement-token"}`, string(plaintext))
}

func TestCredentialService_TestReportsUnreadableCredentials(t *testing.T) {
	db, encryptor := setupCredentialTestDB(t)
	ctx := context.Background()
	provider := createTestProvider(t, db, encryptor, true)
	cred := createCredentialWithKey(t, ctx, services.NewCredentialService(db, cryptotest.SingleKey(encryptor)), provider.ID)

	result, err := services.NewCredentialService(db, otherKeyCipher(t)).Test(ctx, provider.ID, cred.ID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Equal(t, "DECRYPTION_ERROR", result.Code)
	assert.Equal(t, "Failed to decrypt credentials", result.Error)
}
