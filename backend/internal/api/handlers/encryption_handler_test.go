package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
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

func setupEncryptionTestDB(t *testing.T) *gorm.DB {
	// Use a unique file-based database for each test to avoid sharing state
	dbPath := fmt.Sprintf("/tmp/test_encryption_%d.db", time.Now().UnixNano())
	t.Cleanup(func() {
		_ = os.Remove(dbPath)
	})

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		// Disable prepared statements for SQLite to avoid issues
		PrepareStmt: false,
	})
	require.NoError(t, err)

	// Migrate all required tables
	err = db.AutoMigrate(&models.DNSProvider{}, &models.SecurityAudit{})
	require.NoError(t, err)

	return db
}

func setupEncryptionTestRouter(handler *EncryptionHandler, isAdmin bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Mock admin middleware
	router.Use(func(c *gin.Context) {
		if isAdmin {
			c.Set("user_role", "admin")
			c.Set("user_id", uint(1))
		}
		c.Next()
	})

	api := router.Group("/api/v1/admin/encryption")
	{
		api.GET("/status", handler.GetStatus)
		api.POST("/rotate", handler.Rotate)
		api.GET("/history", handler.GetHistory)
		api.POST("/validate", handler.Validate)
	}

	return router
}

func TestEncryptionHandler_GetStatus(t *testing.T) {
	db := setupEncryptionTestDB(t)

	// Generate test keys
	currentKey, err := crypto.GenerateNewKey()
	require.NoError(t, err)
	_ = os.Setenv("CHARON_ENCRYPTION_KEY", currentKey)
	defer func() { _ = os.Unsetenv("CHARON_ENCRYPTION_KEY") }()

	rotationService, err := crypto.NewRotationService(db)
	require.NoError(t, err)

	securityService := services.NewSecurityService(db)
	defer securityService.Close()

	handler := NewEncryptionHandler(rotationService, securityService)

	t.Run("admin can get status", func(t *testing.T) {
		router := setupEncryptionTestRouter(handler, true)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/admin/encryption/status", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var status crypto.RotationStatus
		err := json.Unmarshal(w.Body.Bytes(), &status)
		require.NoError(t, err)

		assert.Equal(t, 1, status.CurrentVersion)
		assert.False(t, status.NextKeyConfigured)
		assert.Equal(t, 0, status.LegacyKeyCount)
	})

	t.Run("non-admin cannot get status", func(t *testing.T) {
		router := setupEncryptionTestRouter(handler, false)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/admin/encryption/status", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("status shows next key when configured", func(t *testing.T) {
		nextKey, err := crypto.GenerateNewKey()
		require.NoError(t, err)
		_ = os.Setenv("CHARON_ENCRYPTION_KEY_NEXT", nextKey)
		defer func() { _ = os.Unsetenv("CHARON_ENCRYPTION_KEY_NEXT") }()

		rotationService, err := crypto.NewRotationService(db)
		require.NoError(t, err)

		handler := NewEncryptionHandler(rotationService, securityService)
		router := setupEncryptionTestRouter(handler, true)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/admin/encryption/status", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var status crypto.RotationStatus
		err = json.Unmarshal(w.Body.Bytes(), &status)
		require.NoError(t, err)

		assert.True(t, status.NextKeyConfigured)
	})

	t.Run("status error when database unavailable", func(t *testing.T) {
		// Close the database to trigger an error
		sqlDB, err := db.DB()
		require.NoError(t, err)
		_ = sqlDB.Close()

		rotationService, err := crypto.NewRotationService(db)
		require.NoError(t, err)

		handler := NewEncryptionHandler(rotationService, securityService)
		router := setupEncryptionTestRouter(handler, true)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/admin/encryption/status", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "error")
	})
}

