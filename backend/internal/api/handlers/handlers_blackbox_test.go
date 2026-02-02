package handlers_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/api/handlers"
	"github.com/Wikid82/charon/backend/internal/models"
)

func setupImportTestDB(t *testing.T) *gorm.DB {
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect to test database")
	}
	_ = db.AutoMigrate(&models.ImportSession{}, &models.ProxyHost{}, &models.Location{})
	// Register cleanup to close database connection
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			defer func() { _ = sqlDB.Close() }()
		}
	})
	return db
}

func TestImportHandler_GetStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)

	// Case 1: No active session, no mount
	handler := handlers.NewImportHandler(db, "echo", "/tmp", "")
	router := gin.New()
	router.DELETE("/import/cancel", handler.Cancel)

	session := models.ImportSession{
		UUID:   "test-uuid",
		Status: "pending",
	}
	db.Create(&session)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/import/cancel?session_uuid=test-uuid", http.NoBody)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var updatedSession models.ImportSession
	db.First(&updatedSession, session.ID)
	assert.Equal(t, "rejected", updatedSession.Status)
}

func TestImportHandler_Commit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)
	handler := handlers.NewImportHandler(db, "echo", "/tmp", "")
	router := gin.New()
	router.POST("/import/commit", handler.Commit)

	session := models.ImportSession{
		UUID:       "test-uuid",
		Status:     "reviewing",
		ParsedData: `{"hosts": [{"domain_names": "example.com", "forward_host": "127.0.0.1", "forward_port": 8080}]}`,
	}
	db.Create(&session)

	payload := map[string]any{
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
	_ = os.Chmod(fakeCaddy, 0o755) //nolint:gosec // G302: test script needs exec permissions

	tmpDir := t.TempDir()
	handler := handlers.NewImportHandler(db, fakeCaddy, tmpDir, "")
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

	// The fake caddy script returns empty JSON, so import may produce zero hosts.
	// The handler now treats zero-host uploads without imports as a bad request (400).
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestImportHandler_GetPreview_WithContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)
	tmpDir := t.TempDir()
	handler := handlers.NewImportHandler(db, "echo", tmpDir, "")
	router := gin.New()
	router.GET("/import/preview", handler.GetPreview)

	// Case: Active session with source file
	content := "example.com {\n  reverse_proxy localhost:8080\n}"
	sourceFile := filepath.Join(tmpDir, "source.caddyfile")
	err := os.WriteFile(sourceFile, []byte(content), 0o644) //nolint:gosec // G306: test file
	assert.NoError(t, err)

	// Case: Active session with source file
	session := models.ImportSession{
		UUID:       uuid.NewString(),
		Status:     "pending",
		ParsedData: `{"hosts": []}`,
		SourceFile: sourceFile,
	}
	db.Create(&session)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/import/preview", http.NoBody)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result map[string]any
	err = json.Unmarshal(w.Body.Bytes(), &result)
	assert.NoError(t, err)

	assert.Equal(t, content, result["caddyfile_content"])
}

func TestImportHandler_Commit_Errors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)
	handler := handlers.NewImportHandler(db, "echo", "/tmp", "")
	router := gin.New()
	router.POST("/import/commit", handler.Commit)

	// Case 1: Invalid JSON
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/import/commit", bytes.NewBufferString("invalid"))
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Case 2: Session not found
	payload := map[string]any{
		"session_uuid": "non-existent",
		"resolutions":  map[string]string{},
	}
	body, _ := json.Marshal(payload)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/import/commit", bytes.NewBuffer(body))
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// Case 3: Invalid ParsedData
	session := models.ImportSession{
		UUID:       "invalid-data-uuid",
		Status:     "reviewing",
		ParsedData: "invalid-json",
	}
	db.Create(&session)

	payload = map[string]any{
		"session_uuid": "invalid-data-uuid",
		"resolutions":  map[string]string{},
	}
	body, _ = json.Marshal(payload)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/import/commit", bytes.NewBuffer(body))
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestImportHandler_Cancel_Errors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)
	handler := handlers.NewImportHandler(db, "echo", "/tmp", "")
	router := gin.New()
	router.DELETE("/import/cancel", handler.Cancel)

	// Case 1: Session not found
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/import/cancel?session_uuid=non-existent", http.NoBody)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCheckMountedImport(t *testing.T) {
	db := setupImportTestDB(t)
	tmpDir := t.TempDir()
	mountPath := filepath.Join(tmpDir, "mounted.caddyfile")

	// Use fake caddy script
	cwd, _ := os.Getwd()
	fakeCaddy := filepath.Join(cwd, "testdata", "fake_caddy.sh")
	_ = os.Chmod(fakeCaddy, 0o755) //nolint:gosec // G302: test script needs exec permissions

	// Case 1: File does not exist
	err := handlers.CheckMountedImport(db, mountPath, fakeCaddy, tmpDir)
	assert.NoError(t, err)

	// Case 2: File exists, not processed
	err = os.WriteFile(mountPath, []byte("example.com"), 0o644) //nolint:gosec // G306: test file
	assert.NoError(t, err)

	err = handlers.CheckMountedImport(db, mountPath, fakeCaddy, tmpDir)
	assert.NoError(t, err)

	// Check if session created (transient preview behavior: no DB session should be created)
	var count int64
	db.Model(&models.ImportSession{}).Where("source_file = ?", mountPath).Count(&count)
	assert.Equal(t, int64(0), count)

	// Case 3: Already processed
	err = handlers.CheckMountedImport(db, mountPath, fakeCaddy, tmpDir)
	assert.NoError(t, err)
}

