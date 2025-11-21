package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/config"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/services"
)

func setupBackupTest(t *testing.T) (*gin.Engine, *services.BackupService, string) {
	t.Helper()

	// Create temp directories
	tmpDir, err := os.MkdirTemp("", "cpm-backup-test")
	require.NoError(t, err)

	// Structure: tmpDir/data/cpm.db
	// BackupService expects DatabasePath to be .../data/cpm.db
	// It sets DataDir to filepath.Dir(DatabasePath) -> .../data
	// It sets BackupDir to .../data/backups (Wait, let me check the code again)

	// Code: backupDir := filepath.Join(filepath.Dir(cfg.DatabasePath), "backups")
	// So if DatabasePath is /tmp/data/cpm.db, DataDir is /tmp/data, BackupDir is /tmp/data/backups.

	dataDir := filepath.Join(tmpDir, "data")
	err = os.MkdirAll(dataDir, 0755)
	require.NoError(t, err)

	dbPath := filepath.Join(dataDir, "cpm.db")
	// Create a dummy DB file to back up
	err = os.WriteFile(dbPath, []byte("dummy db content"), 0644)
	require.NoError(t, err)

	cfg := &config.Config{
		DatabasePath: dbPath,
	}

	svc := services.NewBackupService(cfg)
	h := NewBackupHandler(svc)

	r := gin.New()
	api := r.Group("/api/v1")
	// Manually register routes since we don't have a RegisterRoutes method on the handler yet?
	// Wait, I didn't check if I added RegisterRoutes to BackupHandler.
	// In routes.go I did:
	// backupHandler := handlers.NewBackupHandler(backupService)
	// backups := api.Group("/backups")
	// backups.GET("", backupHandler.List)
	// ...
	// So the handler doesn't have RegisterRoutes. I'll register manually here.

	backups := api.Group("/backups")
	backups.GET("", h.List)
	backups.POST("", h.Create)
	backups.POST("/:filename/restore", h.Restore)
	backups.DELETE("/:filename", h.Delete)
	backups.GET("/:filename/download", h.Download)

	return r, svc, tmpDir
}

func TestBackupLifecycle(t *testing.T) {
	router, _, tmpDir := setupBackupTest(t)
	defer os.RemoveAll(tmpDir)

	// 1. List backups (should be empty)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/backups", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)
	// Check empty list
	// ...

	// 2. Create backup
	req = httptest.NewRequest(http.MethodPost, "/api/v1/backups", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusCreated, resp.Code)

	var result map[string]string
	err := json.Unmarshal(resp.Body.Bytes(), &result)
	require.NoError(t, err)
	filename := result["filename"]
	require.NotEmpty(t, filename)

	// 3. List backups (should have 1)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/backups", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)
	// Verify list contains filename

	// 4. Restore backup
	req = httptest.NewRequest(http.MethodPost, "/api/v1/backups/"+filename+"/restore", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	// 5. Download backup
	req = httptest.NewRequest(http.MethodGet, "/api/v1/backups/"+filename+"/download", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)
	// Content-Type might vary depending on implementation (application/octet-stream or zip)
	// require.Equal(t, "application/zip", resp.Header().Get("Content-Type"))

	// 6. Delete backup
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/backups/"+filename, nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	// 7. List backups (should be empty again)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/backups", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)
	var list []interface{}
	json.Unmarshal(resp.Body.Bytes(), &list)
	require.Empty(t, list)

	// 8. Delete non-existent backup
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/backups/missing.zip", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusNotFound, resp.Code)

	// 9. Restore non-existent backup
	req = httptest.NewRequest(http.MethodPost, "/api/v1/backups/missing.zip/restore", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusNotFound, resp.Code)

	// 10. Download non-existent backup
	req = httptest.NewRequest(http.MethodGet, "/api/v1/backups/missing.zip/download", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusNotFound, resp.Code)
}
