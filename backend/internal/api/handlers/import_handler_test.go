package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
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
	db.AutoMigrate(&models.ImportSession{}, &models.ProxyHost{}, &models.Location{})
	return db
}

func TestImportHandler_GetStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)

	// Case 1: No active session, no mount
	handler := handlers.NewImportHandler(db, "echo", "/tmp", "")
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

	// Case 2: No DB session but has mounted Caddyfile
	tmpDir := t.TempDir()
	mountPath := filepath.Join(tmpDir, "mounted.caddyfile")
	os.WriteFile(mountPath, []byte("example.com"), 0644)

	handler2 := handlers.NewImportHandler(db, "echo", "/tmp", mountPath)
	router2 := gin.New()
	router2.GET("/import/status", handler2.GetStatus)

	w = httptest.NewRecorder()
	router2.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, true, resp["has_pending"])
	session := resp["session"].(map[string]interface{})
	assert.Equal(t, "transient", session["state"])
	assert.Equal(t, mountPath, session["source_file"])

	// Case 3: Active DB session (takes precedence over mount)
	dbSession := models.ImportSession{
		UUID:       uuid.NewString(),
		Status:     "pending",
		ParsedData: `{"hosts": []}`,
	}
	db.Create(&dbSession)

	w = httptest.NewRecorder()
	router2.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, true, resp["has_pending"])
	session = resp["session"].(map[string]interface{})
	assert.Equal(t, "pending", session["state"]) // DB session, not transient
}

func TestImportHandler_GetPreview(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)
	handler := handlers.NewImportHandler(db, "echo", "/tmp", "")
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
	handler := handlers.NewImportHandler(db, "echo", "/tmp", "")
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
	handler := handlers.NewImportHandler(db, "echo", "/tmp", "")
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
	err := os.WriteFile(sourceFile, []byte(content), 0644)
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
	req, _ := http.NewRequest("GET", "/import/preview", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result map[string]interface{}
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
	payload := map[string]interface{}{
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

	payload = map[string]interface{}{
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
	req, _ := http.NewRequest("DELETE", "/import/cancel?session_uuid=non-existent", nil)
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
	os.Chmod(fakeCaddy, 0755)

	// Case 1: File does not exist
	err := handlers.CheckMountedImport(db, mountPath, fakeCaddy, tmpDir)
	assert.NoError(t, err)

	// Case 2: File exists, not processed
	err = os.WriteFile(mountPath, []byte("example.com"), 0644)
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
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
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
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	preview := resp["preview"].(map[string]interface{})
	conflicts := preview["conflicts"].([]interface{})
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
	os.MkdirAll(backupDir, 0755)
	content := "backup content"
	backupFile := filepath.Join(backupDir, "source.caddyfile")
	os.WriteFile(backupFile, []byte(content), 0644)

	// Case: Active session with missing source file but existing backup
	session := models.ImportSession{
		UUID:       uuid.NewString(),
		Status:     "pending",
		ParsedData: `{"hosts": []}`,
		SourceFile: "/non/existent/source.caddyfile",
	}
	db.Create(&session)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/import/preview", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)

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
	req, _ := http.NewRequest("GET", "/api/v1/import/status", nil)
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
	err := os.WriteFile(mountPath, []byte(content), 0644)
	assert.NoError(t, err)

	// Use fake caddy script
	cwd, _ := os.Getwd()
	fakeCaddy := filepath.Join(cwd, "testdata", "fake_caddy_hosts.sh")
	os.Chmod(fakeCaddy, 0755)

	handler := handlers.NewImportHandler(db, fakeCaddy, tmpDir, mountPath)
	router := gin.New()
	router.GET("/import/preview", handler.GetPreview)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/import/preview", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "Response body: %s", w.Body.String())
	var result map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &result)
	assert.NoError(t, err)

	// Verify transient session
	session, ok := result["session"].(map[string]interface{})
	assert.True(t, ok, "session should be present in response")
	assert.Equal(t, "transient", session["state"])
	assert.Equal(t, mountPath, session["source_file"])

	// Verify preview contains hosts
	preview, ok := result["preview"].(map[string]interface{})
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
	os.Chmod(fakeCaddy, 0755)

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
	var uploadResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &uploadResp)
	session := uploadResp["session"].(map[string]interface{})
	sessionID := session["id"].(string)

	// Now commit the transient upload
	commitPayload := map[string]interface{}{
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
	err := os.WriteFile(mountPath, []byte("mounted.com"), 0644)
	assert.NoError(t, err)

	// Use fake caddy script
	cwd, _ := os.Getwd()
	fakeCaddy := filepath.Join(cwd, "testdata", "fake_caddy_hosts.sh")
	os.Chmod(fakeCaddy, 0755)

	handler := handlers.NewImportHandler(db, fakeCaddy, tmpDir, mountPath)
	router := gin.New()
	router.POST("/import/commit", handler.Commit)

	// Commit the mount with a random session ID (transient)
	sessionID := uuid.NewString()
	commitPayload := map[string]interface{}{
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
	os.Chmod(fakeCaddy, 0755)

	handler := handlers.NewImportHandler(db, fakeCaddy, tmpDir, "")
	router := gin.New()
	router.POST("/import/upload", handler.Upload)
	router.DELETE("/import/cancel", handler.Cancel)

	// Upload to create transient file
	uploadPayload := map[string]string{
		"content":  "test.com",
		"filename": "Caddyfile",
	}
	uploadBody, _ := json.Marshal(uploadPayload)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/import/upload", bytes.NewBuffer(uploadBody))
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Extract session ID and file path
	var uploadResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &uploadResp)
	session := uploadResp["session"].(map[string]interface{})
	sessionID := session["id"].(string)
	sourceFile := session["source_file"].(string)

	// Verify file exists
	_, err := os.Stat(sourceFile)
	assert.NoError(t, err)

	// Cancel should delete the file
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/import/cancel?session_uuid="+sessionID, nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify file deleted
	_, err = os.Stat(sourceFile)
	assert.True(t, os.IsNotExist(err))
}

func TestImportHandler_Errors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)
	handler := handlers.NewImportHandler(db, "echo", "/tmp", "")
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

			var resp map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			assert.NoError(t, err)
			assert.Equal(t, tt.hasImport, resp["has_imports"])

			imports := resp["imports"].([]interface{})
			assert.Len(t, imports, len(tt.imports))
		})
	}
}

func TestImportHandler_UploadMulti(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportTestDB(t)
	tmpDir := t.TempDir()

	// Use fake caddy script
	cwd, _ := os.Getwd()
	fakeCaddy := filepath.Join(cwd, "testdata", "fake_caddy_hosts.sh")
	os.Chmod(fakeCaddy, 0755)

	handler := handlers.NewImportHandler(db, fakeCaddy, tmpDir, "")
	router := gin.New()
	router.POST("/import/upload-multi", handler.UploadMulti)

	t.Run("single Caddyfile", func(t *testing.T) {
		payload := map[string]interface{}{
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

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NotNil(t, resp["session"])
		assert.NotNil(t, resp["preview"])
	})

	t.Run("Caddyfile with site files", func(t *testing.T) {
		payload := map[string]interface{}{
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

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		session := resp["session"].(map[string]interface{})
		assert.Equal(t, "transient", session["state"])
	})

	t.Run("missing Caddyfile", func(t *testing.T) {
		payload := map[string]interface{}{
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
		payload := map[string]interface{}{
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
		payload := map[string]interface{}{
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
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Contains(t, resp["error"], "empty")
	})
}