func TestImportHandler_Upload_Failure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)

	// Use fake caddy script that fails
	cwd, _ := os.Getwd()
	fakeCaddy := filepath.Join(cwd, "testdata", "fake_caddy_fail.sh")

	tmpDir := t.TempDir()
	handler := handlers.NewImportHandler(db, fakeCaddy, tmpDir, "")
	router := gin.New()
	router.POST("/import/upload", handler.Upload)

	payload := map[string]string{
		"content":  "invalid caddyfile",
		"filename": "Caddyfile",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/import/upload", bytes.NewBuffer(body))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	// The error message comes from Upload -> ImportFile -> "import failed: ..."
	assert.Contains(t, resp["error"], "import failed")
}

func TestImportHandler_Upload_Conflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)

	// Pre-create a host to cause conflict
	db.Create(&models.ProxyHost{
		DomainNames: "example.com",
		ForwardHost: "127.0.0.1",
		ForwardPort: 9090,
	})

	// Use fake caddy script that returns hosts
	cwd, _ := os.Getwd()
	fakeCaddy := filepath.Join(cwd, "testdata", "fake_caddy_hosts.sh")

	tmpDir := t.TempDir()
	handler := handlers.NewImportHandler(db, fakeCaddy, tmpDir, "")
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

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify response contains conflict in preview (upload is transient)
	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	preview := resp["preview"].(map[string]any)
	conflicts := preview["conflicts"].([]any)
	found := false
	for _, c := range conflicts {
		if c.(string) == "example.com" || strings.Contains(c.(string), "example.com") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected conflict for example.com in preview")
}

func TestImportHandler_GetPreview_BackupContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)
	tmpDir := t.TempDir()
	handler := handlers.NewImportHandler(db, "echo", tmpDir, "")
	router := gin.New()
	router.GET("/import/preview", handler.GetPreview)

	// Create backup file
	backupDir := filepath.Join(tmpDir, "backups")
	_ = os.MkdirAll(backupDir, 0o755) //nolint:gosec // G301: test dir
	content := "backup content"
	backupFile := filepath.Join(backupDir, "source.caddyfile")
	_ = os.WriteFile(backupFile, []byte(content), 0o644) //nolint:gosec // G306: test file

	// Case: Active session with missing source file but existing backup
	session := models.ImportSession{
		UUID:       uuid.NewString(),
		Status:     "pending",
		ParsedData: `{"hosts": []}`,
		SourceFile: "/non/existent/source.caddyfile",
	}
	db.Create(&session)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/import/preview", http.NoBody)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &result)

	assert.Equal(t, content, result["caddyfile_content"])
}

func TestImportHandler_RegisterRoutes(t *testing.T) {
	db := setupImportTestDB(t)
	handler := handlers.NewImportHandler(db, "echo", "/tmp", "")
	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	// Verify routes exist by making requests
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/import/status", http.NoBody)
	router.ServeHTTP(w, req)
	assert.NotEqual(t, http.StatusNotFound, w.Code)
}

func TestImportHandler_GetPreview_TransientMount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)
	tmpDir := t.TempDir()
	mountPath := filepath.Join(tmpDir, "mounted.caddyfile")

	// Create a mounted Caddyfile
	content := "example.com"
	err := os.WriteFile(mountPath, []byte(content), 0o644) //nolint:gosec // G306: test file
	assert.NoError(t, err)

	// Use fake caddy script
	cwd, _ := os.Getwd()
	fakeCaddy := filepath.Join(cwd, "testdata", "fake_caddy_hosts.sh")
	_ = os.Chmod(fakeCaddy, 0o755) //nolint:gosec // G302: test script needs exec permissions

	handler := handlers.NewImportHandler(db, fakeCaddy, tmpDir, mountPath)
	router := gin.New()
	router.GET("/import/preview", handler.GetPreview)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/import/preview", http.NoBody)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "Response body: %s", w.Body.String())
	var result map[string]any
	err = json.Unmarshal(w.Body.Bytes(), &result)
	assert.NoError(t, err)

	// Verify transient session
	session, ok := result["session"].(map[string]any)
	assert.True(t, ok, "session should be present in response")
	assert.Equal(t, "transient", session["state"])
	assert.Equal(t, mountPath, session["source_file"])

	// Verify preview contains hosts
	preview, ok := result["preview"].(map[string]any)
	assert.True(t, ok, "preview should be present in response")
	assert.NotNil(t, preview["hosts"])

	// Verify content
	assert.Equal(t, content, result["caddyfile_content"])
}

