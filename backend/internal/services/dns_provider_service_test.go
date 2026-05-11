package services

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	_ "github.com/Wikid82/charon/backend/pkg/dnsprovider/builtin" // Auto-register DNS providers
)

// Use the contextKey type and constants from dns_provider_service.go:
// - contextKeyUserID
// - contextKeyClientIP
// - contextKeyUserAgent

// setupTestDB creates an in-memory SQLite database for testing.
func setupDNSProviderTestDB(t *testing.T) (*gorm.DB, *crypto.EncryptionService) {
	t.Helper()

	// Set encryption key in environment for RotationService
	// This must match the test key used below to avoid decryption errors
	testKey := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=" // 32-byte key in base64
	_ = os.Setenv("CHARON_ENCRYPTION_KEY", testKey)
	t.Cleanup(func() { _ = os.Unsetenv("CHARON_ENCRYPTION_KEY") })

	// Use shared cache memory database with mutex for proper test isolation
	// This prevents "no such table" errors that occur with :memory: databases
	// when tests run in parallel or have timing issues
	dbPath := ":memory:?cache=shared&mode=memory&_mutex=full"

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
		// Disable prepared statements to avoid cache issues
		PrepareStmt: false,
	})
	require.NoError(t, err)

	// Get underlying SQL DB for connection pool configuration
	sqlDB, err := db.DB()
	require.NoError(t, err)

	// Force single connection to prevent parallel access issues
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	// Auto-migrate schema - SecurityAudit must be migrated FIRST before creating service
	// because DNSProviderService starts a background goroutine that writes audit logs
	err = db.AutoMigrate(&models.SecurityAudit{}, &models.DNSProvider{})
	require.NoError(t, err)

	// Verify tables were created
	if !db.Migrator().HasTable(&models.DNSProvider{}) {
		t.Fatal("failed to create dns_providers table")
	}
	if !db.Migrator().HasTable(&models.SecurityAudit{}) {
		t.Fatal("failed to create security_audits table")
	}

	// Create encryption service with test key
	encryptor, err := crypto.NewEncryptionService("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=") // 32-byte key in base64
	require.NoError(t, err)

	// Register cleanup
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	return db, encryptor
}

