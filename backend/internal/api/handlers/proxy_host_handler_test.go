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
}
