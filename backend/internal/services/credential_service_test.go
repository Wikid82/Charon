package services_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	_ "github.com/Wikid82/charon/backend/pkg/dnsprovider/builtin" // Register built-in providers
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupCredentialTestDB(t *testing.T) (*gorm.DB, *crypto.EncryptionService) {
	// Use test name for unique database to avoid test interference
	// Enable WAL mode and busytimeout to prevent locking issues during concurrent tests
	dbName := fmt.Sprintf("file:%s?mode=memory&cache=shared&_journal_mode=WAL&_busy_timeout=5000", t.Name())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	require.NoError(t, err)

	// Close database connection when test completes
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})

	err = db.AutoMigrate(
		&models.DNSProvider{},
		&models.DNSProviderCredential{},
		&models.SecurityAudit{},
	)
	require.NoError(t, err)

	// Create encryption service with test key (32 bytes base64 encoded)
	testKey := "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=" // "0123456789abcdef0123456789abcdef" base64 encoded
	encryptor, err := crypto.NewEncryptionService(testKey)
	require.NoError(t, err)

	return db, encryptor
}

func createTestProvider(t *testing.T, db *gorm.DB, encryptor *crypto.EncryptionService, multiCred bool) *models.DNSProvider {
	creds := map[string]string{"api_token": "test-token"}
	credsJSON, _ := json.Marshal(creds)
	encrypted, _ := encryptor.Encrypt(credsJSON)

	provider := &models.DNSProvider{
		UUID:                 uuid.New().String(),
		Name:                 "Test Provider",
		ProviderType:         "cloudflare",
		Enabled:              true,
		UseMultiCredentials:  multiCred,
		CredentialsEncrypted: encrypted,
		KeyVersion:           1,
		PropagationTimeout:   120,
		PollingInterval:      5,
	}

	err := db.Create(provider).Error
	require.NoError(t, err)
	return provider
}

func TestCredentialService_Create(t *testing.T) {
	db, encryptor := setupCredentialTestDB(t)
	service := services.NewCredentialService(db, encryptor)
	ctx := context.Background()

	// Create provider with multi-credential enabled
	provider := createTestProvider(t, db, encryptor, true)

	req := services.CreateCredentialRequest{
		Label:      "Production Credential",
		ZoneFilter: "example.com",
		Credentials: map[string]string{
			"api_token": "prod-token-123",
		},
		PropagationTimeout: 180,
		PollingInterval:    10,
		Enabled:            true,
	}

	cred, err := service.Create(ctx, provider.ID, req)
	require.NoError(t, err)
	assert.NotNil(t, cred)
	assert.Equal(t, "Production Credential", cred.Label)
	assert.Equal(t, "example.com", cred.ZoneFilter)
	assert.Equal(t, provider.ID, cred.DNSProviderID)
	assert.Equal(t, 180, cred.PropagationTimeout)
	assert.Equal(t, 10, cred.PollingInterval)
	assert.True(t, cred.Enabled)
	assert.NotEmpty(t, cred.UUID)
	assert.NotEmpty(t, cred.CredentialsEncrypted)
}

func TestCredentialService_Create_MultiCredentialNotEnabled(t *testing.T) {
	db, encryptor := setupCredentialTestDB(t)
	service := services.NewCredentialService(db, encryptor)
	ctx := context.Background()

	// Create provider without multi-credential enabled
	provider := createTestProvider(t, db, encryptor, false)

	req := services.CreateCredentialRequest{
		Label:       "Test",
		Credentials: map[string]string{"api_token": "token"},
	}

	_, err := service.Create(ctx, provider.ID, req)
	assert.ErrorIs(t, err, services.ErrMultiCredentialNotEnabled)
}

func TestCredentialService_Create_InvalidCredentials(t *testing.T) {
	db, encryptor := setupCredentialTestDB(t)
	service := services.NewCredentialService(db, encryptor)
	ctx := context.Background()

	provider := createTestProvider(t, db, encryptor, true)

	req := services.CreateCredentialRequest{
		Label:       "Test",
		Credentials: map[string]string{}, // Missing required field
	}

	_, err := service.Create(ctx, provider.ID, req)
	assert.Error(t, err)
}