func TestImportHandler_Commit_TransientUpload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)
	tmpDir := t.TempDir()

	// Use fake caddy script
	cwd, _ := os.Getwd()
	fakeCaddy := filepath.Join(cwd, "testdata", "fake_caddy_hosts.sh")
	_ = os.Chmod(fakeCaddy, 0o755) //nolint:gosec // G302: test script needs exec permissions

	handler := handlers.NewImportHandler(db, fakeCaddy, tmpDir, "")
	router := gin.New()
	router.POST("/import/upload", handler.Upload)
	router.POST("/import/commit", handler.Commit)

	// First upload to create transient session
	uploadPayload := map[string]string{
		"content":  "uploaded.com",
		"filename": "Caddyfile",
	}
	uploadBody, _ := json.Marshal(uploadPayload)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/import/upload", bytes.NewBuffer(uploadBody))
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Extract session ID
	var uploadResp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &uploadResp)
	session := uploadResp["session"].(map[string]any)
	sessionID := session["id"].(string)

	// Now commit the transient upload
	commitPayload := map[string]any{
		"session_uuid": sessionID,
		"resolutions": map[string]string{
			"uploaded.com": "import",
		},
	}
	commitBody, _ := json.Marshal(commitPayload)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/import/commit", bytes.NewBuffer(commitBody))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify host created
	var host models.ProxyHost
	err := db.Where("domain_names = ?", "uploaded.com").First(&host).Error
	assert.NoError(t, err)
	assert.Equal(t, "uploaded.com", host.DomainNames)

	// Verify session persisted
	var importSession models.ImportSession
	err = db.Where("uuid = ?", sessionID).First(&importSession).Error
	assert.NoError(t, err)
	assert.Equal(t, "committed", importSession.Status)
}

func TestImportHandler_Commit_TransientMount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)
	tmpDir := t.TempDir()
	mountPath := filepath.Join(tmpDir, "mounted.caddyfile")

	// Create a mounted Caddyfile
	err := os.WriteFile(mountPath, []byte("mounted.com"), 0o644) //nolint:gosec // G306: test file
	assert.NoError(t, err)

	// Use fake caddy script
	cwd, _ := os.Getwd()
	fakeCaddy := filepath.Join(cwd, "testdata", "fake_caddy_hosts.sh")
	_ = os.Chmod(fakeCaddy, 0o755) //nolint:gosec // G302: test script needs exec permissions

	handler := handlers.NewImportHandler(db, fakeCaddy, tmpDir, mountPath)
	router := gin.New()
	router.POST("/import/commit", handler.Commit)

	// Commit the mount with a random session ID (transient)
	sessionID := uuid.NewString()
	commitPayload := map[string]any{
		"session_uuid": sessionID,
		"resolutions": map[string]string{
			"mounted.com": "import",
		},
	}
	commitBody, _ := json.Marshal(commitPayload)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/import/commit", bytes.NewBuffer(commitBody))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify host created
	var host models.ProxyHost
	err = db.Where("domain_names = ?", "mounted.com").First(&host).Error
	assert.NoError(t, err)

	// Verify session persisted
	var importSession models.ImportSession
	err = db.Where("uuid = ?", sessionID).First(&importSession).Error
	assert.NoError(t, err)
	assert.Equal(t, "committed", importSession.Status)
}

func TestImportHandler_Cancel_TransientUpload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)
	tmpDir := t.TempDir()

	// Use fake caddy script
	cwd, _ := os.Getwd()
	fakeCaddy := filepath.Join(cwd, "testdata", "fake_caddy_hosts.sh")
	_ = os.Chmod(fakeCaddy, 0o755) //nolint:gosec // G302: test script needs exec permissions

	handler := handlers.NewImportHandler(db, fakeCaddy, tmpDir, "")
	router := gin.New()
	router.POST("/import/commit", handler.Commit)
	router.DELETE("/import/cancel", handler.Cancel)

	// Commit - Session Not Found
	body := map[string]any{
		"session_uuid": "non-existent",
		"resolutions":  map[string]string{},
	}
	jsonBody, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/import/commit", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// Cancel - Session Not Found
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/import/cancel?session_uuid=non-existent", http.NoBody)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestImportHandler_DetectImports(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)
	handler := handlers.NewImportHandler(db, "echo", "/tmp", "")
	router := gin.New()
	router.POST("/import/detect-imports", handler.DetectImports)

	tests := []struct {
		name      string
		content   string
		hasImport bool
		imports   []string
	}{
		{
			name:      "no imports",
			content:   "example.com { reverse_proxy localhost:8080 }",
			hasImport: false,
			imports:   []string{},
		},
		{
			name:      "single import",
			content:   "import sites/*\nexample.com { reverse_proxy localhost:8080 }",
			hasImport: true,
			imports:   []string{"sites/*"},
		},
		{
			name:      "multiple imports",
			content:   "import sites/*\nimport config/ssl.conf\nexample.com { reverse_proxy localhost:8080 }",
			hasImport: true,
			imports:   []string{"sites/*", "config/ssl.conf"},
		},
		{
			name:      "import with comment",
			content:   "import sites/* # Load all sites\nexample.com { reverse_proxy localhost:8080 }",
			hasImport: true,
			imports:   []string{"sites/*"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := map[string]string{"content": tt.content}
			body, _ := json.Marshal(payload)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/import/detect-imports", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var resp map[string]any
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			assert.NoError(t, err)
			assert.Equal(t, tt.hasImport, resp["has_imports"])

			imports := resp["imports"].([]any)
			assert.Len(t, imports, len(tt.imports))
		})
	}
}

