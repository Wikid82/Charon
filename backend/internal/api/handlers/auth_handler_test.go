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
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupAuthHandler(t *testing.T) (*AuthHandler, *gorm.DB) {
	dbName := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	require.NoError(t, err)
	db.AutoMigrate(&models.User{}, &models.Setting{})

	cfg := config.Config{JWTSecret: "test-secret"}
	authService := services.NewAuthService(db, cfg)
	return NewAuthHandler(authService), db
}

func TestAuthHandler_Login(t *testing.T) {
	handler, db := setupAuthHandler(t)

	// Create user
	user := &models.User{
		UUID:  uuid.NewString(),
		Email: "test@example.com",
		Name:  "Test User",
	}
	user.SetPassword("password123")
	db.Create(user)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/login", handler.Login)

	// Success
	body := map[string]string{
		"email":    "test@example.com",
		"password": "password123",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/login", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "token")
}

func TestAuthHandler_Register(t *testing.T) {
	handler, _ := setupAuthHandler(t)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/register", handler.Register)

	body := map[string]string{
		"email":    "new@example.com",
		"password": "password123",
		"name":     "New User",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/register", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "new@example.com")
}

func TestAuthHandler_Register_Duplicate(t *testing.T) {
	handler, db := setupAuthHandler(t)
	db.Create(&models.User{UUID: uuid.NewString(), Email: "dup@example.com", Name: "Dup"})

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/register", handler.Register)

	body := map[string]string{
		"email":    "dup@example.com",
		"password": "password123",
		"name":     "Dup User",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/register", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestAuthHandler_Logout(t *testing.T) {
	handler, _ := setupAuthHandler(t)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/logout", handler.Logout)

	req := httptest.NewRequest("POST", "/logout", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Logged out")
	// Check cookie
	cookie := w.Result().Cookies()[0]
	assert.Equal(t, "auth_token", cookie.Name)
	assert.Equal(t, -1, cookie.MaxAge)
}

func TestAuthHandler_Me(t *testing.T) {
	handler, db := setupAuthHandler(t)

	// Create user that matches the middleware ID
	user := &models.User{
		UUID:  uuid.NewString(),
		Email: "me@example.com",
		Name:  "Me User",
		Role:  "admin",
	}
	db.Create(user)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	// Simulate middleware
	r.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Set("role", user.Role)
		c.Next()
	})
	r.GET("/me", handler.Me)

	req := httptest.NewRequest("GET", "/me", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, float64(user.ID), resp["user_id"])
	assert.Equal(t, "admin", resp["role"])
	assert.Equal(t, "Me User", resp["name"])
	assert.Equal(t, "me@example.com", resp["email"])
}

func TestAuthHandler_ChangePassword(t *testing.T) {
	handler, db := setupAuthHandler(t)

	// Create user
	user := &models.User{
		UUID:  uuid.NewString(),
		Email: "change@example.com",
		Name:  "Change User",
	}
	user.SetPassword("oldpassword")
	db.Create(user)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	// Simulate middleware
	r.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Next()
	})
	r.POST("/change-password", handler.ChangePassword)

	body := map[string]string{
		"old_password": "oldpassword",
		"new_password": "newpassword123",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/change-password", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Password updated successfully")

	// Verify password changed
	var updatedUser models.User
	db.First(&updatedUser, user.ID)
	assert.True(t, updatedUser.CheckPassword("newpassword123"))
}

func TestAuthHandler_ChangePassword_WrongOld(t *testing.T) {
	handler, db := setupAuthHandler(t)
	user := &models.User{UUID: uuid.NewString(), Email: "wrong@example.com"}
	user.SetPassword("correct")
	db.Create(user)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Next()
	})
	r.POST("/change-password", handler.ChangePassword)

	body := map[string]string{
		"old_password": "wrong",
		"new_password": "newpassword",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/change-password", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