func TestCredentialService_List(t *testing.T) {
	db, encryptor := setupCredentialTestDB(t)
	service := services.NewCredentialService(db, encryptor)
	ctx := context.Background()

	provider := createTestProvider(t, db, encryptor, true)

	// Create multiple credentials with slight delay to avoid SQLite locking
	for i := 0; i < 3; i++ {
		req := services.CreateCredentialRequest{
			Label:       "Credential " + string(rune('A'+i)),
			ZoneFilter:  "",
			Credentials: map[string]string{"api_token": "token"},
		}
		_, err := service.Create(ctx, provider.ID, req)
		require.NoError(t, err)
		if i < 2 {
			time.Sleep(10 * time.Millisecond)
		}
	}

	creds, err := service.List(ctx, provider.ID)
	require.NoError(t, err)
	assert.Len(t, creds, 3)
}

func TestCredentialService_Get(t *testing.T) {
	db, encryptor := setupCredentialTestDB(t)
	service := services.NewCredentialService(db, encryptor)
	ctx := context.Background()

	provider := createTestProvider(t, db, encryptor, true)

	req := services.CreateCredentialRequest{
		Label:       "Test",
		Credentials: map[string]string{"api_token": "token"},
	}
	created, err := service.Create(ctx, provider.ID, req)
	require.NoError(t, err)

	cred, err := service.Get(ctx, provider.ID, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, cred.ID)
	assert.Equal(t, created.Label, cred.Label)
}

func TestCredentialService_Get_NotFound(t *testing.T) {
	db, encryptor := setupCredentialTestDB(t)
	service := services.NewCredentialService(db, encryptor)
	ctx := context.Background()

	provider := createTestProvider(t, db, encryptor, true)

	_, err := service.Get(ctx, provider.ID, 9999)
	assert.ErrorIs(t, err, services.ErrCredentialNotFound)
}

func TestCredentialService_Update(t *testing.T) {
	db, encryptor := setupCredentialTestDB(t)
	service := services.NewCredentialService(db, encryptor)
	ctx := context.Background()

	provider := createTestProvider(t, db, encryptor, true)

	req := services.CreateCredentialRequest{
		Label:       "Original",
		ZoneFilter:  "example.com",
		Credentials: map[string]string{"api_token": "token"},
	}
	created, err := service.Create(ctx, provider.ID, req)
	require.NoError(t, err)

	newLabel := "Updated Label"
	newZone := "*.example.com"
	enabled := false
	updateReq := services.UpdateCredentialRequest{
		Label:      &newLabel,
		ZoneFilter: &newZone,
		Enabled:    &enabled,
	}

	updated, err := service.Update(ctx, provider.ID, created.ID, updateReq)
	require.NoError(t, err)
	assert.Equal(t, "Updated Label", updated.Label)
	assert.Equal(t, "*.example.com", updated.ZoneFilter)
	assert.False(t, updated.Enabled)
}

func TestCredentialService_Delete(t *testing.T) {
	db, encryptor := setupCredentialTestDB(t)
	service := services.NewCredentialService(db, encryptor)
	ctx := context.Background()

	provider := createTestProvider(t, db, encryptor, true)

	req := services.CreateCredentialRequest{
		Label:       "To Delete",
		Credentials: map[string]string{"api_token": "token"},
	}
	created, err := service.Create(ctx, provider.ID, req)
	require.NoError(t, err)

	err = service.Delete(ctx, provider.ID, created.ID)
	require.NoError(t, err)

	_, err = service.Get(ctx, provider.ID, created.ID)
	assert.ErrorIs(t, err, services.ErrCredentialNotFound)
}

func TestCredentialService_Test(t *testing.T) {
	db, encryptor := setupCredentialTestDB(t)
	service := services.NewCredentialService(db, encryptor)
	ctx := context.Background()

	provider := createTestProvider(t, db, encryptor, true)

	req := services.CreateCredentialRequest{
		Label:       "Test",
		Credentials: map[string]string{"api_token": "token"},
	}
	created, err := service.Create(ctx, provider.ID, req)
	require.NoError(t, err)

	result, err := service.Test(ctx, provider.ID, created.ID)
	require.NoError(t, err)
	assert.NotNil(t, result)
	// Note: Actual test will depend on testDNSProviderCredentials implementation
}

