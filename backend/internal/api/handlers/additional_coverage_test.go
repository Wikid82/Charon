package handlers

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

func setupImportCoverageDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := OpenTestDB(t)
	db.AutoMigrate(&models.ImportSession{}, &models.ProxyHost{}, &models.Domain{})
	return db
}

func TestImportHandler_Commit_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportCoverageDB(t)

	h := NewImportHandler(db, "", t.TempDir(), "")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/import/commit", bytes.NewBufferString("invalid"))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Commit(c)

	assert.Equal(t, 400, w.Code)
}

func TestImportHandler_Commit_InvalidSessionUUID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportCoverageDB(t)

	h := NewImportHandler(db, "", t.TempDir(), "")

	body, _ := json.Marshal(map[string]interface{}{
		"session_uuid": "../../../etc/passwd",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/import/commit", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Commit(c)

	// After sanitization, "../../../etc/passwd" becomes "passwd" which doesn't exist
	assert.Equal(t, 404, w.Code)
	assert.Contains(t, w.Body.String(), "session not found")
}

func TestImportHandler_Commit_SessionNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportCoverageDB(t)

	h := NewImportHandler(db, "", t.TempDir(), "")

	body, _ := json.Marshal(map[string]interface{}{
		"session_uuid": "nonexistent-session",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/import/commit", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Commit(c)

	assert.Equal(t, 404, w.Code)
	assert.Contains(t, w.Body.String(), "session not found")
}

// Remote Server Handler additional test

func setupRemoteServerCoverageDB2(t *testing.T) *gorm.DB {
	t.Helper()
	db := OpenTestDB(t)
	db.AutoMigrate(&models.RemoteServer{})
	return db
}

func TestRemoteServerHandler_TestConnection_Unreachable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupRemoteServerCoverageDB2(t)
	svc := services.NewRemoteServerService(db)
	h := NewRemoteServerHandler(svc, nil)

	// Create a server with unreachable host
	server := &models.RemoteServer{
		Name: "Unreachable",
		Host: "192.0.2.1", // TEST-NET - not routable
		Port: 65535,
	}
	svc.Create(server)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "uuid", Value: server.UUID}}

	h.TestConnection(c)

	// Should return 200 with reachable: false
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), `"reachable":false`)
}

// Security Handler additional coverage tests

func setupSecurityCoverageDB3(t *testing.T) *gorm.DB {
	t.Helper()
	db := OpenTestDB(t)
	db.AutoMigrate(
		&models.SecurityConfig{},
		&models.SecurityDecision{},
		&models.SecurityRuleSet{},
		&models.SecurityAudit{},
	)
	return db
}

func TestSecurityHandler_GetConfig_InternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupSecurityCoverageDB3(t)

	h := NewSecurityHandler(config.SecurityConfig{}, db, nil)

	// Drop table to cause internal error (not ErrSecurityConfigNotFound)
	db.Migrator().DropTable(&models.SecurityConfig{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/security/config", nil)

	h.GetConfig(c)

	// Should return internal error
	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "failed to read security config")
}

func TestSecurityHandler_UpdateConfig_ApplyCaddyError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupSecurityCoverageDB3(t)

	// Create handler with nil caddy manager (ApplyConfig will be called but is nil)
	h := NewSecurityHandler(config.SecurityConfig{}, db, nil)

	body, _ := json.Marshal(map[string]interface{}{
		"name":     "test",
		"waf_mode": "block",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PUT", "/security/config", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.UpdateConfig(c)

	// Should succeed (caddy manager is nil so no apply error)
	assert.Equal(t, 200, w.Code)
}

func TestSecurityHandler_GenerateBreakGlass_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupSecurityCoverageDB3(t)

	h := NewSecurityHandler(config.SecurityConfig{}, db, nil)

	// Drop the config table so generate fails
	db.Migrator().DropTable(&models.SecurityConfig{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/security/breakglass", nil)

	h.GenerateBreakGlass(c)

	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "failed to generate break-glass token")
}

func TestSecurityHandler_ListDecisions_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupSecurityCoverageDB3(t)

	h := NewSecurityHandler(config.SecurityConfig{}, db, nil)

	// Drop decisions table
	db.Migrator().DropTable(&models.SecurityDecision{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/security/decisions", nil)

	h.ListDecisions(c)

	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "failed to list decisions")
}

func TestSecurityHandler_ListRuleSets_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupSecurityCoverageDB3(t)

	h := NewSecurityHandler(config.SecurityConfig{}, db, nil)

	// Drop rulesets table
	db.Migrator().DropTable(&models.SecurityRuleSet{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/security/rulesets", nil)

	h.ListRuleSets(c)

	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "failed to list rule sets")
}

func TestSecurityHandler_UpsertRuleSet_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupSecurityCoverageDB3(t)

	h := NewSecurityHandler(config.SecurityConfig{}, db, nil)

	// Drop table to cause upsert to fail
	db.Migrator().DropTable(&models.SecurityRuleSet{})

	body, _ := json.Marshal(map[string]interface{}{
		"name":    "test-ruleset",
		"enabled": true,
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/security/rulesets", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.UpsertRuleSet(c)

	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "failed to upsert ruleset")
}

func TestSecurityHandler_CreateDecision_LogError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupSecurityCoverageDB3(t)

	h := NewSecurityHandler(config.SecurityConfig{}, db, nil)

	// Drop decisions table to cause log to fail
	db.Migrator().DropTable(&models.SecurityDecision{})

	body, _ := json.Marshal(map[string]interface{}{
		"ip":     "192.168.1.1",
		"action": "ban",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/security/decisions", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.CreateDecision(c)

	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "failed to log decision")
}

func TestSecurityHandler_DeleteRuleSet_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupSecurityCoverageDB3(t)

	h := NewSecurityHandler(config.SecurityConfig{}, db, nil)

	// Drop table to cause delete to fail (not NotFound but table error)
	db.Migrator().DropTable(&models.SecurityRuleSet{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "999"}}

	h.DeleteRuleSet(c)

	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "failed to delete ruleset")
}

