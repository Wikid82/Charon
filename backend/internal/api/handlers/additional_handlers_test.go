package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ============== Health Handler Tests ==============
// Note: TestHealthHandler already exists in health_handler_test.go

func Test_getLocalIP_Additional(t *testing.T) {
	// This function should return empty string or valid IP
	ip := getLocalIP()
	// Just verify it doesn't panic and returns a string
	t.Logf("getLocalIP returned: %s", ip)
}

// ============== Feature Flags Handler Tests ==============
// Note: setupFeatureFlagsTestRouter and related tests exist in feature_flags_handler_coverage_test.go

func TestFeatureFlagsHandler_GetFlags_FromShortEnv(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&models.Setting{})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewFeatureFlagsHandler(db)
	router.GET("/flags", handler.GetFlags)

	// Set short environment variable (without "feature." prefix)
	os.Setenv("CERBERUS_ENABLED", "true")
	defer os.Unsetenv("CERBERUS_ENABLED")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/flags", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]bool
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.True(t, response["feature.cerberus.enabled"])
}

func TestFeatureFlagsHandler_UpdateFlags_UnknownFlag(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&models.Setting{})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewFeatureFlagsHandler(db)
	router.PUT("/flags", handler.UpdateFlags)

	payload := map[string]bool{
		"unknown.flag": true,
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/flags", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// Should succeed but unknown flag should be ignored
	assert.Equal(t, http.StatusOK, w.Code)
}

// ============== Domain Handler Tests ==============
// Note: setupDomainTestRouter exists in domain_handler_test.go

func TestDomainHandler_List_Additional(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&models.Domain{})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewDomainHandler(db, nil)
	router.GET("/domains", handler.List)

	// Create test domains
	domain1 := models.Domain{UUID: uuid.New().String(), Name: "example.com"}
	domain2 := models.Domain{UUID: uuid.New().String(), Name: "test.com"}
	require.NoError(t, db.Create(&domain1).Error)
	require.NoError(t, db.Create(&domain2).Error)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/domains", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response []models.Domain
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Len(t, response, 2)
}

func TestDomainHandler_List_Empty_Additional(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&models.Domain{})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewDomainHandler(db, nil)
	router.GET("/domains", handler.List)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/domains", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response []models.Domain
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Len(t, response, 0)
}

func TestDomainHandler_Create_Additional(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&models.Domain{})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewDomainHandler(db, nil)
	router.POST("/domains", handler.Create)

	payload := map[string]string{"name": "newdomain.com"}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/domains", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response models.Domain
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "newdomain.com", response.Name)
}

func TestDomainHandler_Create_MissingName_Additional(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&models.Domain{})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewDomainHandler(db, nil)
	router.POST("/domains", handler.Create)

	payload := map[string]string{}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/domains", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDomainHandler_Delete_Additional(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&models.Domain{})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewDomainHandler(db, nil)
	router.DELETE("/domains/:id", handler.Delete)

	testUUID := uuid.New().String()
	domain := models.Domain{UUID: testUUID, Name: "todelete.com"}
	require.NoError(t, db.Create(&domain).Error)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/domains/"+testUUID, nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify deleted
	var count int64
	db.Model(&models.Domain{}).Where("uuid = ?", testUUID).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestDomainHandler_Delete_NotFound_Additional(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&models.Domain{})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewDomainHandler(db, nil)
	router.DELETE("/domains/:id", handler.Delete)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/domains/nonexistent", nil)
	router.ServeHTTP(w, req)

	// Should still return OK (delete is idempotent)
	assert.Equal(t, http.StatusOK, w.Code)
}

// ============== Notification Handler Tests ==============

func TestNotificationHandler_List_Additional(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&models.Notification{})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()

	notifService := services.NewNotificationService(db)
	handler := NewNotificationHandler(notifService)
	router.GET("/notifications", handler.List)
	router.PUT("/notifications/:id/read", handler.MarkAsRead)
	router.PUT("/notifications/read-all", handler.MarkAllAsRead)

	// Create test notifications
	notif1 := models.Notification{Title: "Test 1", Message: "Message 1", Read: false}
	notif2 := models.Notification{Title: "Test 2", Message: "Message 2", Read: true}
	require.NoError(t, db.Create(&notif1).Error)
	require.NoError(t, db.Create(&notif2).Error)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/notifications", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response []models.Notification
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Len(t, response, 2)
}