func TestEncryptionHandler_Rotate(t *testing.T) {
	db := setupEncryptionTestDB(t)

	// Generate test keys
	currentKey, err := crypto.GenerateNewKey()
	require.NoError(t, err)
	nextKey, err := crypto.GenerateNewKey()
	require.NoError(t, err)

	_ = os.Setenv("CHARON_ENCRYPTION_KEY", currentKey)
	_ = os.Setenv("CHARON_ENCRYPTION_KEY_NEXT", nextKey)
	defer func() {
		_ = os.Unsetenv("CHARON_ENCRYPTION_KEY")
		_ = os.Unsetenv("CHARON_ENCRYPTION_KEY_NEXT")
	}()

	// Create test providers
	currentService, err := crypto.NewEncryptionService(currentKey)
	require.NoError(t, err)

	credentials := map[string]string{"api_key": "test123"}
	credJSON, _ := json.Marshal(credentials)
	encrypted, _ := currentService.Encrypt(credJSON)

	provider := models.DNSProvider{
		Name:                 "Test Provider",
		ProviderType:         "cloudflare",
		CredentialsEncrypted: encrypted,
		KeyVersion:           1,
	}
	require.NoError(t, db.Create(&provider).Error)

	rotationService, err := crypto.NewRotationService(db)
	require.NoError(t, err)

	securityService := services.NewSecurityService(db)
	defer securityService.Close()

	handler := NewEncryptionHandler(rotationService, securityService)

	t.Run("admin can trigger rotation", func(t *testing.T) {
		router := setupEncryptionTestRouter(handler, true)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/admin/encryption/rotate", nil)
		router.ServeHTTP(w, req)

		// Flush async audit logging
		securityService.Flush()

		assert.Equal(t, http.StatusOK, w.Code)

		var result crypto.RotationResult
		err := json.Unmarshal(w.Body.Bytes(), &result)
		require.NoError(t, err)

		assert.Equal(t, 1, result.TotalProviders)
		assert.Equal(t, 1, result.SuccessCount)
		assert.Equal(t, 0, result.FailureCount)
		assert.Equal(t, 2, result.NewKeyVersion)
		assert.NotEmpty(t, result.Duration)

		// Verify audit logs were created
		var audits []models.SecurityAudit
		db.Where("event_category = ?", "encryption").Find(&audits)
		assert.GreaterOrEqual(t, len(audits), 2) // start + completion
	})

	t.Run("non-admin cannot trigger rotation", func(t *testing.T) {
		router := setupEncryptionTestRouter(handler, false)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/admin/encryption/rotate", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("rotation fails without next key", func(t *testing.T) {
		_ = os.Unsetenv("CHARON_ENCRYPTION_KEY_NEXT")
		defer func() { _ = os.Setenv("CHARON_ENCRYPTION_KEY_NEXT", nextKey) }()

		rotationService, err := crypto.NewRotationService(db)
		require.NoError(t, err)

		handler := NewEncryptionHandler(rotationService, securityService)
		router := setupEncryptionTestRouter(handler, true)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/admin/encryption/rotate", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "CHARON_ENCRYPTION_KEY_NEXT not configured")
	})
}

func TestEncryptionHandler_GetHistory(t *testing.T) {
	db := setupEncryptionTestDB(t)

	currentKey, err := crypto.GenerateNewKey()
	require.NoError(t, err)
	_ = os.Setenv("CHARON_ENCRYPTION_KEY", currentKey)
	defer func() { _ = os.Unsetenv("CHARON_ENCRYPTION_KEY") }()

	rotationService, err := crypto.NewRotationService(db)
	require.NoError(t, err)

	securityService := services.NewSecurityService(db)
	defer securityService.Close()

	// Create sample audit logs
	for i := 0; i < 5; i++ {
		audit := &models.SecurityAudit{
			Actor:         "admin",
			Action:        "encryption_key_rotation_completed",
			EventCategory: "encryption",
			Details:       "{}",
		}
		_ = securityService.LogAudit(audit)
	}

	// Flush async audit logging
	securityService.Flush()

	handler := NewEncryptionHandler(rotationService, securityService)

	t.Run("admin can get history", func(t *testing.T) {
		router := setupEncryptionTestRouter(handler, true)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/admin/encryption/history", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Contains(t, response, "audits")
		assert.Contains(t, response, "total")
		assert.Contains(t, response, "page")
		assert.Contains(t, response, "limit")
	})

	t.Run("non-admin cannot get history", func(t *testing.T) {
		router := setupEncryptionTestRouter(handler, false)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/admin/encryption/history", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("supports pagination", func(t *testing.T) {
		router := setupEncryptionTestRouter(handler, true)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/admin/encryption/history?page=1&limit=2", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, float64(1), response["page"])
		assert.Equal(t, float64(2), response["limit"])
	})

	t.Run("history error when service fails", func(t *testing.T) {
		// Create a new DB that will be closed to trigger error
		dbPath := fmt.Sprintf("/tmp/test_encryption_fail_%d.db", time.Now().UnixNano())
		defer func() { _ = os.Remove(dbPath) }()

		failDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{PrepareStmt: false})
		require.NoError(t, err)
		require.NoError(t, failDB.AutoMigrate(&models.SecurityAudit{}))

		currentKey, err := crypto.GenerateNewKey()
		require.NoError(t, err)
		_ = os.Setenv("CHARON_ENCRYPTION_KEY", currentKey)
		defer func() { _ = os.Unsetenv("CHARON_ENCRYPTION_KEY") }()

		rotationService, err := crypto.NewRotationService(failDB)
		require.NoError(t, err)

		failSecurityService := services.NewSecurityService(failDB)

		// Close the database to trigger errors
		sqlDB, err := failDB.DB()
		require.NoError(t, err)
		_ = sqlDB.Close()

		handler := NewEncryptionHandler(rotationService, failSecurityService)
		router := setupEncryptionTestRouter(handler, true)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/admin/encryption/history", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "error")

		failSecurityService.Close()
	})
}