func TestCredentialService_GetCredentialForDomain_ExactMatch(t *testing.T) {
	db, encryptor := setupCredentialTestDB(t)
	service := services.NewCredentialService(db, encryptor)
	ctx := context.Background()

	provider := createTestProvider(t, db, encryptor, true)

	// Create exact match credential
	req := services.CreateCredentialRequest{
		Label:       "Exact Match",
		ZoneFilter:  "example.com",
		Credentials: map[string]string{"api_token": "exact-token"},
	}
	exactCred, err := service.Create(ctx, provider.ID, req)
	require.NoError(t, err)

	// Create catch-all credential
	req2 := services.CreateCredentialRequest{
		Label:       "Catch All",
		ZoneFilter:  "",
		Credentials: map[string]string{"api_token": "catchall-token"},
	}
	_, err = service.Create(ctx, provider.ID, req2)
	require.NoError(t, err)

	// Test exact match
	cred, err := service.GetCredentialForDomain(ctx, provider.ID, "example.com")
	require.NoError(t, err)
	assert.Equal(t, exactCred.ID, cred.ID)
	assert.Equal(t, "Exact Match", cred.Label)
}

func TestCredentialService_GetCredentialForDomain_WildcardMatch(t *testing.T) {
	db, encryptor := setupCredentialTestDB(t)
	service := services.NewCredentialService(db, encryptor)
	ctx := context.Background()

	provider := createTestProvider(t, db, encryptor, true)

	// Create wildcard credential
	req := services.CreateCredentialRequest{
		Label:       "Wildcard",
		ZoneFilter:  "*.example.com",
		Credentials: map[string]string{"api_token": "wildcard-token"},
	}
	wildcardCred, err := service.Create(ctx, provider.ID, req)
	require.NoError(t, err)

	// Create catch-all
	req2 := services.CreateCredentialRequest{
		Label:       "Catch All",
		ZoneFilter:  "",
		Credentials: map[string]string{"api_token": "catchall-token"},
	}
	_, err = service.Create(ctx, provider.ID, req2)
	require.NoError(t, err)

	// Test wildcard match
	cred, err := service.GetCredentialForDomain(ctx, provider.ID, "app.example.com")
	require.NoError(t, err)
	assert.Equal(t, wildcardCred.ID, cred.ID)
	assert.Equal(t, "Wildcard", cred.Label)
}

func TestCredentialService_GetCredentialForDomain_CatchAll(t *testing.T) {
	db, encryptor := setupCredentialTestDB(t)
	service := services.NewCredentialService(db, encryptor)
	ctx := context.Background()

	provider := createTestProvider(t, db, encryptor, true)

	// Create catch-all credential
	req := services.CreateCredentialRequest{
		Label:       "Catch All",
		ZoneFilter:  "",
		Credentials: map[string]string{"api_token": "catchall-token"},
	}
	catchallCred, err := service.Create(ctx, provider.ID, req)
	require.NoError(t, err)

	// Test catch-all match
	cred, err := service.GetCredentialForDomain(ctx, provider.ID, "random.domain.com")
	require.NoError(t, err)
	assert.Equal(t, catchallCred.ID, cred.ID)
	assert.Equal(t, "Catch All", cred.Label)
}

func TestCredentialService_GetCredentialForDomain_NoMatch(t *testing.T) {
	db, encryptor := setupCredentialTestDB(t)
	service := services.NewCredentialService(db, encryptor)
	ctx := context.Background()

	provider := createTestProvider(t, db, encryptor, true)

	// Create specific credential without catch-all
	req := services.CreateCredentialRequest{
		Label:       "Specific",
		ZoneFilter:  "example.com",
		Credentials: map[string]string{"api_token": "token"},
	}
	_, err := service.Create(ctx, provider.ID, req)
	require.NoError(t, err)

	// Test no match
	_, err = service.GetCredentialForDomain(ctx, provider.ID, "other.com")
	assert.ErrorIs(t, err, services.ErrNoMatchingCredential)
}

func TestCredentialService_GetCredentialForDomain_MultiCredNotEnabled(t *testing.T) {
	db, encryptor := setupCredentialTestDB(t)
	service := services.NewCredentialService(db, encryptor)
	ctx := context.Background()

	// Create provider without multi-credential enabled
	provider := createTestProvider(t, db, encryptor, false)

	cred, err := service.GetCredentialForDomain(ctx, provider.ID, "example.com")
	require.NoError(t, err)
	assert.Nil(t, cred) // Should return nil when not using multi-credentials
}

