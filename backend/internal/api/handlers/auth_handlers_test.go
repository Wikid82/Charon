package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupAuthHandlersTestDB(t *testing.T) *gorm.DB {
	dsn := filepath.Join(t.TempDir(), "test.db") + "?_busy_timeout=5000&_journal_mode=WAL"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(
		&models.AuthUser{},
		&models.AuthProvider{},
		&models.AuthPolicy{},
		&models.ProxyHost{},
	)
	require.NoError(t, err)

	return db
}

func setupAuthTestRouter(db *gorm.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	userHandler := NewAuthUserHandler(db)
	providerHandler := NewAuthProviderHandler(db)
	policyHandler := NewAuthPolicyHandler(db)

	api := router.Group("/api/v1")

	// Auth User routes
	api.GET("/security/users", userHandler.List)
	api.GET("/security/users/stats", userHandler.Stats)
	api.GET("/security/users/:uuid", userHandler.Get)
	api.POST("/security/users", userHandler.Create)
	api.PUT("/security/users/:uuid", userHandler.Update)
	api.DELETE("/security/users/:uuid", userHandler.Delete)

	// Auth Provider routes
	api.GET("/security/providers", providerHandler.List)
	api.GET("/security/providers/:uuid", providerHandler.Get)
	api.POST("/security/providers", providerHandler.Create)
	api.PUT("/security/providers/:uuid", providerHandler.Update)
	api.DELETE("/security/providers/:uuid", providerHandler.Delete)

	// Auth Policy routes
	api.GET("/security/policies", policyHandler.List)
	api.GET("/security/policies/:uuid", policyHandler.Get)
	api.POST("/security/policies", policyHandler.Create)
	api.PUT("/security/policies/:uuid", policyHandler.Update)
	api.DELETE("/security/policies/:uuid", policyHandler.Delete)

	return router
}

// ==================== Auth User Tests ====================

func TestAuthUserHandler_List(t *testing.T) {
	db := setupAuthHandlersTestDB(t)
	router := setupAuthTestRouter(db)

	// Create test users
	user := models.AuthUser{Username: "testuser", Email: "test@example.com", Enabled: true}
	user.SetPassword("password123")
	db.Create(&user)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/security/users", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var users []models.AuthUser
	err := json.Unmarshal(w.Body.Bytes(), &users)
	assert.NoError(t, err)
	assert.Len(t, users, 1)
	assert.Equal(t, "testuser", users[0].Username)
}

