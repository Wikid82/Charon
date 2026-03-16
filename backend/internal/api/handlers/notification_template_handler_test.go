package handlers

import (
	"encoding/json"
	"fmt"
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

	svc := services.NewNotificationService(db, nil)
	h := NewNotificationTemplateHandler(svc)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", uint(1))
		c.Next()
	})
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
	req = httptest.NewRequest(http.MethodGet, "/api/v1/notifications/templates", http.NoBody)
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
	var previewResp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &previewResp))
	require.NotEmpty(t, previewResp["rendered"])

	// Delete
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/notifications/templates/"+created.ID, http.NoBody)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestNotificationTemplateHandler_Create_InvalidJSON(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationTemplate{}))
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationTemplateHandler(svc)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", uint(1))
		c.Next()
	})
	r.POST("/api/templates", h.Create)

	req := httptest.NewRequest(http.MethodPost, "/api/templates", strings.NewReader(`{invalid}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestNotificationTemplateHandler_Update_InvalidJSON(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationTemplate{}))
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationTemplateHandler(svc)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", uint(1))
		c.Next()
	})
	r.PUT("/api/templates/:id", h.Update)

	req := httptest.NewRequest(http.MethodPut, "/api/templates/test-id", strings.NewReader(`{invalid}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestNotificationTemplateHandler_Preview_InvalidJSON(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationTemplate{}))
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationTemplateHandler(svc)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", uint(1))
		c.Next()
	})
	r.POST("/api/templates/preview", h.Preview)

	req := httptest.NewRequest(http.MethodPost, "/api/templates/preview", strings.NewReader(`{invalid}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestNotificationTemplateHandler_AdminRequired(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationTemplate{}))
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationTemplateHandler(svc)

	r := gin.New()
	r.POST("/api/templates", h.Create)
	r.PUT("/api/templates/:id", h.Update)
	r.DELETE("/api/templates/:id", h.Delete)

	createReq := httptest.NewRequest(http.MethodPost, "/api/templates", strings.NewReader(`{"name":"x","config":"{}"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)
	require.Equal(t, http.StatusForbidden, createW.Code)

	updateReq := httptest.NewRequest(http.MethodPut, "/api/templates/test-id", strings.NewReader(`{"name":"x","config":"{}"}`))
	updateReq.Header.Set("Content-Type", "application/json")
	updateW := httptest.NewRecorder()
	r.ServeHTTP(updateW, updateReq)
	require.Equal(t, http.StatusForbidden, updateW.Code)

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/templates/test-id", http.NoBody)
	deleteW := httptest.NewRecorder()
	r.ServeHTTP(deleteW, deleteReq)
	require.Equal(t, http.StatusForbidden, deleteW.Code)
}

func TestNotificationTemplateHandler_List_DBError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationTemplate{}))
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationTemplateHandler(svc)

	r := gin.New()
	r.GET("/api/templates", h.List)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	req := httptest.NewRequest(http.MethodGet, "/api/templates", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestNotificationTemplateHandler_WriteOps_DBError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationTemplate{}))
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationTemplateHandler(svc)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", uint(1))
		c.Next()
	})
	r.POST("/api/templates", h.Create)
	r.PUT("/api/templates/:id", h.Update)
	r.DELETE("/api/templates/:id", h.Delete)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	createReq := httptest.NewRequest(http.MethodPost, "/api/templates", strings.NewReader(`{"name":"x","config":"{}"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)
	require.Equal(t, http.StatusInternalServerError, createW.Code)

	updateReq := httptest.NewRequest(http.MethodPut, "/api/templates/test-id", strings.NewReader(`{"id":"test-id","name":"x","config":"{}"}`))
	updateReq.Header.Set("Content-Type", "application/json")
	updateW := httptest.NewRecorder()
	r.ServeHTTP(updateW, updateReq)
	require.Equal(t, http.StatusInternalServerError, updateW.Code)

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/templates/test-id", http.NoBody)
	deleteW := httptest.NewRecorder()
	r.ServeHTTP(deleteW, deleteReq)
	require.Equal(t, http.StatusInternalServerError, deleteW.Code)
}

func TestNotificationTemplateHandler_WriteOps_PermissionErrorResponse(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationTemplate{}))

	createHook := "test_notification_template_permission_create"
	updateHook := "test_notification_template_permission_update"
	deleteHook := "test_notification_template_permission_delete"

	require.NoError(t, db.Callback().Create().Before("gorm:create").Register(createHook, func(tx *gorm.DB) {
		_ = tx.AddError(fmt.Errorf("permission denied"))
	}))
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register(updateHook, func(tx *gorm.DB) {
		_ = tx.AddError(fmt.Errorf("permission denied"))
	}))
	require.NoError(t, db.Callback().Delete().Before("gorm:delete").Register(deleteHook, func(tx *gorm.DB) {
		_ = tx.AddError(fmt.Errorf("permission denied"))
	}))
	t.Cleanup(func() {
		_ = db.Callback().Create().Remove(createHook)
		_ = db.Callback().Update().Remove(updateHook)
		_ = db.Callback().Delete().Remove(deleteHook)
	})

	svc := services.NewNotificationService(db, nil)
	h := NewNotificationTemplateHandler(svc)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", uint(1))
		c.Next()
	})
	r.POST("/api/templates", h.Create)
	r.PUT("/api/templates/:id", h.Update)
	r.DELETE("/api/templates/:id", h.Delete)

	createReq := httptest.NewRequest(http.MethodPost, "/api/templates", strings.NewReader(`{"name":"x","config":"{}"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)
	require.Equal(t, http.StatusInternalServerError, createW.Code)
	require.Contains(t, createW.Body.String(), "permissions_write_denied")

	updateReq := httptest.NewRequest(http.MethodPut, "/api/templates/test-id", strings.NewReader(`{"id":"test-id","name":"x","config":"{}"}`))
	updateReq.Header.Set("Content-Type", "application/json")
	updateW := httptest.NewRecorder()
	r.ServeHTTP(updateW, updateReq)
	require.Equal(t, http.StatusInternalServerError, updateW.Code)
	require.Contains(t, updateW.Body.String(), "permissions_write_denied")

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/templates/test-id", http.NoBody)
	deleteW := httptest.NewRecorder()
	r.ServeHTTP(deleteW, deleteReq)
	require.Equal(t, http.StatusInternalServerError, deleteW.Code)
	require.Contains(t, deleteW.Body.String(), "permissions_write_denied")
}
