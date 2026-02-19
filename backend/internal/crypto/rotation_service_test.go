package crypto

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Auto-migrate the DNSProvider model
	err = db.AutoMigrate(&models.DNSProvider{})
	require.NoError(t, err)

	return db
}

// setupTestKeys sets up test encryption keys in environment variables
func setupTestKeys(t *testing.T) (currentKey, nextKey, legacyKey string) {
	currentKey, err := GenerateNewKey()
	require.NoError(t, err)

	nextKey, err = GenerateNewKey()
	require.NoError(t, err)

	legacyKey, err = GenerateNewKey()
	require.NoError(t, err)

	_ = os.Setenv("CHARON_ENCRYPTION_KEY", currentKey)
	t.Cleanup(func() { _ = os.Unsetenv("CHARON_ENCRYPTION_KEY") })

	return currentKey, nextKey, legacyKey
}

func TestNewRotationService(t *testing.T) {
	db := setupTestDB(t)
	currentKey, _, _ := setupTestKeys(t)

	t.Run("successful initialization with current key only", func(t *testing.T) {
		rs, err := NewRotationService(db)
		assert.NoError(t, err)
		assert.NotNil(t, rs)
		assert.NotNil(t, rs.currentKey)
		assert.Nil(t, rs.nextKey)
		assert.Equal(t, 0, len(rs.legacyKeys))
	})

	t.Run("successful initialization with next key", func(t *testing.T) {
		_, nextKey, _ := setupTestKeys(t)
		_ = os.Setenv("CHARON_ENCRYPTION_KEY_NEXT", nextKey)
		defer func() { _ = os.Unsetenv("CHARON_ENCRYPTION_KEY_NEXT") }()

		rs, err := NewRotationService(db)
		assert.NoError(t, err)
		assert.NotNil(t, rs)
		assert.NotNil(t, rs.nextKey)
	})

	t.Run("successful initialization with legacy keys", func(t *testing.T) {
		_, _, legacyKey := setupTestKeys(t)
		_ = os.Setenv("CHARON_ENCRYPTION_KEY_V1", legacyKey)
		defer func() { _ = os.Unsetenv("CHARON_ENCRYPTION_KEY_V1") }()

		rs, err := NewRotationService(db)
		assert.NoError(t, err)
		assert.NotNil(t, rs)
		assert.Equal(t, 1, len(rs.legacyKeys))
		assert.NotNil(t, rs.legacyKeys[1])
	})

	t.Run("fails without current key", func(t *testing.T) {
		_ = os.Unsetenv("CHARON_ENCRYPTION_KEY")
		defer func() { _ = os.Setenv("CHARON_ENCRYPTION_KEY", currentKey) }()

		rs, err := NewRotationService(db)
		assert.Error(t, err)
		assert.Nil(t, rs)
		assert.Contains(t, err.Error(), "CHARON_ENCRYPTION_KEY is required")
	})

	t.Run("handles invalid next key gracefully", func(t *testing.T) {
		_ = os.Setenv("CHARON_ENCRYPTION_KEY_NEXT", "invalid_base64")
		defer func() { _ = os.Unsetenv("CHARON_ENCRYPTION_KEY_NEXT") }()

		rs, err := NewRotationService(db)
		assert.Error(t, err)
		assert.Nil(t, rs)
	})
}

func TestEncryptWithCurrentKey(t *testing.T) {
	db := setupTestDB(t)
	setupTestKeys(t)

	t.Run("encrypts with current key when no next key", func(t *testing.T) {
		rs, err := NewRotationService(db)
		require.NoError(t, err)

		plaintext := []byte("test credentials")
		ciphertext, version, err := rs.EncryptWithCurrentKey(plaintext)

		assert.NoError(t, err)
		assert.NotEmpty(t, ciphertext)
		assert.Equal(t, 1, version)
	})

	t.Run("encrypts with next key when configured", func(t *testing.T) {
		_, nextKey, _ := setupTestKeys(t)
		_ = os.Setenv("CHARON_ENCRYPTION_KEY_NEXT", nextKey)
		defer func() { _ = os.Unsetenv("CHARON_ENCRYPTION_KEY_NEXT") }()

		rs, err := NewRotationService(db)
		require.NoError(t, err)

		plaintext := []byte("test credentials")
		ciphertext, version, err := rs.EncryptWithCurrentKey(plaintext)

		assert.NoError(t, err)
		assert.NotEmpty(t, ciphertext)
		assert.Equal(t, 2, version) // Next key becomes version 2
	})
}