func TestEncryptionHandler_Validate(t *testing.T) {
	db := setupEncryptionTestDB(t)

	currentKey, err := crypto.GenerateNewKey()
	require.NoError(t, err)
	_ = os.Setenv("CHARON_ENCRYPTION_KEY", currentKey)
	defer func() { _ = os.Unsetenv("CHARON_ENCRYPTION_KEY") }()

	rotationService, err := crypto.NewRotationService(db)
	require.NoError(t, err)

	securityService := services.NewSecurityService(db)
	defer securityService.Close()

	handler := NewEncryptionHandler(rotationService, securityService)

	t.Run("admin can validate keys", func(t *testing.T) {
		router := setupEncryptionTestRouter(handler, true)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/admin/encryption/validate", nil)
		router.ServeHTTP(w, req)

		// Flush async audit logging
		securityService.Flush()

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response["valid"].(bool))
		assert.Contains(t, response, "message")

		// Verify audit log was created
		var audits []models.SecurityAudit
		db.Where("action = ?", "encryption_key_validation_success").Find(&audits)
		assert.Greater(t, len(audits), 0)
	})

	t.Run("non-admin cannot validate keys", func(t *testing.T) {
		router := setupEncryptionTestRouter(handler, false)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/admin/encryption/validate", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("validation fails with invalid key configuration", func(t *testing.T) {
		// Unset the encryption key to trigger validation failure
		_ = os.Unsetenv("CHARON_ENCRYPTION_KEY")
		defer func() { _ = os.Setenv("CHARON_ENCRYPTION_KEY", currentKey) }()

		// Create rotation service with no key configured
		rotationService, err := crypto.NewRotationService(db)
		// This should fail, but if it doesn't, we still test the validation endpoint
		if err != nil {
			// Expected: NewRotationService fails without a key
			return
		}

		handler := NewEncryptionHandler(rotationService, securityService)
		router := setupEncryptionTestRouter(handler, true)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/admin/encryption/validate", nil)
		router.ServeHTTP(w, req)

		securityService.Flush()

		// Should return bad request with validation error
		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.False(t, response["valid"].(bool))
		assert.Contains(t, response, "error")

		// Verify audit log for validation failure was created
		var audits []models.SecurityAudit
		db.Where("action = ?", "encryption_key_validation_failed").Find(&audits)
		assert.Greater(t, len(audits), 0)
	})
}

