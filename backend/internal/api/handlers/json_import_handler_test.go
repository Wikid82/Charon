package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

func setupJSONTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{})
	require.NoError(t, err)

	return db
}

func TestNewJSONImportHandler(t *testing.T) {
	db := setupJSONTestDB(t)
	handler := NewJSONImportHandler(db)

	assert.NotNil(t, handler)
	assert.NotNil(t, handler.db)
	assert.NotNil(t, handler.proxyHostSvc)
}

func TestJSONImportHandler_RegisterRoutes(t *testing.T) {
	db := setupJSONTestDB(t)
	handler := NewJSONImportHandler(db)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	routes := router.Routes()
	routePaths := make(map[string]bool)
	for _, r := range routes {
		routePaths[r.Method+":"+r.Path] = true
	}

	assert.True(t, routePaths["POST:/api/v1/import/json/upload"])
	assert.True(t, routePaths["POST:/api/v1/import/json/commit"])
	assert.True(t, routePaths["POST:/api/v1/import/json/cancel"])
}

func TestJSONImportHandler_Upload_CharonFormat(t *testing.T) {
	db := setupJSONTestDB(t)
	handler := NewJSONImportHandler(db)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	charonExport := CharonExport{
		Version:    "1.0.0",
		ExportedAt: time.Now(),
		ProxyHosts: []CharonProxyHost{
			{
				UUID:             "test-uuid-1",
				Name:             "Test Host",
				DomainNames:      "example.com",
				ForwardScheme:    "http",
				ForwardHost:      "192.168.1.100",
				ForwardPort:      8080,
				SSLForced:        true,
				WebsocketSupport: true,
				Enabled:          true,
			},
		},
		AccessLists: []CharonAccessList{
			{
				UUID:    "acl-uuid-1",
				Name:    "Test ACL",
				Type:    "whitelist",
				Enabled: true,
			},
		},
	}

	content, _ := json.Marshal(charonExport)
	body, _ := json.Marshal(map[string]string{"content": string(content)})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/import/json/upload", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response, "session")
	session := response["session"].(map[string]any)
	assert.Equal(t, "charon", session["source_type"])

	assert.Contains(t, response, "charon_export")
	charonInfo := response["charon_export"].(map[string]any)
	assert.Equal(t, "1.0.0", charonInfo["version"])
}

func TestJSONImportHandler_Upload_NPMFormatFallback(t *testing.T) {
	db := setupJSONTestDB(t)
	handler := NewJSONImportHandler(db)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	npmExport := NPMExport{
		ProxyHosts: []NPMProxyHost{
			{
				ID:            1,
				DomainNames:   []string{"npm-example.com"},
				ForwardScheme: "http",
				ForwardHost:   "192.168.1.100",
				ForwardPort:   8080,
				Enabled:       true,
			},
		},
	}

	content, _ := json.Marshal(npmExport)
	body, _ := json.Marshal(map[string]string{"content": string(content)})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/import/json/upload", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	session := response["session"].(map[string]any)
	assert.Equal(t, "npm", session["source_type"])

	assert.Contains(t, response, "npm_export")
}

