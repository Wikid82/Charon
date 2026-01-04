package handlers_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wikid82/charon/backend/internal/api/handlers"
	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupCredentialHandlerTest(t *testing.T) (*gin.Engine, *gorm.DB, *models.DNSProvider) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Use test name for unique database with WAL mode to avoid locking issues
	dbName := fmt.Sprintf("file:%s?mode=memory&cache=shared&_journal_mode=WAL", t.Name())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	require.NoError(t, err)

	// Close database connection when test completes
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	})

	err = db.AutoMigrate(
		&models.DNSProvider{},
		&models.DNSProviderCredential{},
		&models.SecurityAudit{},
	)
	require.NoError(t, err)

	testKey := "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=" // "0123456789abcdef0123456789abcdef" base64 encoded
	encryptor, err := crypto.NewEncryptionService(testKey)
	require.NoError(t, err)

	// Create test provider with multi-credential enabled
	creds := map[string]string{"api_token": "test-token"}
	credsJSON, _ := json.Marshal(creds)
	encrypted, _ := encryptor.Encrypt(credsJSON)

	provider := &models.DNSProvider{
		UUID:                 uuid.New().String(),
		Name:                 "Test Provider",
		ProviderType:         "cloudflare",
		Enabled:              true,
		UseMultiCredentials:  true,
		CredentialsEncrypted: encrypted,
		KeyVersion:           1,
		PropagationTimeout:   120,
		PollingInterval:      5,
	}
	err = db.Create(provider).Error
	require.NoError(t, err)

	credService := services.NewCredentialService(db, encryptor)
	credHandler := handlers.NewCredentialHandler(credService)

	router.GET("/api/v1/dns-providers/:id/credentials", credHandler.List)
	router.POST("/api/v1/dns-providers/:id/credentials", credHandler.Create)
	router.GET("/api/v1/dns-providers/:id/credentials/:cred_id", credHandler.Get)
	router.PUT("/api/v1/dns-providers/:id/credentials/:cred_id", credHandler.Update)
	router.DELETE("/api/v1/dns-providers/:id/credentials/:cred_id", credHandler.Delete)
	router.POST("/api/v1/dns-providers/:id/credentials/:cred_id/test", credHandler.Test)
	router.POST("/api/v1/dns-providers/:id/enable-multi-credentials", credHandler.EnableMultiCredentials)

	return router, db, provider
}

func TestCredentialHandler_Create(t *testing.T) {
	router, _, provider := setupCredentialHandlerTest(t)

	reqBody := map[string]interface{}{
		"label":       "Test Credential",
		"zone_filter": "example.com",
		"credentials": map[string]string{
			"api_token": "test-token-123",
		},
		"propagation_timeout": 180,
		"polling_interval":    10,
		"enabled":             true,
	}
	body, _ := json.Marshal(reqBody)

	url := fmt.Sprintf("/api/v1/dns-providers/%d/credentials", provider.ID)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response models.DNSProviderCredential
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Test Credential", response.Label)
	assert.Equal(t, "example.com", response.ZoneFilter)
}

