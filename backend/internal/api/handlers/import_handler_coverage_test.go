package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/caddy"
)

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
	gin.SetMode(gin.TestMode)

	db := setupImportCoverageTestDB(t)

	mockSvc := new(MockImporterService)
	h := NewImportHandler(db, "caddy", "/tmp", "/tmp")
	h.importerservice = mockSvc

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)
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
	gin.SetMode(gin.TestMode)

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
	gin.SetMode(gin.TestMode)

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
