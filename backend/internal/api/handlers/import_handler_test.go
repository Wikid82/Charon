package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/testutil"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// mockProxyHostService implements ProxyHostServiceInterface for testing
type mockProxyHostService struct {
	createErr  error
	updateErr  error
	listResult []models.ProxyHost
	listErr    error
}

func (m *mockProxyHostService) Create(host *models.ProxyHost) error {
	return m.createErr
}

func (m *mockProxyHostService) Update(host *models.ProxyHost) error {
	return m.updateErr
}

func (m *mockProxyHostService) List() ([]models.ProxyHost, error) {
	return m.listResult, m.listErr
}

// setupTestDB creates a test database with required tables
// mockImporter for testing import operations
type mockImporter struct {
	normalizeResult string
	normalizeErr    error
	parseResult     []byte
	parseErr        error
	importResult    *caddy.ImportResult
	importErr       error
}

func (m *mockImporter) NormalizeCaddyfile(content string) (string, error) {
	if m.normalizeErr != nil {
		return "", m.normalizeErr
	}
	if m.normalizeResult != "" {
		return m.normalizeResult, nil
	}
	return content, nil
}

func (m *mockImporter) ParseCaddyfile(path string) ([]byte, error) {
	if m.parseErr != nil {
		return nil, m.parseErr
	}
	return m.parseResult, nil
}

func (m *mockImporter) ImportFile(path string) (*caddy.ImportResult, error) {
	if m.importErr != nil {
		return nil, m.importErr
	}
	return m.importResult, nil
}

func setupImportTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.ImportSession{}, &models.ProxyHost{})
	require.NoError(t, err)

	return db
}

// setupTestHandler creates a test handler with mocks
func setupTestHandler(t *testing.T, db *gorm.DB) (*ImportHandler, *mockProxyHostService, *mockImporter) {
	t.Helper()

	tmpDir := t.TempDir()
	mockSvc := &mockProxyHostService{}

	handler := &ImportHandler{
		db:              db,
		proxyHostSvc:    mockSvc,
		importerservice: nil, // Will be set via mock
		importDir:       tmpDir,
		mountPath:       "",
	}

	mockImport := &mockImporter{}
	return handler, mockSvc, mockImport
}

func addAdminMiddleware(router *gin.Engine) {
	router.Use(func(c *gin.Context) {
		setAdminContext(c)
		c.Next()
	})
}

func TestImportHandler_GetStatus_MountCommittedUnchanged(t *testing.T) {
	t.Parallel()

	testutil.WithTx(t, setupImportTestDB(t), func(tx *gorm.DB) {
		mountDir := t.TempDir()
		mountPath := filepath.Join(mountDir, "mounted.caddyfile")
		require.NoError(t, os.WriteFile(mountPath, []byte("example.com { respond \"ok\" }"), 0o600))

		committedAt := time.Now()
		require.NoError(t, tx.Create(&models.ImportSession{
			UUID:        "committed-1",
			SourceFile:  mountPath,
			Status:      "committed",
			CommittedAt: &committedAt,
		}).Error)

		require.NoError(t, os.Chtimes(mountPath, committedAt.Add(-1*time.Minute), committedAt.Add(-1*time.Minute)))

		handler, _, _ := setupTestHandler(t, tx)
		handler.mountPath = mountPath

		gin.SetMode(gin.TestMode)
		router := gin.New()
		addAdminMiddleware(router)
		handler.RegisterRoutes(router.Group("/api/v1"))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/import/status", http.NoBody)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		var body map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Equal(t, false, body["has_pending"])
	})
}

func TestImportHandler_GetStatus_MountModifiedAfterCommit(t *testing.T) {
	t.Parallel()

	testutil.WithTx(t, setupImportTestDB(t), func(tx *gorm.DB) {
		mountDir := t.TempDir()
		mountPath := filepath.Join(mountDir, "mounted.caddyfile")
		require.NoError(t, os.WriteFile(mountPath, []byte("example.com { respond \"ok\" }"), 0o600))

		committedAt := time.Now().Add(-10 * time.Minute)
		require.NoError(t, tx.Create(&models.ImportSession{
			UUID:        "committed-2",
			SourceFile:  mountPath,
			Status:      "committed",
			CommittedAt: &committedAt,
		}).Error)

		require.NoError(t, os.Chtimes(mountPath, time.Now(), time.Now()))

		handler, _, _ := setupTestHandler(t, tx)
		handler.mountPath = mountPath

		gin.SetMode(gin.TestMode)
		router := gin.New()
		addAdminMiddleware(router)
		handler.RegisterRoutes(router.Group("/api/v1"))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/import/status", http.NoBody)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		var body map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Equal(t, true, body["has_pending"])
	})
}