func TestCredentialHandler_Create_InvalidProviderID(t *testing.T) {
	router, _, _ := setupCredentialHandlerTest(t)

	reqBody := map[string]interface{}{
		"label":       "Test",
		"credentials": map[string]string{"api_token": "token"},
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/api/v1/dns-providers/invalid/credentials", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCredentialHandler_List(t *testing.T) {
	router, db, provider := setupCredentialHandlerTest(t)

	// Create test credentials
	testKey := "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	encryptor, _ := crypto.NewEncryptionService(testKey)
	credService := services.NewCredentialService(db, encryptor)

	for i := 0; i < 3; i++ {
		req := services.CreateCredentialRequest{
			Label:       "Credential " + string(rune('A'+i)),
			Credentials: map[string]string{"api_token": "token"},
		}
		_, err := credService.Create(testContext(), provider.ID, req)
		require.NoError(t, err)
	}

	url := fmt.Sprintf("/api/v1/dns-providers/%d/credentials", provider.ID)
	req, _ := http.NewRequest("GET", url, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response []models.DNSProviderCredential
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Len(t, response, 3)
}

func TestCredentialHandler_Get(t *testing.T) {
	router, db, provider := setupCredentialHandlerTest(t)

	testKey := "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	encryptor, _ := crypto.NewEncryptionService(testKey)
	credService := services.NewCredentialService(db, encryptor)

	createReq := services.CreateCredentialRequest{
		Label:       "Test Credential",
		Credentials: map[string]string{"api_token": "token"},
	}
	created, err := credService.Create(testContext(), provider.ID, createReq)
	require.NoError(t, err)

	url := fmt.Sprintf("/api/v1/dns-providers/%d/credentials/%d", provider.ID, created.ID)
	req, _ := http.NewRequest("GET", url, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.DNSProviderCredential
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, created.ID, response.ID)
}

func TestCredentialHandler_Get_NotFound(t *testing.T) {
	router, _, provider := setupCredentialHandlerTest(t)

	url := fmt.Sprintf("/api/v1/dns-providers/%d/credentials/9999", provider.ID)
	req, _ := http.NewRequest("GET", url, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCredentialHandler_Update(t *testing.T) {
	router, db, provider := setupCredentialHandlerTest(t)

	testKey := "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	encryptor, _ := crypto.NewEncryptionService(testKey)
	credService := services.NewCredentialService(db, encryptor)

	createReq := services.CreateCredentialRequest{
		Label:       "Original",
		Credentials: map[string]string{"api_token": "token"},
	}
	created, err := credService.Create(testContext(), provider.ID, createReq)
	require.NoError(t, err)

	updateBody := map[string]interface{}{
		"label":       "Updated Label",
		"zone_filter": "*.example.com",
		"enabled":     false,
	}
	body, _ := json.Marshal(updateBody)

	url := fmt.Sprintf("/api/v1/dns-providers/%d/credentials/%d", provider.ID, created.ID)
	req, _ := http.NewRequest("PUT", url, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.DNSProviderCredential
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Updated Label", response.Label)
	assert.Equal(t, "*.example.com", response.ZoneFilter)
	assert.False(t, response.Enabled)
}

func TestCredentialHandler_Delete(t *testing.T) {
	router, db, provider := setupCredentialHandlerTest(t)

	testKey := "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	encryptor, _ := crypto.NewEncryptionService(testKey)
	credService := services.NewCredentialService(db, encryptor)

	createReq := services.CreateCredentialRequest{
		Label:       "To Delete",
		Credentials: map[string]string{"api_token": "token"},
	}
	created, err := credService.Create(testContext(), provider.ID, createReq)
	require.NoError(t, err)

	url := fmt.Sprintf("/api/v1/dns-providers/%d/credentials/%d", provider.ID, created.ID)
	req, _ := http.NewRequest("DELETE", url, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)

	// Verify deletion
	_, err = credService.Get(testContext(), provider.ID, created.ID)
	assert.ErrorIs(t, err, services.ErrCredentialNotFound)
}

func TestCredentialHandler_Test(t *testing.T) {
	router, db, provider := setupCredentialHandlerTest(t)

	testKey := "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	encryptor, _ := crypto.NewEncryptionService(testKey)
	credService := services.NewCredentialService(db, encryptor)

	createReq := services.CreateCredentialRequest{
		Label:       "Test",
		Credentials: map[string]string{"api_token": "token"},
	}
	created, err := credService.Create(testContext(), provider.ID, createReq)
	require.NoError(t, err)

	url := fmt.Sprintf("/api/v1/dns-providers/%d/credentials/%d/test", provider.ID, created.ID)
	req, _ := http.NewRequest("POST", url, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response services.TestResult
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
}

func TestCredentialHandler_EnableMultiCredentials(t *testing.T) {
	router, db, _ := setupCredentialHandlerTest(t)

	// Create provider without multi-credential enabled
	testKey := "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	encryptor, _ := crypto.NewEncryptionService(testKey)
	creds := map[string]string{"api_token": "test-token"}
	credsJSON, _ := json.Marshal(creds)
	encrypted, _ := encryptor.Encrypt(credsJSON)

	provider := &models.DNSProvider{
		UUID:                 uuid.New().String(),
		Name:                 "Provider to Enable",
		ProviderType:         "cloudflare",
		Enabled:              true,
		UseMultiCredentials:  false,
		CredentialsEncrypted: encrypted,
		KeyVersion:           1,
	}
	err := db.Create(provider).Error
	require.NoError(t, err)

	url := fmt.Sprintf("/api/v1/dns-providers/%d/enable-multi-credentials", provider.ID)
	req, _ := http.NewRequest("POST", url, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify provider was updated
	var updatedProvider models.DNSProvider
	err = db.First(&updatedProvider, provider.ID).Error
	require.NoError(t, err)
	assert.True(t, updatedProvider.UseMultiCredentials)
}

func testContext() *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	return c
}