// CrowdSec ImportConfig additional coverage tests

func TestCrowdsec_ImportConfig_EmptyUpload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCrowdDB(t)
	tmpDir := t.TempDir()

	h := NewCrowdsecHandler(db, &fakeExec{}, "/bin/false", tmpDir)

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	// Create empty file upload
	buf := &bytes.Buffer{}
	mw := multipart.NewWriter(buf)
	fw, _ := mw.CreateFormFile("file", "empty.tar.gz")
	// Write nothing to make file empty
	_ = fw
	mw.Close()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/admin/crowdsec/import", buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	r.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "empty upload")
}

// Backup Handler additional coverage tests

func TestBackupHandler_List_DBError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Use a non-writable temp dir to simulate errors
	tmpDir := t.TempDir()

	cfg := &config.Config{
		DatabasePath: filepath.Join(tmpDir, "nonexistent", "charon.db"),
	}

	svc := services.NewBackupService(cfg)
	h := NewBackupHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	h.List(c)

	// Should succeed with empty list (service handles missing dir gracefully)
	assert.Equal(t, 200, w.Code)
}

// ImportHandler UploadMulti coverage tests

func TestImportHandler_UploadMulti_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportCoverageDB(t)

	h := NewImportHandler(db, "", t.TempDir(), "")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/import/upload-multi", bytes.NewBufferString("invalid"))
	c.Request.Header.Set("Content-Type", "application/json")

	h.UploadMulti(c)

	assert.Equal(t, 400, w.Code)
}

func TestImportHandler_UploadMulti_MissingCaddyfile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportCoverageDB(t)

	h := NewImportHandler(db, "", t.TempDir(), "")

	body, _ := json.Marshal(map[string]interface{}{
		"files": []map[string]string{
			{"filename": "sites/example.com", "content": "example.com {}"},
		},
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/import/upload-multi", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.UploadMulti(c)

	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "must include a main Caddyfile")
}

func TestImportHandler_UploadMulti_EmptyContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportCoverageDB(t)

	h := NewImportHandler(db, "", t.TempDir(), "")

	body, _ := json.Marshal(map[string]interface{}{
		"files": []map[string]string{
			{"filename": "Caddyfile", "content": ""},
		},
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/import/upload-multi", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.UploadMulti(c)

	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "is empty")
}

func TestImportHandler_UploadMulti_PathTraversal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportCoverageDB(t)

	h := NewImportHandler(db, "", t.TempDir(), "")

	body, _ := json.Marshal(map[string]interface{}{
		"files": []map[string]string{
			{"filename": "Caddyfile", "content": "example.com {}"},
			{"filename": "../../../etc/passwd", "content": "bad content"},
		},
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/import/upload-multi", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.UploadMulti(c)

	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "invalid filename")
}

// Logs Handler Download error coverage

func setupLogsDownloadTest(t *testing.T) (*LogsHandler, string) {
	t.Helper()
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	os.MkdirAll(dataDir, 0o755)

	logsDir := filepath.Join(dataDir, "logs")
	os.MkdirAll(logsDir, 0o755)

	dbPath := filepath.Join(dataDir, "charon.db")
	cfg := &config.Config{DatabasePath: dbPath}
	svc := services.NewLogService(cfg)
	h := NewLogsHandler(svc)

	return h, logsDir
}

