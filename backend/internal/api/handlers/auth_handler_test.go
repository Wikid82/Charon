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

func setupAuthHandler(t *testing.T) (*AuthHandler, *gorm.DB) {
db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
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
req, _ := http.NewRequest("POST", "/login", bytes.NewBuffer(jsonBody))
req.Header.Set("Content-Type", "application/json")
w := httptest.NewRecorder()
r.ServeHTTP(w, req)

assert.Equal(t, http.StatusOK, w.Code)
assert.Contains(t, w.Body.String(), "token")

// Failure
body["password"] = "wrong"
jsonBody, _ = json.Marshal(body)
req, _ = http.NewRequest("POST", "/login", bytes.NewBuffer(jsonBody))
req.Header.Set("Content-Type", "application/json")
w = httptest.NewRecorder()
r.ServeHTTP(w, req)

assert.Equal(t, http.StatusUnauthorized, w.Code)
}
