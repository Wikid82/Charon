package handlers

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/caddy"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
)

func setupTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()

	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}))

	h := NewProxyHostHandler(db, nil)
	r := gin.New()
	api := r.Group("/api/v1")
	h.RegisterRoutes(api)

	return r, db
}

func TestProxyHostLifecycle(t *testing.T) {
	router, _ := setupTestRouter(t)

	body := `{"name":"Media","domain_names":"media.example.com","forward_scheme":"http","forward_host":"media","forward_port":32400,"enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/proxy-hosts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusCreated, resp.Code)

	var created models.ProxyHost
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &created))
	require.Equal(t, "media.example.com", created.DomainNames)

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/proxy-hosts", nil)
	listResp := httptest.NewRecorder()
	router.ServeHTTP(listResp, listReq)
	require.Equal(t, http.StatusOK, listResp.Code)

	var hosts []models.ProxyHost
	require.NoError(t, json.Unmarshal(listResp.Body.Bytes(), &hosts))
	require.Len(t, hosts, 1)

	// Get by ID
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/proxy-hosts/"+created.UUID, nil)
	getResp := httptest.NewRecorder()
	router.ServeHTTP(getResp, getReq)
	require.Equal(t, http.StatusOK, getResp.Code)

	var fetched models.ProxyHost
	require.NoError(t, json.Unmarshal(getResp.Body.Bytes(), &fetched))
	require.Equal(t, created.UUID, fetched.UUID)

	// Update
	updateBody := `{"name":"Media Updated","domain_names":"media.example.com","forward_scheme":"http","forward_host":"media","forward_port":32400,"enabled":false}`
	updateReq := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/"+created.UUID, strings.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateResp := httptest.NewRecorder()
	router.ServeHTTP(updateResp, updateReq)
	require.Equal(t, http.StatusOK, updateResp.Code)

	var updated models.ProxyHost
	require.NoError(t, json.Unmarshal(updateResp.Body.Bytes(), &updated))
	require.Equal(t, "Media Updated", updated.Name)
	require.False(t, updated.Enabled)

	// Delete
	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/proxy-hosts/"+created.UUID, nil)
	delResp := httptest.NewRecorder()
	router.ServeHTTP(delResp, delReq)
	require.Equal(t, http.StatusOK, delResp.Code)

	// Verify Delete
	getReq2 := httptest.NewRequest(http.MethodGet, "/api/v1/proxy-hosts/"+created.UUID, nil)
	getResp2 := httptest.NewRecorder()
	router.ServeHTTP(getResp2, getReq2)
	require.Equal(t, http.StatusNotFound, getResp2.Code)
}

func TestProxyHostErrors(t *testing.T) {
	// Mock Caddy Admin API that fails
	caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer caddyServer.Close()

	// Setup DB
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.CaddyConfig{}))

	// Setup Caddy Manager
	tmpDir := t.TempDir()
	client := caddy.NewClient(caddyServer.URL)
	manager := caddy.NewManager(client, db, tmpDir)

	// Setup Handler
	h := NewProxyHostHandler(db, manager)
	r := gin.New()
	api := r.Group("/api/v1")
	h.RegisterRoutes(api)

	// Test Create - Bind Error
	req := httptest.NewRequest(http.MethodPost, "/api/v1/proxy-hosts", strings.NewReader(`invalid json`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	require.Equal(t, http.StatusBadRequest, resp.Code)

	// Test Create - Apply Config Error
	body := `{"name":"Fail Host","domain_names":"fail-unique-456.local","forward_scheme":"http","forward_host":"localhost","forward_port":8080,"enabled":true}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/proxy-hosts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	require.Equal(t, http.StatusInternalServerError, resp.Code)

	// Create a host for Update/Delete/Get tests (manually in DB to avoid handler error)
	host := models.ProxyHost{
		UUID:          uuid.NewString(),
		Name:          "Existing Host",
		DomainNames:   "exist.local",
		ForwardScheme: "http",
		ForwardHost:   "localhost",
		ForwardPort:   8080,
		Enabled:       true,
	}
	db.Create(&host)

	// Test Get - Not Found
	req = httptest.NewRequest(http.MethodGet, "/api/v1/proxy-hosts/non-existent-uuid", nil)
	resp = httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	require.Equal(t, http.StatusNotFound, resp.Code)

	// Test Update - Not Found
	req = httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/non-existent-uuid", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	require.Equal(t, http.StatusNotFound, resp.Code)

	// Test Update - Bind Error
	req = httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/"+host.UUID, strings.NewReader(`invalid json`))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	require.Equal(t, http.StatusBadRequest, resp.Code)

	// Test Update - Apply Config Error
	updateBody := `{"name":"Fail Host Update","domain_names":"fail-unique-update.local","forward_scheme":"http","forward_host":"localhost","forward_port":8080,"enabled":true}`
	req = httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/"+host.UUID, strings.NewReader(updateBody))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	require.Equal(t, http.StatusInternalServerError, resp.Code)

	// Test Delete - Not Found
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/proxy-hosts/non-existent-uuid", nil)
	resp = httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	require.Equal(t, http.StatusNotFound, resp.Code)

	// Test Delete - Apply Config Error
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/proxy-hosts/"+host.UUID, nil)
	resp = httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	require.Equal(t, http.StatusInternalServerError, resp.Code)

	// Test TestConnection - Bind Error
	req = httptest.NewRequest(http.MethodPost, "/api/v1/proxy-hosts/test", strings.NewReader(`invalid json`))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	require.Equal(t, http.StatusBadRequest, resp.Code)

	// Test TestConnection - Connection Failure
	testBody := `{"forward_host": "invalid.host.local", "forward_port": 12345}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/proxy-hosts/test", strings.NewReader(testBody))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	require.Equal(t, http.StatusBadGateway, resp.Code)
}

func TestProxyHostValidation(t *testing.T) {
	router, db := setupTestRouter(t)

	// Invalid JSON
	req := httptest.NewRequest(http.MethodPost, "/api/v1/proxy-hosts", strings.NewReader(`{invalid json}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusBadRequest, resp.Code)

	// Create a host first
	host := &models.ProxyHost{
		UUID:        "valid-uuid",
		DomainNames: "valid.com",
	}
	db.Create(host)

	// Update with invalid JSON
	req = httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/valid-uuid", strings.NewReader(`{invalid json}`))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestProxyHostConnection(t *testing.T) {
	router, _ := setupTestRouter(t)

	// 1. Test Invalid Input (Missing Host)
	body := `{"forward_port": 80}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/proxy-hosts/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusBadRequest, resp.Code)

	// 2. Test Connection Failure (Unreachable Port)
	// Use a reserved port or localhost port that is likely closed
	body = `{"forward_host": "localhost", "forward_port": 54321}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/proxy-hosts/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	// It should return 502 Bad Gateway
	require.Equal(t, http.StatusBadGateway, resp.Code)

	// 3. Test Connection Success
	// Start a local listener
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()

	addr := l.Addr().(*net.TCPAddr)

	body = fmt.Sprintf(`{"forward_host": "%s", "forward_port": %d}`, addr.IP.String(), addr.Port)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/proxy-hosts/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)
}