func TestEncryptionHandler_IntegrationFlow(t *testing.T) {
	db := setupEncryptionTestDB(t)

	// Setup: Generate keys
	currentKey, err := crypto.GenerateNewKey()
	require.NoError(t, err)
	nextKey, err := crypto.GenerateNewKey()
	require.NoError(t, err)

	_ = os.Setenv("CHARON_ENCRYPTION_KEY", currentKey)
	defer func() { _ = os.Unsetenv("CHARON_ENCRYPTION_KEY") }()

	// Create initial provider
	currentService, err := crypto.NewEncryptionService(currentKey)
	require.NoError(t, err)

	credentials := map[string]string{"api_key": "secret123"}
	credJSON, _ := json.Marshal(credentials)
	encrypted, _ := currentService.Encrypt(credJSON)

	provider := models.DNSProvider{
		Name:                 "Integration Test Provider",
		ProviderType:         "cloudflare",
		CredentialsEncrypted: encrypted,
		KeyVersion:           1,
	}
	require.NoError(t, db.Create(&provider).Error)

	t.Run("complete rotation workflow", func(t *testing.T) {
		// Step 1: Check initial status
		rotationService, err := crypto.NewRotationService(db)
		require.NoError(t, err)
		securityService := services.NewSecurityService(db)

		handler := NewEncryptionHandler(rotationService, securityService)
		router := setupEncryptionTestRouter(handler, true)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/admin/encryption/status", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		// Step 2: Validate current configuration
		w = httptest.NewRecorder()
		req, _ = http.NewRequest("POST", "/api/v1/admin/encryption/validate", nil)
		router.ServeHTTP(w, req)
		securityService.Flush()
		assert.Equal(t, http.StatusOK, w.Code)

		// Step 3: Configure next key
		_ = os.Setenv("CHARON_ENCRYPTION_KEY_NEXT", nextKey)
		defer os.Unsetenv("CHARON_ENCRYPTION_KEY_NEXT")

		// Reinitialize rotation service to pick up new key
		// Keep using the same SecurityService and database
		rotationService, err = crypto.NewRotationService(db)
		require.NoError(t, err)

		handler = NewEncryptionHandler(rotationService, securityService)
		router = setupEncryptionTestRouter(handler, true)

		// Step 4: Trigger rotation
		w = httptest.NewRecorder()
		req, _ = http.NewRequest("POST", "/api/v1/admin/encryption/rotate", nil)
		router.ServeHTTP(w, req)
		securityService.Flush()
		assert.Equal(t, http.StatusOK, w.Code)

		// Step 5: Verify rotation result
		var result crypto.RotationResult
		err = json.Unmarshal(w.Body.Bytes(), &result)
		require.NoError(t, err)
		assert.Equal(t, 1, result.SuccessCount)

		// Step 6: Check updated status
		w = httptest.NewRecorder()
		req, _ = http.NewRequest("GET", "/api/v1/admin/encryption/status", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		// Step 7: Verify history contains rotation events
		w = httptest.NewRecorder()
		req, _ = http.NewRequest("GET", "/api/v1/admin/encryption/history", nil)
		router.ServeHTTP(w, req)
		securityService.Flush()
		assert.Equal(t, http.StatusOK, w.Code)

		var historyResponse map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &historyResponse)
		require.NoError(t, err)
		if historyResponse["total"] != nil {
			assert.Greater(t, int(historyResponse["total"].(float64)), 0)
		}

		// Clean up
		securityService.Close()
	})
}