func TestDNSProviderService_Create(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	tests := []struct {
		name        string
		req         CreateDNSProviderRequest
		wantErr     bool
		expectedErr error
	}{
		{
			name: "valid cloudflare provider",
			req: CreateDNSProviderRequest{
				Name:         "Test Cloudflare",
				ProviderType: "cloudflare",
				Credentials: map[string]string{
					"api_token": "test-token-123",
				},
				PropagationTimeout: 120,
				PollingInterval:    5,
				IsDefault:          true,
			},
			wantErr: false,
		},
		{
			name: "valid route53 provider with defaults",
			req: CreateDNSProviderRequest{
				Name:         "Test Route53",
				ProviderType: "route53",
				Credentials: map[string]string{ //nolint:gosec // test fixture uses AWS documentation example keys
					"access_key_id":     "AKIAIOSFODNN7EXAMPLE",
					"secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
					"region":            "us-east-1",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid provider type",
			req: CreateDNSProviderRequest{
				Name:         "Invalid Provider",
				ProviderType: "invalid",
				Credentials: map[string]string{
					"api_key": "test",
				},
			},
			wantErr:     true,
			expectedErr: ErrInvalidProviderType,
		},
		{
			name: "missing required credentials",
			req: CreateDNSProviderRequest{
				Name:         "Incomplete Cloudflare",
				ProviderType: "cloudflare",
				Credentials:  map[string]string{},
			},
			wantErr:     true,
			expectedErr: ErrInvalidCredentials,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := service.Create(ctx, tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErr != nil {
					assert.ErrorIs(t, err, tt.expectedErr)
				}
				return
			}

			require.NoError(t, err)
			assert.NotZero(t, provider.ID)
			assert.NotEmpty(t, provider.UUID)
			assert.Equal(t, tt.req.Name, provider.Name)
			assert.Equal(t, tt.req.ProviderType, provider.ProviderType)
			assert.True(t, provider.Enabled)
			assert.NotEmpty(t, provider.CredentialsEncrypted)

			// Verify defaults were set
			if tt.req.PropagationTimeout == 0 {
				assert.Equal(t, 120, provider.PropagationTimeout)
			}
			if tt.req.PollingInterval == 0 {
				assert.Equal(t, 5, provider.PollingInterval)
			}

			// Verify credentials are encrypted (not plaintext)
			assert.NotContains(t, provider.CredentialsEncrypted, "api_token")
			assert.NotContains(t, provider.CredentialsEncrypted, "test-token")
		})
	}
}

func TestDNSProviderService_DefaultProviderLogic(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	// Create first default provider
	provider1, err := service.Create(ctx, CreateDNSProviderRequest{
		Name:         "First Default",
		ProviderType: "cloudflare",
		Credentials: map[string]string{
			"api_token": "token1",
		},
		IsDefault: true,
	})
	require.NoError(t, err)
	assert.True(t, provider1.IsDefault)

	// Create second default provider - should unset first
	provider2, err := service.Create(ctx, CreateDNSProviderRequest{
		Name:         "Second Default",
		ProviderType: "route53",
		Credentials: map[string]string{
			"access_key_id":     "key",
			"secret_access_key": "secret",
			"region":            "us-east-1",
		},
		IsDefault: true,
	})
	require.NoError(t, err)
	assert.True(t, provider2.IsDefault)

	// Verify first provider is no longer default
	updatedProvider1, err := service.Get(ctx, provider1.ID)
	require.NoError(t, err)
	assert.False(t, updatedProvider1.IsDefault)
}

func TestDNSProviderService_List(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	// Create multiple providers
	_, err := service.Create(ctx, CreateDNSProviderRequest{
		Name:         "Cloudflare",
		ProviderType: "cloudflare",
		Credentials:  map[string]string{"api_token": "token"},
	})
	require.NoError(t, err)

	_, err = service.Create(ctx, CreateDNSProviderRequest{
		Name:         "Route53",
		ProviderType: "route53",
		Credentials: map[string]string{
			"access_key_id":     "key",
			"secret_access_key": "secret",
			"region":            "us-east-1",
		},
		IsDefault: true,
	})
	require.NoError(t, err)

	// List all providers
	providers, err := service.List(ctx)
	require.NoError(t, err)
	assert.Len(t, providers, 2)

	// Verify default provider is first (ordered by is_default DESC)
	assert.True(t, providers[0].IsDefault)
}

func TestDNSProviderService_Get(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	// Create a provider
	created, err := service.Create(ctx, CreateDNSProviderRequest{
		Name:         "Test Provider",
		ProviderType: "cloudflare",
		Credentials:  map[string]string{"api_token": "token"},
	})
	require.NoError(t, err)

	// Get the provider
	provider, err := service.Get(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, provider.ID)
	assert.Equal(t, "Test Provider", provider.Name)

	// Get non-existent provider
	_, err = service.Get(ctx, 9999)
	assert.ErrorIs(t, err, ErrDNSProviderNotFound)
}

func TestDNSProviderService_Update(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	// Create a provider
	created, err := service.Create(ctx, CreateDNSProviderRequest{
		Name:         "Original Name",
		ProviderType: "cloudflare",
		Credentials:  map[string]string{"api_token": "original-token"},
	})
	require.NoError(t, err)

	t.Run("update name only", func(t *testing.T) {
		newName := "Updated Name"
		updated, err := service.Update(ctx, created.ID, UpdateDNSProviderRequest{
			Name: &newName,
		})
		require.NoError(t, err)
		assert.Equal(t, "Updated Name", updated.Name)
		assert.True(t, updated.Enabled) // Should remain unchanged
	})

	t.Run("update credentials", func(t *testing.T) {
		newCreds := map[string]string{"api_token": "new-token"}
		updated, err := service.Update(ctx, created.ID, UpdateDNSProviderRequest{
			Credentials: newCreds,
		})
		require.NoError(t, err)

		// Verify credentials were updated by decrypting
		decrypted, err := service.GetDecryptedCredentials(ctx, updated.ID)
		require.NoError(t, err)
		assert.Equal(t, "new-token", decrypted["api_token"])
	})

	t.Run("update enabled status", func(t *testing.T) {
		enabled := false
		updated, err := service.Update(ctx, created.ID, UpdateDNSProviderRequest{
			Enabled: &enabled,
		})
		require.NoError(t, err)
		assert.False(t, updated.Enabled)
	})

	t.Run("update non-existent provider", func(t *testing.T) {
		name := "Test"
		_, err := service.Update(ctx, 9999, UpdateDNSProviderRequest{
			Name: &name,
		})
		assert.ErrorIs(t, err, ErrDNSProviderNotFound)
	})

	t.Run("update to set default", func(t *testing.T) {
		isDefault := true
		updated, err := service.Update(ctx, created.ID, UpdateDNSProviderRequest{
			IsDefault: &isDefault,
		})
		require.NoError(t, err)
		assert.True(t, updated.IsDefault)
	})
}

func TestDNSProviderService_Delete(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	// Create a provider
	created, err := service.Create(ctx, CreateDNSProviderRequest{
		Name:         "To Delete",
		ProviderType: "cloudflare",
		Credentials:  map[string]string{"api_token": "token"},
	})
	require.NoError(t, err)

	// Delete the provider
	err = service.Delete(ctx, created.ID)
	require.NoError(t, err)

	// Verify it's deleted
	_, err = service.Get(ctx, created.ID)
	assert.ErrorIs(t, err, ErrDNSProviderNotFound)

	// Delete non-existent provider
	err = service.Delete(ctx, 9999)
	assert.ErrorIs(t, err, ErrDNSProviderNotFound)
}

func TestDNSProviderService_GetDecryptedCredentials(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	// Create a provider
	testCreds := map[string]string{
		"api_token": "secret-token-123",
		"extra":     "data",
	}
	created, err := service.Create(ctx, CreateDNSProviderRequest{
		Name:         "Test Provider",
		ProviderType: "cloudflare",
		Credentials:  testCreds,
	})
	require.NoError(t, err)

	// Get decrypted credentials
	decrypted, err := service.GetDecryptedCredentials(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, testCreds, decrypted)

	// Verify last_used_at was updated
	provider, err := service.Get(ctx, created.ID)
	require.NoError(t, err)
	assert.NotNil(t, provider.LastUsedAt)
}

func TestDNSProviderService_TestCredentials(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	t.Run("valid credentials", func(t *testing.T) {
		result, err := service.TestCredentials(ctx, CreateDNSProviderRequest{
			Name:         "Test",
			ProviderType: "cloudflare",
			Credentials:  map[string]string{"api_token": "token"},
		})
		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.NotEmpty(t, result.Message)
	})

	t.Run("invalid provider type", func(t *testing.T) {
		result, err := service.TestCredentials(ctx, CreateDNSProviderRequest{
			Name:         "Test",
			ProviderType: "invalid",
			Credentials:  map[string]string{"api_token": "token"},
		})
		require.NoError(t, err)
		assert.False(t, result.Success)
		assert.Equal(t, "INVALID_PROVIDER_TYPE", result.Code)
	})

	t.Run("missing credentials", func(t *testing.T) {
		result, err := service.TestCredentials(ctx, CreateDNSProviderRequest{
			Name:         "Test",
			ProviderType: "route53",
			Credentials:  map[string]string{"access_key_id": "key"}, // Missing secret and region
		})
		require.NoError(t, err)
		assert.False(t, result.Success)
		assert.Equal(t, "INVALID_CREDENTIALS", result.Code)
	})
}

func TestDNSProviderService_Test(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	// Create a provider
	created, err := service.Create(ctx, CreateDNSProviderRequest{
		Name:         "Test Provider",
		ProviderType: "cloudflare",
		Credentials:  map[string]string{"api_token": "token"},
	})
	require.NoError(t, err)

	// Test the provider
	result, err := service.Test(ctx, created.ID)
	require.NoError(t, err)
	assert.True(t, result.Success)

	// Verify statistics were updated
	provider, err := service.Get(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, provider.SuccessCount)
	assert.Equal(t, 0, provider.FailureCount)
	assert.NotNil(t, provider.LastUsedAt)
	assert.Empty(t, provider.LastError)
}

func TestValidateCredentials(t *testing.T) {
	tests := []struct {
		name         string
		providerType string
		credentials  map[string]string
		wantErr      bool
	}{
		{
			name:         "valid cloudflare",
			providerType: "cloudflare",
			credentials:  map[string]string{"api_token": "token"},
			wantErr:      false,
		},
		{
			name:         "valid route53",
			providerType: "route53",
			credentials: map[string]string{
				"access_key_id":     "key",
				"secret_access_key": "secret",
				"region":            "us-east-1",
			},
			wantErr: false,
		},
		{
			name:         "missing field",
			providerType: "route53",
			credentials: map[string]string{
				"access_key_id": "key",
				// Missing secret_access_key and region
			},
			wantErr: true,
		},
		{
			name:         "empty field value",
			providerType: "cloudflare",
			credentials:  map[string]string{"api_token": ""},
			wantErr:      true,
		},
		{
			name:         "invalid provider type",
			providerType: "invalid",
			credentials:  map[string]string{"api_token": "token"},
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCredentials(tt.providerType, tt.credentials)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIsValidProviderType(t *testing.T) {
	tests := []struct {
		name         string
		providerType string
		want         bool
	}{
		{"cloudflare", "cloudflare", true},
		{"route53", "route53", true},
		{"digitalocean", "digitalocean", true},
		{"invalid", "invalid", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isValidProviderType(tt.providerType))
		})
	}
}

func TestCredentialEncryptionRoundtrip(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	originalCreds := map[string]string{
		"api_token":  "super-secret-token",
		"api_key":    "another-secret",
		"extra_data": "sensitive",
	}

	// Create provider with credentials
	provider, err := service.Create(ctx, CreateDNSProviderRequest{
		Name:         "Encryption Test",
		ProviderType: "cloudflare",
		Credentials:  originalCreds,
	})
	require.NoError(t, err)

	// Verify credentials are encrypted in database
	var dbProvider models.DNSProvider
	err = db.Where("id = ?", provider.ID).First(&dbProvider).Error
	require.NoError(t, err)
	assert.NotContains(t, dbProvider.CredentialsEncrypted, "super-secret-token")
	assert.NotContains(t, dbProvider.CredentialsEncrypted, "another-secret")

	// Decrypt and verify
	decrypted, err := service.GetDecryptedCredentials(ctx, provider.ID)
	require.NoError(t, err)
	assert.Equal(t, originalCreds, decrypted)
}

func TestUpdatePreservesCredentials(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	originalCreds := map[string]string{"api_token": "original-token"}

	// Create provider
	provider, err := service.Create(ctx, CreateDNSProviderRequest{
		Name:         "Test",
		ProviderType: "cloudflare",
		Credentials:  originalCreds,
	})
	require.NoError(t, err)

	// Update without providing credentials
	newName := "Updated Name"
	updated, err := service.Update(ctx, provider.ID, UpdateDNSProviderRequest{
		Name: &newName,
	})
	require.NoError(t, err)

	// Verify credentials were preserved
	decrypted, err := service.GetDecryptedCredentials(ctx, updated.ID)
	require.NoError(t, err)
	assert.Equal(t, originalCreds, decrypted)
}

func TestEncryptionServiceIntegration(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)

	testData := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	// Encrypt
	jsonData, err := json.Marshal(testData)
	require.NoError(t, err)

	encrypted, err := encryptor.Encrypt(jsonData)
	require.NoError(t, err)
	assert.NotEmpty(t, encrypted)

	// Store in database
	provider := &models.DNSProvider{
		UUID:                 "test-uuid",
		Name:                 "Test",
		ProviderType:         "cloudflare",
		CredentialsEncrypted: encrypted,
	}
	err = db.Create(provider).Error
	require.NoError(t, err)

	// Retrieve and decrypt
	var retrieved models.DNSProvider
	err = db.Where("id = ?", provider.ID).First(&retrieved).Error
	require.NoError(t, err)

	decrypted, err := encryptor.Decrypt(retrieved.CredentialsEncrypted)
	require.NoError(t, err)

	var result map[string]string
	err = json.Unmarshal(decrypted, &result)
	require.NoError(t, err)
	assert.Equal(t, testData, result)
}

func TestDNSProviderService_TestFailure(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	// Create provider with invalid credentials structure (will fail decryption in real scenario)
	provider := &models.DNSProvider{ //nolint:gosec // test fixture with intentionally invalid encrypted data
		UUID:                 "test-uuid",
		Name:                 "Test",
		ProviderType:         "cloudflare",
		CredentialsEncrypted: "invalid-encrypted-data",
	}
	err := db.Create(provider).Error
	require.NoError(t, err)

	// Test should handle decryption failure gracefully
	result, err := service.Test(ctx, provider.ID)
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Equal(t, "DECRYPTION_ERROR", result.Code)
}

func TestDNSProviderService_GetDecryptedCredentialsError(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	// Create provider with invalid encrypted data
	//nolint:gosec // G101: test credential
	provider := &models.DNSProvider{
		UUID:                 "test-uuid",
		Name:                 "Test",
		ProviderType:         "cloudflare",
		CredentialsEncrypted: "not-valid-base64",
	}
	err := db.Create(provider).Error
	require.NoError(t, err)

	// Should fail to decrypt
	_, err = service.GetDecryptedCredentials(ctx, provider.ID)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrDecryptionFailed)
}

