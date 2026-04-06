package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

func setupNPMTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{})
	require.NoError(t, err)

	return db
}

func TestNewNPMImportHandler(t *testing.T) {
	db := setupNPMTestDB(t)
	handler := NewNPMImportHandler(db)

	assert.NotNil(t, handler)
	assert.NotNil(t, handler.db)
	assert.NotNil(t, handler.proxyHostSvc)
}

func TestNPMImportHandler_RegisterRoutes(t *testing.T) {
	db := setupNPMTestDB(t)
	handler := NewNPMImportHandler(db)

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	routes := router.Routes()
	routePaths := make(map[string]bool)
	for _, r := range routes {
		routePaths[r.Method+":"+r.Path] = true
	}

	assert.True(t, routePaths["POST:/api/v1/import/npm/upload"])
	assert.True(t, routePaths["POST:/api/v1/import/npm/commit"])
	assert.True(t, routePaths["POST:/api/v1/import/npm/cancel"])
}

func TestNPMImportHandler_Upload_ValidNPMExport(t *testing.T) {
	db := setupNPMTestDB(t)
	handler := NewNPMImportHandler(db)

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	npmExport := NPMExport{
		ProxyHosts: []NPMProxyHost{
			{
				ID:                    1,
				DomainNames:           []string{"example.com"},
				ForwardScheme:         "http",
				ForwardHost:           "192.168.1.100",
				ForwardPort:           8080,
				SSLForced:             true,
				AllowWebsocketUpgrade: true,
				Enabled:               true,
			},
			{
				ID:            2,
				DomainNames:   []string{"test.com", "www.test.com"},
				ForwardScheme: "https",
				ForwardHost:   "192.168.1.101",
				ForwardPort:   443,
				Enabled:       true,
			},
		},
		AccessLists: []NPMAccessList{
			{
				ID:   1,
				Name: "Test ACL",
			},
		},
	}

	content, _ := json.Marshal(npmExport)
	body, _ := json.Marshal(map[string]string{"content": string(content)})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/import/npm/upload", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response, "session")
	assert.Contains(t, response, "preview")
	assert.Contains(t, response, "npm_export")

	preview := response["preview"].(map[string]any)
	hosts := preview["hosts"].([]any)
	assert.Len(t, hosts, 2)
}

func TestNPMImportHandler_Upload_EmptyExport(t *testing.T) {
	db := setupNPMTestDB(t)
	handler := NewNPMImportHandler(db)

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	npmExport := NPMExport{
		ProxyHosts: []NPMProxyHost{},
	}

	content, _ := json.Marshal(npmExport)
	body, _ := json.Marshal(map[string]string{"content": string(content)})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/import/npm/upload", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestNPMImportHandler_Upload_InvalidJSON(t *testing.T) {
	db := setupNPMTestDB(t)
	handler := NewNPMImportHandler(db)

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	body, _ := json.Marshal(map[string]string{"content": "not valid json"})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/import/npm/upload", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestNPMImportHandler_Upload_ConflictDetection(t *testing.T) {
	db := setupNPMTestDB(t)

	existingHost := models.ProxyHost{
		UUID:          "existing-uuid",
		DomainNames:   "example.com",
		ForwardScheme: "http",
		ForwardHost:   "old-server",
		ForwardPort:   80,
		Enabled:       true,
	}
	db.Create(&existingHost)

	handler := NewNPMImportHandler(db)

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	npmExport := NPMExport{
		ProxyHosts: []NPMProxyHost{
			{
				ID:            1,
				DomainNames:   []string{"example.com"},
				ForwardScheme: "http",
				ForwardHost:   "new-server",
				ForwardPort:   8080,
				Enabled:       true,
			},
		},
	}

	content, _ := json.Marshal(npmExport)
	body, _ := json.Marshal(map[string]string{"content": string(content)})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/import/npm/upload", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response, "conflict_details")
	conflictDetails := response["conflict_details"].(map[string]any)
	assert.Contains(t, conflictDetails, "example.com")
}

