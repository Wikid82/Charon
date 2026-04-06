package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/models"
)

type importCoverageProxyHostSvcStub struct{}

func (importCoverageProxyHostSvcStub) Create(host *models.ProxyHost) error { return nil }
func (importCoverageProxyHostSvcStub) Update(host *models.ProxyHost) error { return nil }
func (importCoverageProxyHostSvcStub) List() ([]models.ProxyHost, error) {
	return []models.ProxyHost{}, nil
}

func setupReadOnlyImportDB(t *testing.T) *gorm.DB {
	t.Helper()

	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "import_ro.db")

	rwDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, rwDB.AutoMigrate(&models.ImportSession{}))
	sqlDB, err := rwDB.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	require.NoError(t, os.Chmod(dbPath, 0o400))

	roDB, err := gorm.Open(sqlite.Open("file:"+dbPath+"?mode=ro"), &gorm.Config{})
	require.NoError(t, err)

	t.Cleanup(func() {
		if roSQLDB, dbErr := roDB.DB(); dbErr == nil {
			_ = roSQLDB.Close()
		}
	})

	return roDB
}

func setupImportCoverageTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}
	return db
}

// MockImporterService implements handlers.ImporterService
type MockImporterService struct {
	mock.Mock
}

func (m *MockImporterService) NormalizeCaddyfile(content string) (string, error) {
	args := m.Called(content)
	return args.String(0), args.Error(1)
}

func (m *MockImporterService) ParseCaddyfile(path string) ([]byte, error) {
	args := m.Called(path)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockImporterService) ImportFile(path string) (*caddy.ImportResult, error) {
	args := m.Called(path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*caddy.ImportResult), args.Error(1)
}

func (m *MockImporterService) ExtractHosts(caddyJSON []byte) (*caddy.ImportResult, error) {
	args := m.Called(caddyJSON)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*caddy.ImportResult), args.Error(1)
}

func (m *MockImporterService) ValidateCaddyBinary() error {
	args := m.Called()
	return args.Error(0)
}

// TestUploadMulti_EmptyList covers the manual check for len(Files) == 0
func TestUploadMulti_EmptyList(t *testing.T) {

	db := setupImportCoverageTestDB(t)

	mockSvc := new(MockImporterService)
	h := NewImportHandler(db, "caddy", "/tmp", "/tmp")
	h.importerservice = mockSvc

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)
	r.Use(func(c *gin.Context) {
		setAdminContext(c)
		c.Next()
	})
	r.POST("/upload-multi", h.UploadMulti)

	// Create JSON with empty files list
	req := map[string]interface{}{
		"files": []interface{}{},
	}
	body, _ := json.Marshal(req)

	request, _ := http.NewRequest("POST", "/upload-multi", bytes.NewBuffer(body))
	request.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, request)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	// Matched Gin validation error
	assert.Contains(t, w.Body.String(), "Error:Field validation for 'Files' failed on the 'min' tag")
}

// TestUploadMulti_FileServerDetected covers the logic where parsable routes trigger a warning
// because they contain file_server but no valid reverse_proxy hosts
func TestUploadMulti_FileServerDetected(t *testing.T) {

	db := setupImportCoverageTestDB(t)
	mockSvc := new(MockImporterService)

	// Return a result that has empty Forward host/port (not importable)
	// AND contains a "file_server" warning
	mockResult := &caddy.ImportResult{
		Hosts: []caddy.ParsedHost{
			{
				DomainNames: "files.example.com",
				Warnings:    []string{"directive 'file_server' detected"},
			},
		},
	}
	mockSvc.On("ImportFile", mock.AnythingOfType("string")).Return(mockResult, nil)

	h := NewImportHandler(db, "caddy", "/tmp", "/tmp")
	h.importerservice = mockSvc
	// Override import dir to temp
	h.importDir = t.TempDir()

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)
	r.Use(func(c *gin.Context) {
		setAdminContext(c)
		c.Next()
	})
	r.POST("/upload-multi", h.UploadMulti)

	req := map[string]interface{}{
		"files": []interface{}{
			map[string]string{
				"filename": "Caddyfile",
				"content":  "files.example.com { file_server }",
			},
		},
	}
	body, _ := json.Marshal(req)

	request, _ := http.NewRequest("POST", "/upload-multi", bytes.NewBuffer(body))
	request.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, request)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "File server directives are not supported")
}

// TestUploadMulti_NoSitesParsed covers successfull parsing but 0 result hosts
func TestUploadMulti_NoSitesParsed(t *testing.T) {

	db := setupImportCoverageTestDB(t)
	mockSvc := new(MockImporterService)

	// Return empty result
	mockResult := &caddy.ImportResult{
		Hosts: []caddy.ParsedHost{},
	}
	mockSvc.On("ImportFile", mock.AnythingOfType("string")).Return(mockResult, nil)

	h := NewImportHandler(db, "caddy", "/tmp", "/tmp")
	h.importerservice = mockSvc
	h.importDir = t.TempDir()

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)
	r.Use(func(c *gin.Context) {
		setAdminContext(c)
		c.Next()
	})
	r.POST("/upload-multi", h.UploadMulti)

	req := map[string]interface{}{
		"files": []interface{}{
			map[string]string{
				"filename": "Caddyfile",
				"content":  "# just a comment",
			},
		},
	}
	body, _ := json.Marshal(req)

	request, _ := http.NewRequest("POST", "/upload-multi", bytes.NewBuffer(body))
	request.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, request)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "no sites parsed")
}