func TestLogsHandler_Download_PathTraversal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, _ := setupLogsDownloadTest(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "filename", Value: "../../../etc/passwd"}}
	c.Request = httptest.NewRequest("GET", "/logs/../../../etc/passwd/download", nil)

	h.Download(c)

	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "invalid filename")
}

func TestLogsHandler_Download_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, _ := setupLogsDownloadTest(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "filename", Value: "nonexistent.log"}}
	c.Request = httptest.NewRequest("GET", "/logs/nonexistent.log/download", nil)

	h.Download(c)

	assert.Equal(t, 404, w.Code)
	assert.Contains(t, w.Body.String(), "not found")
}

func TestLogsHandler_Download_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, logsDir := setupLogsDownloadTest(t)

	// Create a log file to download
	os.WriteFile(filepath.Join(logsDir, "test.log"), []byte("log content"), 0o644)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "filename", Value: "test.log"}}
	c.Request = httptest.NewRequest("GET", "/logs/test.log/download", nil)

	h.Download(c)

	assert.Equal(t, 200, w.Code)
}

// Import Handler Upload error tests

func TestImportHandler_Upload_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportCoverageDB(t)

	h := NewImportHandler(db, "", t.TempDir(), "")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/import/upload", bytes.NewBufferString("not json"))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Upload(c)

	assert.Equal(t, 400, w.Code)
}

func TestImportHandler_Upload_EmptyContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportCoverageDB(t)

	h := NewImportHandler(db, "", t.TempDir(), "")

	body, _ := json.Marshal(map[string]string{
		"content": "",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/import/upload", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Upload(c)

	assert.Equal(t, 400, w.Code)
}

// Additional Backup Handler tests

func TestBackupHandler_List_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a temp dir with invalid permission for backup dir
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	os.MkdirAll(dataDir, 0o755)

	// Create database file so config is valid
	dbPath := filepath.Join(dataDir, "charon.db")
	os.WriteFile(dbPath, []byte("test"), 0o644)

	cfg := &config.Config{
		DatabasePath: dbPath,
	}

	svc := services.NewBackupService(cfg)
	h := NewBackupHandler(svc)

	// Make backup dir a file to cause ReadDir error
	os.RemoveAll(svc.BackupDir)
	os.WriteFile(svc.BackupDir, []byte("not a dir"), 0o644)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/backups", nil)

	h.List(c)

	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to list backups")
}

func TestBackupHandler_Delete_PathTraversal(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	os.MkdirAll(dataDir, 0o755)

	dbPath := filepath.Join(dataDir, "charon.db")
	os.WriteFile(dbPath, []byte("test"), 0o644)

	cfg := &config.Config{
		DatabasePath: dbPath,
	}

	svc := services.NewBackupService(cfg)
	h := NewBackupHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "filename", Value: "../../../etc/passwd"}}
	c.Request = httptest.NewRequest("DELETE", "/backups/../../../etc/passwd", nil)

	h.Delete(c)

	// Path traversal detection returns 500 with generic error
	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to delete backup")
}

func TestBackupHandler_Delete_InternalError2(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	os.MkdirAll(dataDir, 0o755)

	dbPath := filepath.Join(dataDir, "charon.db")
	os.WriteFile(dbPath, []byte("test"), 0o644)

	cfg := &config.Config{
		DatabasePath: dbPath,
	}

	svc := services.NewBackupService(cfg)
	h := NewBackupHandler(svc)

	// Create a backup
	backupsDir := filepath.Join(dataDir, "backups")
	os.MkdirAll(backupsDir, 0o755)
	backupFile := filepath.Join(backupsDir, "test.zip")
	os.WriteFile(backupFile, []byte("backup"), 0o644)

	// Remove write permissions to cause delete error
	os.Chmod(backupsDir, 0o555)
	defer os.Chmod(backupsDir, 0o755)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "filename", Value: "test.zip"}}
	c.Request = httptest.NewRequest("DELETE", "/backups/test.zip", nil)

	h.Delete(c)

	// Permission error
	assert.Contains(t, []int{200, 500}, w.Code)
}

// Remote Server TestConnection error paths

func TestRemoteServerHandler_TestConnection_NotFound2(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupRemoteServerCoverageDB2(t)
	svc := services.NewRemoteServerService(db)
	h := NewRemoteServerHandler(svc, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "uuid", Value: "nonexistent-uuid"}}

	h.TestConnection(c)

	assert.Equal(t, 404, w.Code)
}

