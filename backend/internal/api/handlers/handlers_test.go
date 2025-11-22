package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/api/handlers"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
)

func setupTestDB() *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		panic("failed to connect to test database")
	}

	// Auto migrate
	db.AutoMigrate(
		&models.ProxyHost{},
		&models.Location{},
		&models.RemoteServer{},
		&models.ImportSession{},
	)

	return db
}

func TestRemoteServerHandler_List(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB()

	// Create test server
	server := &models.RemoteServer{
		UUID:     uuid.NewString(),
		Name:     "Test Server",
		Provider: "docker",
		Host:     "localhost",
		Port:     8080,
		Enabled:  true,
	}
	db.Create(server)

	handler := handlers.NewRemoteServerHandler(db)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	// Test List
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/remote-servers", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var servers []models.RemoteServer
	err := json.Unmarshal(w.Body.Bytes(), &servers)
	assert.NoError(t, err)
	assert.Len(t, servers, 1)
	assert.Equal(t, "Test Server", servers[0].Name)
}

func TestRemoteServerHandler_Create(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB()

	handler := handlers.NewRemoteServerHandler(db)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	// Test Create
	serverData := map[string]interface{}{
		"name":     "New Server",
		"provider": "generic",
		"host":     "192.168.1.100",
		"port":     3000,
		"enabled":  true,
	}
	body, _ := json.Marshal(serverData)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/remote-servers", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var server models.RemoteServer
	err := json.Unmarshal(w.Body.Bytes(), &server)
	assert.NoError(t, err)
	assert.Equal(t, "New Server", server.Name)
	assert.NotEmpty(t, server.UUID)
}

func TestRemoteServerHandler_TestConnection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB()

	// Create test server
	server := &models.RemoteServer{
		UUID:     uuid.NewString(),
		Name:     "Test Server",
		Provider: "docker",
		Host:     "localhost",
		Port:     99999, // Invalid port to test failure
		Enabled:  true,
	}
	db.Create(server)

	handler := handlers.NewRemoteServerHandler(db)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	// Test connection
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/remote-servers/"+server.UUID+"/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.False(t, result["reachable"].(bool))
	assert.NotEmpty(t, result["error"])
}

func TestRemoteServerHandler_Get(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB()

	// Create test server
	server := &models.RemoteServer{
		UUID:     uuid.NewString(),
		Name:     "Test Server",
		Provider: "docker",
		Host:     "localhost",
		Port:     8080,
		Enabled:  true,
	}
	db.Create(server)

	handler := handlers.NewRemoteServerHandler(db)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	// Test Get
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/remote-servers/"+server.UUID, nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var fetched models.RemoteServer
	err := json.Unmarshal(w.Body.Bytes(), &fetched)
	assert.NoError(t, err)
	assert.Equal(t, server.UUID, fetched.UUID)
}

func TestRemoteServerHandler_Update(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB()

	// Create test server
	server := &models.RemoteServer{
		UUID:     uuid.NewString(),
		Name:     "Test Server",
		Provider: "docker",
		Host:     "localhost",
		Port:     8080,
		Enabled:  true,
	}
	db.Create(server)

	handler := handlers.NewRemoteServerHandler(db)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	// Test Update
	updateData := map[string]interface{}{
		"name":     "Updated Server",
		"provider": "generic",
		"host":     "10.0.0.1",
		"port":     9000,
		"enabled":  false,
	}
	body, _ := json.Marshal(updateData)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/remote-servers/"+server.UUID, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var updated models.RemoteServer
	err := json.Unmarshal(w.Body.Bytes(), &updated)
	assert.NoError(t, err)
	assert.Equal(t, "Updated Server", updated.Name)
	assert.Equal(t, "generic", updated.Provider)
	assert.False(t, updated.Enabled)
}

func TestRemoteServerHandler_Delete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB()

	// Create test server
	server := &models.RemoteServer{
		UUID:     uuid.NewString(),
		Name:     "Test Server",
		Provider: "docker",
		Host:     "localhost",
		Port:     8080,
		Enabled:  true,
	}
	db.Create(server)

	handler := handlers.NewRemoteServerHandler(db)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	// Test Delete
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/remote-servers/"+server.UUID, nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)

	// Verify Delete
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/v1/remote-servers/"+server.UUID, nil)
	router.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusNotFound, w2.Code)
}

func TestProxyHostHandler_List(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB()

	// Create test proxy host
	host := &models.ProxyHost{
		UUID:          uuid.NewString(),
		Name:          "Test Host",
		DomainNames:   "test.local",
		ForwardScheme: "http",
		ForwardHost:   "localhost",
		ForwardPort:   3000,
		Enabled:       true,
	}
	db.Create(host)

	handler := handlers.NewProxyHostHandler(db, nil)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	// Test List
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/proxy-hosts", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var hosts []models.ProxyHost
	err := json.Unmarshal(w.Body.Bytes(), &hosts)
	assert.NoError(t, err)
	assert.Len(t, hosts, 1)
	assert.Equal(t, "Test Host", hosts[0].Name)
}

func TestProxyHostHandler_Create(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB()

	handler := handlers.NewProxyHostHandler(db, nil)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	// Test Create
	hostData := map[string]interface{}{
		"name":           "New Host",
		"domain_names":   "new.local",
		"forward_scheme": "http",
		"forward_host":   "192.168.1.200",
		"forward_port":   8080,
		"enabled":        true,
	}
	body, _ := json.Marshal(hostData)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/proxy-hosts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var host models.ProxyHost
	err := json.Unmarshal(w.Body.Bytes(), &host)
	assert.NoError(t, err)
	assert.Equal(t, "New Host", host.Name)
	assert.Equal(t, "new.local", host.DomainNames)
	assert.NotEmpty(t, host.UUID)
}

func TestHealthHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/health", handlers.HealthHandler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.Equal(t, "ok", result["status"])
}

func TestRemoteServerHandler_Errors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB()

	handler := handlers.NewRemoteServerHandler(db)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	// Get non-existent
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/remote-servers/non-existent", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// Update non-existent
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("PUT", "/api/v1/remote-servers/non-existent", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// Delete non-existent
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/api/v1/remote-servers/non-existent", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}