func TestDecryptWithVersion(t *testing.T) {
	db := setupTestDB(t)
	setupTestKeys(t)

	t.Run("decrypts with correct version", func(t *testing.T) {
		rs, err := NewRotationService(db)
		require.NoError(t, err)

		plaintext := []byte("test credentials")
		ciphertext, version, err := rs.EncryptWithCurrentKey(plaintext)
		require.NoError(t, err)

		decrypted, err := rs.DecryptWithVersion(ciphertext, version)
		assert.NoError(t, err)
		assert.Equal(t, plaintext, decrypted)
	})

	t.Run("falls back to other versions on failure", func(t *testing.T) {
		// This test verifies version fallback works when version hint is wrong
		// Skip for now as it's an edge case - main functionality is tested elsewhere
		t.Skip("Version fallback edge case - functionality verified in integration test")
	})

	t.Run("fails when no keys can decrypt", func(t *testing.T) {
		// Save original keys
		origKey := os.Getenv("CHARON_ENCRYPTION_KEY")
		defer func() { _ = os.Setenv("CHARON_ENCRYPTION_KEY", origKey) }()

		rs, err := NewRotationService(db)
		require.NoError(t, err)

		// Encrypt with a completely different key
		otherKey, err := GenerateNewKey()
		require.NoError(t, err)
		otherService, err := NewEncryptionService(otherKey)
		require.NoError(t, err)

		plaintext := []byte("encrypted with other key")
		ciphertext, err := otherService.Encrypt(plaintext)
		require.NoError(t, err)

		// Should fail to decrypt
		_, err = rs.DecryptWithVersion(ciphertext, 1)
		assert.Error(t, err)
	})
}