func TestUpload_ImportsDetectedNoImportableHosts(t *testing.T) {

	db := setupImportCoverageTestDB(t)
	mockSvc := new(MockImporterService)
	mockSvc.On("NormalizeCaddyfile", mock.AnythingOfType("string")).Return("import sites/*.caddy # include\n", nil)
	mockSvc.On("ImportFile", mock.AnythingOfType("string")).Return(&caddy.ImportResult{
		Hosts: []caddy.ParsedHost{},
	}, nil)

	tmpImport := t.TempDir()
	h := NewImportHandler(db, "caddy", tmpImport, "")
	h.importerservice = mockSvc

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)
	r.Use(func(c *gin.Context) {
		setAdminContext(c)
		c.Next()
	})
	r.POST("/upload", h.Upload)

	req := map[string]interface{}{
		"filename": "Caddyfile",
		"content":  "import sites/*.caddy # include\n",
	}
	body, _ := json.Marshal(req)
	request, _ := http.NewRequest("POST", "/upload", bytes.NewBuffer(body))
	request.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, request)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "imports")
	mockSvc.AssertExpectations(t)
}

func TestUploadMulti_RequiresMainCaddyfile(t *testing.T) {

	db := setupImportCoverageTestDB(t)
	h := NewImportHandler(db, "caddy", t.TempDir(), "")

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)
	r.Use(func(c *gin.Context) {
		setAdminContext(c)
		c.Next()
	})
	r.POST("/upload-multi", h.UploadMulti)

	req := map[string]interface{}{
		"files": []interface{}{
			map[string]string{"filename": "sites/site1.caddy", "content": "example.com { reverse_proxy localhost:8080 }"},
		},
	}
	body, _ := json.Marshal(req)
	request, _ := http.NewRequest("POST", "/upload-multi", bytes.NewBuffer(body))
	request.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, request)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "must include a main Caddyfile")
}

func TestUploadMulti_RejectsEmptyFileContent(t *testing.T) {

	db := setupImportCoverageTestDB(t)
	h := NewImportHandler(db, "caddy", t.TempDir(), "")

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)
	r.Use(func(c *gin.Context) {
		setAdminContext(c)
		c.Next()
	})
	r.POST("/upload-multi", h.UploadMulti)

	req := map[string]interface{}{
		"files": []interface{}{
			map[string]string{"filename": "Caddyfile", "content": "   "},
		},
	}
	body, _ := json.Marshal(req)
	request, _ := http.NewRequest("POST", "/upload-multi", bytes.NewBuffer(body))
	request.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, request)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "is empty")
}

func TestCommitAndCancel_InvalidSessionUUID(t *testing.T) {

	db := setupImportCoverageTestDB(t)
	tmpImport := t.TempDir()
	h := NewImportHandler(db, "caddy", tmpImport, "")

	r := gin.New()
	r.Use(func(c *gin.Context) {
		setAdminContext(c)
		c.Next()
	})
	h.RegisterRoutes(r.Group("/api/v1"))

	commitBody := map[string]interface{}{"session_uuid": ".", "resolutions": map[string]string{}}
	commitBytes, _ := json.Marshal(commitBody)
	wCommit := httptest.NewRecorder()
	reqCommit, _ := http.NewRequest(http.MethodPost, "/api/v1/import/commit", bytes.NewBuffer(commitBytes))
	reqCommit.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wCommit, reqCommit)
	assert.Equal(t, http.StatusBadRequest, wCommit.Code)

	wCancelMissing := httptest.NewRecorder()
	reqCancelMissing, _ := http.NewRequest(http.MethodDelete, "/api/v1/import/cancel", http.NoBody)
	r.ServeHTTP(wCancelMissing, reqCancelMissing)
	assert.Equal(t, http.StatusBadRequest, wCancelMissing.Code)

	wCancel := httptest.NewRecorder()
	reqCancel, _ := http.NewRequest(http.MethodDelete, "/api/v1/import/cancel?session_uuid=.", http.NoBody)
	r.ServeHTTP(wCancel, reqCancel)
	assert.Equal(t, http.StatusBadRequest, wCancel.Code)
}

func TestCancel_RemovesTransientUpload(t *testing.T) {

	db := setupImportCoverageTestDB(t)
	tmpImport := t.TempDir()
	h := NewImportHandler(db, "caddy", tmpImport, "")

	uploadsDir := filepath.Join(tmpImport, "uploads")
	require.NoError(t, os.MkdirAll(uploadsDir, 0o750))
	sid := "test-sid"
	uploadPath := filepath.Join(uploadsDir, sid+".caddyfile")
	require.NoError(t, os.WriteFile(uploadPath, []byte("example.com { reverse_proxy localhost:8080 }"), 0o600))

	r := gin.New()
	r.Use(func(c *gin.Context) {
		setAdminContext(c)
		c.Next()
	})
	h.RegisterRoutes(r.Group("/api/v1"))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/import/cancel?session_uuid="+sid, http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	_, statErr := os.Stat(uploadPath)
	assert.True(t, os.IsNotExist(statErr))
}