func TestProxyHostWithCaddyIntegration(t *testing.T) {
	// Mock Caddy Admin API
	caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/load" && r.Method == "POST" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer caddyServer.Close()

	// Setup DB
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.CaddyConfig{}))

	// Setup Caddy Manager
	tmpDir := t.TempDir()
	client := caddy.NewClient(caddyServer.URL)
	manager := caddy.NewManager(client, db, tmpDir)

	// Setup Handler
	h := NewProxyHostHandler(db, manager)
	r := gin.New()
	api := r.Group("/api/v1")
	h.RegisterRoutes(api)

	// Test Create with Caddy Sync
	body := `{"name":"Caddy Host","domain_names":"caddy.local","forward_scheme":"http","forward_host":"localhost","forward_port":8080,"enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/proxy-hosts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	require.Equal(t, http.StatusCreated, resp.Code)

	// Test Update with Caddy Sync
	var createdHost models.ProxyHost
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &createdHost))

	updateBody := `{"name":"Updated Caddy Host","domain_names":"caddy.local","forward_scheme":"http","forward_host":"localhost","forward_port":8081,"enabled":true}`
	req = httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/"+createdHost.UUID, strings.NewReader(updateBody))
	req.Header.Set("Content-Type", "application/json")

	resp = httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	// Test Delete with Caddy Sync
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/proxy-hosts/"+createdHost.UUID, nil)
	resp = httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)
}