func TestImportHandler_DetectImports_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)
	handler := handlers.NewImportHandler(db, "echo", "/tmp", "")
	router := gin.New()
	router.POST("/import/detect-imports", handler.DetectImports)

	// Invalid JSON
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/import/detect-imports", strings.NewReader("invalid"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestImportHandler_UploadMulti(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)
	tmpDir := t.TempDir()

	// Use fake caddy script
	cwd, _ := os.Getwd()
	fakeCaddy := filepath.Join(cwd, "testdata", "fake_caddy_hosts.sh")
	_ = os.Chmod(fakeCaddy, 0o755) //nolint:gosec // G302: test script needs exec permissions

	handler := handlers.NewImportHandler(db, fakeCaddy, tmpDir, "")
	router := gin.New()
	router.POST("/import/upload-multi", handler.UploadMulti)

	t.Run("single Caddyfile", func(t *testing.T) {
		payload := map[string]any{
			"files": []map[string]string{
				{"filename": "Caddyfile", "content": "example.com"},
			},
		}
		body, _ := json.Marshal(payload)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/import/upload-multi", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NotNil(t, resp["session"])
		assert.NotNil(t, resp["preview"])
	})

	t.Run("Caddyfile with site files", func(t *testing.T) {
		payload := map[string]any{
			"files": []map[string]string{
				{"filename": "Caddyfile", "content": "import sites/*\n"},
				{"filename": "sites/site1", "content": "site1.com"},
				{"filename": "sites/site2", "content": "site2.com"},
			},
		}
		body, _ := json.Marshal(payload)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/import/upload-multi", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		session := resp["session"].(map[string]any)
		assert.Equal(t, "transient", session["state"])
	})

	t.Run("missing Caddyfile", func(t *testing.T) {
		payload := map[string]any{
			"files": []map[string]string{
				{"filename": "sites/site1", "content": "site1.com"},
			},
		}
		body, _ := json.Marshal(payload)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/import/upload-multi", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("path traversal in filename", func(t *testing.T) {
		payload := map[string]any{
			"files": []map[string]string{
				{"filename": "Caddyfile", "content": "import sites/*\n"},
				{"filename": "../etc/passwd", "content": "sensitive"},
			},
		}
		body, _ := json.Marshal(payload)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/import/upload-multi", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("empty file content", func(t *testing.T) {
		payload := map[string]any{
			"files": []map[string]string{
				{"filename": "Caddyfile", "content": "example.com"},
				{"filename": "sites/site1", "content": "   "},
			},
		}
		body, _ := json.Marshal(payload)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/import/upload-multi", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var resp map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Contains(t, resp["error"], "empty")
	})
}

// Additional tests for comprehensive coverage

func TestImportHandler_Cancel_MissingSessionUUID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)
	handler := handlers.NewImportHandler(db, "echo", "/tmp", "")
	router := gin.New()
	router.DELETE("/import/cancel", handler.Cancel)

	// Missing session_uuid parameter
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/import/cancel", http.NoBody)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "session_uuid required", resp["error"])
}

func TestImportHandler_Cancel_InvalidSessionUUID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)
	handler := handlers.NewImportHandler(db, "echo", "/tmp", "")
	router := gin.New()
	router.DELETE("/import/cancel", handler.Cancel)

	// Test "." which becomes empty after filepath.Base processing
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/import/cancel?session_uuid=.", http.NoBody)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "invalid session_uuid", resp["error"])
}

func TestImportHandler_Commit_InvalidSessionUUID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)
	handler := handlers.NewImportHandler(db, "echo", "/tmp", "")
	router := gin.New()
	router.POST("/import/commit", handler.Commit)

	// Test "." which becomes empty after filepath.Base processing
	payload := map[string]any{
		"session_uuid": ".",
		"resolutions":  map[string]string{},
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/import/commit", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "invalid session_uuid", resp["error"])
}

// mockProxyHostService is a mock implementation of ProxyHostServiceInterface for testing.
type mockProxyHostService struct {
	createFunc func(host *models.ProxyHost) error
	updateFunc func(host *models.ProxyHost) error
	listFunc   func() ([]models.ProxyHost, error)
}

func (m *mockProxyHostService) Create(host *models.ProxyHost) error {
	if m.createFunc != nil {
		return m.createFunc(host)
	}
	return nil
}

func (m *mockProxyHostService) Update(host *models.ProxyHost) error {
	if m.updateFunc != nil {
		return m.updateFunc(host)
	}
	return nil
}

func (m *mockProxyHostService) List() ([]models.ProxyHost, error) {
	if m.listFunc != nil {
		return m.listFunc()
	}
	return []models.ProxyHost{}, nil
}