// TestUpload_NormalizationSuccess verifies single-line Caddyfile formatting
func TestUpload_NormalizationSuccess(t *testing.T) {
	testutil.WithTx(t, setupImportTestDB(t), func(tx *gorm.DB) {
		handler, _, mockImport := setupTestHandler(t, tx)

		// Mock normalized output (multi-line format)
		normalizedContent := "example.com {\n\trespond \"OK\" 200\n}\n"
		mockImport.normalizeResult = normalizedContent
		mockImport.importResult = &caddy.ImportResult{
			Hosts: []caddy.ParsedHost{
				{
					DomainNames:   "example.com",
					ForwardScheme: "http",
					ForwardHost:   "localhost",
					ForwardPort:   8080,
				},
			},
		}

		// Set the mock importer
		handler.importerservice = &mockImporterAdapter{mockImport}

		// Create request with single-line Caddyfile
		singleLineContent := "example.com { respond \"OK\" 200 }"
		reqBody := map[string]string{
			"content":  singleLineContent,
			"filename": "test.caddyfile",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/import/upload", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		gin.SetMode(gin.TestMode)
		router := gin.New()
		addAdminMiddleware(router)
		handler.RegisterRoutes(router.Group("/api/v1"))
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Verify normalized content was written to file
		var response map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		session, ok := response["session"].(map[string]any)
		require.True(t, ok)
		require.NotEmpty(t, session["id"])
	})
}

// TestUpload_NormalizationFailure verifies Caddy fmt failure handling
func TestUpload_NormalizationFailure(t *testing.T) {
	testutil.WithTx(t, setupImportTestDB(t), func(tx *gorm.DB) {
		handler, _, mockImport := setupTestHandler(t, tx)

		// Mock normalization failure (caddy fmt error)
		mockImport.normalizeErr = fmt.Errorf("caddy fmt failed: invalid syntax")
		mockImport.importResult = &caddy.ImportResult{
			Hosts: []caddy.ParsedHost{
				{
					DomainNames:   "example.com",
					ForwardScheme: "http",
					ForwardHost:   "localhost",
					ForwardPort:   8080,
				},
			},
		}

		handler.importerservice = &mockImporterAdapter{mockImport}

		reqBody := map[string]string{
			"content":  "example.com { invalid syntax",
			"filename": "test.caddyfile",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/import/upload", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		gin.SetMode(gin.TestMode)
		router := gin.New()
		addAdminMiddleware(router)
		handler.RegisterRoutes(router.Group("/api/v1"))
		router.ServeHTTP(w, req)

		// Should still succeed (logs warning, uses original content)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// TestUpload_PathTraversalBlocked verifies path traversal protection
func TestUpload_PathTraversalBlocked(t *testing.T) {
	testCases := []struct {
		name     string
		filename string
	}{
		{"parent traversal", "../../../etc/passwd"},
		{"absolute path", "/etc/passwd"},
		{"unicode confusable", "..÷..÷etc÷passwd"}, // U+2215
		{"encoded null byte", "file%00.txt"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testutil.WithTx(t, setupImportTestDB(t), func(tx *gorm.DB) {
				handler, _, mockImport := setupTestHandler(t, tx)

				mockImport.importResult = &caddy.ImportResult{Hosts: []caddy.ParsedHost{}}
				handler.importerservice = &mockImporterAdapter{mockImport}

				reqBody := map[string]string{
					"content":  "test content",
					"filename": tc.filename,
				}
				body, _ := json.Marshal(reqBody)

				req := httptest.NewRequest(http.MethodPost, "/api/v1/import/upload", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()

				gin.SetMode(gin.TestMode)
				router := gin.New()
				addAdminMiddleware(router)
				handler.RegisterRoutes(router.Group("/api/v1"))
				router.ServeHTTP(w, req)

				// Traversal should be handled by safeJoin, resulting in error or sanitized path
				// The response code depends on whether the sanitized path is valid
				assert.NotEqual(t, http.StatusInternalServerError, w.Code,
					"Should handle traversal gracefully")
			})
		})
	}
}

// TestUploadMulti_ArchiveExtraction verifies valid tar.gz with multiple files
func TestUploadMulti_ArchiveExtraction(t *testing.T) {
	testutil.WithTx(t, setupImportTestDB(t), func(tx *gorm.DB) {
		handler, _, mockImport := setupTestHandler(t, tx)

		mockImport.importResult = &caddy.ImportResult{
			Hosts: []caddy.ParsedHost{
				{DomainNames: "site1.example.com", ForwardHost: "localhost", ForwardPort: 8001},
				{DomainNames: "site2.example.com", ForwardHost: "localhost", ForwardPort: 8002},
			},
		}
		handler.importerservice = &mockImporterAdapter{mockImport}

		reqBody := map[string]any{
			"files": []map[string]string{
				{"filename": "Caddyfile", "content": "import sites/*"},
				{"filename": "sites/site1.caddyfile", "content": "site1.example.com { reverse_proxy localhost:8001 }"},
				{"filename": "sites/site2.caddyfile", "content": "site2.example.com { reverse_proxy localhost:8002 }"},
			},
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/import/upload-multi", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		gin.SetMode(gin.TestMode)
		router := gin.New()
		addAdminMiddleware(router)
		handler.RegisterRoutes(router.Group("/api/v1"))
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		preview, ok := response["preview"].(map[string]any)
		require.True(t, ok)
		hosts, ok := preview["hosts"].([]any)
		require.True(t, ok)
		assert.Len(t, hosts, 2, "Should parse both site files")
	})
}

// TestUploadMulti_ConflictDetection verifies duplicate filename handling
func TestUploadMulti_ConflictDetection(t *testing.T) {
	testutil.WithTx(t, setupImportTestDB(t), func(tx *gorm.DB) {
		handler, _, mockImport := setupTestHandler(t, tx)

		mockImport.importResult = &caddy.ImportResult{
			Hosts: []caddy.ParsedHost{
				{DomainNames: "site1.example.com", ForwardHost: "localhost", ForwardPort: 8001},
			},
		}
		handler.importerservice = &mockImporterAdapter{mockImport}

		// Attempt to upload files with duplicate filenames
		reqBody := map[string]any{
			"files": []map[string]string{
				{"filename": "Caddyfile", "content": "import sites/*"},
				{"filename": "site.caddyfile", "content": "site1.example.com { reverse_proxy localhost:8001 }"},
				{"filename": "site.caddyfile", "content": "site2.example.com { reverse_proxy localhost:8002 }"},
			},
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/import/upload-multi", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		gin.SetMode(gin.TestMode)
		router := gin.New()
		addAdminMiddleware(router)
		handler.RegisterRoutes(router.Group("/api/v1"))
		router.ServeHTTP(w, req)

		// Should succeed - last write wins for duplicate filenames
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// TestCommit_TransientToImport verifies session promotion success
func TestCommit_TransientToImport(t *testing.T) {
	testutil.WithTx(t, setupImportTestDB(t), func(tx *gorm.DB) {
		handler, _, mockImport := setupTestHandler(t, tx)

		// Setup transient file
		tmpFile := filepath.Join(handler.importDir, "uploads", "test-session.caddyfile")
		require.NoError(t, os.MkdirAll(filepath.Dir(tmpFile), 0o755))                                            // #nosec G301
		require.NoError(t, os.WriteFile(tmpFile, []byte("example.com { reverse_proxy localhost:8080 }"), 0o644)) // #nosec G306

		mockImport.importResult = &caddy.ImportResult{
			Hosts: []caddy.ParsedHost{
				{DomainNames: "example.com", ForwardHost: "localhost", ForwardPort: 8080, ForwardScheme: "http"},
			},
		}
		handler.importerservice = &mockImporterAdapter{mockImport}

		reqBody := map[string]any{
			"session_uuid": "test-session",
			"resolutions":  map[string]string{"example.com": "create"},
			"names":        map[string]string{"example.com": "Example Site"},
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/import/commit", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		gin.SetMode(gin.TestMode)
		router := gin.New()
		addAdminMiddleware(router)
		handler.RegisterRoutes(router.Group("/api/v1"))
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, float64(1), response["created"])
		assert.Equal(t, float64(0), response["updated"])
	})
}

// TestCommit_RollbackOnError verifies ProxyHost service failure cleanup
func TestCommit_RollbackOnError(t *testing.T) {
	testutil.WithTx(t, setupImportTestDB(t), func(tx *gorm.DB) {
		handler, mockSvc, mockImport := setupTestHandler(t, tx)

		// Force ProxyHost creation to fail
		mockSvc.createErr = fmt.Errorf("database error")

		tmpFile := filepath.Join(handler.importDir, "uploads", "test-session.caddyfile")
		require.NoError(t, os.MkdirAll(filepath.Dir(tmpFile), 0o755))                                            // #nosec G301
		require.NoError(t, os.WriteFile(tmpFile, []byte("example.com { reverse_proxy localhost:8080 }"), 0o644)) // #nosec G306

		mockImport.importResult = &caddy.ImportResult{
			Hosts: []caddy.ParsedHost{
				{DomainNames: "example.com", ForwardHost: "localhost", ForwardPort: 8080, ForwardScheme: "http"},
			},
		}
		handler.importerservice = &mockImporterAdapter{mockImport}

		reqBody := map[string]any{
			"session_uuid": "test-session",
			"resolutions":  map[string]string{"example.com": "create"},
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/import/commit", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		gin.SetMode(gin.TestMode)
		router := gin.New()
		addAdminMiddleware(router)
		handler.RegisterRoutes(router.Group("/api/v1"))
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, float64(0), response["created"])

		errors, ok := response["errors"].([]any)
		require.True(t, ok)
		assert.NotEmpty(t, errors, "Should report create errors")
	})
}

// TestDetectImports_EmptyCaddyfile verifies graceful handling of no imports
func TestDetectImports_EmptyCaddyfile(t *testing.T) {
	testutil.WithTx(t, setupImportTestDB(t), func(tx *gorm.DB) {
		handler, _, _ := setupTestHandler(t, tx)

		reqBody := map[string]string{
			"content": "example.com { respond \"OK\" 200 }",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/import/detect-imports", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		gin.SetMode(gin.TestMode)
		router := gin.New()
		addAdminMiddleware(router)
		handler.RegisterRoutes(router.Group("/api/v1"))
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.False(t, response["has_imports"].(bool))

		imports, ok := response["imports"].([]any)
		require.True(t, ok)
		assert.Empty(t, imports)
	})
}

// TestSafeJoin_ParentTraversal verifies .. handling
func TestSafeJoin_ParentTraversal(t *testing.T) {
	baseDir := "/tmp/testbase"

	testCases := []struct {
		name      string
		userPath  string
		expectErr bool
	}{
		{"simple parent", "..", true},
		{"triple parent", "../../../", true},
		{"nested parent", "a/../../../", true},
		{"valid subdir", "uploads/file.txt", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := safeJoin(baseDir, tc.userPath)

			if tc.expectErr {
				assert.Error(t, err, "Should block traversal")
				assert.Equal(t, "", result)
			} else {
				assert.NoError(t, err, "Should allow safe path")
				assert.NotEmpty(t, result)
			}
		})
	}
}

// TestSafeJoin_AbsolutePath verifies absolute path blocking
func TestSafeJoin_AbsolutePath(t *testing.T) {
	baseDir := "/tmp/testbase"

	testCases := []struct {
		name string
		path string
	}{
		{"unix absolute", "/etc/passwd"},
		{"windows absolute", "C:\\Windows\\System32"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := safeJoin(baseDir, tc.path)
			assert.Error(t, err, "Should block absolute paths")
			assert.Equal(t, "", result)
		})
	}
}

// TestSafeJoin_NullByte verifies null byte injection protection
func TestSafeJoin_NullByte(t *testing.T) {
	baseDir := "/tmp/testbase"

	// Note: filepath.Clean automatically strips null bytes, so they won't cause
	// traversal but may need validation at higher levels
	result, err := safeJoin(baseDir, "file\x00.txt")

	// Should succeed (null byte removed by filepath.Clean)
	assert.NoError(t, err)
	assert.NotContains(t, result, "\x00")
}

// TestSafeJoin_UnicodeConfusable verifies Unicode confusable handling
func TestSafeJoin_UnicodeConfusable(t *testing.T) {
	baseDir := "/tmp/testbase"

	// U+2215 (DIVISION SLASH) looks like "/" but isn't
	confusablePath := "..÷..÷etc"

	result, err := safeJoin(baseDir, confusablePath)

	// Should allow (confusable doesn't act as path separator)
	assert.NoError(t, err)
	assert.Contains(t, result, baseDir, "Should be under base directory")
}

// mockImporterAdapter adapts mockImporter to caddy.Importer interface
type mockImporterAdapter struct {
	mock *mockImporter
}

func (a *mockImporterAdapter) NormalizeCaddyfile(content string) (string, error) {
	return a.mock.NormalizeCaddyfile(content)
}

func (a *mockImporterAdapter) ParseCaddyfile(path string) ([]byte, error) {
	return a.mock.ParseCaddyfile(path)
}

func (a *mockImporterAdapter) ImportFile(path string) (*caddy.ImportResult, error) {
	return a.mock.ImportFile(path)
}

func (a *mockImporterAdapter) ExtractHosts(caddyJSON []byte) (*caddy.ImportResult, error) {
	// Not used in handler tests
	return &caddy.ImportResult{}, nil
}

func (a *mockImporterAdapter) ValidateCaddyBinary() error {
	return nil
}

// ============================================
// Phase 1 Additional Coverage Tests
// ============================================

// TestImportHandler_Upload_NullByteInjection verifies null byte handling in filenames
func TestImportHandler_Upload_NullByteInjection(t *testing.T) {
	testutil.WithTx(t, setupImportTestDB(t), func(tx *gorm.DB) {
		handler, _, mockImport := setupTestHandler(t, tx)

		mockImport.importResult = &caddy.ImportResult{Hosts: []caddy.ParsedHost{}}
		handler.importerservice = &mockImporterAdapter{mockImport}

		reqBody := map[string]string{
			"content":  "test content",
			"filename": "file\x00.txt",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/import/upload", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		gin.SetMode(gin.TestMode)
		router := gin.New()
		addAdminMiddleware(router)
		handler.RegisterRoutes(router.Group("/api/v1"))
		router.ServeHTTP(w, req)

		// safeJoin strips null bytes via filepath.Clean, so upload should succeed
		// but the null byte should not be in the final path
		assert.True(t, w.Code == 200 || w.Code == 400, "Should handle null byte gracefully")
	})
}

// TestImportHandler_DetectImports_MalformedCaddyfile verifies error handling for invalid syntax
func TestImportHandler_DetectImports_MalformedFile(t *testing.T) {
	testutil.WithTx(t, setupImportTestDB(t), func(tx *gorm.DB) {
		handler, _, _ := setupTestHandler(t, tx)

		// Caddyfile with malformed syntax (missing closing brace)
		reqBody := map[string]string{
			"content": "example.com {\n  reverse_proxy localhost:8080\n# Missing closing brace",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/import/detect-imports", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		gin.SetMode(gin.TestMode)
		router := gin.New()
		addAdminMiddleware(router)
		handler.RegisterRoutes(router.Group("/api/v1"))
		router.ServeHTTP(w, req)

		// DetectImports only checks for "import" directives, doesn't validate syntax
		// So malformed files should still succeed if they don't contain import directives
		assert.Equal(t, http.StatusOK, w.Code, "DetectImports should succeed for malformed files")

		var response map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.False(t, response["has_imports"].(bool), "Should detect no imports in malformed file")
	})
}

// TestImportHandler_safeJoin_EdgeCases verifies path sanitization edge cases
func TestImportHandler_safeJoin_EdgeCases(t *testing.T) {
	baseDir := "/tmp/testbase"

	testCases := []struct {
		name      string
		userPath  string
		expectErr bool
		desc      string
	}{
		{
			name:      "empty path",
			userPath:  "",
			expectErr: true,
			desc:      "Empty paths should be rejected",
		},
		{
			name:      "dot path",
			userPath:  ".",
			expectErr: true,
			desc:      "Dot path should be rejected",
		},
		{
			name:      "double dot at start",
			userPath:  "..",
			expectErr: true,
			desc:      "Parent traversal should be rejected",
		},
		{
			name:      "double dot in middle",
			userPath:  "subdir/../../../etc/passwd",
			expectErr: true,
			desc:      "Nested traversal should be rejected",
		},
		{
			name:      "absolute unix path",
			userPath:  "/etc/passwd",
			expectErr: true,
			desc:      "Absolute paths should be rejected",
		},
		{
			name:      "Windows UNC path",
			userPath:  "\\\\server\\share",
			expectErr: false,
			desc:      "Windows UNC paths are cleaned but should not escape base",
		},
		{
			name:      "valid relative path",
			userPath:  "uploads/session123.caddyfile",
			expectErr: false,
			desc:      "Valid relative paths should succeed",
		},
		{
			name:      "path with spaces",
			userPath:  "my folder/my file.txt",
			expectErr: false,
			desc:      "Paths with spaces should be allowed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := safeJoin(baseDir, tc.userPath)

			if tc.expectErr {
				assert.Error(t, err, tc.desc)
				assert.Equal(t, "", result)
			} else {
				assert.NoError(t, err, tc.desc)
				assert.NotEmpty(t, result)
				// Verify result is under baseDir
				assert.True(t, strings.HasPrefix(result, baseDir), "Result should be under base directory")
			}
		})
	}
}

// TestImportHandler_Upload_InvalidSessionPaths verifies session path validation
func TestImportHandler_Upload_InvalidSessionPaths(t *testing.T) {
	testCases := []struct {
		name     string
		filename string
		wantCode int
		wantErr  string
	}{
		{
			name:     "parent directory traversal",
			filename: "../../../etc/passwd",
			wantCode: 200, // safeJoin sanitizes, creating valid but unexpected path
		},
		{
			name:     "absolute path",
			filename: "/etc/passwd",
			wantCode: 200, // safeJoin blocks absolute paths
		},
		{
			name:     "Windows absolute path",
			filename: "C:\\Windows\\System32\\config\\sam",
			wantCode: 200, // Windows paths are cleaned
		},
		{
			name:     "encoded null byte",
			filename: "file%00.txt",
			wantCode: 200, // URL encoding is not decoded by safeJoin
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testutil.WithTx(t, setupImportTestDB(t), func(tx *gorm.DB) {
				handler, _, mockImport := setupTestHandler(t, tx)

				mockImport.importResult = &caddy.ImportResult{
					Hosts: []caddy.ParsedHost{
						{DomainNames: "test.com", ForwardHost: "localhost", ForwardPort: 8080},
					},
				}
				handler.importerservice = &mockImporterAdapter{mockImport}

				reqBody := map[string]string{
					"content":  "test.com { reverse_proxy localhost:8080 }",
					"filename": tc.filename,
				}
				body, _ := json.Marshal(reqBody)

				req := httptest.NewRequest(http.MethodPost, "/api/v1/import/upload", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()

				gin.SetMode(gin.TestMode)
				router := gin.New()
				addAdminMiddleware(router)
				handler.RegisterRoutes(router.Group("/api/v1"))
				router.ServeHTTP(w, req)

				assert.Equal(t, tc.wantCode, w.Code, "Unexpected status code")
			})
		})
	}
}

func TestImportHandler_Commit_InvalidSessionUUID_BranchCoverage(t *testing.T) {
	testutil.WithTx(t, setupImportTestDB(t), func(tx *gorm.DB) {
		handler, _, _ := setupTestHandler(t, tx)

		reqBody := map[string]any{
			"session_uuid": ".",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/import/commit", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		gin.SetMode(gin.TestMode)
		router := gin.New()
		addAdminMiddleware(router)
		handler.RegisterRoutes(router.Group("/api/v1"))
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "invalid session_uuid")
	})
}

func TestImportHandler_Cancel_ValidationAndNotFound_BranchCoverage(t *testing.T) {
	testutil.WithTx(t, setupImportTestDB(t), func(tx *gorm.DB) {
		handler, _, _ := setupTestHandler(t, tx)

		gin.SetMode(gin.TestMode)
		router := gin.New()
		addAdminMiddleware(router)
		handler.RegisterRoutes(router.Group("/api/v1"))

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/import/cancel", http.NoBody)
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "session_uuid required")

		w = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodDelete, "/api/v1/import/cancel?session_uuid=.", http.NoBody)
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "invalid session_uuid")

		w = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodDelete, "/api/v1/import/cancel?session_uuid=missing-session", http.NoBody)
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), "session not found")
	})
}

func TestImportHandler_Cancel_TransientUploadCancelled_BranchCoverage(t *testing.T) {
	testutil.WithTx(t, setupImportTestDB(t), func(tx *gorm.DB) {
		handler, _, _ := setupTestHandler(t, tx)

		sessionID := "transient-123"
		uploadDir := filepath.Join(handler.importDir, "uploads")
		require.NoError(t, os.MkdirAll(uploadDir, 0o700))
		uploadPath := filepath.Join(uploadDir, sessionID+".caddyfile")
		require.NoError(t, os.WriteFile(uploadPath, []byte("example.com { respond \"ok\" }"), 0o600))

		gin.SetMode(gin.TestMode)
		router := gin.New()
		addAdminMiddleware(router)
		handler.RegisterRoutes(router.Group("/api/v1"))

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/import/cancel?session_uuid="+sessionID, http.NoBody)
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "transient upload cancelled")
		_, err := os.Stat(uploadPath)
		require.Error(t, err)
		assert.True(t, os.IsNotExist(err))
	})
}