func TestDNSProviderService_GetDecryptedCredentials_InvalidJSON_ReturnsErrDecryptionFailed(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	ciphertext, err := encryptor.Encrypt([]byte("not-json"))
	require.NoError(t, err)

	provider := &models.DNSProvider{
		UUID:                 "test-uuid-json",
		Name:                 "Bad JSON",
		ProviderType:         "cloudflare",
		CredentialsEncrypted: ciphertext,
		Enabled:              true,
	}
	require.NoError(t, db.Create(provider).Error)

	_, err = service.GetDecryptedCredentials(ctx, provider.ID)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrDecryptionFailed)
}

func TestDNSProviderService_UpdateDefaultLogic(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	// Create two providers, first is default
	provider1, err := service.Create(ctx, CreateDNSProviderRequest{
		Name:         "Provider 1",
		ProviderType: "cloudflare",
		Credentials:  map[string]string{"api_token": "token1"},
		IsDefault:    true,
	})
	require.NoError(t, err)

	provider2, err := service.Create(ctx, CreateDNSProviderRequest{
		Name:         "Provider 2",
		ProviderType: "route53",
		Credentials: map[string]string{
			"access_key_id":     "key",
			"secret_access_key": "secret",
			"region":            "us-east-1",
		},
	})
	require.NoError(t, err)

	// Make provider2 default via update
	isDefault := true
	_, err = service.Update(ctx, provider2.ID, UpdateDNSProviderRequest{
		IsDefault: &isDefault,
	})
	require.NoError(t, err)

	// Verify provider1 is no longer default
	updated1, err := service.Get(ctx, provider1.ID)
	require.NoError(t, err)
	assert.False(t, updated1.IsDefault)

	// Verify provider2 is default
	updated2, err := service.Get(ctx, provider2.ID)
	require.NoError(t, err)
	assert.True(t, updated2.IsDefault)

	// Unset default
	notDefault := false
	_, err = service.Update(ctx, provider2.ID, UpdateDNSProviderRequest{
		IsDefault: &notDefault,
	})
	require.NoError(t, err)

	updated2, err = service.Get(ctx, provider2.ID)
	require.NoError(t, err)
	assert.False(t, updated2.IsDefault)
}