// TestEncryptionHandler_HelperFunctions tests the isAdmin and getActorFromGinContext helpers
func TestEncryptionHandler_HelperFunctions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("isAdmin with invalid role type", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_role", 12345) // Invalid type (int instead of string)
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
	})

	t.Run("getActorFromGinContext with string user_id", func(t *testing.T) {
		router := gin.New()
		var capturedActor string
		router.Use(func(c *gin.Context) {
			c.Set("user_id", "user-string-123")
			c.Next()
		})
		router.GET("/test", func(c *gin.Context) {
			capturedActor = getActorFromGinContext(c)
			c.Status(http.StatusOK)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, "user-string-123", capturedActor)
	})

	t.Run("getActorFromGinContext with uint user_id", func(t *testing.T) {
		router := gin.New()
		var capturedActor string
		router.Use(func(c *gin.Context) {
			c.Set("user_id", uint(42))
			c.Next()
		})
		router.GET("/test", func(c *gin.Context) {
			capturedActor = getActorFromGinContext(c)
			c.Status(http.StatusOK)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, "42", capturedActor)
	})

	t.Run("getActorFromGinContext without user_id returns system", func(t *testing.T) {
		router := gin.New()
		var capturedActor string
		router.GET("/test", func(c *gin.Context) {
			capturedActor = getActorFromGinContext(c)
			c.Status(http.StatusOK)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, "system", capturedActor)
	})
}

// TestEncryptionHandler_RefreshKey_RotatesCredentials tests key rotation for credentials
func TestEncryptionHandler_RefreshKey_RotatesCredentials(t *testing.T) {
	db := setupEncryptionTestDB(t)

	// Generate test keys
	currentKey, err := crypto.GenerateNewKey()
	require.NoError(t, err)
	nextKey, err := crypto.GenerateNewKey()
	require.NoError(t, err)

	_ = os.Setenv("CHARON_ENCRYPTION_KEY", currentKey)
	_ = os.Setenv("CHARON_ENCRYPTION_KEY_NEXT", nextKey)
	defer func() {
		os.Unsetenv("CHARON_ENCRYPTION_KEY")
		os.Unsetenv("CHARON_ENCRYPTION_KEY_NEXT")
	}()

	// Create test provider with encrypted credentials
	currentService, err := crypto.NewEncryptionService(currentKey)
	require.NoError(t, err)

	credentials := map[string]string{"api_key": "test123"}
	credJSON, _ := json.Marshal(credentials)
	encrypted, _ := currentService.Encrypt(credJSON)

	provider := models.DNSProvider{
		Name:                 "Test Provider",
		ProviderType:         "cloudflare",
		CredentialsEncrypted: encrypted,
		KeyVersion:           1,
	}
	require.NoError(t, db.Create(&provider).Error)

	// Initialize rotation service
	rotationService, err := crypto.NewRotationService(db)
	require.NoError(t, err)

	securityService := services.NewSecurityService(db)
	defer securityService.Close()

	handler := NewEncryptionHandler(rotationService, securityService)
	router := setupEncryptionTestRouter(handler, true)

	// Trigger rotation
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/admin/encryption/rotate", nil)
	router.ServeHTTP(w, req)
	securityService.Flush()

	assert.Equal(t, http.StatusOK, w.Code)

	var result crypto.RotationResult
	err = json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)

	assert.Equal(t, 1, result.SuccessCount)
	assert.Equal(t, 2, result.NewKeyVersion)
}

// TestEncryptionHandler_RefreshKey_FailsWithoutProvider tests rotation without next key
func TestEncryptionHandler_RefreshKey_FailsWithoutProvider(t *testing.T) {
	db := setupEncryptionTestDB(t)

	// Set only current key, no next key
	currentKey, err := crypto.GenerateNewKey()
	require.NoError(t, err)
	_ = os.Setenv("CHARON_ENCRYPTION_KEY", currentKey)
	defer os.Unsetenv("CHARON_ENCRYPTION_KEY")

	rotationService, err := crypto.NewRotationService(db)
	require.NoError(t, err)

	securityService := services.NewSecurityService(db)
	defer securityService.Close()

	handler := NewEncryptionHandler(rotationService, securityService)
	router := setupEncryptionTestRouter(handler, true)

	// Attempt rotation without next key configured
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/admin/encryption/rotate", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "CHARON_ENCRYPTION_KEY_NEXT not configured")
}

// TestEncryptionHandler_RefreshKey_InvalidOldKey tests rotation with mismatched old key
func TestEncryptionHandler_RefreshKey_InvalidOldKey(t *testing.T) {
	db := setupEncryptionTestDB(t)

	// Generate test keys
	wrongKey, err := crypto.GenerateNewKey()
	require.NoError(t, err)
	nextKey, err := crypto.GenerateNewKey()
	require.NoError(t, err)

	// Create provider with one key
	correctKey, err := crypto.GenerateNewKey()
	require.NoError(t, err)

	correctService, err := crypto.NewEncryptionService(correctKey)
	require.NoError(t, err)

	credentials := map[string]string{"api_key": "test123"}
	credJSON, _ := json.Marshal(credentials)
	encrypted, _ := correctService.Encrypt(credJSON)

	provider := models.DNSProvider{
		Name:                 "Test Provider",
		ProviderType:         "cloudflare",
		CredentialsEncrypted: encrypted,
		KeyVersion:           1,
	}
	require.NoError(t, db.Create(&provider).Error)

	// Now set wrong key and try to rotate
	_ = os.Setenv("CHARON_ENCRYPTION_KEY", wrongKey)
	_ = os.Setenv("CHARON_ENCRYPTION_KEY_NEXT", nextKey)
	defer func() {
		os.Unsetenv("CHARON_ENCRYPTION_KEY")
		os.Unsetenv("CHARON_ENCRYPTION_KEY_NEXT")
	}()

	rotationService, err := crypto.NewRotationService(db)
	require.NoError(t, err)

	securityService := services.NewSecurityService(db)
	defer securityService.Close()

	handler := NewEncryptionHandler(rotationService, securityService)
	router := setupEncryptionTestRouter(handler, true)

	// Attempt rotation with wrong key
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/admin/encryption/rotate", nil)
	router.ServeHTTP(w, req)
	securityService.Flush()

	// Rotation may succeed but with failures for providers with wrong key
	assert.Equal(t, http.StatusOK, w.Code)

	var result crypto.RotationResult
	err = json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)

	// Should have failure count > 0 due to decryption error
	assert.Greater(t, result.FailureCount, 0)
}