func TestAuthUserHandler_Get(t *testing.T) {
	db := setupAuthHandlersTestDB(t)
	router := setupAuthTestRouter(db)

	user := models.AuthUser{Username: "testuser", Email: "test@example.com"}
	user.SetPassword("password123")
	db.Create(&user)

	t.Run("found", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/security/users/"+user.UUID, nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var result models.AuthUser
		json.Unmarshal(w.Body.Bytes(), &result)
		assert.Equal(t, "testuser", result.Username)
	})

	t.Run("not found", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/security/users/nonexistent", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestAuthUserHandler_Create(t *testing.T) {
	db := setupAuthHandlersTestDB(t)
	router := setupAuthTestRouter(db)

	t.Run("success", func(t *testing.T) {
		body := map[string]interface{}{
			"username": "newuser",
			"email":    "new@example.com",
			"name":     "New User",
			"password": "password123",
			"roles":    "user",
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/security/users", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var result models.AuthUser
		json.Unmarshal(w.Body.Bytes(), &result)
		assert.Equal(t, "newuser", result.Username)
		assert.True(t, result.Enabled)
	})

	t.Run("with additional emails", func(t *testing.T) {
		body := map[string]interface{}{
			"username":          "multiemail",
			"email":             "primary@example.com",
			"password":          "password123",
			"additional_emails": "alt1@example.com,alt2@example.com",
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/security/users", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var result models.AuthUser
		json.Unmarshal(w.Body.Bytes(), &result)
		assert.Equal(t, "multiemail", result.Username)
		assert.Equal(t, "alt1@example.com,alt2@example.com", result.AdditionalEmails)
	})

	t.Run("invalid email", func(t *testing.T) {
		body := map[string]interface{}{
			"username": "baduser",
			"email":    "not-an-email",
			"password": "password123",
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/security/users", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestAuthUserHandler_Update(t *testing.T) {
	db := setupAuthHandlersTestDB(t)
	router := setupAuthTestRouter(db)

	user := models.AuthUser{Username: "testuser", Email: "test@example.com", Enabled: true}
	user.SetPassword("password123")
	db.Create(&user)

	t.Run("success", func(t *testing.T) {
		body := map[string]interface{}{
			"name":    "Updated Name",
			"enabled": false,
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/api/v1/security/users/"+user.UUID, bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var result models.AuthUser
		json.Unmarshal(w.Body.Bytes(), &result)
		assert.Equal(t, "Updated Name", result.Name)
		assert.False(t, result.Enabled)
	})

	t.Run("update additional emails", func(t *testing.T) {
		body := map[string]interface{}{
			"additional_emails": "newalt@example.com",
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/api/v1/security/users/"+user.UUID, bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var result models.AuthUser
		json.Unmarshal(w.Body.Bytes(), &result)
		assert.Equal(t, "newalt@example.com", result.AdditionalEmails)
	})

	t.Run("not found", func(t *testing.T) {
		body := map[string]interface{}{"name": "Test"}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/api/v1/security/users/nonexistent", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestAuthUserHandler_Delete(t *testing.T) {
	db := setupAuthHandlersTestDB(t)
	router := setupAuthTestRouter(db)

	t.Run("success", func(t *testing.T) {
		user := models.AuthUser{Username: "deleteuser", Email: "delete@example.com"}
		user.SetPassword("password123")
		db.Create(&user)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/api/v1/security/users/"+user.UUID, nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Verify deleted
		var count int64
		db.Model(&models.AuthUser{}).Where("uuid = ?", user.UUID).Count(&count)
		assert.Equal(t, int64(0), count)
	})

	t.Run("cannot delete last admin", func(t *testing.T) {
		admin := models.AuthUser{Username: "admin", Email: "admin@example.com", Roles: "admin"}
		admin.SetPassword("password123")
		db.Create(&admin)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/api/v1/security/users/"+admin.UUID, nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "last admin")
	})

	t.Run("not found", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/api/v1/security/users/nonexistent", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestAuthUserHandler_Stats(t *testing.T) {
	db := setupAuthHandlersTestDB(t)
	router := setupAuthTestRouter(db)

	user1 := models.AuthUser{Username: "stats_user1", Email: "stats1@example.com", Enabled: true, MFAEnabled: true}
	user1.SetPassword("password123")
	// user2 needs Enabled: false, but GORM's default:true overrides the zero value
	// So we create it first then update
	user2 := models.AuthUser{Username: "stats_user2", Email: "stats2@example.com", MFAEnabled: false}
	user2.SetPassword("password123")
	require.NoError(t, db.Create(&user1).Error)
	require.NoError(t, db.Create(&user2).Error)
	// Explicitly set Enabled to false after create
	require.NoError(t, db.Model(&user2).Update("enabled", false).Error)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/security/users/stats", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var stats map[string]int64
	err := json.Unmarshal(w.Body.Bytes(), &stats)
	require.NoError(t, err)
	assert.Equal(t, int64(2), stats["total"])
	assert.Equal(t, int64(1), stats["enabled"])
	assert.Equal(t, int64(1), stats["with_mfa"])
}

// ==================== Auth Provider Tests ====================

func TestAuthProviderHandler_List(t *testing.T) {
	db := setupAuthHandlersTestDB(t)
	router := setupAuthTestRouter(db)

	provider := models.AuthProvider{Name: "TestProvider", Type: "oidc", ClientID: "id", ClientSecret: "secret"}
	db.Create(&provider)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/security/providers", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var providers []models.AuthProvider
	json.Unmarshal(w.Body.Bytes(), &providers)
	assert.Len(t, providers, 1)
}

func TestAuthProviderHandler_Get(t *testing.T) {
	db := setupAuthHandlersTestDB(t)
	router := setupAuthTestRouter(db)

	provider := models.AuthProvider{Name: "TestProvider", Type: "oidc", ClientID: "id", ClientSecret: "secret"}
	db.Create(&provider)

	t.Run("found", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/security/providers/"+provider.UUID, nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("not found", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/security/providers/nonexistent", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestAuthProviderHandler_Create(t *testing.T) {
	db := setupAuthHandlersTestDB(t)
	router := setupAuthTestRouter(db)

	t.Run("success", func(t *testing.T) {
		body := map[string]interface{}{
			"name":          "NewProvider",
			"type":          "oidc",
			"client_id":     "client123",
			"client_secret": "secret456",
			"issuer_url":    "https://auth.example.com",
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/security/providers", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
	})

	t.Run("missing required fields", func(t *testing.T) {
		body := map[string]interface{}{
			"name": "Incomplete",
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/security/providers", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestAuthProviderHandler_Update(t *testing.T) {
	db := setupAuthHandlersTestDB(t)
	router := setupAuthTestRouter(db)

	provider := models.AuthProvider{Name: "TestProvider", Type: "oidc", ClientID: "id", ClientSecret: "secret", Enabled: true}
	db.Create(&provider)

	t.Run("success", func(t *testing.T) {
		body := map[string]interface{}{
			"name":    "UpdatedProvider",
			"enabled": false,
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/api/v1/security/providers/"+provider.UUID, bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var result models.AuthProvider
		json.Unmarshal(w.Body.Bytes(), &result)
		assert.Equal(t, "UpdatedProvider", result.Name)
		assert.False(t, result.Enabled)
	})

	t.Run("not found", func(t *testing.T) {
		body := map[string]interface{}{"name": "Test"}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/api/v1/security/providers/nonexistent", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestAuthProviderHandler_Delete(t *testing.T) {
	db := setupAuthHandlersTestDB(t)
	router := setupAuthTestRouter(db)

	provider := models.AuthProvider{Name: "DeleteProvider", Type: "oidc", ClientID: "id", ClientSecret: "secret"}
	db.Create(&provider)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/security/providers/"+provider.UUID, nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ==================== Auth Policy Tests ====================

func TestAuthPolicyHandler_List(t *testing.T) {
	db := setupAuthHandlersTestDB(t)
	router := setupAuthTestRouter(db)

	policy := models.AuthPolicy{Name: "TestPolicy", AllowedRoles: "admin"}
	db.Create(&policy)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/security/policies", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var policies []models.AuthPolicy
	json.Unmarshal(w.Body.Bytes(), &policies)
	assert.Len(t, policies, 1)
}

func TestAuthPolicyHandler_Get(t *testing.T) {
	db := setupAuthHandlersTestDB(t)
	router := setupAuthTestRouter(db)

	policy := models.AuthPolicy{Name: "TestPolicy"}
	db.Create(&policy)

	t.Run("found", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/security/policies/"+policy.UUID, nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("not found", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/security/policies/nonexistent", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestAuthPolicyHandler_Create(t *testing.T) {
	db := setupAuthHandlersTestDB(t)
	router := setupAuthTestRouter(db)

	t.Run("success", func(t *testing.T) {
		body := map[string]interface{}{
			"name":            "NewPolicy",
			"description":     "A test policy",
			"allowed_roles":   "admin,user",
			"session_timeout": 3600,
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/security/policies", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var result models.AuthPolicy
		json.Unmarshal(w.Body.Bytes(), &result)
		assert.Equal(t, "NewPolicy", result.Name)
		assert.True(t, result.Enabled)
	})

	t.Run("missing required fields", func(t *testing.T) {
		body := map[string]interface{}{
			"description": "No name",
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/security/policies", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestAuthPolicyHandler_Update(t *testing.T) {
	db := setupAuthHandlersTestDB(t)
	router := setupAuthTestRouter(db)

	policy := models.AuthPolicy{Name: "TestPolicy", Enabled: true}
	db.Create(&policy)

	t.Run("success", func(t *testing.T) {
		body := map[string]interface{}{
			"name":        "UpdatedPolicy",
			"require_mfa": true,
			"enabled":     false,
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/api/v1/security/policies/"+policy.UUID, bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var result models.AuthPolicy
		json.Unmarshal(w.Body.Bytes(), &result)
		assert.Equal(t, "UpdatedPolicy", result.Name)
		assert.True(t, result.RequireMFA)
		assert.False(t, result.Enabled)
	})

	t.Run("not found", func(t *testing.T) {
		body := map[string]interface{}{"name": "Test"}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/api/v1/security/policies/nonexistent", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestAuthPolicyHandler_Delete(t *testing.T) {
	db := setupAuthHandlersTestDB(t)
	router := setupAuthTestRouter(db)

	t.Run("success", func(t *testing.T) {
		policy := models.AuthPolicy{Name: "DeletePolicy"}
		db.Create(&policy)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/api/v1/security/policies/"+policy.UUID, nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("cannot delete policy in use", func(t *testing.T) {
		policy := models.AuthPolicy{Name: "InUsePolicy"}
		db.Create(&policy)

		// Create a proxy host using this policy
		host := models.ProxyHost{
			UUID:         "test-host-uuid",
			DomainNames:  "test.com",
			ForwardHost:  "localhost",
			ForwardPort:  80,
			AuthPolicyID: &policy.ID,
		}
		db.Create(&host)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/api/v1/security/policies/"+policy.UUID, nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "in use")
	})

	t.Run("not found", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/api/v1/security/policies/nonexistent", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestAuthPolicyHandler_GetByID(t *testing.T) {
	db := setupAuthHandlersTestDB(t)
	handler := NewAuthPolicyHandler(db)

	policy := models.AuthPolicy{Name: "TestPolicy"}
	db.Create(&policy)

	t.Run("found", func(t *testing.T) {
		result, err := handler.GetByID(policy.ID)
		assert.NoError(t, err)
		assert.Equal(t, "TestPolicy", result.Name)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := handler.GetByID(9999)
		assert.Error(t, err)
	})
}
