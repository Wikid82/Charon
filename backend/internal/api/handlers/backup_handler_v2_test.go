package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// setupBackupSettingsTest builds a BackupHandler backed by a real (in-memory)
// database, since UpdateSettings/GetSettings require one to persist
// backup.* Setting rows.
func setupBackupSettingsTest(t *testing.T) *gin.Engine {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "data", "charon.db")
	require.NoError(t, os.MkdirAll(filepath.Dir(dbPath), 0o750))

	rawDB, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	require.NoError(t, rawDB.Close())

	gdb, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, gdb.AutoMigrate(&models.Setting{}))

	cfg := &config.Config{DatabasePath: dbPath}
	svc := services.NewBackupService(cfg, gdb, nil)
	t.Cleanup(svc.Stop)

	h := NewBackupHandler(svc)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", uint(1))
		c.Next()
	})
	api := router.Group("/api/v1")
	api.GET("/backups/settings", h.GetSettings)
	api.PUT("/backups/settings", h.UpdateSettings)

	return router
}

// setupBackupTestV2 mirrors setupBackupTest but additionally registers the
// settings/upload/validate routes added in Issue #32 Commit 3.
func setupBackupTestV2(t *testing.T) (*gin.Engine, *services.BackupService, string) {
	t.Helper()
	router, svc, tmpDir := setupBackupTest(t)
	h := NewBackupHandler(svc)

	api := router.Group("/api/v1")
	api.GET("/backups/settings", h.GetSettings)
	api.PUT("/backups/settings", h.UpdateSettings)
	api.POST("/backups/upload", h.Upload)
	api.POST("/backups/:filename/validate", h.Validate)

	return router, svc, tmpDir
}

func TestBackupHandler_GetSettings_Defaults(t *testing.T) {
	router, _, tmpDir := setupBackupTestV2(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/backups/settings", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	require.Contains(t, body, "schedule_cron")
	require.Contains(t, body, "retention_count")
}

func TestBackupHandler_UpdateSettings_Success(t *testing.T) {
	router := setupBackupSettingsTest(t)

	payload := map[string]any{"retention_count": 3, "schedule_cron": "*/30 * * * *"}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/backups/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	var respBody map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &respBody))
	require.Equal(t, float64(3), respBody["retention_count"])
	require.Equal(t, "*/30 * * * *", respBody["schedule_cron"])
}

func TestBackupHandler_UpdateSettings_InvalidCron(t *testing.T) {
	router := setupBackupSettingsTest(t)

	payload := map[string]any{"schedule_cron": "not a cron"}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/backups/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusBadRequest, resp.Code)

	var respBody map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &respBody))
	require.Equal(t, "backup_invalid_cron", respBody["error_code"])
}

func TestBackupHandler_UpdateSettings_InvalidRetention(t *testing.T) {
	router := setupBackupSettingsTest(t)

	payload := map[string]any{"retention_count": 0}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/backups/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusBadRequest, resp.Code)

	var respBody map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &respBody))
	require.Equal(t, "backup_invalid_retention_count", respBody["error_code"])
}

func TestBackupHandler_UpdateSettings_EncryptionKeyMissing(t *testing.T) {
	router := setupBackupSettingsTest(t)

	payload := map[string]any{"encryption_enabled": true}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/backups/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusServiceUnavailable, resp.Code)

	var respBody map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &respBody))
	require.Equal(t, "encryption_key_missing", respBody["error_code"])
}

func TestBackupHandler_UpdateSettings_InvalidJSON(t *testing.T) {
	router, _, tmpDir := setupBackupTestV2(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	req := httptest.NewRequest(http.MethodPut, "/api/v1/backups/settings", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestBackupHandler_Validate_Success(t *testing.T) {
	router, _, tmpDir := setupBackupTestV2(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusCreated, resp.Code)
	var created map[string]string
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &created))

	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/backups/"+created["filename"]+"/validate", http.NoBody)
	resp2 := httptest.NewRecorder()
	router.ServeHTTP(resp2, req2)
	require.Equal(t, http.StatusOK, resp2.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(resp2.Body.Bytes(), &body))
	require.Equal(t, true, body["valid"])
	require.Equal(t, "ok", body["database_integrity"])
}

func TestBackupHandler_Validate_NotFound(t *testing.T) {
	router, _, tmpDir := setupBackupTestV2(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/missing.zip/validate", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusNotFound, resp.Code)
}

func buildMultipartUpload(t *testing.T, fieldName, filename string, content []byte, extraFields map[string]string) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(fieldName, filename)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	for k, v := range extraFields {
		require.NoError(t, writer.WriteField(k, v))
	}
	require.NoError(t, writer.Close())
	return body, writer.FormDataContentType()
}

func TestBackupHandler_Upload_RawSQLite_Success(t *testing.T) {
	router, svc, tmpDir := setupBackupTestV2(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Build a tiny real sqlite file (with users/proxy_hosts tables so the
	// wrapped archive passes ValidateBackup's V6 sanity check) to upload.
	dbPath := filepath.Join(t.TempDir(), "upload.db")
	createValidSQLiteDBWithCharonTables(t, dbPath)
	content, err := os.ReadFile(dbPath) // #nosec G304 -- test fixture path
	require.NoError(t, err)

	body, contentType := buildMultipartUpload(t, "file", "upload.db", content, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/upload", body)
	req.Header.Set("Content-Type", contentType)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusCreated, resp.Code, resp.Body.String())

	var respBody map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &respBody))
	filename, _ := respBody["filename"].(string)
	require.NotEmpty(t, filename)
	require.FileExists(t, filepath.Join(svc.BackupDir, filename))
}

func TestBackupHandler_Upload_UnrecognizedFileType(t *testing.T) {
	router, _, tmpDir := setupBackupTestV2(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	body, contentType := buildMultipartUpload(t, "file", "not-a-backup.txt", []byte("plain text content"), nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/upload", body)
	req.Header.Set("Content-Type", contentType)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestBackupHandler_Upload_MissingFile(t *testing.T) {
	router, _, tmpDir := setupBackupTestV2(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	writer := multipart.NewWriter(&bytes.Buffer{})
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/upload", bytes.NewReader([]byte{}))
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusBadRequest, resp.Code)
}

func createValidSQLiteDBWithCharonTables(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite3", path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT)")
	require.NoError(t, err)
	_, err = db.Exec("INSERT INTO users (email) VALUES ('admin@example.com')")
	require.NoError(t, err)
	_, err = db.Exec("CREATE TABLE proxy_hosts (id INTEGER PRIMARY KEY, domain_names TEXT)")
	require.NoError(t, err)
}

func TestDetectBackupUploadKind(t *testing.T) {
	t.Run("zip", func(t *testing.T) {
		kind, err := detectBackupUploadKind([]byte("PK\x03\x04rest"))
		require.NoError(t, err)
		require.Equal(t, uploadKindZip, kind)
	})
	t.Run("age", func(t *testing.T) {
		kind, err := detectBackupUploadKind([]byte("age-encryption.org/v1\n"))
		require.NoError(t, err)
		require.Equal(t, uploadKindAge, kind)
	})
	t.Run("sqlite", func(t *testing.T) {
		kind, err := detectBackupUploadKind([]byte("SQLite format 3\x00"))
		require.NoError(t, err)
		require.Equal(t, uploadKindSQLite, kind)
	})
	t.Run("unrecognized", func(t *testing.T) {
		_, err := detectBackupUploadKind([]byte("nope"))
		require.Error(t, err)
	})
}