func TestJSONImportHandler_Upload_UnrecognizedFormat(t *testing.T) {
	db := setupJSONTestDB(t)
	handler := NewJSONImportHandler(db)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	unknownFormat := map[string]any{
		"some_field": "some_value",
		"other":      123,
	}

	content, _ := json.Marshal(unknownFormat)
	body, _ := json.Marshal(map[string]string{"content": string(content)})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/import/json/upload", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestJSONImportHandler_Upload_InvalidJSON(t *testing.T) {
	db := setupJSONTestDB(t)
	handler := NewJSONImportHandler(db)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	body, _ := json.Marshal(map[string]string{"content": "{invalid json"})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/import/json/upload", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestJSONImportHandler_Commit_CharonFormat(t *testing.T) {
	db := setupJSONTestDB(t)
	handler := NewJSONImportHandler(db)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	charonExport := CharonExport{
		Version:    "1.0.0",
		ExportedAt: time.Now(),
		ProxyHosts: []CharonProxyHost{
			{
				UUID:          "test-uuid-1",
				Name:          "Test Host",
				DomainNames:   "newcharon.com",
				ForwardScheme: "http",
				ForwardHost:   "192.168.1.100",
				ForwardPort:   8080,
				Enabled:       true,
			},
		},
	}

	// Step 1: Upload to get session ID
	content, _ := json.Marshal(charonExport)
	uploadBody, _ := json.Marshal(map[string]string{"content": string(content)})

	uploadReq := httptest.NewRequest(http.MethodPost, "/api/v1/import/json/upload", bytes.NewReader(uploadBody))
	uploadReq.Header.Set("Content-Type", "application/json")
	uploadW := httptest.NewRecorder()

	router.ServeHTTP(uploadW, uploadReq)
	require.Equal(t, http.StatusOK, uploadW.Code)

	var uploadResponse map[string]any
	err := json.Unmarshal(uploadW.Body.Bytes(), &uploadResponse)
	require.NoError(t, err)

	session := uploadResponse["session"].(map[string]any)
	sessionID := session["id"].(string)

	// Step 2: Commit with session UUID
	commitBody, _ := json.Marshal(map[string]any{
		"session_uuid": sessionID,
		"resolutions":  map[string]string{},
		"names":        map[string]string{"newcharon.com": "Custom Name"},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/import/json/commit", bytes.NewReader(commitBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]any
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(1), response["created"])

	var host models.ProxyHost
	db.Where("domain_names = ?", "newcharon.com").First(&host)
	assert.Equal(t, "Custom Name", host.Name)
}

func TestJSONImportHandler_Commit_NPMFormatFallback(t *testing.T) {
	db := setupJSONTestDB(t)
	handler := NewJSONImportHandler(db)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	npmExport := NPMExport{
		ProxyHosts: []NPMProxyHost{
			{
				ID:            1,
				DomainNames:   []string{"newnpm.com"},
				ForwardScheme: "http",
				ForwardHost:   "192.168.1.100",
				ForwardPort:   8080,
				Enabled:       true,
			},
		},
	}

	// Step 1: Upload to get session ID
	content, _ := json.Marshal(npmExport)
	uploadBody, _ := json.Marshal(map[string]string{"content": string(content)})

	uploadReq := httptest.NewRequest(http.MethodPost, "/api/v1/import/json/upload", bytes.NewReader(uploadBody))
	uploadReq.Header.Set("Content-Type", "application/json")
	uploadW := httptest.NewRecorder()

	router.ServeHTTP(uploadW, uploadReq)
	require.Equal(t, http.StatusOK, uploadW.Code)

	var uploadResponse map[string]any
	err := json.Unmarshal(uploadW.Body.Bytes(), &uploadResponse)
	require.NoError(t, err)

	session := uploadResponse["session"].(map[string]any)
	sessionID := session["id"].(string)

	// Step 2: Commit with session UUID
	commitBody, _ := json.Marshal(map[string]any{
		"session_uuid": sessionID,
		"resolutions":  map[string]string{},
		"names":        map[string]string{},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/import/json/commit", bytes.NewReader(commitBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]any
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(1), response["created"])
}

func TestJSONImportHandler_Commit_SessionNotFound(t *testing.T) {
	db := setupJSONTestDB(t)
	handler := NewJSONImportHandler(db)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	// Try to commit with a non-existent session
	commitBody, _ := json.Marshal(map[string]any{
		"session_uuid": "non-existent-uuid",
		"resolutions":  map[string]string{},
		"names":        map[string]string{},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/import/json/commit", bytes.NewReader(commitBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response["error"], "session not found")
}

func TestJSONImportHandler_Cancel(t *testing.T) {
	db := setupJSONTestDB(t)
	handler := NewJSONImportHandler(db)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	charonExport := CharonExport{
		Version:    "1.0.0",
		ExportedAt: time.Now(),
		ProxyHosts: []CharonProxyHost{
			{
				UUID:          "cancel-test-uuid",
				Name:          "Cancel Test",
				DomainNames:   "cancel-test.com",
				ForwardScheme: "http",
				ForwardHost:   "192.168.1.100",
				ForwardPort:   8080,
				Enabled:       true,
			},
		},
	}

	// Step 1: Upload to get session ID
	content, _ := json.Marshal(charonExport)
	uploadBody, _ := json.Marshal(map[string]string{"content": string(content)})

	uploadReq := httptest.NewRequest(http.MethodPost, "/api/v1/import/json/upload", bytes.NewReader(uploadBody))
	uploadReq.Header.Set("Content-Type", "application/json")
	uploadW := httptest.NewRecorder()

	router.ServeHTTP(uploadW, uploadReq)
	require.Equal(t, http.StatusOK, uploadW.Code)

	var uploadResponse map[string]any
	err := json.Unmarshal(uploadW.Body.Bytes(), &uploadResponse)
	require.NoError(t, err)

	session := uploadResponse["session"].(map[string]any)
	sessionID := session["id"].(string)

	// Step 2: Cancel the session
	cancelBody, _ := json.Marshal(map[string]any{
		"session_uuid": sessionID,
	})

	cancelReq := httptest.NewRequest(http.MethodPost, "/api/v1/import/json/cancel", bytes.NewReader(cancelBody))
	cancelReq.Header.Set("Content-Type", "application/json")
	cancelW := httptest.NewRecorder()

	router.ServeHTTP(cancelW, cancelReq)

	assert.Equal(t, http.StatusOK, cancelW.Code)

	var cancelResponse map[string]any
	err = json.Unmarshal(cancelW.Body.Bytes(), &cancelResponse)
	require.NoError(t, err)

	assert.Equal(t, "cancelled", cancelResponse["status"])

	// Step 3: Try to commit with cancelled session (should fail)
	commitBody, _ := json.Marshal(map[string]any{
		"session_uuid": sessionID,
		"resolutions":  map[string]string{},
		"names":        map[string]string{},
	})

	commitReq := httptest.NewRequest(http.MethodPost, "/api/v1/import/json/commit", bytes.NewReader(commitBody))
	commitReq.Header.Set("Content-Type", "application/json")
	commitW := httptest.NewRecorder()

	router.ServeHTTP(commitW, commitReq)

	assert.Equal(t, http.StatusNotFound, commitW.Code)
}

func TestJSONImportHandler_ConflictDetection(t *testing.T) {
	db := setupJSONTestDB(t)

	existingHost := models.ProxyHost{
		UUID:          "existing-uuid",
		DomainNames:   "conflict.com",
		ForwardScheme: "http",
		ForwardHost:   "old-server",
		ForwardPort:   80,
		Enabled:       true,
	}
	db.Create(&existingHost)

	handler := NewJSONImportHandler(db)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	charonExport := CharonExport{
		Version: "1.0.0",
		ProxyHosts: []CharonProxyHost{
			{
				UUID:          "new-uuid",
				DomainNames:   "conflict.com",
				ForwardScheme: "http",
				ForwardHost:   "new-server",
				ForwardPort:   8080,
				Enabled:       true,
			},
		},
	}

	content, _ := json.Marshal(charonExport)
	body, _ := json.Marshal(map[string]string{"content": string(content)})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/import/json/upload", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	conflictDetails := response["conflict_details"].(map[string]any)
	assert.Contains(t, conflictDetails, "conflict.com")
}

func TestJSONImportHandler_IsCharonFormat(t *testing.T) {
	db := setupJSONTestDB(t)
	handler := NewJSONImportHandler(db)

	tests := []struct {
		name     string
		export   CharonExport
		expected bool
	}{
		{
			name:     "with version",
			export:   CharonExport{Version: "1.0.0"},
			expected: true,
		},
		{
			name: "with proxy hosts",
			export: CharonExport{
				ProxyHosts: []CharonProxyHost{{DomainNames: "test.com"}},
			},
			expected: true,
		},
		{
			name:     "empty export",
			export:   CharonExport{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.isCharonFormat(tt.export)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsValidJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid object", `{"key": "value"}`, true},
		{"valid array", `[1, 2, 3]`, true},
		{"valid string", `"hello"`, true},
		{"valid number", `123`, true},
		{"empty string", "", true},
		{"whitespace only", "   ", true},
		{"invalid json", `{key: "value"}`, false},
		{"incomplete", `{"key":`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidJSON(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJSONImportHandler_ConvertCharonToImportResult(t *testing.T) {
	db := setupJSONTestDB(t)
	handler := NewJSONImportHandler(db)

	charonExport := CharonExport{
		Version:    "1.0.0",
		ExportedAt: time.Now(),
		ProxyHosts: []CharonProxyHost{
			{
				UUID:             "uuid-1",
				Name:             "Host 1",
				DomainNames:      "host1.com",
				ForwardScheme:    "https",
				ForwardHost:      "backend1",
				ForwardPort:      443,
				SSLForced:        true,
				WebsocketSupport: true,
			},
			{
				UUID:          "uuid-2",
				DomainNames:   "",
				ForwardScheme: "http",
				ForwardHost:   "backend2",
				ForwardPort:   80,
			},
		},
	}

	result := handler.convertCharonToImportResult(charonExport)

	assert.Len(t, result.Hosts, 1)
	assert.Len(t, result.Errors, 1)

	host := result.Hosts[0]
	assert.Equal(t, "host1.com", host.DomainNames)
	assert.Equal(t, "https", host.ForwardScheme)
	assert.Equal(t, "backend1", host.ForwardHost)
	assert.Equal(t, 443, host.ForwardPort)
	assert.True(t, host.SSLForced)
	assert.True(t, host.WebsocketSupport)
}