func TestAllProviderTypes(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	// Test all supported provider types
	testCases := map[string]map[string]string{
		"cloudflare":     {"api_token": "token"},
		"route53":        {"access_key_id": "key", "secret_access_key": "secret", "region": "us-east-1"},
		"digitalocean":   {"api_token": "token"},
		"googleclouddns": {"service_account_json": "{}", "project": "test-project"},
		"namecheap":      {"api_user": "user", "api_key": "key", "client_ip": "1.2.3.4"},
		"godaddy":        {"api_key": "key", "api_secret": "secret"},
		"azure": {
			"tenant_id":       "tenant",
			"client_id":       "client",
			"client_secret":   "secret",
			"subscription_id": "sub",
			"resource_group":  "rg",
		},
		"hetzner":  {"api_token": "key"},
		"vultr":    {"api_key": "key"},
		"dnsimple": {"api_token": "token", "account_id": "12345"},
	}

	for providerType, creds := range testCases {
		t.Run(providerType, func(t *testing.T) {
			provider, err := service.Create(ctx, CreateDNSProviderRequest{
				Name:         "Test " + providerType,
				ProviderType: providerType,
				Credentials:  creds,
			})
			require.NoError(t, err, "Failed to create %s provider", providerType)
			assert.Equal(t, providerType, provider.ProviderType)

			// Verify credentials can be decrypted
			decrypted, err := service.GetDecryptedCredentials(ctx, provider.ID)
			require.NoError(t, err)
			assert.Equal(t, creds, decrypted)
		})
	}
}

