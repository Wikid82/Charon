package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// setupBackupRemoteHandlerNonAdminTest mirrors setupBackupRemoteHandlerTest
// but registers every route under a non-admin role, so each handler's
// requireAdmin guard (identical shape across all six routes) can be
// exercised directly rather than only through the admin-role fixture.
func setupBackupRemoteHandlerNonAdminTest(t *testing.T) *gin.Engine {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.RemoteStorageTarget{}, &models.BackupRecord{}, &models.BackupRemoteCopy{}, &models.Setting{}))

	svc := services.NewBackupRemoteService(db, testEncryptionService(t), t.TempDir())
	h := NewBackupRemoteHandler(svc)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "user")
		c.Set("userID", uint(1))
		c.Next()
	})
	api := router.Group("/api/v1/backups/remote-targets")
	api.GET("", h.List)
	api.POST("", h.Create)
	api.PUT("/:uuid", h.Update)
	api.DELETE("/:uuid", h.Delete)
	api.POST("/:uuid/test", h.Test)
	api.POST("/test-draft", h.TestDraft)

	return router
}

func TestBackupRemoteHandler_AllRoutes_RequireAdmin(t *testing.T) {
	router := setupBackupRemoteHandlerNonAdminTest(t)

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"list", http.MethodGet, "/api/v1/backups/remote-targets"},
		{"create", http.MethodPost, "/api/v1/backups/remote-targets"},
		{"update", http.MethodPut, "/api/v1/backups/remote-targets/some-uuid"},
		{"delete", http.MethodDelete, "/api/v1/backups/remote-targets/some-uuid"},
		{"test", http.MethodPost, "/api/v1/backups/remote-targets/some-uuid/test"},
		{"test-draft", http.MethodPost, "/api/v1/backups/remote-targets/test-draft"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewReader([]byte("{}")))
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)
			require.Equal(t, http.StatusForbidden, resp.Code, "%s must require admin", tt.name)
		})
	}
}

// TestBackupRemoteHandler_List_WithItems proves the response-building loop
// (toRemoteTargetResponse per target) runs for a non-empty list, not just
// the empty-list case already covered by TestBackupRemoteHandler_List_Empty.
func TestBackupRemoteHandler_List_WithItems(t *testing.T) {
	router, db := setupBackupRemoteHandlerTest(t, testEncryptionService(t))

	target := models.RemoteStorageTarget{Name: "NAS", Type: "sftp", ConfigJSON: `{"host":"10.0.0.5"}`, SecretsEncrypted: "ciphertext"}
	require.NoError(t, db.Create(&target).Error)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/backups/remote-targets", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	var body []map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	require.Len(t, body, 1)
	require.Equal(t, "NAS", body[0]["name"])
	require.Equal(t, true, body[0]["secrets_set"])
}

// TestBackupRemoteHandler_Create_DefaultsEnabledTrue proves a Create
// request omitting "enabled" defaults the target to enabled=true.
func TestBackupRemoteHandler_Create_DefaultsEnabledTrue(t *testing.T) {
	router, db := setupBackupRemoteHandlerTest(t, testEncryptionService(t))

	payload := map[string]any{
		"name":   "NAS",
		"type":   "sftp",
		"config": map[string]any{"host": "10.0.0.5", "host_key_fingerprint": "SHA256:abc"},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/remote-targets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusCreated, resp.Code, resp.Body.String())

	var target models.RemoteStorageTarget
	require.NoError(t, db.Where("name = ?", "NAS").First(&target).Error)
	require.True(t, target.Enabled)
}

// TestBackupRemoteHandler_Update_NotFound proves the Update error branch
// maps a not-found target to 404 via respondRemoteTargetError.
func TestBackupRemoteHandler_Update_NotFound(t *testing.T) {
	router, _ := setupBackupRemoteHandlerTest(t, testEncryptionService(t))

	payload := map[string]any{"name": "New Name"}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/backups/remote-targets/does-not-exist", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusNotFound, resp.Code)
}

// TestBackupRemoteHandler_Update_InvalidJSON mirrors
// TestBackupRemoteHandler_Create_InvalidJSON for the Update route.
func TestBackupRemoteHandler_Update_InvalidJSON(t *testing.T) {
	router, db := setupBackupRemoteHandlerTest(t, testEncryptionService(t))

	target := models.RemoteStorageTarget{Name: "NAS", Type: "sftp", ConfigJSON: `{"host":"10.0.0.5"}`}
	require.NoError(t, db.Create(&target).Error)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/backups/remote-targets/"+target.UUID, bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusBadRequest, resp.Code)
}