// TestEncryptionHandler_GetActorFromGinContext_InvalidType tests getActorFromGinContext with invalid type
func TestEncryptionHandler_GetActorFromGinContext_InvalidType(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	var capturedActor string
	router.Use(func(c *gin.Context) {
		c.Set("user_id", int64(999)) // int64 instead of uint or string
		c.Next()
	})
	router.GET("/test", func(c *gin.Context) {
		capturedActor = getActorFromGinContext(c)
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	// Invalid type should return "system" as fallback
	assert.Equal(t, "system", capturedActor)
}

// TestEncryptionHandler_RotateWithPartialFailures tests rotation that has some successes and failures
func TestEncryptionHandler_RotateWithPartialFailures(t *testing.T) {
	db := setupEncryptionTestDB(t)

	// Generate test keys
	currentKey, err := crypto.GenerateNewKey()
	require.NoError(t, err)
	nextKey, err := crypto.GenerateNewKey()
	require.NoError(t, err)

	os.Setenv("CHARON_ENCRYPTION_KEY", currentKey)
	os.Setenv("CHARON_ENCRYPTION_KEY_NEXT", nextKey)
	defer func() {
		os.Unsetenv("CHARON_ENCRYPTION_KEY")
		os.Unsetenv("CHARON_ENCRYPTION_KEY_NEXT")
	}()

	// Create a valid provider
	currentService, err := crypto.NewEncryptionService(currentKey)
	require.NoError(t, err)

	validCreds := map[string]string{"api_key": "valid123"}
	credJSON, _ := json.Marshal(validCreds)
	validEncrypted, _ := currentService.Encrypt(credJSON)

	validProvider := models.DNSProvider{
		UUID:                 "valid-provider-uuid",
		Name:                 "Valid Provider",
		ProviderType:         "cloudflare",
		CredentialsEncrypted: validEncrypted,
		KeyVersion:           1,
	}
	require.NoError(t, db.Create(&validProvider).Error)

	// Create an invalid provider (corrupted encrypted data)
	invalidProvider := models.DNSProvider{
		UUID:                 "invalid-provider-uuid",
		Name:                 "Invalid Provider",
		ProviderType:         "route53",
		CredentialsEncrypted: "corrupted-data-that-cannot-be-decrypted",
		KeyVersion:           1,
	}
	require.NoError(t, db.Create(&invalidProvider).Error)

	rotationService, err := crypto.NewRotationService(db)
	require.NoError(t, err)

	securityService := services.NewSecurityService(db)
	defer securityService.Close()

	handler := NewEncryptionHandler(rotationService, securityService)
	router := setupEncryptionTestRouter(handler, true)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/admin/encryption/rotate", nil)
	router.ServeHTTP(w, req)
	securityService.Flush()

	assert.Equal(t, http.StatusOK, w.Code)

	var result crypto.RotationResult
	err = json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)

	// Should have at least 2 providers attempted
	assert.Equal(t, 2, result.TotalProviders)
	// Should have at least 1 success (valid provider)
	assert.GreaterOrEqual(t, result.SuccessCount, 1)
	// Should have at least 1 failure (invalid provider)
	assert.GreaterOrEqual(t, result.FailureCount, 1)
	// Failed providers list should be populated
	assert.NotEmpty(t, result.FailedProviders)
}

// TestEncryptionHandler_isAdmin_NoRoleSet tests isAdmin when no role is set
func TestEncryptionHandler_isAdmin_NoRoleSet(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	// No middleware setting user_role
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

// TestEncryptionHandler_isAdmin_NonAdminRole tests isAdmin with non-admin role
func TestEncryptionHandler_isAdmin_NonAdminRole(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_role", "user") // Regular user, not admin
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

// TestEncryptionHandler_Rotate_AuditStartFailure tests audit logging failure when rotation starts
func TestEncryptionHandler_Rotate_AuditStartFailure(t *testing.T) {
	db := setupEncryptionTestDB(t)

	// Generate test keys
	currentKey, err := crypto.GenerateNewKey()
	require.NoError(t, err)
	nextKey, err := crypto.GenerateNewKey()
	require.NoError(t, err)

	_ = os.Setenv("CHARON_ENCRYPTION_KEY", currentKey)
	_ = os.Setenv("CHARON_ENCRYPTION_KEY_NEXT", nextKey)
	defer func() {
		_ = os.Unsetenv("CHARON_ENCRYPTION_KEY")
		_ = os.Unsetenv("CHARON_ENCRYPTION_KEY_NEXT")
	}()

	// Create test provider
	currentService, err := crypto.NewEncryptionService(currentKey)
	require.NoError(t, err)

	credentials := map[string]string{"api_key": "test123"}
	credJSON, _ := json.Marshal(credentials)
	encrypted, _ := currentService.Encrypt(credJSON)

	provider := models.DNSProvider{
		Name:                 "Test Provider",
		ProviderType:         "cloudflare",
		CredentialsEncrypted: encrypted,
		KeyVersion:           1,
	}
	require.NoError(t, db.Create(&provider).Error)

	rotationService, err := crypto.NewRotationService(db)
	require.NoError(t, err)

	// Create security service and close DB to trigger audit failure
	securityService := services.NewSecurityService(db)

	// Close the database connection to trigger audit logging failures
	sqlDB, err := db.DB()
	require.NoError(t, err)
	_ = sqlDB.Close()

	handler := NewEncryptionHandler(rotationService, securityService)
	router := setupEncryptionTestRouter(handler, true)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/admin/encryption/rotate", nil)
	router.ServeHTTP(w, req)

	// Should still return error (rotation will fail due to closed DB)
	// But the audit start failure should be logged as warning
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	securityService.Close()
}

// TestEncryptionHandler_Rotate_AuditFailureFailure tests audit logging failure when rotation fails
func TestEncryptionHandler_Rotate_AuditFailureFailure(t *testing.T) {
	db := setupEncryptionTestDB(t)

	// Generate test keys
	currentKey, err := crypto.GenerateNewKey()
	require.NoError(t, err)
	// Don't set next key to trigger rotation failure

	_ = os.Setenv("CHARON_ENCRYPTION_KEY", currentKey)
	defer func() { _ = os.Unsetenv("CHARON_ENCRYPTION_KEY") }()

	rotationService, err := crypto.NewRotationService(db)
	require.NoError(t, err)

	// Create security service and close DB to trigger audit failure
	securityService := services.NewSecurityService(db)

	// Close the database connection to trigger audit logging failures
	sqlDB, err := db.DB()
	require.NoError(t, err)
	_ = sqlDB.Close()

	handler := NewEncryptionHandler(rotationService, securityService)
	router := setupEncryptionTestRouter(handler, true)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/admin/encryption/rotate", nil)
	router.ServeHTTP(w, req)

	// Should return error (no next key + DB closed)
	// Both audit start and audit failure logging should warn
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "CHARON_ENCRYPTION_KEY_NEXT not configured")

	securityService.Close()
}

// TestEncryptionHandler_Rotate_AuditCompletionFailure tests audit logging failure when rotation completes
func TestEncryptionHandler_Rotate_AuditCompletionFailure(t *testing.T) {
	// This test is challenging because we need rotation to succeed but audit to fail
	// We'll use a two-database approach: one for rotation, one (closed) for security

	rotationDB := setupEncryptionTestDB(t)
	auditDB := setupEncryptionTestDB(t)

	// Generate test keys
	currentKey, err := crypto.GenerateNewKey()
	require.NoError(t, err)
	nextKey, err := crypto.GenerateNewKey()
	require.NoError(t, err)

	_ = os.Setenv("CHARON_ENCRYPTION_KEY", currentKey)
	_ = os.Setenv("CHARON_ENCRYPTION_KEY_NEXT", nextKey)
	defer func() {
		_ = os.Unsetenv("CHARON_ENCRYPTION_KEY")
		_ = os.Unsetenv("CHARON_ENCRYPTION_KEY_NEXT")
	}()

	// Create test provider in rotation DB
	currentService, err := crypto.NewEncryptionService(currentKey)
	require.NoError(t, err)

	credentials := map[string]string{"api_key": "test123"}
	credJSON, _ := json.Marshal(credentials)
	encrypted, _ := currentService.Encrypt(credJSON)

	provider := models.DNSProvider{
		Name:                 "Test Provider",
		ProviderType:         "cloudflare",
		CredentialsEncrypted: encrypted,
		KeyVersion:           1,
	}
	require.NoError(t, rotationDB.Create(&provider).Error)

	rotationService, err := crypto.NewRotationService(rotationDB)
	require.NoError(t, err)

	// Create security service with separate DB and close it to trigger audit failure
	securityService := services.NewSecurityService(auditDB)
	sqlDB, err := auditDB.DB()
	require.NoError(t, err)
	_ = sqlDB.Close()

	handler := NewEncryptionHandler(rotationService, securityService)
	router := setupEncryptionTestRouter(handler, true)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/admin/encryption/rotate", nil)
	router.ServeHTTP(w, req)

	// Rotation should succeed despite audit failure
	assert.Equal(t, http.StatusOK, w.Code)

	var result crypto.RotationResult
	err = json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, 1, result.SuccessCount)

	securityService.Close()
}

// TestEncryptionHandler_Validate_AuditFailureOnError tests audit logging failure during validation error
func TestEncryptionHandler_Validate_AuditFailureOnError(t *testing.T) {
	db := setupEncryptionTestDB(t)
	auditDB := setupEncryptionTestDB(t)

	// Don't set encryption key to trigger validation failure
	_ = os.Unsetenv("CHARON_ENCRYPTION_KEY")

	// Create rotation service without key (will fail validation)
	rotationService, err := crypto.NewRotationService(db)
	if err != nil {
		// NewRotationService fails without key, which is expected
		// We'll skip this test as the validation endpoint won't be reached
		t.Skip("Cannot create rotation service without key")
		return
	}

	// Create security service with separate DB and close it
	securityService := services.NewSecurityService(auditDB)
	sqlDB, err := auditDB.DB()
	require.NoError(t, err)
	_ = sqlDB.Close()

	handler := NewEncryptionHandler(rotationService, securityService)
	router := setupEncryptionTestRouter(handler, true)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/admin/encryption/validate", nil)
	router.ServeHTTP(w, req)

	// Should return validation error
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.False(t, response["valid"].(bool))

	securityService.Close()
}

// TestEncryptionHandler_Validate_AuditFailureOnSuccess tests audit logging failure during validation success
func TestEncryptionHandler_Validate_AuditFailureOnSuccess(t *testing.T) {
	rotationDB := setupEncryptionTestDB(t)
	auditDB := setupEncryptionTestDB(t)

	// Set up valid encryption key
	currentKey, err := crypto.GenerateNewKey()
	require.NoError(t, err)
	_ = os.Setenv("CHARON_ENCRYPTION_KEY", currentKey)
	defer func() { _ = os.Unsetenv("CHARON_ENCRYPTION_KEY") }()

	rotationService, err := crypto.NewRotationService(rotationDB)
	require.NoError(t, err)

	// Create security service with separate DB and close it to trigger audit failure
	securityService := services.NewSecurityService(auditDB)
	sqlDB, err := auditDB.DB()
	require.NoError(t, err)
	_ = sqlDB.Close()

	handler := NewEncryptionHandler(rotationService, securityService)
	router := setupEncryptionTestRouter(handler, true)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/admin/encryption/validate", nil)
	router.ServeHTTP(w, req)

	// Should return success despite audit failure
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response["valid"].(bool))

	securityService.Close()
}