// TestImportHandler_Commit_UpdateFailure tests the error logging path when Update fails (line 676)
func TestImportHandler_Commit_UpdateFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)

	// Create an existing host that we'll try to overwrite
	existingHost := models.ProxyHost{
		UUID:        uuid.NewString(),
		DomainNames: "existing.com",
	}
	db.Create(&existingHost)

	// Create an import session with a host matching the existing one
	session := models.ImportSession{
		UUID:   uuid.NewString(),
		Status: "reviewing",
		ParsedData: `{
			"hosts": [
				{
					"domain_names": "existing.com",
					"forward_host": "192.168.1.1",
					"forward_port": 80,
					"forward_scheme": "http"
				}
			]
		}`,
	}
	db.Create(&session)

	// Create a mock service that returns existing hosts and fails on Update
	mockSvc := &mockProxyHostService{
		listFunc: func() ([]models.ProxyHost, error) {
			return []models.ProxyHost{existingHost}, nil
		},
		updateFunc: func(host *models.ProxyHost) error {
			return errors.New("mock update failure: database connection lost")
		},
	}

	handler := handlers.NewImportHandlerWithService(db, mockSvc, "echo", "/tmp", "")
	router := gin.New()
	router.POST("/import/commit", handler.Commit)

	// Request to overwrite existing.com
	payload := map[string]any{
		"session_uuid": session.UUID,
		"resolutions": map[string]string{
			"existing.com": "overwrite",
		},
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/import/commit", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// The commit should complete but with errors (line 676 executed)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)

	// Should have errors due to update failure
	respErrors, ok := resp["errors"].([]interface{})
	assert.True(t, ok, "expected errors array in response")
	assert.Greater(t, len(respErrors), 0, "expected at least one error")
	assert.Contains(t, respErrors[0].(string), "existing.com")
	assert.Contains(t, respErrors[0].(string), "mock update failure")

	// updated count should be 0
	assert.Equal(t, float64(0), resp["updated"])
}

// TestImportHandler_Commit_CreateFailure tests the error logging path when Create fails (line 682)
func TestImportHandler_Commit_CreateFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)

	// Create an existing host to cause a duplicate error
	existingHost := models.ProxyHost{
		UUID:        uuid.NewString(),
		DomainNames: "duplicate.com",
	}
	db.Create(&existingHost)

	// Create an import session that tries to create a duplicate host
	session := models.ImportSession{
		UUID:   uuid.NewString(),
		Status: "reviewing",
		ParsedData: `{
			"hosts": [
				{
					"domain_names": "duplicate.com",
					"forward_host": "192.168.1.1",
					"forward_port": 80,
					"forward_scheme": "http"
				}
			]
		}`,
	}
	db.Create(&session)

	handler := handlers.NewImportHandler(db, "echo", "/tmp", "")
	router := gin.New()
	router.POST("/import/commit", handler.Commit)

	// Don't provide resolution, so it defaults to create (not overwrite)
	payload := map[string]any{
		"session_uuid": session.UUID,
		"resolutions":  map[string]string{},
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/import/commit", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// The commit should complete but with errors
	// Line 682 should be executed: logging the create error
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)

	// Should have errors due to duplicate domain
	errors, ok := resp["errors"].([]interface{})
	assert.True(t, ok)
	assert.Greater(t, len(errors), 0)
	// Verify the error mentions the duplicate
	assert.Contains(t, errors[0].(string), "duplicate.com")
}

// TestUpload_NormalizationSuccess tests the success path where NormalizeCaddyfile succeeds (line 271)
func TestUpload_NormalizationSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)

	// Use fake caddy script that handles both fmt and adapt
	cwd, _ := os.Getwd()
	fakeCaddy := filepath.Join(cwd, "testdata", "fake_caddy_fmt_success.sh")
	_ = os.Chmod(fakeCaddy, 0o755) //nolint:gosec // G302: test script needs exec permissions

	tmpDir := t.TempDir()
	handler := handlers.NewImportHandler(db, fakeCaddy, tmpDir, "")
	router := gin.New()
	router.POST("/import/upload", handler.Upload)

	// Use single-line Caddyfile format (triggers normalization)
	singleLineCaddyfile := `test.local { reverse_proxy localhost:3000 }`

	payload := map[string]string{
		"content":  singleLineCaddyfile,
		"filename": "Caddyfile",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/import/upload", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// Should succeed with 200 (normalization worked)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify response contains hosts (parsing succeeded)
	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Verify preview contains hosts
	preview, ok := response["preview"].(map[string]any)
	assert.True(t, ok, "response should contain preview")
	hosts, ok := preview["hosts"].([]any)
	assert.True(t, ok, "preview should contain hosts")
	assert.Greater(t, len(hosts), 0, "should have at least one parsed host")
}

