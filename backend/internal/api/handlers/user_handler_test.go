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

	body := map[string]string{
		"name":     "Admin",
		"email":    "admin@example.com",
		"password": "password123",
	}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/setup", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "Setup completed successfully")

	// Try again -> should fail (already setup)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/setup", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}
