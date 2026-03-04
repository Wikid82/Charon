package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// errorExec is a mock that returns errors for all operations
type errorExec struct{}

func (f *errorExec) Start(ctx context.Context, binPath, configDir string) (int, error) {
	return 0, errors.New("failed to start crowdsec")
}
func (f *errorExec) Stop(ctx context.Context, configDir string) error {
	return errors.New("failed to stop crowdsec")
}
func (f *errorExec) Status(ctx context.Context, configDir string) (running bool, pid int, err error) {
	return false, 0, errors.New("failed to get status")
}

func TestCrowdsec_Start_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCrowdDB(t)
	tmpDir := t.TempDir()

	h := newTestCrowdsecHandler(t, db, &errorExec{}, "/bin/false", tmpDir)

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/start", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "failed to start crowdsec")
}

func TestCrowdsec_Stop_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCrowdDB(t)
	tmpDir := t.TempDir()

	h := newTestCrowdsecHandler(t, db, &errorExec{}, "/bin/false", tmpDir)

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/stop", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "failed to stop crowdsec")
}

func TestCrowdsec_Status_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCrowdDB(t)
	tmpDir := t.TempDir()

	h := newTestCrowdsecHandler(t, db, &errorExec{}, "/bin/false", tmpDir)

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/status", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "failed to get status")
}

// ReadFile tests
func TestCrowdsec_ReadFile_MissingPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCrowdDB(t)
	tmpDir := t.TempDir()

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/file", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "path required")
}

func TestCrowdsec_ReadFile_PathTraversal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCrowdDB(t)
	tmpDir := t.TempDir()

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	// Attempt path traversal
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/file?path=../../../etc/passwd", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid path")
}

func TestCrowdsec_ReadFile_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCrowdDB(t)
	tmpDir := t.TempDir()

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/file?path=nonexistent.conf", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "file not found")
}

// WriteFile tests
func TestCrowdsec_WriteFile_InvalidPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCrowdDB(t)
	tmpDir := t.TempDir()

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/file", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid payload")
}

func TestCrowdsec_WriteFile_MissingPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCrowdDB(t)
	tmpDir := t.TempDir()

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	payload := map[string]string{"content": "test"}
	b, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/file", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "path required")
}

func TestCrowdsec_WriteFile_PathTraversal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCrowdDB(t)
	tmpDir := t.TempDir()

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	// Attempt path traversal
	payload := map[string]string{"path": "../../../etc/malicious.conf", "content": "bad"}
	b, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/file", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid path")
}

// ExportConfig tests
func TestCrowdsec_ExportConfig_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCrowdDB(t)
	// Use a non-existent directory
	nonExistentDir := "/tmp/crowdsec-nonexistent-dir-12345"
	_ = os.RemoveAll(nonExistentDir) // Make sure it doesn't exist

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", nonExistentDir)
	// remove any cache dir created during handler init so Export sees missing dir
	_ = os.RemoveAll(nonExistentDir)

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/export", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "crowdsec config not found")
}

// ListFiles tests
func TestCrowdsec_ListFiles_EmptyDir(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCrowdDB(t)
	tmpDir := t.TempDir()

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/files", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	// Files may be nil or empty array when dir is empty
	files := resp["files"]
	if files != nil {
		assert.Len(t, files.([]any), 0)
	}
}

func TestCrowdsec_ListFiles_NonExistent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCrowdDB(t)
	nonExistentDir := "/tmp/crowdsec-nonexistent-dir-67890"
	_ = os.RemoveAll(nonExistentDir)

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", nonExistentDir)

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/files", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	// Should return empty array (nil) for non-existent dir
	// The files key should exist
	_, ok := resp["files"]
	assert.True(t, ok)
}

// ImportConfig error cases
func TestCrowdsec_ImportConfig_NoFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCrowdDB(t)
	tmpDir := t.TempDir()

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/import", http.NoBody)
	req.Header.Set("Content-Type", "multipart/form-data")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "file required")
}

// Additional ReadFile test with nested path that exists
func TestCrowdsec_ReadFile_NestedPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCrowdDB(t)
	tmpDir := t.TempDir()

	// Create a nested file in the data dir
	_ = os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0o750)                                         // #nosec G301 -- test fixture
	_ = os.WriteFile(filepath.Join(tmpDir, "subdir", "test.conf"), []byte("nested content"), 0o600) // #nosec G306 -- test fixture

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/file?path=subdir/test.conf", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "nested content", resp["content"])
}

// Test WriteFile when backup fails (simulate by making dir unwritable)
func TestCrowdsec_WriteFile_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCrowdDB(t)
	tmpDir := t.TempDir()

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	payload := map[string]string{"path": "new.conf", "content": "new content"}
	b, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/file", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "written")

	// Verify file was created
	content, err := os.ReadFile(filepath.Join(tmpDir, "new.conf")) //nolint:gosec // G304: Test file in temp directory
	assert.NoError(t, err)
	assert.Equal(t, "new content", string(content))
}

func TestCrowdsec_ListPresets_Disabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCrowdDB(t)
	t.Setenv("FEATURE_CERBERUS_ENABLED", "false")
	tmpDir := t.TempDir()

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/presets", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCrowdsec_ListPresets_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCrowdDB(t)
	tmpDir := t.TempDir()

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/presets", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	presets, ok := resp["presets"].([]any)
	assert.True(t, ok)
	assert.Greater(t, len(presets), 0)
}

func TestCrowdsec_PullPreset_Validation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCrowdDB(t)
	tmpDir := t.TempDir()

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)
	h.Hub = nil // simulate hub unavailable

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/presets/pull", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/presets/pull", bytes.NewReader([]byte(`{"slug":"demo"}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestCrowdsec_ApplyPreset_Validation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCrowdDB(t)
	tmpDir := t.TempDir()

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)
	h.Hub = nil

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/presets/apply", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/presets/apply", bytes.NewReader([]byte(`{"slug":"demo"}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}