// TestUpload_NormalizationFallback tests the fallback path where NormalizeCaddyfile fails (line 269)
func TestUpload_NormalizationFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)

	// Use fake caddy script that fails fmt but succeeds on adapt
	cwd, _ := os.Getwd()
	fakeCaddy := filepath.Join(cwd, "testdata", "fake_caddy_fmt_fail.sh")
	_ = os.Chmod(fakeCaddy, 0o755) //nolint:gosec // G302: test script needs exec permissions

	tmpDir := t.TempDir()
	handler := handlers.NewImportHandler(db, fakeCaddy, tmpDir, "")
	router := gin.New()
	router.POST("/import/upload", handler.Upload)

	// Valid Caddyfile that would parse successfully (even if normalization fails)
	caddyfile := `test.local {
    reverse_proxy localhost:3000
}`

	payload := map[string]string{
		"content":  caddyfile,
		"filename": "Caddyfile",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/import/upload", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// Should still succeed (falls back to original content)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify hosts were parsed from original content
	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Verify preview contains hosts
	preview, ok := response["preview"].(map[string]any)
	assert.True(t, ok, "response should contain preview")
	hosts, ok := preview["hosts"].([]any)
	assert.True(t, ok, "preview should contain hosts")
	assert.Greater(t, len(hosts), 0, "should have at least one parsed host from original content")
}

// TestCommit_OverwriteAction tests that overwrite preserves certificate ID
func TestCommit_OverwriteAction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)

	// Create existing host with certificate association
	existingHost := models.ProxyHost{
		UUID:          uuid.NewString(),
		DomainNames:   "ssl-site.com",
		ForwardHost:   "10.0.0.1",
		ForwardPort:   80,
		CertificateID: ptrToUint(42), // Existing certificate reference
	}
	db.Create(&existingHost)

	// Create session with host matching existing one
	session := models.ImportSession{
		UUID:   uuid.NewString(),
		Status: "reviewing",
		ParsedData: `{
			"hosts": [
				{
					"domain_names": "ssl-site.com",
					"forward_host": "192.168.1.100",
					"forward_port": 8080,
					"forward_scheme": "https"
				}
			]
		}`,
	}
	db.Create(&session)

	handler := handlers.NewImportHandler(db, "echo", "/tmp", "")
	router := gin.New()
	router.POST("/import/commit", handler.Commit)

	payload := map[string]any{
		"session_uuid": session.UUID,
		"resolutions": map[string]string{
			"ssl-site.com": "overwrite",
		},
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/import/commit", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, float64(1), resp["updated"], "should update one host")

	// Verify the host was updated but certificate was preserved
	var updatedHost models.ProxyHost
	db.Where("domain_names = ?", "ssl-site.com").First(&updatedHost)
	assert.Equal(t, "192.168.1.100", updatedHost.ForwardHost, "forward host should be updated")
	assert.Equal(t, 8080, updatedHost.ForwardPort, "forward port should be updated")
	assert.NotNil(t, updatedHost.CertificateID, "certificate ID should be preserved")
	assert.Equal(t, uint(42), *updatedHost.CertificateID, "certificate ID value should be preserved")
	assert.Equal(t, existingHost.UUID, updatedHost.UUID, "UUID should be preserved")
}

// ptrToUint is a helper to create a pointer to uint
func ptrToUint(v uint) *uint {
	return &v
}

// TestCommit_RenameAction tests that rename appends suffix
func TestCommit_RenameAction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)

	// Create existing host
	existingHost := models.ProxyHost{
		UUID:        uuid.NewString(),
		DomainNames: "app.example.com",
		ForwardHost: "10.0.0.1",
		ForwardPort: 80,
	}
	db.Create(&existingHost)

	// Create session with conflicting host
	session := models.ImportSession{
		UUID:   uuid.NewString(),
		Status: "reviewing",
		ParsedData: `{
			"hosts": [
				{
					"domain_names": "app.example.com",
					"forward_host": "192.168.1.100",
					"forward_port": 9000,
					"forward_scheme": "http"
				}
			]
		}`,
	}
	db.Create(&session)

	handler := handlers.NewImportHandler(db, "echo", "/tmp", "")
	router := gin.New()
	router.POST("/import/commit", handler.Commit)

	payload := map[string]any{
		"session_uuid": session.UUID,
		"resolutions": map[string]string{
			"app.example.com": "rename",
		},
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/import/commit", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, float64(1), resp["created"], "should create one host with renamed domain")

	// Verify the renamed host was created
	var renamedHost models.ProxyHost
	err := db.Where("domain_names = ?", "app.example.com-imported").First(&renamedHost).Error
	assert.NoError(t, err, "renamed host should exist")
	assert.Equal(t, "192.168.1.100", renamedHost.ForwardHost)
	assert.Equal(t, 9000, renamedHost.ForwardPort)

	// Verify original host is unchanged
	var originalHost models.ProxyHost
	err = db.Where("domain_names = ?", "app.example.com").First(&originalHost).Error
	assert.NoError(t, err)
	assert.Equal(t, "10.0.0.1", originalHost.ForwardHost)
}