func TestNPMImportHandler_Commit_CreateNew(t *testing.T) {
	db := setupNPMTestDB(t)
	handler := NewNPMImportHandler(db)

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	npmExport := NPMExport{
		ProxyHosts: []NPMProxyHost{
			{
				ID:            1,
				DomainNames:   []string{"newhost.com"},
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

	uploadReq := httptest.NewRequest(http.MethodPost, "/api/v1/import/npm/upload", bytes.NewReader(uploadBody))
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

	req := httptest.NewRequest(http.MethodPost, "/api/v1/import/npm/commit", bytes.NewReader(commitBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]any
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(1), response["created"])
	assert.Equal(t, float64(0), response["updated"])
	assert.Equal(t, float64(0), response["skipped"])

	var host models.ProxyHost
	db.Where("domain_names = ?", "newhost.com").First(&host)
	assert.NotEmpty(t, host.UUID)
	assert.Equal(t, "192.168.1.100", host.ForwardHost)
}

func TestNPMImportHandler_Commit_SkipAction(t *testing.T) {
	db := setupNPMTestDB(t)
	handler := NewNPMImportHandler(db)

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	npmExport := NPMExport{
		ProxyHosts: []NPMProxyHost{
			{
				ID:            1,
				DomainNames:   []string{"skipme.com"},
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

	uploadReq := httptest.NewRequest(http.MethodPost, "/api/v1/import/npm/upload", bytes.NewReader(uploadBody))
	uploadReq.Header.Set("Content-Type", "application/json")
	uploadW := httptest.NewRecorder()

	router.ServeHTTP(uploadW, uploadReq)
	require.Equal(t, http.StatusOK, uploadW.Code)

	var uploadResponse map[string]any
	err := json.Unmarshal(uploadW.Body.Bytes(), &uploadResponse)
	require.NoError(t, err)

	session := uploadResponse["session"].(map[string]any)
	sessionID := session["id"].(string)

	// Step 2: Commit with skip resolution
	commitBody, _ := json.Marshal(map[string]any{
		"session_uuid": sessionID,
		"resolutions":  map[string]string{"skipme.com": "skip"},
		"names":        map[string]string{},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/import/npm/commit", bytes.NewReader(commitBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]any
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(0), response["created"])
	assert.Equal(t, float64(1), response["skipped"])
}

func TestNPMImportHandler_Commit_SessionNotFound(t *testing.T) {
	db := setupNPMTestDB(t)
	handler := NewNPMImportHandler(db)

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	// Try to commit with a non-existent session
	commitBody, _ := json.Marshal(map[string]any{
		"session_uuid": "non-existent-uuid",
		"resolutions":  map[string]string{},
		"names":        map[string]string{},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/import/npm/commit", bytes.NewReader(commitBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response["error"], "session not found")
}

func TestNPMImportHandler_Cancel(t *testing.T) {
	db := setupNPMTestDB(t)
	handler := NewNPMImportHandler(db)

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	npmExport := NPMExport{
		ProxyHosts: []NPMProxyHost{
			{
				ID:            1,
				DomainNames:   []string{"cancel-test.com"},
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

	uploadReq := httptest.NewRequest(http.MethodPost, "/api/v1/import/npm/upload", bytes.NewReader(uploadBody))
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

	cancelReq := httptest.NewRequest(http.MethodPost, "/api/v1/import/npm/cancel", bytes.NewReader(cancelBody))
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

	commitReq := httptest.NewRequest(http.MethodPost, "/api/v1/import/npm/commit", bytes.NewReader(commitBody))
	commitReq.Header.Set("Content-Type", "application/json")
	commitW := httptest.NewRecorder()

	router.ServeHTTP(commitW, commitReq)

	assert.Equal(t, http.StatusNotFound, commitW.Code)
}

func TestNPMImportHandler_Cancel_RequiresValidJSONBody(t *testing.T) {
	db := setupNPMTestDB(t)
	handler := NewNPMImportHandler(db)

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	t.Run("missing body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/import/npm/cancel", http.NoBody)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/import/npm/cancel", bytes.NewBufferString("{"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("empty object payload", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/import/npm/cancel", bytes.NewBufferString("{}"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		var resp map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "session_uuid required", resp["error"])
	})

	t.Run("missing session_uuid payload", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/import/npm/cancel", bytes.NewBufferString(`{"foo":"bar"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		var resp map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "session_uuid required", resp["error"])
	})
}

func TestNPMImportHandler_ConvertNPMToImportResult(t *testing.T) {
	db := setupNPMTestDB(t)
	handler := NewNPMImportHandler(db)

	npmExport := NPMExport{
		ProxyHosts: []NPMProxyHost{
			{
				ID:                    1,
				DomainNames:           []string{"test.com", "www.test.com"},
				ForwardScheme:         "https",
				ForwardHost:           "backend",
				ForwardPort:           443,
				SSLForced:             true,
				AllowWebsocketUpgrade: true,
				CachingEnabled:        true,
				AdvancedConfig:        "proxy_set_header X-Custom value;",
			},
			{
				ID:          2,
				DomainNames: []string{},
			},
		},
	}

	result := handler.convertNPMToImportResult(npmExport)

	assert.Len(t, result.Hosts, 1)
	assert.Len(t, result.Errors, 1)

	host := result.Hosts[0]
	assert.Equal(t, "test.com,www.test.com", host.DomainNames)
	assert.Equal(t, "https", host.ForwardScheme)
	assert.Equal(t, "backend", host.ForwardHost)
	assert.Equal(t, 443, host.ForwardPort)
	assert.True(t, host.SSLForced)
	assert.True(t, host.WebsocketSupport)
	assert.Len(t, host.Warnings, 2) // Caching + Advanced config warnings
}
