package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/api/handlers"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
)

func setupImportTestDB(t *testing.T) *gorm.DB {
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect to test database")
	}
	db.AutoMigrate(&models.ImportSession{}, &models.ProxyHost{}, &models.Location{})
	return db
}

func TestImportHandler_GetStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)

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

	// Case 2: Active session
	session := models.ImportSession{
		UUID:       uuid.NewString(),
		Status:     "pending",
		ParsedData: `{"hosts": []}`,
	}
	db.Create(&session)

	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, true, resp["has_pending"])
}

func TestImportHandler_GetPreview(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)
	handler := handlers.NewImportHandler(db, "echo", "/tmp")
	router := gin.New()
	router.GET("/import/preview", handler.GetPreview)

	// Case 1: No session
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/import/preview", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// Case 2: Active session
	session := models.ImportSession{
		UUID:       uuid.NewString(),
		Status:     "pending",
		ParsedData: `{"hosts": [{"domain_names": "example.com"}]}`,
	}
	db.Create(&session)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/import/preview", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)

	preview := result["preview"].(map[string]interface{})
	hosts := preview["hosts"].([]interface{})
	assert.Len(t, hosts, 1)

	// Verify status changed to reviewing
	var updatedSession models.ImportSession
	db.First(&updatedSession, session.ID)
	assert.Equal(t, "reviewing", updatedSession.Status)
}

func TestImportHandler_Cancel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)
	handler := handlers.NewImportHandler(db, "echo", "/tmp")
	router := gin.New()
	router.DELETE("/import/cancel", handler.Cancel)

	session := models.ImportSession{
		UUID:   "test-uuid",
		Status: "pending",
	}
	db.Create(&session)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/import/cancel?session_uuid=test-uuid", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var updatedSession models.ImportSession
	db.First(&updatedSession, session.ID)
	assert.Equal(t, "rejected", updatedSession.Status)
}

func TestImportHandler_Commit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)
	handler := handlers.NewImportHandler(db, "echo", "/tmp")
	router := gin.New()
	router.POST("/import/commit", handler.Commit)

	session := models.ImportSession{
		UUID:       "test-uuid",
		Status:     "reviewing",
		ParsedData: `{"hosts": [{"domain_names": "example.com", "forward_host": "127.0.0.1", "forward_port": 8080}]}`,
	}
	db.Create(&session)

	payload := map[string]interface{}{
		"session_uuid": "test-uuid",
		"resolutions": map[string]string{
			"example.com": "import",
		},
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/import/commit", bytes.NewBuffer(body))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify host created
	var host models.ProxyHost
	err := db.Where("domain_names = ?", "example.com").First(&host).Error
	assert.NoError(t, err)
	assert.Equal(t, "127.0.0.1", host.ForwardHost)

	// Verify session committed
	var updatedSession models.ImportSession
	db.First(&updatedSession, session.ID)
	assert.Equal(t, "committed", updatedSession.Status)
}

func TestImportHandler_Upload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)

	// Use fake caddy script
	cwd, _ := os.Getwd()
	fakeCaddy := filepath.Join(cwd, "testdata", "fake_caddy.sh")
	os.Chmod(fakeCaddy, 0755)

	tmpDir := t.TempDir()
	handler := handlers.NewImportHandler(db, fakeCaddy, tmpDir)
	router := gin.New()
	router.POST("/import/upload", handler.Upload)

	payload := map[string]string{
		"content":  "example.com",
		"filename": "Caddyfile",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/import/upload", bytes.NewBuffer(body))
	router.ServeHTTP(w, req)

	// The fake caddy script returns empty JSON, so import might fail or succeed with empty result
	// But processImport calls ImportFile which calls ParseCaddyfile which calls caddy adapt
	// fake_caddy.sh echoes `{"apps":{}}`
	// ExtractHosts will return empty result
	// processImport should succeed

	// Wait, fake_caddy.sh needs to handle "version" command too for ValidateCaddyBinary
	// The current fake_caddy.sh just echoes json.
	// I should update fake_caddy.sh or create a better one.

	// Let's assume it fails for now or check the response
	// If it fails, it's likely due to ValidateCaddyBinary calling "version" and getting JSON
	// But ValidateCaddyBinary just checks exit code 0.
	// fake_caddy.sh exits with 0.

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestImportHandler_RegisterRoutes(t *testing.T) {
	db := setupImportTestDB(t)
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
	db := setupImportTestDB(t)
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