func TestUpload_ReadOnlyDBRespondsWithPermissionError(t *testing.T) {

	roDB := setupReadOnlyImportDB(t)
	mockSvc := new(MockImporterService)
	mockSvc.On("NormalizeCaddyfile", mock.AnythingOfType("string")).Return("example.com { reverse_proxy localhost:8080 }", nil)
	mockSvc.On("ImportFile", mock.AnythingOfType("string")).Return(&caddy.ImportResult{
		Hosts: []caddy.ParsedHost{{DomainNames: "example.com", ForwardHost: "localhost", ForwardPort: 8080}},
	}, nil)

	h := NewImportHandler(roDB, "caddy", t.TempDir(), "")
	h.importerservice = mockSvc

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)
	r.Use(func(c *gin.Context) {
		setAdminContext(c)
		c.Next()
	})
	r.POST("/upload", h.Upload)

	body, _ := json.Marshal(map[string]any{
		"filename": "Caddyfile",
		"content":  "example.com { reverse_proxy localhost:8080 }",
	})
	req, _ := http.NewRequest(http.MethodPost, "/upload", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "permissions_db_readonly")
}

func TestUploadMulti_ReadOnlyDBRespondsWithPermissionError(t *testing.T) {

	roDB := setupReadOnlyImportDB(t)
	mockSvc := new(MockImporterService)
	mockSvc.On("ImportFile", mock.AnythingOfType("string")).Return(&caddy.ImportResult{
		Hosts: []caddy.ParsedHost{{DomainNames: "multi.example.com", ForwardHost: "localhost", ForwardPort: 8081}},
	}, nil)

	h := NewImportHandler(roDB, "caddy", t.TempDir(), "")
	h.importerservice = mockSvc

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)
	r.Use(func(c *gin.Context) {
		setAdminContext(c)
		c.Next()
	})
	r.POST("/upload-multi", h.UploadMulti)

	body, _ := json.Marshal(map[string]any{
		"files": []map[string]string{{
			"filename": "Caddyfile",
			"content":  "multi.example.com { reverse_proxy localhost:8081 }",
		}},
	})
	req, _ := http.NewRequest(http.MethodPost, "/upload-multi", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "permissions_db_readonly")
}

func TestCommit_ReadOnlyDBSaveRespondsWithPermissionError(t *testing.T) {

	roDB := setupReadOnlyImportDB(t)
	mockSvc := new(MockImporterService)
	mockSvc.On("ImportFile", mock.AnythingOfType("string")).Return(&caddy.ImportResult{
		Hosts: []caddy.ParsedHost{{DomainNames: "commit.example.com", ForwardHost: "localhost", ForwardPort: 8080}},
	}, nil)

	importDir := t.TempDir()
	uploadsDir := filepath.Join(importDir, "uploads")
	require.NoError(t, os.MkdirAll(uploadsDir, 0o750))
	sid := "readonly-commit-session"
	require.NoError(t, os.WriteFile(filepath.Join(uploadsDir, sid+".caddyfile"), []byte("commit.example.com { reverse_proxy localhost:8080 }"), 0o600))

	h := NewImportHandlerWithService(roDB, importCoverageProxyHostSvcStub{}, "caddy", importDir, "", nil)
	h.importerservice = mockSvc

	r := gin.New()
	r.Use(func(c *gin.Context) {
		setAdminContext(c)
		c.Next()
	})
	r.POST("/commit", h.Commit)

	body, _ := json.Marshal(map[string]any{"session_uuid": sid, "resolutions": map[string]string{}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/commit", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "permissions_db_readonly")
}

func TestCancel_ReadOnlyDBSaveRespondsWithPermissionError(t *testing.T) {

	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "cancel_ro.db")

	rwDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, rwDB.AutoMigrate(&models.ImportSession{}))
	require.NoError(t, rwDB.Create(&models.ImportSession{UUID: "readonly-cancel", Status: "pending"}).Error)
	rwSQLDB, err := rwDB.DB()
	require.NoError(t, err)
	require.NoError(t, rwSQLDB.Close())
	require.NoError(t, os.Chmod(dbPath, 0o400))

	roDB, err := gorm.Open(sqlite.Open("file:"+dbPath+"?mode=ro"), &gorm.Config{})
	require.NoError(t, err)
	if roSQLDB, dbErr := roDB.DB(); dbErr == nil {
		t.Cleanup(func() { _ = roSQLDB.Close() })
	}

	h := NewImportHandler(roDB, "caddy", t.TempDir(), "")

	r := gin.New()
	r.Use(func(c *gin.Context) {
		setAdminContext(c)
		c.Next()
	})
	r.DELETE("/cancel", h.Cancel)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/cancel?session_uuid=readonly-cancel", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "permissions_db_readonly")
}
