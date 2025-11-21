package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/config"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestUserLoginAfterEmailChange(t *testing.T) {
	// Setup DB
	dbName := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	require.NoError(t, err)
	db.AutoMigrate(&models.User{}, &models.Setting{})

	// Setup Services and Handlers
	cfg := config.Config{}
	authService := services.NewAuthService(db, cfg)
	authHandler := NewAuthHandler(authService)
	userHandler := NewUserHandler(db)

	// Setup Router
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Register Routes
	r.POST("/auth/login", authHandler.Login)

	// Mock Auth Middleware for UpdateProfile
	r.POST("/user/profile", func(c *gin.Context) {
		// Simulate authenticated user
		var user models.User
		db.First(&user)
		c.Set("userID", user.ID)
		c.Set("role", user.Role)
		c.Next()
	}, userHandler.UpdateProfile)

	// 1. Create Initial User
	initialEmail := "initial@example.com"
	password := "password123"
	user, err := authService.Register(initialEmail, password, "Test User")
	require.NoError(t, err)
	require.NotNil(t, user)

	// 2. Login with Initial Credentials (Verify it works)
	loginBody := map[string]string{
		"email":    initialEmail,
		"password": password,
	}
	jsonBody, _ := json.Marshal(loginBody)
	req, _ := http.NewRequest("POST", "/auth/login", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, "Initial login should succeed")

	// 3. Update Profile (Change Email)
	newEmail := "updated@example.com"
	updateBody := map[string]string{
		"name":  "Test User Updated",
		"email": newEmail,
	}
	jsonUpdate, _ := json.Marshal(updateBody)
	req, _ = http.NewRequest("POST", "/user/profile", bytes.NewBuffer(jsonUpdate))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, "Update profile should succeed")

	// Verify DB update
	var updatedUser models.User
	db.First(&updatedUser, user.ID)
	assert.Equal(t, newEmail, updatedUser.Email, "Email should be updated in DB")

	// 4. Login with New Email
	loginBodyNew := map[string]string{
		"email":    newEmail,
		"password": password,
	}
	jsonBodyNew, _ := json.Marshal(loginBodyNew)
	req, _ = http.NewRequest("POST", "/auth/login", bytes.NewBuffer(jsonBodyNew))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// This is where the user says it fails
	assert.Equal(t, http.StatusOK, w.Code, "Login with new email should succeed")
	if w.Code != http.StatusOK {
		t.Logf("Response Body: %s", w.Body.String())
	}

	// 5. Login with New Email (Different Case)
	loginBodyCase := map[string]string{
		"email":    "Updated@Example.com", // Different case
		"password": password,
	}
	jsonBodyCase, _ := json.Marshal(loginBodyCase)
	req, _ = http.NewRequest("POST", "/auth/login", bytes.NewBuffer(jsonBodyCase))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// If this fails, it confirms case sensitivity issue
	assert.Equal(t, http.StatusOK, w.Code, "Login with mixed case email should succeed")
}
