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

func setupDomainTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()

	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Domain{}))

	h := NewDomainHandler(db)
	r := gin.New()

	// Manually register routes since DomainHandler doesn't have a RegisterRoutes method yet
	// or we can just register them here for testing
	r.GET("/api/v1/domains", h.List)
	r.POST("/api/v1/domains", h.Create)
	r.DELETE("/api/v1/domains/:id", h.Delete)

	return r, db
}

func TestDomainLifecycle(t *testing.T) {
	router, _ := setupDomainTestRouter(t)

	// 1. Create Domain
	body := `{"name":"example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/domains", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusCreated, resp.Code)

	var created models.Domain
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &created))
	require.Equal(t, "example.com", created.Name)
	require.NotEmpty(t, created.UUID)

	// 2. List Domains
	req = httptest.NewRequest(http.MethodGet, "/api/v1/domains", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	var list []models.Domain
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &list))
	require.Len(t, list, 1)
	require.Equal(t, "example.com", list[0].Name)

	// 3. Delete Domain
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/domains/"+created.UUID, nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	// 4. Verify Deletion
	req = httptest.NewRequest(http.MethodGet, "/api/v1/domains", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &list))
	require.Len(t, list, 0)
}

func TestDomainErrors(t *testing.T) {
	router, _ := setupDomainTestRouter(t)

	// 1. Create Invalid JSON
	req := httptest.NewRequest(http.MethodPost, "/api/v1/domains", strings.NewReader(`{invalid}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusBadRequest, resp.Code)

	// 2. Create Missing Name
	req = httptest.NewRequest(http.MethodPost, "/api/v1/domains", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusBadRequest, resp.Code)
}
