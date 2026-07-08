package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

func setupBackupRemoteHandlerTest(t *testing.T, enc *crypto.EncryptionService) (*gin.Engine, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.RemoteStorageTarget{}, &models.BackupRecord{}, &models.BackupRemoteCopy{}, &models.Setting{}))

	svc := services.NewBackupRemoteService(db, enc, t.TempDir())
	h := NewBackupRemoteHandler(svc)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", uint(1))
		c.Next()
	})
	api := router.Group("/api/v1/backups/remote-targets")
	api.GET("", h.List)
	api.POST("", h.Create)
	api.PUT("/:uuid", h.Update)
	api.DELETE("/:uuid", h.Delete)
	api.POST("/:uuid/test", h.Test)

	return router, db
}

func testEncryptionService(t *testing.T) *crypto.EncryptionService {
	t.Helper()
	enc, err := crypto.NewEncryptionService(base64.StdEncoding.EncodeToString(make([]byte, 32)))
	require.NoError(t, err)
	return enc
}

func TestBackupRemoteHandler_List_Empty(t *testing.T) {
	router, _ := setupBackupRemoteHandlerTest(t, testEncryptionService(t))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/backups/remote-targets", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, "[]", resp.Body.String())
}

func TestBackupRemoteHandler_Create_Success(t *testing.T) {
	router, _ := setupBackupRemoteHandlerTest(t, testEncryptionService(t))

	payload := map[string]any{
		"name": "Home NAS",
		"type": "sftp",
		"config": map[string]any{
			"host":                 "10.0.0.5",
			"port":                 22,
			"path":                 "/backups",
			"host_key_fingerprint": "SHA256:abc123",
		},
		"secrets": map[string]any{"password": "hunter2"},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/remote-targets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusCreated, resp.Code, resp.Body.String())

	var respBody map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &respBody))
	require.Equal(t, "Home NAS", respBody["name"])
	require.Equal(t, true, respBody["secrets_set"])
	require.NotContains(t, resp.Body.String(), "hunter2")
}

func TestBackupRemoteHandler_Create_SSRFRejected(t *testing.T) {
	router, _ := setupBackupRemoteHandlerTest(t, testEncryptionService(t))

	payload := map[string]any{
		"name":   "Evil",
		"type":   "sftp",
		"config": map[string]any{"host": "169.254.169.254"},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/remote-targets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestBackupRemoteHandler_Create_EncryptionKeyMissing(t *testing.T) {
	router, _ := setupBackupRemoteHandlerTest(t, nil)

	payload := map[string]any{
		"name":   "Home NAS",
		"type":   "sftp",
		"config": map[string]any{"host": "10.0.0.5", "host_key_fingerprint": "SHA256:abc"},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/remote-targets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusServiceUnavailable, resp.Code)

	var respBody map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &respBody))
	require.Equal(t, "encryption_key_missing", respBody["error_code"])
}

func TestBackupRemoteHandler_UpdateAndDelete(t *testing.T) {
	router, db := setupBackupRemoteHandlerTest(t, testEncryptionService(t))

	target := models.RemoteStorageTarget{Name: "Old Name", Type: "sftp", ConfigJSON: `{"host":"10.0.0.5"}`}
	require.NoError(t, db.Create(&target).Error)

	updatePayload := map[string]any{
		"name":   "New Name",
		"config": map[string]any{"host": "10.0.0.6", "host_key_fingerprint": "SHA256:xyz"},
	}
	body, _ := json.Marshal(updatePayload)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/backups/remote-targets/"+target.UUID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())

	var respBody map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &respBody))
	require.Equal(t, "New Name", respBody["name"])

	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/backups/remote-targets/"+target.UUID, http.NoBody)
	delResp := httptest.NewRecorder()
	router.ServeHTTP(delResp, delReq)
	require.Equal(t, http.StatusOK, delResp.Code)

	var count int64
	db.Model(&models.RemoteStorageTarget{}).Where("uuid = ?", target.UUID).Count(&count)
	require.Equal(t, int64(0), count)
}

func TestBackupRemoteHandler_Test_EncryptionKeyMissing(t *testing.T) {
	router, db := setupBackupRemoteHandlerTest(t, nil)

	// A target that already has an encrypted secrets blob (e.g. created
	// before CHARON_ENCRYPTION_KEY was removed) cannot be tested without
	// the key.
	target := models.RemoteStorageTarget{Name: "Home NAS", Type: "sftp", ConfigJSON: `{"host":"10.0.0.5"}`, SecretsEncrypted: "ciphertext"}
	require.NoError(t, db.Create(&target).Error)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/remote-targets/"+target.UUID+"/test", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusServiceUnavailable, resp.Code)
}

func TestBackupRemoteHandler_Test_NotFound(t *testing.T) {
	router, _ := setupBackupRemoteHandlerTest(t, testEncryptionService(t))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/remote-targets/does-not-exist/test", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusNotFound, resp.Code)
}

func TestBackupRemoteHandler_Create_InvalidJSON(t *testing.T) {
	router, _ := setupBackupRemoteHandlerTest(t, testEncryptionService(t))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/remote-targets", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusBadRequest, resp.Code)
}