func TestGetPreview_WithConflictDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)
	tmpDir := t.TempDir()
	mountPath := filepath.Join(tmpDir, "mounted.caddyfile")

	// Create a mounted Caddyfile
	content := "conflict.example.com"
	err := os.WriteFile(mountPath, []byte(content), 0o644) //nolint:gosec // G306: test file
	assert.NoError(t, err)

	// Pre-create an existing host that conflicts
	existingHost := models.ProxyHost{
		UUID:          uuid.NewString(),
		DomainNames:   "conflict.example.com",
		ForwardScheme: "http",
		ForwardHost:   "10.0.0.1",
		ForwardPort:   80,
		SSLForced:     false,
		Enabled:       true,
	}
	db.Create(&existingHost)

	// Use fake caddy script that returns the conflicting host
	cwd, _ := os.Getwd()
	fakeCaddy := filepath.Join(cwd, "testdata", "fake_caddy_conflict.sh")
	_ = os.Chmod(fakeCaddy, 0o755) //nolint:gosec // G302: test script needs exec permissions

	handler := handlers.NewImportHandler(db, fakeCaddy, tmpDir, mountPath)
	router := gin.New()
	router.GET("/import/preview", handler.GetPreview)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/import/preview", http.NoBody)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]any
	err = json.Unmarshal(w.Body.Bytes(), &result)
	assert.NoError(t, err)

	// Check for conflict_details
	conflictDetails, ok := result["conflict_details"].(map[string]any)
	assert.True(t, ok, "should have conflict_details")

	if len(conflictDetails) > 0 {
		// Verify conflict contains existing and imported info
		for domain, details := range conflictDetails {
			assert.Equal(t, "conflict.example.com", domain)
			detailsMap := details.(map[string]any)
			assert.NotNil(t, detailsMap["existing"])
			assert.NotNil(t, detailsMap["imported"])
		}
	}
}

func TestSafeJoin_PathTraversalCases(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)
	tmpDir := t.TempDir()
	handler := handlers.NewImportHandler(db, "echo", tmpDir, "")
	router := gin.New()
	router.POST("/import/upload-multi", handler.UploadMulti)

	tests := []struct {
		name          string
		filename      string
		expectStatus  int
		expectErrorIn string
	}{
		{
			name:          "double dot prefix",
			filename:      "../etc/passwd",
			expectStatus:  http.StatusBadRequest,
			expectErrorIn: "invalid filename",
		},
		{
			name:          "hidden double dot",
			filename:      "sites/../../../etc/passwd",
			expectStatus:  http.StatusBadRequest,
			expectErrorIn: "invalid filename",
		},
		{
			name:          "absolute path",
			filename:      "/etc/passwd",
			expectStatus:  http.StatusBadRequest,
			expectErrorIn: "invalid filename",
		},
		{
			name:         "valid nested path",
			filename:     "sites/site1.conf",
			expectStatus: http.StatusOK, // or StatusBadRequest for no hosts, but not path traversal error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := map[string]any{
				"files": []map[string]string{
					{"filename": "Caddyfile", "content": "import sites/*\n"},
					{"filename": tt.filename, "content": "test content"},
				},
			}
			body, _ := json.Marshal(payload)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/import/upload-multi", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			if tt.expectErrorIn != "" {
				assert.Equal(t, tt.expectStatus, w.Code)
				var resp map[string]any
				_ = json.Unmarshal(w.Body.Bytes(), &resp)
				assert.Contains(t, resp["error"], tt.expectErrorIn)
			}
		})
	}
}

func TestCommit_SkipAction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)

	session := models.ImportSession{
		UUID:   uuid.NewString(),
		Status: "reviewing",
		ParsedData: `{
			"hosts": [
				{
					"domain_names": "skip-me.com",
					"forward_host": "192.168.1.1",
					"forward_port": 80,
					"forward_scheme": "http"
				},
				{
					"domain_names": "keep-existing.com",
					"forward_host": "192.168.1.2",
					"forward_port": 80,
					"forward_scheme": "http"
				}
			]
		}`,
	}
	db.Create(&session)

	handler := handlers.NewImportHandler(db, "echo", "/tmp", "")
	router := gin.New()
	router.POST("/import/commit", handler.Commit)

	payload := map[string]any{
		"session_uuid": session.UUID,
		"resolutions": map[string]string{
			"skip-me.com":       "skip",
			"keep-existing.com": "keep",
		},
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/import/commit", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, float64(2), resp["skipped"], "should skip two hosts")
	assert.Equal(t, float64(0), resp["created"], "should not create any hosts")

	// Verify hosts were not created
	var count int64
	db.Model(&models.ProxyHost{}).Where("domain_names IN ?", []string{"skip-me.com", "keep-existing.com"}).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestCommit_CustomNames(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)

	session := models.ImportSession{
		UUID:   uuid.NewString(),
		Status: "reviewing",
		ParsedData: `{
			"hosts": [
				{
					"domain_names": "api.example.com",
					"forward_host": "192.168.1.1",
					"forward_port": 3000,
					"forward_scheme": "http"
				}
			]
		}`,
	}
	db.Create(&session)

	handler := handlers.NewImportHandler(db, "echo", "/tmp", "")
	router := gin.New()
	router.POST("/import/commit", handler.Commit)

	payload := map[string]any{
		"session_uuid": session.UUID,
		"resolutions": map[string]string{
			"api.example.com": "import",
		},
		"names": map[string]string{
			"api.example.com": "Production API Server",
		},
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/import/commit", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify the custom name was applied
	var host models.ProxyHost
	err := db.Where("domain_names = ?", "api.example.com").First(&host).Error
	assert.NoError(t, err)
	assert.Equal(t, "Production API Server", host.Name)
}

func TestGetStatus_AlreadyCommittedMount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)
	tmpDir := t.TempDir()
	mountPath := filepath.Join(tmpDir, "mounted.caddyfile")

	// Create mounted file
	err := os.WriteFile(mountPath, []byte("example.com"), 0o644) //nolint:gosec // G306: test file
	assert.NoError(t, err)

	// Create a committed session for this mount with a future time
	now := time.Now().Add(1 * time.Hour) // Future time to simulate already committed
	committedSession := models.ImportSession{
		UUID:        uuid.NewString(),
		Status:      "committed",
		SourceFile:  mountPath,
		CommittedAt: &now,
	}
	db.Create(&committedSession)

	handler := handlers.NewImportHandler(db, "echo", tmpDir, mountPath)
	router := gin.New()
	router.GET("/import/status", handler.GetStatus)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/import/status", http.NoBody)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, false, resp["has_pending"], "should not show pending for already committed mount")
}

