package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
)

func setupTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}))

	h := NewProxyHostHandler(db)
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
	router, _ := setupTestRouter(t)

	// Get non-existent
	req := httptest.NewRequest(http.MethodGet, "/api/v1/proxy-hosts/non-existent-uuid", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusNotFound, resp.Code)

	// Update non-existent
	updateBody := `{"name":"Media Updated"}`
	updateReq := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/non-existent-uuid", strings.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateResp := httptest.NewRecorder()
	router.ServeHTTP(updateResp, updateReq)
	require.Equal(t, http.StatusNotFound, updateResp.Code)

	// Delete non-existent
	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/proxy-hosts/non-existent-uuid", nil)
	delResp := httptest.NewRecorder()
	router.ServeHTTP(delResp, delReq)
	require.Equal(t, http.StatusNotFound, delResp.Code)
}