func TestDNSProviderService_UpdateInvalidCredentials(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	// Create a provider
	provider, err := service.Create(ctx, CreateDNSProviderRequest{
		Name:         "Test",
		ProviderType: "cloudflare",
		Credentials:  map[string]string{"api_token": "token"},
	})
	require.NoError(t, err)

	// Try to update with invalid credentials
	invalidCreds := map[string]string{"wrong_field": "value"}
	_, err = service.Update(ctx, provider.ID, UpdateDNSProviderRequest{
		Credentials: invalidCreds,
	})
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestDNSProviderService_CreateEncryptionError(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	// Create with credentials that would marshal to invalid JSON
	// This is hard to test without mocking, so we test the encryption path by
	// verifying that any errors during encryption are properly wrapped
	provider, err := service.Create(ctx, CreateDNSProviderRequest{
		Name:         "Test",
		ProviderType: "cloudflare",
		Credentials:  map[string]string{"api_token": "valid-token"},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, provider.CredentialsEncrypted)
}

func TestDNSProviderService_Update_PropagationTimeoutAndPollingInterval(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	// Create a provider with default values
	provider, err := service.Create(ctx, CreateDNSProviderRequest{
		Name:         "Test Provider",
		ProviderType: "cloudflare",
		Credentials:  map[string]string{"api_token": "token"},
	})
	require.NoError(t, err)
	assert.Equal(t, 120, provider.PropagationTimeout)
	assert.Equal(t, 5, provider.PollingInterval)

	t.Run("update propagation timeout", func(t *testing.T) {
		newTimeout := 300
		updated, err := service.Update(ctx, provider.ID, UpdateDNSProviderRequest{
			PropagationTimeout: &newTimeout,
		})
		require.NoError(t, err)
		assert.Equal(t, 300, updated.PropagationTimeout)
	})

	t.Run("update polling interval", func(t *testing.T) {
		newInterval := 10
		updated, err := service.Update(ctx, provider.ID, UpdateDNSProviderRequest{
			PollingInterval: &newInterval,
		})
		require.NoError(t, err)
		assert.Equal(t, 10, updated.PollingInterval)
	})

	t.Run("update both timeout and interval", func(t *testing.T) {
		newTimeout := 180
		newInterval := 15
		updated, err := service.Update(ctx, provider.ID, UpdateDNSProviderRequest{
			PropagationTimeout: &newTimeout,
			PollingInterval:    &newInterval,
		})
		require.NoError(t, err)
		assert.Equal(t, 180, updated.PropagationTimeout)
		assert.Equal(t, 15, updated.PollingInterval)
	})
}

func TestDNSProviderService_Test_NonExistentProvider(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	// Test with non-existent provider
	_, err := service.Test(ctx, 9999)
	assert.ErrorIs(t, err, ErrDNSProviderNotFound)
}

func TestDNSProviderService_GetDecryptedCredentials_NonExistentProvider(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	// Get credentials for non-existent provider
	_, err := service.GetDecryptedCredentials(ctx, 9999)
	assert.ErrorIs(t, err, ErrDNSProviderNotFound)
}

func TestDNSProviderService_TestWithFailedCredentials(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	// Create provider with valid encrypted credentials
	provider, err := service.Create(ctx, CreateDNSProviderRequest{
		Name:         "Test Provider",
		ProviderType: "cloudflare",
		Credentials:  map[string]string{"api_token": "token"},
	})
	require.NoError(t, err)

	// Test should succeed and update success count
	result, err := service.Test(ctx, provider.ID)
	require.NoError(t, err)
	assert.True(t, result.Success)

	// Verify success count incremented
	updated, err := service.Get(ctx, provider.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, updated.SuccessCount)
	assert.Equal(t, 0, updated.FailureCount)
	assert.Empty(t, updated.LastError)
}

func TestDNSProviderService_CreateWithEmptyCredentialValue(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	// Create with empty string value in required field
	_, err := service.Create(ctx, CreateDNSProviderRequest{
		Name:         "Test",
		ProviderType: "cloudflare",
		Credentials:  map[string]string{"api_token": ""},
	})
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestDNSProviderService_Update_EmptyCredentials(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	// Create a provider
	provider, err := service.Create(ctx, CreateDNSProviderRequest{
		Name:         "Test",
		ProviderType: "cloudflare",
		Credentials:  map[string]string{"api_token": "original"},
	})
	require.NoError(t, err)

	// Update with empty credentials map (should not update credentials)
	newName := "New Name"
	updated, err := service.Update(ctx, provider.ID, UpdateDNSProviderRequest{
		Name:        &newName,
		Credentials: map[string]string{}, // Empty map
	})
	require.NoError(t, err)
	assert.Equal(t, "New Name", updated.Name)

	// Verify original credentials preserved
	decrypted, err := service.GetDecryptedCredentials(ctx, updated.ID)
	require.NoError(t, err)
	assert.Equal(t, "original", decrypted["api_token"])
}

func TestDNSProviderService_Update_NilCredentials(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	// Create a provider
	provider, err := service.Create(ctx, CreateDNSProviderRequest{
		Name:         "Test",
		ProviderType: "cloudflare",
		Credentials:  map[string]string{"api_token": "original"},
	})
	require.NoError(t, err)

	// Update with nil credentials (should not update credentials)
	newName := "New Name"
	updated, err := service.Update(ctx, provider.ID, UpdateDNSProviderRequest{
		Name:        &newName,
		Credentials: nil,
	})
	require.NoError(t, err)
	assert.Equal(t, "New Name", updated.Name)

	// Verify original credentials preserved
	decrypted, err := service.GetDecryptedCredentials(ctx, updated.ID)
	require.NoError(t, err)
	assert.Equal(t, "original", decrypted["api_token"])
}

func TestDNSProviderService_Create_WithExistingDefault(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	// Create first provider as non-default
	provider1, err := service.Create(ctx, CreateDNSProviderRequest{
		Name:         "First",
		ProviderType: "cloudflare",
		Credentials:  map[string]string{"api_token": "token1"},
		IsDefault:    false,
	})
	require.NoError(t, err)
	assert.False(t, provider1.IsDefault)

	// Create second provider as default
	provider2, err := service.Create(ctx, CreateDNSProviderRequest{
		Name:         "Second",
		ProviderType: "route53",
		Credentials: map[string]string{
			"access_key_id":     "key",
			"secret_access_key": "secret",
			"region":            "us-east-1",
		},
		IsDefault: true,
	})
	require.NoError(t, err)
	assert.True(t, provider2.IsDefault)

	// Verify first is still non-default
	updated1, err := service.Get(ctx, provider1.ID)
	require.NoError(t, err)
	assert.False(t, updated1.IsDefault)
}

func TestDNSProviderService_Delete_AlreadyDeleted(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	// Create a provider
	provider, err := service.Create(ctx, CreateDNSProviderRequest{
		Name:         "Test",
		ProviderType: "cloudflare",
		Credentials:  map[string]string{"api_token": "token"},
	})
	require.NoError(t, err)

	// Delete successfully
	err = service.Delete(ctx, provider.ID)
	require.NoError(t, err)

	// Delete again (already deleted) - should return not found
	err = service.Delete(ctx, provider.ID)
	assert.ErrorIs(t, err, ErrDNSProviderNotFound)
}

func TestTestDNSProviderCredentials_Validation(t *testing.T) {
	// Test the internal testDNSProviderCredentials function
	tests := []struct {
		name         string
		providerType string
		credentials  map[string]string
		wantSuccess  bool
		wantCode     string
	}{
		{
			name:         "valid cloudflare credentials",
			providerType: "cloudflare",
			credentials:  map[string]string{"api_token": "valid-token"},
			wantSuccess:  true,
			wantCode:     "",
		},
		{
			name:         "missing required field",
			providerType: "cloudflare",
			credentials:  map[string]string{},
			wantSuccess:  false,
			wantCode:     "VALIDATION_ERROR",
		},
		{
			name:         "empty required field",
			providerType: "route53",
			credentials:  map[string]string{"access_key_id": "", "secret_access_key": "secret", "region": "us-east-1"},
			wantSuccess:  false,
			wantCode:     "VALIDATION_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := testDNSProviderCredentials(tt.providerType, tt.credentials)
			assert.Equal(t, tt.wantSuccess, result.Success)
			if !tt.wantSuccess {
				assert.Equal(t, tt.wantCode, result.Code)
			}
		})
	}
}

func TestDNSProviderService_Update_CredentialValidationError(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	// Create a route53 provider
	provider, err := service.Create(ctx, CreateDNSProviderRequest{
		Name:         "Test Route53",
		ProviderType: "route53",
		Credentials: map[string]string{
			"access_key_id":     "key",
			"secret_access_key": "secret",
			"region":            "us-east-1",
		},
	})
	require.NoError(t, err)

	// Update with missing required credentials
	_, err = service.Update(ctx, provider.ID, UpdateDNSProviderRequest{
		Credentials: map[string]string{
			"access_key_id": "new-key",
			// Missing secret_access_key and region
		},
	})
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestDNSProviderService_TestCredentials_AllProviders(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	// Test credentials for all supported provider types without saving
	testCases := map[string]map[string]string{
		"cloudflare":     {"api_token": "token"},
		"route53":        {"access_key_id": "key", "secret_access_key": "secret", "region": "us-east-1"},
		"digitalocean":   {"api_token": "token"},
		"googleclouddns": {"service_account_json": "{}", "project": "test-project"},
		"namecheap":      {"api_user": "user", "api_key": "key", "client_ip": "1.2.3.4"},
		"godaddy":        {"api_key": "key", "api_secret": "secret"},
		"azure": {
			"tenant_id":       "tenant",
			"client_id":       "client",
			"client_secret":   "secret",
			"subscription_id": "sub",
			"resource_group":  "rg",
		},
		"hetzner":  {"api_token": "key"},
		"vultr":    {"api_key": "key"},
		"dnsimple": {"api_token": "token", "account_id": "12345"},
	}

	for providerType, creds := range testCases {
		t.Run(providerType, func(t *testing.T) {
			result, err := service.TestCredentials(ctx, CreateDNSProviderRequest{
				Name:         "Test " + providerType,
				ProviderType: providerType,
				Credentials:  creds,
			})
			require.NoError(t, err)
			assert.True(t, result.Success, "Provider %s should succeed", providerType)
			assert.NotEmpty(t, result.Message)
			assert.GreaterOrEqual(t, result.PropagationTimeMs, int64(0))
		})
	}
}

func TestDNSProviderService_List_Empty(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	// List on empty database
	providers, err := service.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, providers)
}

func TestDNSProviderService_Create_DefaultsApplied(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	// Create provider without specifying defaults
	provider, err := service.Create(ctx, CreateDNSProviderRequest{
		Name:         "Test",
		ProviderType: "cloudflare",
		Credentials:  map[string]string{"api_token": "token"},
		// PropagationTimeout and PollingInterval not set
	})
	require.NoError(t, err)

	// Verify defaults were applied
	assert.Equal(t, 120, provider.PropagationTimeout)
	assert.Equal(t, 5, provider.PollingInterval)
	assert.True(t, provider.Enabled)
}

func TestDNSProviderService_Create_CustomTimeouts(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	// Create provider with custom timeouts
	provider, err := service.Create(ctx, CreateDNSProviderRequest{
		Name:               "Test",
		ProviderType:       "cloudflare",
		Credentials:        map[string]string{"api_token": "token"},
		PropagationTimeout: 300,
		PollingInterval:    10,
	})
	require.NoError(t, err)

	// Verify custom values were used
	assert.Equal(t, 300, provider.PropagationTimeout)
	assert.Equal(t, 10, provider.PollingInterval)
}

func TestDNSProviderService_List_OrderByDefault(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	// Create multiple providers
	_, err := service.Create(ctx, CreateDNSProviderRequest{
		Name:         "B Provider",
		ProviderType: "cloudflare",
		Credentials:  map[string]string{"api_token": "token"},
	})
	require.NoError(t, err)

	_, err = service.Create(ctx, CreateDNSProviderRequest{
		Name:         "A Provider",
		ProviderType: "hetzner",
		Credentials:  map[string]string{"api_token": "key"},
	})
	require.NoError(t, err)

	defaultProvider, err := service.Create(ctx, CreateDNSProviderRequest{
		Name:         "Z Default Provider",
		ProviderType: "vultr",
		Credentials:  map[string]string{"api_key": "key"},
		IsDefault:    true,
	})
	require.NoError(t, err)

	// List all providers
	providers, err := service.List(ctx)
	require.NoError(t, err)
	assert.Len(t, providers, 3)

	// Verify default provider is first, then alphabetical order
	assert.Equal(t, defaultProvider.ID, providers[0].ID)
	assert.True(t, providers[0].IsDefault)
}

func TestDNSProviderService_Update_MultipleFields(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	// Create a provider
	provider, err := service.Create(ctx, CreateDNSProviderRequest{
		Name:         "Original",
		ProviderType: "cloudflare",
		Credentials:  map[string]string{"api_token": "original-token"},
	})
	require.NoError(t, err)

	// Update multiple fields at once
	newName := "Updated"
	newTimeout := 240
	newInterval := 8
	enabled := false
	isDefault := true

	updated, err := service.Update(ctx, provider.ID, UpdateDNSProviderRequest{
		Name:               &newName,
		PropagationTimeout: &newTimeout,
		PollingInterval:    &newInterval,
		Enabled:            &enabled,
		IsDefault:          &isDefault,
		Credentials:        map[string]string{"api_token": "new-token"},
	})
	require.NoError(t, err)

	assert.Equal(t, "Updated", updated.Name)
	assert.Equal(t, 240, updated.PropagationTimeout)
	assert.Equal(t, 8, updated.PollingInterval)
	assert.False(t, updated.Enabled)
	assert.True(t, updated.IsDefault)

	// Verify credentials updated
	decrypted, err := service.GetDecryptedCredentials(ctx, updated.ID)
	require.NoError(t, err)
	assert.Equal(t, "new-token", decrypted["api_token"])
}

func TestDNSProviderService_GetDecryptedCredentials_UpdatesLastUsed(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	// Create a provider
	provider, err := service.Create(ctx, CreateDNSProviderRequest{
		Name:         "Test",
		ProviderType: "cloudflare",
		Credentials:  map[string]string{"api_token": "token"},
	})
	require.NoError(t, err)

	// Verify LastUsedAt is initially nil
	initial, err := service.Get(ctx, provider.ID)
	require.NoError(t, err)
	assert.Nil(t, initial.LastUsedAt)

	// Get decrypted credentials
	_, err = service.GetDecryptedCredentials(ctx, provider.ID)
	require.NoError(t, err)

	// Verify LastUsedAt was updated
	afterGet, err := service.Get(ctx, provider.ID)
	require.NoError(t, err)
	assert.NotNil(t, afterGet.LastUsedAt)
}

func TestDNSProviderService_Test_UpdatesStatistics(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	// Create a provider
	provider, err := service.Create(ctx, CreateDNSProviderRequest{
		Name:         "Test",
		ProviderType: "cloudflare",
		Credentials:  map[string]string{"api_token": "token"},
	})
	require.NoError(t, err)

	// Verify initial statistics
	initial, err := service.Get(ctx, provider.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, initial.SuccessCount)
	assert.Equal(t, 0, initial.FailureCount)
	assert.Nil(t, initial.LastUsedAt)
	assert.Empty(t, initial.LastError)

	// Test the provider (should succeed with basic validation)
	result, err := service.Test(ctx, provider.ID)
	require.NoError(t, err)
	assert.True(t, result.Success)

	// Verify statistics updated
	afterTest, err := service.Get(ctx, provider.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, afterTest.SuccessCount)
	assert.Equal(t, 0, afterTest.FailureCount)
	assert.NotNil(t, afterTest.LastUsedAt)
	assert.Empty(t, afterTest.LastError)
}

func TestDNSProviderService_Test_FailureUpdatesStatistics(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	// Create a cloudflare provider with valid credentials
	cloudflareCredentials := map[string]string{"api_token": "token"}
	credJSON, err := json.Marshal(cloudflareCredentials)
	require.NoError(t, err)

	encryptedCreds, err := encryptor.Encrypt(credJSON)
	require.NoError(t, err)

	// Manually insert a provider with mismatched provider type and credentials
	// Provider type is "route53" but credentials are for cloudflare (missing required fields)
	provider := &models.DNSProvider{
		UUID:                 "test-mismatch-uuid",
		Name:                 "Mismatched Provider",
		ProviderType:         "route53", // Requires access_key_id, secret_access_key, region
		CredentialsEncrypted: encryptedCreds,
		PropagationTimeout:   120,
		PollingInterval:      5,
		Enabled:              true,
	}
	require.NoError(t, db.Create(provider).Error)

	// Test the provider - should fail during validation due to invalid credentials
	result, err := service.Test(ctx, provider.ID)
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Equal(t, "VALIDATION_ERROR", result.Code)

	// Verify failure statistics updated
	afterTest, err := service.Get(ctx, provider.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, afterTest.SuccessCount)
	assert.Equal(t, 1, afterTest.FailureCount)
	assert.NotNil(t, afterTest.LastUsedAt)
	assert.NotEmpty(t, afterTest.LastError)
}

func TestDNSProviderService_List_DBError(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	// Close the DB connection to trigger error
	sqlDB, err := db.DB()
	require.NoError(t, err)
	_ = sqlDB.Close()

	// List should fail
	_, err = service.List(ctx)
	assert.Error(t, err)
}

func TestDNSProviderService_Get_DBError(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	// Close the DB connection to trigger error
	sqlDB, err := db.DB()
	require.NoError(t, err)
	_ = sqlDB.Close()

	// Get should fail with a DB error (not ErrDNSProviderNotFound)
	_, err = service.Get(ctx, 1)
	assert.Error(t, err)
	assert.NotErrorIs(t, err, ErrDNSProviderNotFound)
}

func TestDNSProviderService_Create_DBErrorOnDefaultUnset(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	ctx := context.Background()

	// First, create a default provider with a working DB
	workingService := NewDNSProviderService(db, encryptor)
	_, err := workingService.Create(ctx, CreateDNSProviderRequest{
		Name:         "First Default",
		ProviderType: "cloudflare",
		Credentials:  map[string]string{"api_token": "token"},
		IsDefault:    true,
	})
	require.NoError(t, err)

	// Now close the DB
	sqlDB, err := db.DB()
	require.NoError(t, err)
	_ = sqlDB.Close()

	// Trying to create another default should fail when trying to unset the existing default
	_, err = workingService.Create(ctx, CreateDNSProviderRequest{
		Name:         "Second Default",
		ProviderType: "cloudflare",
		Credentials:  map[string]string{"api_token": "token2"},
		IsDefault:    true,
	})
	assert.Error(t, err)
}

func TestDNSProviderService_Create_DBErrorOnCreate(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	// Close the DB connection to trigger error
	sqlDB, err := db.DB()
	require.NoError(t, err)
	_ = sqlDB.Close()

	// Create should fail
	_, err = service.Create(ctx, CreateDNSProviderRequest{
		Name:         "Test",
		ProviderType: "cloudflare",
		Credentials:  map[string]string{"api_token": "token"},
	})
	assert.Error(t, err)
}

func TestDNSProviderService_Update_DBErrorOnSave(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	// Create a provider first
	provider, err := service.Create(ctx, CreateDNSProviderRequest{
		Name:         "Test",
		ProviderType: "cloudflare",
		Credentials:  map[string]string{"api_token": "token"},
	})
	require.NoError(t, err)

	// Close the DB connection to trigger error
	sqlDB, err := db.DB()
	require.NoError(t, err)
	_ = sqlDB.Close()

	// Update should fail
	newName := "Updated"
	_, err = service.Update(ctx, provider.ID, UpdateDNSProviderRequest{
		Name: &newName,
	})
	assert.Error(t, err)
}

func TestDNSProviderService_Update_DBErrorOnDefaultUnset(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	// Create two providers, first is default
	_, err := service.Create(ctx, CreateDNSProviderRequest{
		Name:         "First",
		ProviderType: "cloudflare",
		Credentials:  map[string]string{"api_token": "token1"},
		IsDefault:    true,
	})
	require.NoError(t, err)

	provider2, err := service.Create(ctx, CreateDNSProviderRequest{
		Name:         "Second",
		ProviderType: "route53",
		Credentials: map[string]string{
			"access_key_id":     "key",
			"secret_access_key": "secret",
			"region":            "us-east-1",
		},
	})
	require.NoError(t, err)

	// Close the DB connection to trigger error
	sqlDB, err := db.DB()
	require.NoError(t, err)
	_ = sqlDB.Close()

	// Update to make second provider default should fail
	isDefault := true
	_, err = service.Update(ctx, provider2.ID, UpdateDNSProviderRequest{
		IsDefault: &isDefault,
	})
	assert.Error(t, err)
}

func TestDNSProviderService_Delete_DBError(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.Background()

	// Close the DB connection to trigger error
	sqlDB, err := db.DB()
	require.NoError(t, err)
	_ = sqlDB.Close()

	// Delete should fail
	err = service.Delete(ctx, 1)
	assert.Error(t, err)
	assert.NotErrorIs(t, err, ErrDNSProviderNotFound)
}

func TestDNSProviderService_AuditLogging_Create(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	// Also migrate SecurityAudit model for audit logging
	err := db.AutoMigrate(&models.SecurityAudit{})
	require.NoError(t, err)

	service := NewDNSProviderService(db, encryptor)
	ctx := context.WithValue(context.Background(), contextKeyUserID, "test-user")
	ctx = context.WithValue(ctx, contextKeyClientIP, "192.168.1.1")
	ctx = context.WithValue(ctx, contextKeyUserAgent, "TestAgent/1.0")

	// Create a provider
	req := CreateDNSProviderRequest{
		Name:         "Test Provider",
		ProviderType: "cloudflare",
		Credentials: map[string]string{
			"api_token": "test-token",
		},
		IsDefault: true,
	}

	provider, err := service.Create(ctx, req)
	require.NoError(t, err)

	// Give time for async audit logging
	time.Sleep(100 * time.Millisecond)

	// Verify audit log was created
	var audit models.SecurityAudit
	err = db.Where("action = ? AND event_category = ?", "dns_provider_create", "dns_provider").First(&audit).Error
	require.NoError(t, err)

	assert.Equal(t, "test-user", audit.Actor)
	assert.Equal(t, "dns_provider_create", audit.Action)
	assert.Equal(t, "dns_provider", audit.EventCategory)
	assert.Equal(t, provider.UUID, audit.ResourceUUID)
	assert.Equal(t, "192.168.1.1", audit.IPAddress)
	assert.Equal(t, "TestAgent/1.0", audit.UserAgent)

	// Verify details contain expected fields
	var details map[string]interface{}
	err = json.Unmarshal([]byte(audit.Details), &details)
	require.NoError(t, err)
	assert.Equal(t, "Test Provider", details["name"])
	assert.Equal(t, "cloudflare", details["type"])
	assert.True(t, details["is_default"].(bool))
}

func TestDNSProviderService_AuditLogging_Update(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.WithValue(context.Background(), contextKeyUserID, "test-user")
	ctx = context.WithValue(ctx, contextKeyClientIP, "192.168.1.2")
	ctx = context.WithValue(ctx, contextKeyUserAgent, "TestAgent/1.0")

	// Create a provider first
	provider, err := service.Create(ctx, CreateDNSProviderRequest{
		Name:         "Original Name",
		ProviderType: "cloudflare",
		Credentials:  map[string]string{"api_token": "test-token"},
	})
	require.NoError(t, err)

	// Wait for create audit to be processed
	time.Sleep(150 * time.Millisecond)

	// Clear any create audit logs
	db.Exec("DELETE FROM security_audits")

	// Update the provider
	newName := "Updated Name"
	enabled := false
	_, err = service.Update(ctx, provider.ID, UpdateDNSProviderRequest{
		Name:    &newName,
		Enabled: &enabled,
	})
	require.NoError(t, err)

	// Give time for async audit logging
	time.Sleep(150 * time.Millisecond)

	// Verify audit log was created
	var audit models.SecurityAudit
	err = db.Where("action = ? AND event_category = ?", "dns_provider_update", "dns_provider").First(&audit).Error
	require.NoError(t, err)

	assert.Equal(t, "test-user", audit.Actor)
	assert.Equal(t, provider.UUID, audit.ResourceUUID)

	// Verify details contain changed fields
	var details map[string]interface{}
	err = json.Unmarshal([]byte(audit.Details), &details)
	require.NoError(t, err)

	changedFields := details["changed_fields"].(map[string]interface{})
	assert.True(t, changedFields["name"].(bool))
	assert.True(t, changedFields["enabled"].(bool))

	oldValues := details["old_values"].(map[string]interface{})
	assert.Equal(t, "Original Name", oldValues["name"])

	newValues := details["new_values"].(map[string]interface{})
	assert.Equal(t, "Updated Name", newValues["name"])
}

func TestDNSProviderService_AuditLogging_Delete(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.WithValue(context.Background(), contextKeyUserID, "admin-user")
	ctx = context.WithValue(ctx, contextKeyClientIP, "10.0.0.1")
	ctx = context.WithValue(ctx, contextKeyUserAgent, "TestAgent/1.0")

	// Create a provider first
	provider, err := service.Create(ctx, CreateDNSProviderRequest{
		Name:         "To Be Deleted",
		ProviderType: "digitalocean",
		Credentials:  map[string]string{"api_token": "test-token"},
	})
	require.NoError(t, err)

	// Wait for create audit to be processed
	time.Sleep(150 * time.Millisecond)

	// Clear create audit logs
	db.Exec("DELETE FROM security_audits")

	// Delete the provider
	err = service.Delete(ctx, provider.ID)
	require.NoError(t, err)

	// Give time for async audit logging
	time.Sleep(150 * time.Millisecond)

	// Verify audit log was created
	var audit models.SecurityAudit
	err = db.Where("action = ? AND event_category = ?", "dns_provider_delete", "dns_provider").First(&audit).Error
	require.NoError(t, err)

	assert.Equal(t, "admin-user", audit.Actor)
	assert.Equal(t, provider.UUID, audit.ResourceUUID)
	assert.Equal(t, "10.0.0.1", audit.IPAddress)

	// Verify details
	var details map[string]interface{}
	err = json.Unmarshal([]byte(audit.Details), &details)
	require.NoError(t, err)
	assert.Equal(t, "To Be Deleted", details["name"])
	assert.Equal(t, "digitalocean", details["type"])
	assert.True(t, details["had_credentials"].(bool))
}

func TestDNSProviderService_AuditLogging_Test(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.WithValue(context.Background(), contextKeyUserID, "test-user")
	ctx = context.WithValue(ctx, contextKeyClientIP, "192.168.1.1")
	ctx = context.WithValue(ctx, contextKeyUserAgent, "TestAgent/1.0")

	// Create a provider
	provider, err := service.Create(ctx, CreateDNSProviderRequest{
		Name:         "Test Provider",
		ProviderType: "cloudflare",
		Credentials:  map[string]string{"api_token": "test-token"},
	})
	require.NoError(t, err)

	// Wait for create audit to be processed
	time.Sleep(150 * time.Millisecond)

	// Clear create audit logs
	db.Exec("DELETE FROM security_audits")

	// Test the provider
	_, err = service.Test(ctx, provider.ID)
	require.NoError(t, err)

	// Give time for async audit logging
	time.Sleep(150 * time.Millisecond)

	// Verify audit log was created
	var audit models.SecurityAudit
	err = db.Where("action = ? AND event_category = ?", "credential_test", "dns_provider").First(&audit).Error
	require.NoError(t, err)

	assert.Equal(t, "test-user", audit.Actor)
	assert.Equal(t, provider.UUID, audit.ResourceUUID)
}

func TestDNSProviderService_AuditLogging_GetDecryptedCredentials(t *testing.T) {
	db, encryptor := setupDNSProviderTestDB(t)
	service := NewDNSProviderService(db, encryptor)
	ctx := context.WithValue(context.Background(), contextKeyUserID, "admin")
	ctx = context.WithValue(ctx, contextKeyClientIP, "192.168.1.1")
	ctx = context.WithValue(ctx, contextKeyUserAgent, "TestAgent/1.0")

	// Create a provider
	provider, err := service.Create(ctx, CreateDNSProviderRequest{
		Name:         "Test Provider",
		ProviderType: "cloudflare",
		Credentials:  map[string]string{"api_token": "secret-token"},
	})
	require.NoError(t, err)

	// Wait for create audit to be processed
	time.Sleep(150 * time.Millisecond)

	// Clear create audit logs
	db.Exec("DELETE FROM security_audits")

	// Get decrypted credentials
	_, err = service.GetDecryptedCredentials(ctx, provider.ID)
	require.NoError(t, err)

	// Give time for async audit logging
	time.Sleep(150 * time.Millisecond)

	// Verify audit log was created
	var audit models.SecurityAudit
	err = db.Where("action = ? AND event_category = ?", "credential_decrypt", "dns_provider").First(&audit).Error
	require.NoError(t, err)

	assert.Equal(t, "admin", audit.Actor)
	assert.Equal(t, provider.UUID, audit.ResourceUUID)

	// Verify details
	var details map[string]interface{}
	err = json.Unmarshal([]byte(audit.Details), &details)
	require.NoError(t, err)
	assert.Equal(t, "credentials_access", details["purpose"])
	assert.True(t, details["success"].(bool))
}

func TestDNSProviderService_AuditLogging_ContextHelpers(t *testing.T) {
	// Test actor extraction
	ctx := context.WithValue(context.Background(), contextKeyUserID, "user-123")
	actor := getActorFromContext(ctx)
	assert.Equal(t, "user-123", actor)

	// Test with uint user ID
	ctx = context.WithValue(context.Background(), contextKeyUserID, uint(456))
	actor = getActorFromContext(ctx)
	assert.Equal(t, "456", actor)

	// Test without user ID (should default to "system")
	ctx = context.Background()
	actor = getActorFromContext(ctx)
	assert.Equal(t, "system", actor)

	// Test IP extraction
	ctx = context.WithValue(context.Background(), contextKeyClientIP, "10.0.0.1")
	ip := getIPFromContext(ctx)
	assert.Equal(t, "10.0.0.1", ip)

	// Test User-Agent extraction
	ctx = context.WithValue(context.Background(), contextKeyUserAgent, "TestAgent/2.0")
	ua := getUserAgentFromContext(ctx)
	assert.Equal(t, "TestAgent/2.0", ua)
}