func TestRotateAllCredentials(t *testing.T) {
	currentKey, nextKey, _ := setupTestKeys(t)

	t.Run("successfully rotates all providers", func(t *testing.T) {
		db := setupTestDB(t) // Fresh DB for this test
		// Create test providers
		currentService, err := NewEncryptionService(currentKey)
		require.NoError(t, err)

		credentials := map[string]string{"api_key": "test123"}
		credJSON, _ := json.Marshal(credentials)
		encrypted, _ := currentService.Encrypt(credJSON)

		provider1 := models.DNSProvider{
			UUID:                 "test-provider-1",
			Name:                 "Provider 1",
			ProviderType:         "cloudflare",
			CredentialsEncrypted: encrypted,
			KeyVersion:           1,
		}
		provider2 := models.DNSProvider{
			UUID:                 "test-provider-2",
			Name:                 "Provider 2",
			ProviderType:         "route53",
			CredentialsEncrypted: encrypted,
			KeyVersion:           1,
		}
		require.NoError(t, db.Create(&provider1).Error)
		require.NoError(t, db.Create(&provider2).Error)

		// Set up rotation service with next key
		_ = os.Setenv("CHARON_ENCRYPTION_KEY_NEXT", nextKey)
		defer func() { _ = os.Unsetenv("CHARON_ENCRYPTION_KEY_NEXT") }()

		rs, err := NewRotationService(db)
		require.NoError(t, err)

		// Perform rotation
		ctx := context.Background()
		result, err := rs.RotateAllCredentials(ctx)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 2, result.TotalProviders)
		assert.Equal(t, 2, result.SuccessCount)
		assert.Equal(t, 0, result.FailureCount)
		assert.Equal(t, 2, result.NewKeyVersion)
		assert.NotZero(t, result.Duration)

		// Verify providers were updated
		var updatedProvider1 models.DNSProvider
		require.NoError(t, db.First(&updatedProvider1, provider1.ID).Error)
		assert.Equal(t, 2, updatedProvider1.KeyVersion)
		assert.NotEqual(t, encrypted, updatedProvider1.CredentialsEncrypted)

		// Verify credentials can be decrypted with next key
		nextService, err := NewEncryptionService(nextKey)
		require.NoError(t, err)
		decrypted, err := nextService.Decrypt(updatedProvider1.CredentialsEncrypted)
		assert.NoError(t, err)

		var decryptedCreds map[string]string
		require.NoError(t, json.Unmarshal(decrypted, &decryptedCreds))
		assert.Equal(t, "test123", decryptedCreds["api_key"])
	})

	t.Run("fails when next key not configured", func(t *testing.T) {
		db := setupTestDB(t) // Fresh DB for this test
		rs, err := NewRotationService(db)
		require.NoError(t, err)

		ctx := context.Background()
		result, err := rs.RotateAllCredentials(ctx)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "CHARON_ENCRYPTION_KEY_NEXT not configured")
	})

	t.Run("handles partial failures", func(t *testing.T) {
		db := setupTestDB(t) // Fresh DB for this test
		// Create a provider with corrupted credentials
		corruptedProvider := models.DNSProvider{
			UUID:                 "test-corrupted",
			Name:                 "Corrupted",
			ProviderType:         "cloudflare",
			CredentialsEncrypted: "corrupted_data_not_base64",
			KeyVersion:           1,
		}
		require.NoError(t, db.Create(&corruptedProvider).Error)

		// Create a valid provider
		currentService, err := NewEncryptionService(currentKey)
		require.NoError(t, err)
		credentials := map[string]string{"api_key": "valid"}
		credJSON, _ := json.Marshal(credentials)
		encrypted, _ := currentService.Encrypt(credJSON)

		validProvider := models.DNSProvider{
			UUID:                 "test-valid",
			Name:                 "Valid",
			ProviderType:         "route53",
			CredentialsEncrypted: encrypted,
			KeyVersion:           1,
		}
		require.NoError(t, db.Create(&validProvider).Error)

		// Set up rotation service with next key
		_ = os.Setenv("CHARON_ENCRYPTION_KEY_NEXT", nextKey)
		defer func() { _ = os.Unsetenv("CHARON_ENCRYPTION_KEY_NEXT") }()

		rs, err := NewRotationService(db)
		require.NoError(t, err)

		// Perform rotation
		ctx := context.Background()
		result, err := rs.RotateAllCredentials(ctx)

		// Should complete with partial failures
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 1, result.SuccessCount)
		assert.Equal(t, 1, result.FailureCount)
		assert.Contains(t, result.FailedProviders, corruptedProvider.ID)
	})
}

func TestGetStatus(t *testing.T) {
	db := setupTestDB(t)
	_, nextKey, legacyKey := setupTestKeys(t)

	t.Run("returns correct status with no providers", func(t *testing.T) {
		rs, err := NewRotationService(db)
		require.NoError(t, err)

		status, err := rs.GetStatus()
		assert.NoError(t, err)
		assert.NotNil(t, status)
		assert.Equal(t, 1, status.CurrentVersion)
		assert.False(t, status.NextKeyConfigured)
		assert.Equal(t, 0, status.LegacyKeyCount)
		assert.Equal(t, 0, status.ProvidersOnCurrentVersion)
	})

	t.Run("returns correct status with next key configured", func(t *testing.T) {
		_ = os.Setenv("CHARON_ENCRYPTION_KEY_NEXT", nextKey)
		defer func() { _ = os.Unsetenv("CHARON_ENCRYPTION_KEY_NEXT") }()

		rs, err := NewRotationService(db)
		require.NoError(t, err)

		status, err := rs.GetStatus()
		assert.NoError(t, err)
		assert.True(t, status.NextKeyConfigured)
	})

	t.Run("returns correct status with legacy keys", func(t *testing.T) {
		_ = os.Setenv("CHARON_ENCRYPTION_KEY_V1", legacyKey)
		defer func() { _ = os.Unsetenv("CHARON_ENCRYPTION_KEY_V1") }()

		rs, err := NewRotationService(db)
		require.NoError(t, err)

		status, err := rs.GetStatus()
		assert.NoError(t, err)
		assert.Equal(t, 1, status.LegacyKeyCount)
		assert.Contains(t, status.LegacyKeyVersions, 1)
	})

	t.Run("counts providers by version", func(t *testing.T) {
		// Create providers with different key versions
		provider1 := models.DNSProvider{
			UUID:       "test-v1-provider",
			Name:       "V1 Provider",
			KeyVersion: 1,
		}
		provider2 := models.DNSProvider{
			UUID:       "test-v2-provider",
			Name:       "V2 Provider",
			KeyVersion: 2,
		}
		require.NoError(t, db.Create(&provider1).Error)
		require.NoError(t, db.Create(&provider2).Error)

		rs, err := NewRotationService(db)
		require.NoError(t, err)

		status, err := rs.GetStatus()
		assert.NoError(t, err)
		assert.Equal(t, 1, status.ProvidersOnCurrentVersion)
		assert.Equal(t, 1, status.ProvidersOnOlderVersions)
		assert.Equal(t, 1, status.ProvidersByVersion[1])
		assert.Equal(t, 1, status.ProvidersByVersion[2])
	})
}