func TestCredentialService_GetCredentialForDomain_MultipleZones(t *testing.T) {
	db, encryptor := setupCredentialTestDB(t)
	service := services.NewCredentialService(db, encryptor)
	ctx := context.Background()

	provider := createTestProvider(t, db, encryptor, true)

	// Create credential with multiple zones
	req := services.CreateCredentialRequest{
		Label:       "Multi-Zone",
		ZoneFilter:  "example.com,example.org",
		Credentials: map[string]string{"api_token": "multi-token"},
	}
	multiCred, err := service.Create(ctx, provider.ID, req)
	require.NoError(t, err)

	// Test first zone
	cred1, err := service.GetCredentialForDomain(ctx, provider.ID, "example.com")
	require.NoError(t, err)
	assert.Equal(t, multiCred.ID, cred1.ID)

	// Test second zone
	cred2, err := service.GetCredentialForDomain(ctx, provider.ID, "example.org")
	require.NoError(t, err)
	assert.Equal(t, multiCred.ID, cred2.ID)
}

func TestCredentialService_EnableMultiCredentials(t *testing.T) {
	db, encryptor := setupCredentialTestDB(t)
	service := services.NewCredentialService(db, encryptor)
	ctx := context.Background()

	// Create provider with credentials but multi-cred disabled
	provider := createTestProvider(t, db, encryptor, false)

	err := service.EnableMultiCredentials(ctx, provider.ID)
	require.NoError(t, err)

	// Verify provider is now in multi-credential mode
	var updatedProvider models.DNSProvider
	err = db.Where("id = ?", provider.ID).First(&updatedProvider).Error
	require.NoError(t, err)
	assert.True(t, updatedProvider.UseMultiCredentials)

	// Verify migrated credential was created
	var creds []models.DNSProviderCredential
	err = db.Where("dns_provider_id = ?", provider.ID).Find(&creds).Error
	require.NoError(t, err)
	assert.Len(t, creds, 1)
	assert.Equal(t, "Default (migrated)", creds[0].Label)
	assert.Equal(t, "", creds[0].ZoneFilter) // Catch-all
}

func TestCredentialService_EnableMultiCredentials_AlreadyEnabled(t *testing.T) {
	db, encryptor := setupCredentialTestDB(t)
	service := services.NewCredentialService(db, encryptor)
	ctx := context.Background()

	// Create provider with multi-cred already enabled
	provider := createTestProvider(t, db, encryptor, true)

	err := service.EnableMultiCredentials(ctx, provider.ID)
	require.NoError(t, err) // Should not error
}

func TestCredentialService_EnableMultiCredentials_NoCredentials(t *testing.T) {
	db, encryptor := setupCredentialTestDB(t)
	service := services.NewCredentialService(db, encryptor)
	ctx := context.Background()

	// Create provider without credentials
	provider := &models.DNSProvider{
		UUID:                "test-uuid",
		Name:                "Empty Provider",
		ProviderType:        "cloudflare",
		Enabled:             true,
		UseMultiCredentials: false,
		KeyVersion:          1,
	}
	err := db.Create(provider).Error
	require.NoError(t, err)

	err = service.EnableMultiCredentials(ctx, provider.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no credentials to migrate")
}

func TestCredentialService_GetCredentialForDomain_IDN(t *testing.T) {
	db, encryptor := setupCredentialTestDB(t)
	service := services.NewCredentialService(db, encryptor)
	ctx := context.Background()

	provider := createTestProvider(t, db, encryptor, true)

	// Create credential for IDN domain (punycode representation)
	req := services.CreateCredentialRequest{
		Label:       "IDN Domain",
		ZoneFilter:  "xn--e1afmkfd.xn--p1ai", // пример.рф in punycode
		Credentials: map[string]string{"api_token": "idn-token"},
	}
	idnCred, err := service.Create(ctx, provider.ID, req)
	require.NoError(t, err)

	// Test IDN match
	cred, err := service.GetCredentialForDomain(ctx, provider.ID, "xn--e1afmkfd.xn--p1ai")
	require.NoError(t, err)
	assert.Equal(t, idnCred.ID, cred.ID)
}