func TestNotificationHandler_MarkAsRead_Additional(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&models.Notification{})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()

	notifService := services.NewNotificationService(db)
	handler := NewNotificationHandler(notifService)
	router.PUT("/notifications/:id/read", handler.MarkAsRead)

	notif := models.Notification{Title: "Test", Message: "Message", Read: false}
	require.NoError(t, db.Create(&notif).Error)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/notifications/"+notif.ID+"/read", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify marked as read
	var updated models.Notification
	require.NoError(t, db.Where("id = ?", notif.ID).First(&updated).Error)
	assert.True(t, updated.Read)
}

func TestNotificationHandler_MarkAllAsRead_Additional(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&models.Notification{})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()

	notifService := services.NewNotificationService(db)
	handler := NewNotificationHandler(notifService)
	router.PUT("/notifications/read-all", handler.MarkAllAsRead)

	// Create multiple unread notifications
	notif1 := models.Notification{Title: "Test 1", Message: "Message 1", Read: false}
	notif2 := models.Notification{Title: "Test 2", Message: "Message 2", Read: false}
	require.NoError(t, db.Create(&notif1).Error)
	require.NoError(t, db.Create(&notif2).Error)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/notifications/read-all", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify all marked as read
	var unread int64
	db.Model(&models.Notification{}).Where("read = ?", false).Count(&unread)
	assert.Equal(t, int64(0), unread)
}

// ============== Logs Handler Tests ==============
// Note: NewLogsHandler requires LogService - tests exist elsewhere

// ============== Docker Handler Tests ==============
// Note: NewDockerHandler requires interfaces - tests exist elsewhere

// ============== CrowdSec Exec Tests ==============

func TestCrowdsecExec_NewDefaultCrowdsecExecutor(t *testing.T) {
	exec := NewDefaultCrowdsecExecutor()
	assert.NotNil(t, exec)
}

func TestDefaultCrowdsecExecutor_isCrowdSecProcess(t *testing.T) {
	exec := NewDefaultCrowdsecExecutor()

	// Test with invalid PID
	result := exec.isCrowdSecProcess(-1)
	assert.False(t, result)

	// Test with current process (should be false since it's not crowdsec)
	result = exec.isCrowdSecProcess(os.Getpid())
	assert.False(t, result)
}

func TestDefaultCrowdsecExecutor_pidFile(t *testing.T) {
	exec := NewDefaultCrowdsecExecutor()
	path := exec.pidFile("/tmp/test")
	assert.Contains(t, path, "crowdsec.pid")
}

func TestDefaultCrowdsecExecutor_Status(t *testing.T) {
	tmpDir := t.TempDir()
	exec := NewDefaultCrowdsecExecutor()
	running, pid, err := exec.Status(context.Background(), tmpDir)
	assert.NoError(t, err)
	// CrowdSec isn't running, so it should show not running
	assert.False(t, running)
	assert.Equal(t, 0, pid)
}

// ============== Import Handler Path Safety Tests ==============

func Test_isSafePathUnderBase_Additional(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		path     string
		wantSafe bool
	}{
		{
			name:     "valid relative path under base",
			base:     "/tmp/data",
			path:     "file.txt",
			wantSafe: true,
		},
		{
			name:     "valid relative path with subdir",
			base:     "/tmp/data",
			path:     "subdir/file.txt",
			wantSafe: true,
		},
		{
			name:     "path traversal attempt",
			base:     "/tmp/data",
			path:     "../../../etc/passwd",
			wantSafe: false,
		},
		{
			name:     "empty path",
			base:     "/tmp/data",
			path:     "",
			wantSafe: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSafePathUnderBase(tt.base, tt.path)
			assert.Equal(t, tt.wantSafe, result)
		})
	}
}