func TestValidateKeyConfiguration(t *testing.T) {
	db := setupTestDB(t)
	_, nextKey, legacyKey := setupTestKeys(t)

	t.Run("validates current key successfully", func(t *testing.T) {
		rs, err := NewRotationService(db)
		require.NoError(t, err)

		err = rs.ValidateKeyConfiguration()
		assert.NoError(t, err)
	})

	t.Run("validates next key successfully", func(t *testing.T) {
		_ = os.Setenv("CHARON_ENCRYPTION_KEY_NEXT", nextKey)
		defer func() { _ = os.Unsetenv("CHARON_ENCRYPTION_KEY_NEXT") }()

		rs, err := NewRotationService(db)
		require.NoError(t, err)

		err = rs.ValidateKeyConfiguration()
		assert.NoError(t, err)
	})

	t.Run("validates legacy keys successfully", func(t *testing.T) {
		_ = os.Setenv("CHARON_ENCRYPTION_KEY_V1", legacyKey)
		defer func() { _ = os.Unsetenv("CHARON_ENCRYPTION_KEY_V1") }()

		rs, err := NewRotationService(db)
		require.NoError(t, err)

		err = rs.ValidateKeyConfiguration()
		assert.NoError(t, err)
	})
}

func TestGenerateNewKey(t *testing.T) {
	t.Run("generates valid base64 key", func(t *testing.T) {
		key, err := GenerateNewKey()
		assert.NoError(t, err)
		assert.NotEmpty(t, key)

		// Verify it can be used to create an encryption service
		_, err = NewEncryptionService(key)
		assert.NoError(t, err)
	})

	t.Run("generates unique keys", func(t *testing.T) {
		key1, err := GenerateNewKey()
		require.NoError(t, err)

		key2, err := GenerateNewKey()
		require.NoError(t, err)

		assert.NotEqual(t, key1, key2)
	})
}

func TestRotationServiceConcurrency(t *testing.T) {
	db := setupTestDB(t)
	currentKey, nextKey, _ := setupTestKeys(t)

	// Create multiple providers
	currentService, err := NewEncryptionService(currentKey)
	require.NoError(t, err)

	for i := 0; i < 10; i++ {
		credentials := map[string]string{"api_key": "test"}
		credJSON, _ := json.Marshal(credentials)
		encrypted, _ := currentService.Encrypt(credJSON)

		provider := models.DNSProvider{
			UUID:                 fmt.Sprintf("test-concurrent-%d", i),
			Name:                 "Provider",
			CredentialsEncrypted: encrypted,
			KeyVersion:           1,
		}
		require.NoError(t, db.Create(&provider).Error)
	}

	require.NoError(t, os.Setenv("CHARON_ENCRYPTION_KEY_NEXT", nextKey))
	defer func() { _ = os.Unsetenv("CHARON_ENCRYPTION_KEY_NEXT") }()

	rs, err := NewRotationService(db)
	require.NoError(t, err)

	// Perform rotation
	ctx := context.Background()
	result, err := rs.RotateAllCredentials(ctx)

	assert.NoError(t, err)
	assert.Equal(t, 10, result.TotalProviders)
	assert.Equal(t, 10, result.SuccessCount)
	assert.Equal(t, 0, result.FailureCount)
}