func TestImportHandler_Commit_SessionSaveWarning(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)

	// Create an import session with one host to create
	session := models.ImportSession{
		UUID:       uuid.NewString(),
		Status:     "reviewing",
		ParsedData: `{"hosts":[{"domain_names":"savewarn.example.com","forward_host":"127.0.0.1","forward_port":8080,"forward_scheme":"http"}]}`,
	}
	if err := db.Create(&session).Error; err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Use a mock proxyHostService that succeeds on Create
	mockSvc := &mockProxyHostService{
		listFunc:   func() ([]models.ProxyHost, error) { return []models.ProxyHost{}, nil },
		createFunc: func(h *models.ProxyHost) error { h.ID = 1; return nil },
	}

	h := handlers.NewImportHandlerWithService(db, mockSvc, "echo", "/tmp", "")
	router := gin.New()
	router.POST("/import/commit", h.Commit)

	// Inject a GORM callback to force an error when updating ImportSession (simulates non-fatal save warning)
	err := db.Callback().Update().Before("gorm:before_update").Register("test:inject_importsession_save_error", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Name == "ImportSession" {
			_ = tx.AddError(errors.New("simulated session save failure"))
		}
	})
	require.NoError(t, err, "Failed to register GORM callback")

	// Capture global logs so we can assert a warning was emitted
	var buf bytes.Buffer
	logger.Init(false, &buf)

	payload := map[string]any{
		"session_uuid": session.UUID,
		"resolutions": map[string]string{
			"savewarn.example.com": "import",
		},
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/import/commit", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// Commit should still succeed (session save warning is non-fatal)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	// Should report created host despite DB save warning
	assert.Equal(t, float64(1), resp["created"])

	// Warning must have been logged
	assert.Contains(t, buf.String(), "failed to save import session")
}

// newTestImportHandler creates an ImportHandler with proper cleanup for tests
func newTestImportHandler(t *testing.T, db *gorm.DB, importDir string, mountPath string) *handlers.ImportHandler {
	handler := handlers.NewImportHandler(db, "caddy", importDir, mountPath)
	t.Cleanup(func() {
		// Cleanup resources if needed
	})
	return handler
}

// TestGetStatus_DatabaseError tests GetStatus when database query fails
func TestGetStatus_DatabaseError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)
	handler := newTestImportHandler(t, db, t.TempDir(), "")

	// Close DB to trigger error
	sqlDB, err := db.DB()
	require.NoError(t, err)
	_ = sqlDB.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/import/status", nil)

	handler.GetStatus(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestGetPreview_MountAlreadyCommitted tests GetPreview when mount is already committed with FUTURE timestamp
func TestGetPreview_MountAlreadyCommitted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)

	// Create mount file
	mountDir := t.TempDir()
	mountPath := filepath.Join(mountDir, "Caddyfile")
	err := os.WriteFile(mountPath, []byte("test.local { reverse_proxy localhost:8080 }"), 0o644) //nolint:gosec // G306: test file
	require.NoError(t, err)

	// Create committed session with FUTURE timestamp (after file mod time)
	now := time.Now().Add(1 * time.Hour)
	session := models.ImportSession{
		UUID:        "test-session",
		SourceFile:  mountPath,
		Status:      "committed",
		CommittedAt: &now,
	}
	db.Create(&session)

	handler := newTestImportHandler(t, db, t.TempDir(), mountPath)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/import/preview", nil)

	handler.GetPreview(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "no pending import")
}

// TestUpload_MkdirAllFailure tests Upload when MkdirAll fails
func TestUpload_MkdirAllFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)

	// Create a FILE where uploads directory should be (blocks MkdirAll)
	importDir := t.TempDir()
	uploadsPath := filepath.Join(importDir, "uploads")
	err := os.WriteFile(uploadsPath, []byte("blocker"), 0o644) //nolint:gosec // G306: test file
	require.NoError(t, err)

	handler := newTestImportHandler(t, db, importDir, "")

	reqBody := `{"content": "test.local { reverse_proxy localhost:8080 }", "filename": "test.caddy"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/import/upload", strings.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Upload(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
