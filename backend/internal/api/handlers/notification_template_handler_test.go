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

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

func TestNotificationTemplateHandler_CRUDAndPreview(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationTemplate{}, &models.Notification{}, &models.NotificationProvider{}))

	svc := services.NewNotificationService(db)
	h := NewNotificationTemplateHandler(svc)

	r := gin.New()
	api := r.Group("/api/v1")
	api.GET("/notifications/templates", h.List)
	api.POST("/notifications/templates", h.Create)
	api.PUT("/notifications/templates/:id", h.Update)
	api.DELETE("/notifications/templates/:id", h.Delete)
	api.POST("/notifications/templates/preview", h.Preview)

	// Create
	payload := `{"name":"test","config":"{\"hello\":\"world\"}"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/templates", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)
	var created models.NotificationTemplate
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	require.NotEmpty(t, created.ID)

	// List
	req = httptest.NewRequest(http.MethodGet, "/api/v1/notifications/templates", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var list []models.NotificationTemplate
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	require.True(t, len(list) >= 1)

	// Update
	updatedPayload := `{"name":"updated","config":"{\"hello\":\"updated\"}"}`
	req = httptest.NewRequest(http.MethodPut, "/api/v1/notifications/templates/"+created.ID, strings.NewReader(updatedPayload))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var up models.NotificationTemplate
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &up))
	require.Equal(t, "updated", up.Name)

	// Preview by id
	previewPayload := `{"template_id":"` + created.ID + `", "data": {}}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/notifications/templates/preview", strings.NewReader(previewPayload))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var previewResp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &previewResp))
	require.NotEmpty(t, previewResp["rendered"])

	// Delete
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/notifications/templates/"+created.ID, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}