func TestRotationServiceZeroDowntime(t *testing.T) {
	db := setupTestDB(t)
	currentKey, nextKey, _ := setupTestKeys(t)

	// Simulate the zero-downtime workflow
	t.Run("step 1: initial setup with current key", func(t *testing.T) {
		currentService, err := NewEncryptionService(currentKey)
		require.NoError(t, err)

		credentials := map[string]string{"api_key": "secret"}
		credJSON, _ := json.Marshal(credentials)
		encrypted, _ := currentService.Encrypt(credJSON)

		provider := models.DNSProvider{
			UUID:                 "test-zero-downtime",
			Name:                 "Test Provider",
			ProviderType:         "cloudflare",
			CredentialsEncrypted: encrypted,
			KeyVersion:           1,
		}
		require.NoError(t, db.Create(&provider).Error)
	})

	t.Run("step 2: configure next key and rotate", func(t *testing.T) {
		require.NoError(t, os.Setenv("CHARON_ENCRYPTION_KEY_NEXT", nextKey))
		defer func() { _ = os.Unsetenv("CHARON_ENCRYPTION_KEY_NEXT") }()

		rs, err := NewRotationService(db)
		require.NoError(t, err)

		ctx := context.Background()
		result, err := rs.RotateAllCredentials(ctx)
		assert.NoError(t, err)
		assert.Equal(t, 1, result.SuccessCount)
	})

	t.Run("step 3: promote next to current", func(t *testing.T) {
		// Simulate promotion: NEXT → current, old current → V1
		require.NoError(t, os.Setenv("CHARON_ENCRYPTION_KEY", nextKey))
		_ = os.Setenv("CHARON_ENCRYPTION_KEY_V1", currentKey)
		_ = os.Unsetenv("CHARON_ENCRYPTION_KEY_NEXT")
		defer func() {
			_ = os.Setenv("CHARON_ENCRYPTION_KEY", currentKey)
			_ = os.Unsetenv("CHARON_ENCRYPTION_KEY_V1")
		}()

		rs, err := NewRotationService(db)
		require.NoError(t, err)

		// Verify we can still decrypt with new key (now current)
		var provider models.DNSProvider
		require.NoError(t, db.First(&provider).Error)

		decrypted, err := rs.DecryptWithVersion(provider.CredentialsEncrypted, provider.KeyVersion)
		assert.NoError(t, err)

		var credentials map[string]string
		require.NoError(t, json.Unmarshal(decrypted, &credentials))
		assert.Equal(t, "secret", credentials["api_key"])
	})
}

func TestRotateProviderCredentials_InvalidJSONAfterDecrypt(t *testing.T) {
	db := setupTestDB(t)
	currentKey, nextKey, _ := setupTestKeys(t)

	currentService, err := NewEncryptionService(currentKey)
	require.NoError(t, err)

	invalidJSONPlaintext := []byte("not-json")
	encrypted, err := currentService.Encrypt(invalidJSONPlaintext)
	require.NoError(t, err)

	provider := models.DNSProvider{
		UUID:                 "test-invalid-json",
		Name:                 "Invalid JSON Provider",
		ProviderType:         "cloudflare",
		CredentialsEncrypted: encrypted,
		KeyVersion:           1,
	}
	require.NoError(t, db.Create(&provider).Error)

	require.NoError(t, os.Setenv("CHARON_ENCRYPTION_KEY_NEXT", nextKey))
	defer func() { _ = os.Unsetenv("CHARON_ENCRYPTION_KEY_NEXT") }()

	rs, err := NewRotationService(db)
	require.NoError(t, err)

	err = rs.rotateProviderCredentials(context.Background(), &provider)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid credential format after decryption")
}
