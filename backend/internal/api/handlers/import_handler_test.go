package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/api/handlers"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
)

func setupImportTestDB() *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		panic("failed to connect to test database")
	}
	db.AutoMigrate(&models.ImportSession{}, &models.ProxyHost{}, &models.Location{})
	return db
}

func TestImportHandler_GetStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB()

	// Case 1: No active session
	handler := handlers.NewImportHandler(db, "echo", "/tmp")
	router := gin.New()
	router.GET("/import/status", handler.GetStatus)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/import/status", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, false, resp["has_pending"])

	// Case 2: Active session exists
	sessionUUID := uuid.NewString()
	session := &models.ImportSession{
		UUID:      sessionUUID,
		Status:    "pending",
		CreatedAt: time.Now(),
	}
	db.Create(session)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/import/status", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, true, resp["has_pending"])

	sessionMap, ok := resp["session"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, sessionUUID, sessionMap["uuid"])
}

func TestImportHandler_Cancel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB()

	// Seed active session
	sessionUUID := uuid.NewString()
	session := &models.ImportSession{
		UUID:      sessionUUID,
		Status:    "reviewing",
		CreatedAt: time.Now(),
	}
	db.Create(session)

	handler := handlers.NewImportHandler(db, "echo", "/tmp")
	router := gin.New()
	router.DELETE("/import/cancel", handler.Cancel)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/import/cancel?session_uuid="+sessionUUID, nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var updated models.ImportSession
	db.First(&updated, "uuid = ?", sessionUUID)
	assert.Equal(t, "rejected", updated.Status)
}

func TestImportHandler_Commit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB()

	// Prepare parsed data
	parsedData := `{"hosts":[{"domain_names":"example.com","forward_scheme":"http","forward_host":"localhost","forward_port":8080,"ssl_forced":true}],"conflicts":[],"errors":[]}`

	// Seed active session
	sessionUUID := uuid.NewString()
	session := &models.ImportSession{
		UUID:       sessionUUID,
		Status:     "reviewing",
		CreatedAt:  time.Now(),
		ParsedData: parsedData,
	}
	db.Create(session)

	handler := handlers.NewImportHandler(db, "echo", "/tmp")
	router := gin.New()
	router.POST("/import/commit", handler.Commit)

	// Commit request
	body := map[string]interface{}{
		"session_uuid": sessionUUID,
		"resolutions":  map[string]string{},
	}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/import/commit", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify session status
	var updatedSession models.ImportSession
	db.First(&updatedSession, "uuid = ?", sessionUUID)
	assert.Equal(t, "committed", updatedSession.Status)

	// Verify proxy host created
	var host models.ProxyHost
	db.First(&host, "domain_names = ?", "example.com")
	assert.Equal(t, "example.com", host.DomainNames)
	assert.Equal(t, "localhost", host.ForwardHost)
}

func TestImportHandler_Upload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB()

	cwd, _ := os.Getwd()
	fakeCaddy := filepath.Join(cwd, "testdata", "fake_caddy.sh")

	handler := handlers.NewImportHandler(db, fakeCaddy, "/tmp")
	router := gin.New()
	router.POST("/import/upload", handler.Upload)

	// Create JSON body
	body := map[string]string{
		"content":  "example.com {\n  reverse_proxy localhost:8080\n}",
		"filename": "Caddyfile",
	}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/import/upload", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify session created in DB
	var session models.ImportSession
	db.First(&session)
	assert.NotEmpty(t, session.UUID)
	assert.Equal(t, "pending", session.Status)
}

func TestImportHandler_GetPreview(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB()

	// Seed active session
	sessionUUID := uuid.NewString()
	session := &models.ImportSession{
		UUID:       sessionUUID,
		Status:     "pending",
		CreatedAt:  time.Now(),
		ParsedData: `{"hosts":[]}`,
	}
	db.Create(session)

	handler := handlers.NewImportHandler(db, "echo", "/tmp")
	router := gin.New()
	router.GET("/import/preview", handler.GetPreview)

	req, _ := http.NewRequest("GET", "/import/preview", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NotNil(t, resp["hosts"])
}

func TestCheckMountedImport(t *testing.T) {
	db := setupImportTestDB()
	tmpDir := t.TempDir()
	mountPath := filepath.Join(tmpDir, "Caddyfile")
	os.WriteFile(mountPath, []byte("example.com"), 0644)

	cwd, _ := os.Getwd()
	fakeCaddy := filepath.Join(cwd, "testdata", "fake_caddy.sh")

	err := handlers.CheckMountedImport(db, mountPath, fakeCaddy, tmpDir)
	assert.NoError(t, err)

	// Verify session created
	var session models.ImportSession
	db.First(&session)
	assert.NotEmpty(t, session.UUID)
}

func TestImportHandler_RegisterRoutes(t *testing.T) {
	db := setupImportTestDB()
	handler := handlers.NewImportHandler(db, "echo", "/tmp")
	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	// Verify routes exist by making requests
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/import/status", nil)
	router.ServeHTTP(w, req)
	assert.NotEqual(t, http.StatusNotFound, w.Code)
}

func TestImportHandler_Errors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB()
	handler := handlers.NewImportHandler(db, "echo", "/tmp")
	router := gin.New()
	router.POST("/import/upload", handler.Upload)
	router.POST("/import/commit", handler.Commit)
	router.DELETE("/import/cancel", handler.Cancel)

	// Upload - Invalid JSON
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/import/upload", bytes.NewBuffer([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Commit - Invalid JSON
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/import/commit", bytes.NewBuffer([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Commit - Session Not Found
	body := map[string]interface{}{
		"session_uuid": "non-existent",
		"resolutions":  map[string]string{},
	}
	jsonBody, _ := json.Marshal(body)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/import/commit", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// Cancel - Session Not Found
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/import/cancel?session_uuid=non-existent", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}