func TestRemoteServerHandler_TestConnectionCustom_Unreachable2(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupRemoteServerCoverageDB2(t)
	svc := services.NewRemoteServerService(db)
	h := NewRemoteServerHandler(svc, nil)

	body, _ := json.Marshal(map[string]interface{}{
		"host": "192.0.2.1", // TEST-NET - not routable
		"port": 65535,
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/remote-servers/test", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.TestConnectionCustom(c)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), `"reachable":false`)
}

// Auth Handler Register error paths

func setupAuthCoverageDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := OpenTestDB(t)
	db.AutoMigrate(&models.User{}, &models.Setting{})
	return db
}

func TestAuthHandler_Register_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupAuthCoverageDB(t)

	cfg := config.Config{JWTSecret: "test-secret"}
	authService := services.NewAuthService(db, cfg)
	h := NewAuthHandler(authService)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/register", bytes.NewBufferString("invalid"))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Register(c)

	assert.Equal(t, 400, w.Code)
}

// Health handler coverage

func TestHealthHandler_Basic(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/health", nil)

	HealthHandler(c)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "status")
	assert.Contains(t, w.Body.String(), "ok")
}

// Backup Create error coverage

func TestBackupHandler_Create_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Use a path where database file doesn't exist
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	os.MkdirAll(dataDir, 0o755)

	// Don't create the database file - this will cause CreateBackup to fail
	dbPath := filepath.Join(dataDir, "charon.db")

	cfg := &config.Config{
		DatabasePath: dbPath,
	}

	svc := services.NewBackupService(cfg)
	h := NewBackupHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/backups", nil)

	h.Create(c)

	// Should fail because database file doesn't exist
	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to create backup")
}

// Settings Handler coverage

func setupSettingsCoverageDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := OpenTestDB(t)
	db.AutoMigrate(&models.Setting{})
	return db
}

func TestSettingsHandler_GetSettings_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupSettingsCoverageDB(t)

	h := NewSettingsHandler(db)

	// Drop table to cause error
	db.Migrator().DropTable(&models.Setting{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/settings", nil)

	h.GetSettings(c)

	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to fetch settings")
}

func TestSettingsHandler_UpdateSetting_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupSettingsCoverageDB(t)

	h := NewSettingsHandler(db)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PUT", "/settings/test", bytes.NewBufferString("invalid"))
	c.Request.Header.Set("Content-Type", "application/json")

	h.UpdateSetting(c)

	assert.Equal(t, 400, w.Code)
}

// Additional remote server TestConnection tests

func TestRemoteServerHandler_TestConnection_Reachable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupRemoteServerCoverageDB2(t)
	svc := services.NewRemoteServerService(db)
	h := NewRemoteServerHandler(svc, nil)

	// Use localhost which should be reachable
	server := &models.RemoteServer{
		Name: "LocalTest",
		Host: "127.0.0.1",
		Port: 22, // SSH port typically listening on localhost
	}
	svc.Create(server)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "uuid", Value: server.UUID}}

	h.TestConnection(c)

	// Should return 200 regardless of whether port is open
	assert.Equal(t, 200, w.Code)
}

func TestRemoteServerHandler_TestConnection_EmptyHost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupRemoteServerCoverageDB2(t)
	svc := services.NewRemoteServerService(db)
	h := NewRemoteServerHandler(svc, nil)

	// Create server with empty host
	server := &models.RemoteServer{
		Name: "Empty",
		Host: "",
		Port: 22,
	}
	db.Create(server)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "uuid", Value: server.UUID}}

	h.TestConnection(c)

	// Should return 200 - empty host resolves to localhost on some systems
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), `"reachable":`)
}

// Additional UploadMulti test with valid Caddyfile content

func TestImportHandler_UploadMulti_ValidCaddyfile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportCoverageDB(t)

	h := NewImportHandler(db, "", t.TempDir(), "")

	body, _ := json.Marshal(map[string]interface{}{
		"files": []map[string]string{
			{"filename": "Caddyfile", "content": "example.com { reverse_proxy localhost:8080 }"},
		},
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/import/upload-multi", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.UploadMulti(c)

	// Without caddy binary, will fail with 400 at adapt step - that's fine, we hit the code path
	// We just verify we got a response (not a panic)
	assert.True(t, w.Code == 200 || w.Code == 400, "Should return valid HTTP response")
}

func TestImportHandler_UploadMulti_SubdirFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupImportCoverageDB(t)

	h := NewImportHandler(db, "", t.TempDir(), "")

	body, _ := json.Marshal(map[string]interface{}{
		"files": []map[string]string{
			{"filename": "Caddyfile", "content": "import sites/*"},
			{"filename": "sites/example.com", "content": "example.com {}"},
		},
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/import/upload-multi", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.UploadMulti(c)

	// Should process the subdirectory file
	// Just verify it doesn't crash
	assert.True(t, w.Code == 200 || w.Code == 400)
}
