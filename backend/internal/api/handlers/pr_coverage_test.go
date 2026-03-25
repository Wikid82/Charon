package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// =============================================================================
// Additional Plugin Handler Tests for Coverage
// =============================================================================

func TestPluginHandler_EnablePlugin_DatabaseUpdateError(t *testing.T) {
	db := OpenTestDBWithMigrations(t)
	pluginLoader := services.NewPluginLoaderService(db, "/tmp/plugins", nil)

	// Create plugin
	plugin := models.Plugin{
		UUID:     "plugin-db-error-uuid",
		Name:     "Test Plugin",
		Type:     "test-type",
		Enabled:  false,
		Status:   models.PluginStatusError,
		FilePath: "/nonexistent/path.so",
	}
	db.Create(&plugin)

	handler := NewPluginHandler(db, pluginLoader)

	// Close DB to trigger error during update
	sqlDB, _ := db.DB()
	_ = sqlDB.Close()

	router := gin.New()
	router.POST("/plugins/:id/enable", handler.EnablePlugin)

	req := httptest.NewRequest(http.MethodPost, "/plugins/1/enable", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPluginHandler_DisablePlugin_DatabaseUpdateError(t *testing.T) {
	db := OpenTestDBWithMigrations(t)
	pluginLoader := services.NewPluginLoaderService(db, "/tmp/plugins", nil)

	// Create plugin
	plugin := models.Plugin{
		UUID:     "plugin-disable-error-uuid",
		Name:     "Test Plugin",
		Type:     "test-type-disable",
		Enabled:  true,
		Status:   models.PluginStatusLoaded,
		FilePath: "/path/to/plugin.so",
	}
	db.Create(&plugin)

	handler := NewPluginHandler(db, pluginLoader)

	// Close DB to trigger error during update
	sqlDB, _ := db.DB()
	_ = sqlDB.Close()

	router := gin.New()
	router.POST("/plugins/:id/disable", handler.DisablePlugin)

	req := httptest.NewRequest(http.MethodPost, "/plugins/1/disable", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPluginHandler_GetPlugin_DatabaseError(t *testing.T) {
	db := OpenTestDBWithMigrations(t)
	pluginLoader := services.NewPluginLoaderService(db, "/tmp/plugins", nil)

	// Create plugin first
	plugin := models.Plugin{
		UUID:     "get-error-uuid",
		Name:     "Get Error",
		Type:     "get-error-type",
		Enabled:  true,
		FilePath: "/path/to/get.so",
	}
	db.Create(&plugin)

	handler := NewPluginHandler(db, pluginLoader)

	// Close DB to trigger database error
	sqlDB, _ := db.DB()
	_ = sqlDB.Close()

	router := gin.New()
	router.GET("/plugins/:id", handler.GetPlugin)

	req := httptest.NewRequest(http.MethodGet, "/plugins/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to get plugin")
}

func TestPluginHandler_EnablePlugin_DatabaseFirstError(t *testing.T) {
	db := OpenTestDBWithMigrations(t)
	pluginLoader := services.NewPluginLoaderService(db, "/tmp/plugins", nil)

	handler := NewPluginHandler(db, pluginLoader)

	// Close DB to trigger error when fetching plugin
	sqlDB, _ := db.DB()
	_ = sqlDB.Close()

	router := gin.New()
	router.POST("/plugins/:id/enable", handler.EnablePlugin)

	req := httptest.NewRequest(http.MethodPost, "/plugins/1/enable", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to get plugin")
}

func TestPluginHandler_DisablePlugin_DatabaseFirstError(t *testing.T) {
	db := OpenTestDBWithMigrations(t)
	pluginLoader := services.NewPluginLoaderService(db, "/tmp/plugins", nil)

	handler := NewPluginHandler(db, pluginLoader)

	// Close DB to trigger error when fetching plugin
	sqlDB, _ := db.DB()
	_ = sqlDB.Close()

	router := gin.New()
	router.POST("/plugins/:id/disable", handler.DisablePlugin)

	req := httptest.NewRequest(http.MethodPost, "/plugins/1/disable", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to get plugin")
}

// =============================================================================
// Encryption Handler - Additional Coverage Tests
// =============================================================================

func TestEncryptionHandler_Validate_NonAdminAccess(t *testing.T) {

	currentKey, _ := crypto.GenerateNewKey()
	require.NoError(t, os.Setenv("CHARON_ENCRYPTION_KEY", currentKey))
	defer func() { require.NoError(t, os.Unsetenv("CHARON_ENCRYPTION_KEY")) }()

	db := setupEncryptionTestDB(t)
	rotationService, _ := crypto.NewRotationService(db)
	securityService := services.NewSecurityService(db)
	defer securityService.Close()

	handler := NewEncryptionHandler(rotationService, securityService)
	router := setupEncryptionTestRouter(handler, false)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/admin/encryption/validate", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestEncryptionHandler_GetHistory_PaginationBoundary(t *testing.T) {

	currentKey, _ := crypto.GenerateNewKey()
	require.NoError(t, os.Setenv("CHARON_ENCRYPTION_KEY", currentKey))
	defer func() { require.NoError(t, os.Unsetenv("CHARON_ENCRYPTION_KEY")) }()

	db := setupEncryptionTestDB(t)
	rotationService, _ := crypto.NewRotationService(db)
	securityService := services.NewSecurityService(db)
	defer securityService.Close()

	handler := NewEncryptionHandler(rotationService, securityService)
	router := setupEncryptionTestRouter(handler, true)

	// Test invalid page number (negative)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/admin/encryption/history?page=-1&limit=10", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Test limit exceeding max (should clamp)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/admin/encryption/history?page=1&limit=200", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &response)
	// limit should not exceed 100
	assert.LessOrEqual(t, response["limit"].(float64), float64(100))
}

func TestEncryptionHandler_GetStatus_VersionInfo(t *testing.T) {

	currentKey, _ := crypto.GenerateNewKey()
	require.NoError(t, os.Setenv("CHARON_ENCRYPTION_KEY", currentKey))
	defer func() {
		require.NoError(t, os.Unsetenv("CHARON_ENCRYPTION_KEY"))
	}()

	db := setupEncryptionTestDB(t)
	rotationService, _ := crypto.NewRotationService(db)
	securityService := services.NewSecurityService(db)
	defer securityService.Close()

	handler := NewEncryptionHandler(rotationService, securityService)
	router := setupEncryptionTestRouter(handler, true)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/admin/encryption/status", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var status crypto.RotationStatus
	err := json.Unmarshal(w.Body.Bytes(), &status)
	assert.NoError(t, err)
	// Verify the status response has expected fields
	assert.True(t, status.CurrentVersion >= 1)
}

// =============================================================================
// Settings Handler - Additional Unique Coverage Tests
// =============================================================================

func TestSettingsHandler_TestPublicURL_RoleNotExists(t *testing.T) {

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)

	handler := NewSettingsHandler(db)

	router := gin.New()
	// Don't set any role
	router.POST("/test-url", handler.TestPublicURL)

	body := `{"url": "https://example.com"}`
	req, _ := http.NewRequest("POST", "/test-url", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestSettingsHandler_TestPublicURL_InvalidURLFormat(t *testing.T) {

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)

	handler := NewSettingsHandler(db)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/test-url", handler.TestPublicURL)

	body := `{"url": "not-a-valid-url"}`
	req, _ := http.NewRequest("POST", "/test-url", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSettingsHandler_TestPublicURL_PrivateIPBlocked_Coverage(t *testing.T) {

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)

	handler := NewSettingsHandler(db)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/test-url", handler.TestPublicURL)

	// SSRF attempt with private IP
	body := `{"url": "http://192.168.1.1"}`
	req, _ := http.NewRequest("POST", "/test-url", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return 200 but with reachable=false due to SSRF protection
	assert.Equal(t, http.StatusOK, w.Code)
	var response map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &response)
	assert.False(t, response["reachable"].(bool))
}

func TestSettingsHandler_ValidatePublicURL_WithTrailingSlash(t *testing.T) {

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)

	handler := NewSettingsHandler(db)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/validate-url", handler.ValidatePublicURL)

	// URL with trailing slash (should normalize and may produce warning)
	body := `{"url": "https://example.com/"}`
	req, _ := http.NewRequest("POST", "/validate-url", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var response map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &response)
	assert.True(t, response["valid"].(bool))
}

func TestSettingsHandler_ValidatePublicURL_MissingScheme(t *testing.T) {

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)

	handler := NewSettingsHandler(db)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/validate-url", handler.ValidatePublicURL)

	// Invalid URL (missing scheme)
	body := `{"url": "example.com"}`
	req, _ := http.NewRequest("POST", "/validate-url", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var response map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &response)
	assert.False(t, response["valid"].(bool))
}

// =============================================================================
// Audit Log Handler - Additional Coverage Tests
// =============================================================================

func TestAuditLogHandler_List_PaginationEdgeCases(t *testing.T) {

	dbPath := fmt.Sprintf("/tmp/test_audit_pagination_%d.db", time.Now().UnixNano())
	t.Cleanup(func() { _ = os.Remove(dbPath) })

	db, _ := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	_ = db.AutoMigrate(&models.SecurityAudit{}, &models.DNSProvider{})

	// Create test audits
	for i := 0; i < 10; i++ {
		db.Create(&models.SecurityAudit{
			Actor:         "user1",
			Action:        fmt.Sprintf("action_%d", i),
			EventCategory: "test",
			Details:       "{}",
		})
	}

	secService := services.NewSecurityService(db)
	defer secService.Close()
	handler := NewAuditLogHandler(secService)

	router := gin.New()
	router.GET("/audit", handler.List)

	// Test with pagination
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/audit?page=2&limit=3", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuditLogHandler_List_CategoryFilter(t *testing.T) {

	dbPath := fmt.Sprintf("/tmp/test_audit_category_%d.db", time.Now().UnixNano())
	t.Cleanup(func() { _ = os.Remove(dbPath) })

	db, _ := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	_ = db.AutoMigrate(&models.SecurityAudit{}, &models.DNSProvider{})

	// Create test audits with different categories
	db.Create(&models.SecurityAudit{
		Actor:         "user1",
		Action:        "action1",
		EventCategory: "encryption",
		Details:       "{}",
	})
	db.Create(&models.SecurityAudit{
		Actor:         "user2",
		Action:        "action2",
		EventCategory: "security",
		Details:       "{}",
	})

	secService := services.NewSecurityService(db)
	defer secService.Close()
	handler := NewAuditLogHandler(secService)

	router := gin.New()
	router.GET("/audit", handler.List)

	// Test with category filter
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/audit?category=encryption", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuditLogHandler_ListByProvider_DatabaseError(t *testing.T) {

	dbPath := fmt.Sprintf("/tmp/test_audit_db_error_%d.db", time.Now().UnixNano())
	t.Cleanup(func() { _ = os.Remove(dbPath) })

	db, _ := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	_ = db.AutoMigrate(&models.SecurityAudit{}, &models.DNSProvider{})

	secService := services.NewSecurityService(db)
	defer secService.Close()
	handler := NewAuditLogHandler(secService)

	// Close DB to trigger error
	sqlDB, _ := db.DB()
	_ = sqlDB.Close()

	router := gin.New()
	router.GET("/audit/provider/:id", handler.ListByProvider)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/audit/provider/1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestAuditLogHandler_ListByProvider_InvalidProviderID(t *testing.T) {

	dbPath := fmt.Sprintf("/tmp/test_audit_invalid_id_%d.db", time.Now().UnixNano())
	t.Cleanup(func() { _ = os.Remove(dbPath) })

	db, _ := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	_ = db.AutoMigrate(&models.SecurityAudit{}, &models.DNSProvider{})

	secService := services.NewSecurityService(db)
	defer secService.Close()
	handler := NewAuditLogHandler(secService)

	router := gin.New()
	router.GET("/audit/provider/:id", handler.ListByProvider)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/audit/provider/invalid", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// =============================================================================
// getActorFromGinContext Additional Coverage
// =============================================================================

func TestGetActorFromGinContext_InvalidUserIDType(t *testing.T) {

	router := gin.New()
	var capturedActor string
	router.Use(func(c *gin.Context) {
		c.Set("user_id", 123.45) // float - invalid type
		c.Next()
	})
	router.GET("/test", func(c *gin.Context) {
		capturedActor = getActorFromGinContext(c)
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	// Should fall back to "system" for invalid type
	assert.Equal(t, "system", capturedActor)
}

// =============================================================================
// isAdmin Additional Coverage
// =============================================================================

func TestIsAdmin_NonAdminRole(t *testing.T) {

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_role", "user") // Not admin
		c.Next()
	})
	router.GET("/test", func(c *gin.Context) {
		if isAdmin(c) {
			c.JSON(http.StatusOK, gin.H{"admin": true})
		} else {
			c.JSON(http.StatusForbidden, gin.H{"admin": false})
		}
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// =============================================================================
// Credential Handler - Additional Coverage Tests
// =============================================================================

func setupCredentialHandlerTestWithCtx(t *testing.T) (*gin.Engine, *gorm.DB, *models.DNSProvider, context.Context) {
	require.NoError(t, os.Setenv("CHARON_ENCRYPTION_KEY", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="))
	t.Cleanup(func() { require.NoError(t, os.Unsetenv("CHARON_ENCRYPTION_KEY")) })

	router := gin.New()

	dbName := fmt.Sprintf("file:%s?mode=memory&cache=shared&_journal_mode=WAL", t.Name())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	require.NoError(t, err)

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

	testKey := "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	encryptor, _ := crypto.NewEncryptionService(testKey)

	creds := map[string]string{"api_token": "test-token"}
	credsJSON, _ := json.Marshal(creds)
	encrypted, _ := encryptor.Encrypt(credsJSON)

	provider := &models.DNSProvider{
		UUID:                 "test-uuid",
		Name:                 "Test Provider",
		ProviderType:         "cloudflare",
		Enabled:              true,
		UseMultiCredentials:  true,
		CredentialsEncrypted: encrypted,
		KeyVersion:           1,
	}
	db.Create(provider)

	credService := services.NewCredentialService(db, encryptor)
	credHandler := NewCredentialHandler(credService)

	router.GET("/api/v1/dns-providers/:id/credentials", credHandler.List)
	router.POST("/api/v1/dns-providers/:id/credentials", credHandler.Create)
	router.GET("/api/v1/dns-providers/:id/credentials/:cred_id", credHandler.Get)
	router.PUT("/api/v1/dns-providers/:id/credentials/:cred_id", credHandler.Update)
	router.DELETE("/api/v1/dns-providers/:id/credentials/:cred_id", credHandler.Delete)
	router.POST("/api/v1/dns-providers/:id/credentials/:cred_id/test", credHandler.Test)
	router.POST("/api/v1/dns-providers/:id/enable-multi-credentials", credHandler.EnableMultiCredentials)

	return router, db, provider, context.Background()
}

func TestCredentialHandler_Update_InvalidProviderType(t *testing.T) {
	router, db, _, _ := setupCredentialHandlerTestWithCtx(t)

	testKey := "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	encryptor, _ := crypto.NewEncryptionService(testKey)

	// Create provider with invalid type
	creds := map[string]string{"api_token": "test-token"}
	credsJSON, _ := json.Marshal(creds)
	encrypted, _ := encryptor.Encrypt(credsJSON)

	provider := &models.DNSProvider{
		UUID:                 "invalid-type-uuid",
		Name:                 "Invalid Type Provider",
		ProviderType:         "nonexistent-provider",
		Enabled:              true,
		UseMultiCredentials:  true,
		CredentialsEncrypted: encrypted,
		KeyVersion:           1,
	}
	db.Create(provider)

	// Create credential
	credService := services.NewCredentialService(db, encryptor)
	createReq := services.CreateCredentialRequest{
		Label:       "Original",
		Credentials: map[string]string{"api_token": "token"},
	}

	// This should fail because provider type doesn't exist
	_, err := credService.Create(context.Background(), provider.ID, createReq)
	if err != nil {
		// Expected - provider type validation fails
		return
	}

	// If it didn't fail, try update with bad credentials
	updateBody := `{"label":"Updated","credentials":{"invalid_field":"value"}}`
	url := fmt.Sprintf("/api/v1/dns-providers/%d/credentials/1", provider.ID)
	req, _ := http.NewRequest("PUT", url, strings.NewReader(updateBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCredentialHandler_List_DatabaseClosed(t *testing.T) {
	require.NoError(t, os.Setenv("CHARON_ENCRYPTION_KEY", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="))
	defer func() { require.NoError(t, os.Unsetenv("CHARON_ENCRYPTION_KEY")) }()

	router := gin.New()

	dbName := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, _ := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	_ = db.AutoMigrate(&models.DNSProvider{}, &models.DNSProviderCredential{})

	testKey := "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	encryptor, _ := crypto.NewEncryptionService(testKey)

	credService := services.NewCredentialService(db, encryptor)
	credHandler := NewCredentialHandler(credService)

	router.GET("/api/v1/dns-providers/:id/credentials", credHandler.List)

	// Close DB to trigger error
	sqlDB, _ := db.DB()
	_ = sqlDB.Close()

	req, _ := http.NewRequest("GET", "/api/v1/dns-providers/1/credentials", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// =============================================================================
// Settings Handler - MaskPasswordForTest Coverage (unique test name)
// =============================================================================

func TestSettingsHandler_MaskPasswordForTestFunction(t *testing.T) {
	tests := []struct {
		name     string
		password string
		expected string
	}{
		{"empty string", "", ""},
		{"non-empty password", "secret123", "********"},
		{"already masked", "********", "********"},
		{"single char", "x", "********"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskPasswordForTest(tt.password)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// =============================================================================
// Credential Handler - Additional Update/Delete Error Paths (unique names)
// =============================================================================

func TestCredentialHandler_Update_NotFoundError(t *testing.T) {
	router, _, provider, _ := setupCredentialHandlerTestWithCtx(t)

	updateBody := `{"label":"Updated","credentials":{"api_token":"new-token"}}`
	url := fmt.Sprintf("/api/v1/dns-providers/%d/credentials/9999", provider.ID)
	req, _ := http.NewRequest("PUT", url, strings.NewReader(updateBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "not found")
}

func TestCredentialHandler_Update_MalformedJSON(t *testing.T) {
	router, _, provider, _ := setupCredentialHandlerTestWithCtx(t)

	url := fmt.Sprintf("/api/v1/dns-providers/%d/credentials/1", provider.ID)
	req, _ := http.NewRequest("PUT", url, strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCredentialHandler_Update_BadCredentialID(t *testing.T) {
	router, _, provider, _ := setupCredentialHandlerTestWithCtx(t)

	url := fmt.Sprintf("/api/v1/dns-providers/%d/credentials/invalid", provider.ID)
	req, _ := http.NewRequest("PUT", url, strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid credential ID")
}

func TestCredentialHandler_Delete_NotFoundError(t *testing.T) {
	router, _, provider, _ := setupCredentialHandlerTestWithCtx(t)

	url := fmt.Sprintf("/api/v1/dns-providers/%d/credentials/9999", provider.ID)
	req, _ := http.NewRequest("DELETE", url, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCredentialHandler_Delete_BadCredentialID(t *testing.T) {
	router, _, provider, _ := setupCredentialHandlerTestWithCtx(t)

	url := fmt.Sprintf("/api/v1/dns-providers/%d/credentials/invalid", provider.ID)
	req, _ := http.NewRequest("DELETE", url, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCredentialHandler_Test_BadCredentialID(t *testing.T) {
	router, _, provider, _ := setupCredentialHandlerTestWithCtx(t)

	url := fmt.Sprintf("/api/v1/dns-providers/%d/credentials/invalid/test", provider.ID)
	req, _ := http.NewRequest("POST", url, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCredentialHandler_EnableMultiCredentials_BadProviderID(t *testing.T) {
	router, _, _, _ := setupCredentialHandlerTestWithCtx(t)

	req, _ := http.NewRequest("POST", "/api/v1/dns-providers/invalid/enable-multi-credentials", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// =============================================================================
// Encryption Handler - Additional Validate Success Test
// =============================================================================

func TestEncryptionHandler_Validate_AdminSuccess(t *testing.T) {

	currentKey, _ := crypto.GenerateNewKey()
	require.NoError(t, os.Setenv("CHARON_ENCRYPTION_KEY", currentKey))
	defer func() { require.NoError(t, os.Unsetenv("CHARON_ENCRYPTION_KEY")) }()

	db := setupEncryptionTestDB(t)
	rotationService, _ := crypto.NewRotationService(db)
	securityService := services.NewSecurityService(db)
	defer securityService.Close()

	handler := NewEncryptionHandler(rotationService, securityService)
	router := setupEncryptionTestRouter(handler, true)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/admin/encryption/validate", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
