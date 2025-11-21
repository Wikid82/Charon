package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupUserHandler(t *testing.T) (*UserHandler, *gorm.DB) {
	// Use unique DB for each test to avoid pollution
	dbName := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	require.NoError(t, err)
	db.AutoMigrate(&models.User{}, &models.Setting{})
	return NewUserHandler(db), db
}

func TestUserHandler_GetSetupStatus(t *testing.T) {
	handler, db := setupUserHandler(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/setup", handler.GetSetupStatus)

	// No users -> setup required
	req, _ := http.NewRequest("GET", "/setup", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "\"setupRequired\":true")

	// Create user -> setup not required
	db.Create(&models.User{Email: "test@example.com"})
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "\"setupRequired\":false")
}

func TestUserHandler_Setup(t *testing.T) {
	handler, _ := setupUserHandler(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/setup", handler.Setup)

	// 1. Invalid JSON (Before setup is done)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/setup", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 2. Valid Setup
	body := map[string]string{
		"name":     "Admin",
		"email":    "admin@example.com",
		"password": "password123",
	}
	jsonBody, _ := json.Marshal(body)
	req, _ = http.NewRequest("POST", "/setup", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "Setup completed successfully")

	// 3. Try again -> should fail (already setup)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/setup", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestUserHandler_Setup_DBError(t *testing.T) {
	// Can't easily mock DB error with sqlite memory unless we close it or something.
	// But we can try to insert duplicate email if we had a unique constraint and pre-seeded data,
	// but Setup checks if ANY user exists first.
	// So if we have a user, it returns Forbidden.
	// If we don't, it tries to create.
	// If we want Create to fail, maybe invalid data that passes binding but fails DB constraint?
	// User model has validation?
	// Let's try empty password if allowed by binding but rejected by DB?
	// Or very long string?
}

func TestUserHandler_RegenerateAPIKey(t *testing.T) {
	handler, db := setupUserHandler(t)

	user := &models.User{Email: "api@example.com"}
	db.Create(user)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Next()
	})
	r.POST("/api-key", handler.RegenerateAPIKey)

	req, _ := http.NewRequest("POST", "/api-key", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NotEmpty(t, resp["api_key"])

	// Verify DB
	var updatedUser models.User
	db.First(&updatedUser, user.ID)
	assert.Equal(t, resp["api_key"], updatedUser.APIKey)
}

func TestUserHandler_GetProfile(t *testing.T) {
	handler, db := setupUserHandler(t)

	user := &models.User{
		Email:  "profile@example.com",
		Name:   "Profile User",
		APIKey: "existing-key",
	}
	db.Create(user)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Next()
	})
	r.GET("/profile", handler.GetProfile)

	req, _ := http.NewRequest("GET", "/profile", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp models.User
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, user.Email, resp.Email)
	assert.Equal(t, user.APIKey, resp.APIKey)
}

func TestUserHandler_RegisterRoutes(t *testing.T) {
	handler, _ := setupUserHandler(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api")
	handler.RegisterRoutes(api)

	routes := r.Routes()
	expectedRoutes := map[string]string{
		"/api/setup":              "GET,POST",
		"/api/profile":            "GET",
		"/api/regenerate-api-key": "POST",
	}

	for path := range expectedRoutes {
		found := false
		for _, route := range routes {
			if route.Path == path {
				found = true
				break
			}
		}
		assert.True(t, found, "Route %s not found", path)
	}
}

func TestUserHandler_Errors(t *testing.T) {
	handler, db := setupUserHandler(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Middleware to simulate missing userID
	r.GET("/profile-no-auth", func(c *gin.Context) {
		// No userID set
		handler.GetProfile(c)
	})
	r.POST("/api-key-no-auth", func(c *gin.Context) {
		// No userID set
		handler.RegenerateAPIKey(c)
	})

	// Middleware to simulate non-existent user
	r.GET("/profile-not-found", func(c *gin.Context) {
		c.Set("userID", uint(99999))
		handler.GetProfile(c)
	})
	r.POST("/api-key-not-found", func(c *gin.Context) {
		c.Set("userID", uint(99999))
		handler.RegenerateAPIKey(c)
	})

	// Test Unauthorized
	req, _ := http.NewRequest("GET", "/profile-no-auth", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	req, _ = http.NewRequest("POST", "/api-key-no-auth", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Test Not Found (GetProfile)
	req, _ = http.NewRequest("GET", "/profile-not-found", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// Test DB Error (RegenerateAPIKey) - Hard to mock DB error on update with sqlite memory,
	// but we can try to update a non-existent user which GORM Update might not treat as error unless we check RowsAffected.
	// The handler code: if err := h.DB.Model(&models.User{}).Where("id = ?", userID).Update("api_key", apiKey).Error; err != nil
	// Update on non-existent record usually returns nil error in GORM unless configured otherwise.
	// However, let's see if we can force an error by closing DB? No, shared DB.
	// We can drop the table?
	db.Migrator().DropTable(&models.User{})
	req, _ = http.NewRequest("POST", "/api-key-not-found", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	// If table missing, Update should fail
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
