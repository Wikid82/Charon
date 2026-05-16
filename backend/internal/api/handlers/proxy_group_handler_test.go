package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupProxyGroupHandlerRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyGroup{}, &models.ProxyHost{}))

	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := NewProxyGroupHandler(db)
	grp := router.Group("/")
	h.RegisterRoutes(grp)
	return router, db
}

func doRequest(router *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	var b []byte
	if body != nil {
		b, _ = json.Marshal(body)
	}
	req, _ := http.NewRequest(method, path, bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestProxyGroupHandler_List_Empty(t *testing.T) {
	router, _ := setupProxyGroupHandlerRouter(t)
	w := doRequest(router, http.MethodGet, "/proxy-groups", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	var result []any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Empty(t, result)
}

func TestProxyGroupHandler_List_WithGroups(t *testing.T) {
	router, db := setupProxyGroupHandlerRouter(t)
	require.NoError(t, db.Create(&models.ProxyGroup{Name: "Beta"}).Error)
	require.NoError(t, db.Create(&models.ProxyGroup{Name: "Alpha"}).Error)

	w := doRequest(router, http.MethodGet, "/proxy-groups", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	var result []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Len(t, result, 2)
	assert.Equal(t, "Alpha", result[0]["name"])
}

func TestProxyGroupHandler_Create_Valid(t *testing.T) {
	router, _ := setupProxyGroupHandlerRouter(t)
	w := doRequest(router, http.MethodPost, "/proxy-groups", map[string]any{
		"name":        "Production",
		"description": "Prod services",
		"color":       "#ff0000",
	})
	assert.Equal(t, http.StatusCreated, w.Code)
	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, "Production", result["name"])
	assert.NotEmpty(t, result["uuid"])
}

func TestProxyGroupHandler_Create_EmptyName_400(t *testing.T) {
	router, _ := setupProxyGroupHandlerRouter(t)
	w := doRequest(router, http.MethodPost, "/proxy-groups", map[string]any{"name": ""})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestProxyGroupHandler_Create_DefaultColor(t *testing.T) {
	router, _ := setupProxyGroupHandlerRouter(t)
	w := doRequest(router, http.MethodPost, "/proxy-groups", map[string]any{"name": "NoColor"})
	assert.Equal(t, http.StatusCreated, w.Code)
	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, "#6366f1", result["color"])
}

func TestProxyGroupHandler_Get_Found(t *testing.T) {
	router, db := setupProxyGroupHandlerRouter(t)
	group := &models.ProxyGroup{Name: "Find Me", Color: "#abc"}
	require.NoError(t, db.Create(group).Error)

	w := doRequest(router, http.MethodGet, "/proxy-groups/"+group.UUID, nil)
	assert.Equal(t, http.StatusOK, w.Code)
	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, "Find Me", result["name"])
	assert.Equal(t, float64(0), result["host_count"])
}

func TestProxyGroupHandler_Get_NotFound_404(t *testing.T) {
	router, _ := setupProxyGroupHandlerRouter(t)
	w := doRequest(router, http.MethodGet, "/proxy-groups/nonexistent-uuid", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestProxyGroupHandler_Update_PartialFields(t *testing.T) {
	router, db := setupProxyGroupHandlerRouter(t)
	group := &models.ProxyGroup{Name: "Old Name", Color: "#111"}
	require.NoError(t, db.Create(group).Error)

	newName := "New Name"
	w := doRequest(router, http.MethodPut, "/proxy-groups/"+group.UUID, map[string]any{
		"name": newName,
	})
	assert.Equal(t, http.StatusOK, w.Code)
	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, "New Name", result["name"])
}

func TestProxyGroupHandler_Update_EmptyName_400(t *testing.T) {
	router, db := setupProxyGroupHandlerRouter(t)
	group := &models.ProxyGroup{Name: "Valid"}
	require.NoError(t, db.Create(group).Error)

	w := doRequest(router, http.MethodPut, "/proxy-groups/"+group.UUID, map[string]any{"name": "  "})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestProxyGroupHandler_Delete_204(t *testing.T) {
	router, db := setupProxyGroupHandlerRouter(t)
	group := &models.ProxyGroup{Name: "Delete Me"}
	require.NoError(t, db.Create(group).Error)

	w := doRequest(router, http.MethodDelete, "/proxy-groups/"+group.UUID, nil)
	assert.Equal(t, http.StatusNoContent, w.Code)

	var count int64
	db.Model(&models.ProxyGroup{}).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestProxyGroupHandler_Delete_NotFound_404(t *testing.T) {
	router, _ := setupProxyGroupHandlerRouter(t)
	w := doRequest(router, http.MethodDelete, "/proxy-groups/nonexistent-uuid", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestProxyGroupHandler_List_ServiceError_500(t *testing.T) {
	router, db := setupProxyGroupHandlerRouter(t)
	require.NoError(t, db.Migrator().DropTable(&models.ProxyGroup{}))
	w := doRequest(router, http.MethodGet, "/proxy-groups", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestProxyGroupHandler_Create_InvalidJSON_400(t *testing.T) {
	router, _ := setupProxyGroupHandlerRouter(t)
	req, _ := http.NewRequest(http.MethodPost, "/proxy-groups", bytes.NewBufferString("not valid json{{"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestProxyGroupHandler_Update_NotFound_404(t *testing.T) {
	router, _ := setupProxyGroupHandlerRouter(t)
	w := doRequest(router, http.MethodPut, "/proxy-groups/nonexistent-uuid", map[string]any{"name": "New"})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestProxyGroupHandler_Update_InvalidJSON_400(t *testing.T) {
	router, db := setupProxyGroupHandlerRouter(t)
	group := &models.ProxyGroup{Name: "Valid Group"}
	require.NoError(t, db.Create(group).Error)
	req, _ := http.NewRequest(http.MethodPut, "/proxy-groups/"+group.UUID, bytes.NewBufferString("not valid json{{"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestProxyGroupHandler_Delete_ServiceError_500(t *testing.T) {
	router, db := setupProxyGroupHandlerRouter(t)
	group := &models.ProxyGroup{Name: "To Delete"}
	require.NoError(t, db.Create(group).Error)
	// Drop proxy_hosts so the unassign step inside Delete's transaction fails.
	require.NoError(t, db.Migrator().DropTable(&models.ProxyHost{}))
	w := doRequest(router, http.MethodDelete, "/proxy-groups/"+group.UUID, nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
