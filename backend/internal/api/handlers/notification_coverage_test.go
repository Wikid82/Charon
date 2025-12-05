package handlers

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

func setupNotificationCoverageDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := OpenTestDB(t)
	db.AutoMigrate(&models.Notification{}, &models.NotificationProvider{}, &models.NotificationTemplate{})
	return db
}

// Notification Handler Tests

func TestNotificationHandler_List_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db)
	h := NewNotificationHandler(svc)

	// Drop the table to cause error
	db.Migrator().DropTable(&models.Notification{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/notifications", nil)

	h.List(c)

	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to list notifications")
}

func TestNotificationHandler_List_UnreadOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db)
	h := NewNotificationHandler(svc)

	// Create some notifications
	svc.Create(models.NotificationTypeInfo, "Test 1", "Message 1")
	svc.Create(models.NotificationTypeInfo, "Test 2", "Message 2")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/notifications?unread=true", nil)

	h.List(c)

	assert.Equal(t, 200, w.Code)
}

func TestNotificationHandler_MarkAsRead_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db)
	h := NewNotificationHandler(svc)

	// Drop table to cause error
	db.Migrator().DropTable(&models.Notification{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "test-id"}}

	h.MarkAsRead(c)

	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to mark notification as read")
}

func TestNotificationHandler_MarkAllAsRead_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db)
	h := NewNotificationHandler(svc)

	// Drop table to cause error
	db.Migrator().DropTable(&models.Notification{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	h.MarkAllAsRead(c)

	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to mark all notifications as read")
}

// Notification Provider Handler Tests

func TestNotificationProviderHandler_List_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db)
	h := NewNotificationProviderHandler(svc)

	// Drop table to cause error
	db.Migrator().DropTable(&models.NotificationProvider{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	h.List(c)

	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to list providers")
}

func TestNotificationProviderHandler_Create_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db)
	h := NewNotificationProviderHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/providers", bytes.NewBufferString("invalid json"))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Create(c)

	assert.Equal(t, 400, w.Code)
}

func TestNotificationProviderHandler_Create_DBError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db)
	h := NewNotificationProviderHandler(svc)

	// Drop table to cause error
	db.Migrator().DropTable(&models.NotificationProvider{})

	provider := models.NotificationProvider{
		Name:     "Test",
		Type:     "webhook",
		URL:      "https://example.com",
		Template: "minimal",
	}
	body, _ := json.Marshal(provider)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/providers", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Create(c)

	assert.Equal(t, 500, w.Code)
}

func TestNotificationProviderHandler_Create_InvalidTemplate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db)
	h := NewNotificationProviderHandler(svc)

	provider := models.NotificationProvider{
		Name:     "Test",
		Type:     "webhook",
		URL:      "https://example.com",
		Template: "custom",
		Config:   "{{.Invalid", // Invalid template syntax
	}
	body, _ := json.Marshal(provider)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/providers", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Create(c)

	assert.Equal(t, 400, w.Code)
}

func TestNotificationProviderHandler_Update_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db)
	h := NewNotificationProviderHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "test-id"}}
	c.Request = httptest.NewRequest("PUT", "/providers/test-id", bytes.NewBufferString("invalid"))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Update(c)

	assert.Equal(t, 400, w.Code)
}

func TestNotificationProviderHandler_Update_InvalidTemplate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db)
	h := NewNotificationProviderHandler(svc)

	// Create a provider first
	provider := models.NotificationProvider{
		Name:     "Test",
		Type:     "webhook",
		URL:      "https://example.com",
		Template: "minimal",
	}
	require.NoError(t, svc.CreateProvider(&provider))

	// Update with invalid template
	provider.Template = "custom"
	provider.Config = "{{.Invalid" // Invalid
	body, _ := json.Marshal(provider)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: provider.ID}}
	c.Request = httptest.NewRequest("PUT", "/providers/"+provider.ID, bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Update(c)

	assert.Equal(t, 400, w.Code)
}

func TestNotificationProviderHandler_Update_DBError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db)
	h := NewNotificationProviderHandler(svc)

	// Drop table to cause error
	db.Migrator().DropTable(&models.NotificationProvider{})

	provider := models.NotificationProvider{
		Name:     "Test",
		Type:     "webhook",
		URL:      "https://example.com",
		Template: "minimal",
	}
	body, _ := json.Marshal(provider)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "test-id"}}
	c.Request = httptest.NewRequest("PUT", "/providers/test-id", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Update(c)

	assert.Equal(t, 500, w.Code)
}

func TestNotificationProviderHandler_Delete_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db)
	h := NewNotificationProviderHandler(svc)

	// Drop table to cause error
	db.Migrator().DropTable(&models.NotificationProvider{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "test-id"}}

	h.Delete(c)

	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to delete provider")
}

func TestNotificationProviderHandler_Test_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db)
	h := NewNotificationProviderHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/providers/test", bytes.NewBufferString("invalid"))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Test(c)

	assert.Equal(t, 400, w.Code)
}

func TestNotificationProviderHandler_Templates(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db)
	h := NewNotificationProviderHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	h.Templates(c)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "minimal")
	assert.Contains(t, w.Body.String(), "detailed")
	assert.Contains(t, w.Body.String(), "custom")
}

func TestNotificationProviderHandler_Preview_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db)
	h := NewNotificationProviderHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/providers/preview", bytes.NewBufferString("invalid"))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Preview(c)

	assert.Equal(t, 400, w.Code)
}

func TestNotificationProviderHandler_Preview_WithData(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db)
	h := NewNotificationProviderHandler(svc)

	payload := map[string]interface{}{
		"template": "minimal",
		"data": map[string]interface{}{
			"Title":   "Custom Title",
			"Message": "Custom Message",
		},
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/providers/preview", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Preview(c)

	assert.Equal(t, 200, w.Code)
}

func TestNotificationProviderHandler_Preview_InvalidTemplate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db)
	h := NewNotificationProviderHandler(svc)

	payload := map[string]interface{}{
		"template": "custom",
		"config":   "{{.Invalid",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/providers/preview", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Preview(c)

	assert.Equal(t, 400, w.Code)
}

// Notification Template Handler Tests

func TestNotificationTemplateHandler_List_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db)
	h := NewNotificationTemplateHandler(svc)

	// Drop table to cause error
	db.Migrator().DropTable(&models.NotificationTemplate{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	h.List(c)

	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "failed to list templates")
}

func TestNotificationTemplateHandler_Create_BadJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db)
	h := NewNotificationTemplateHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/templates", bytes.NewBufferString("invalid"))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Create(c)

	assert.Equal(t, 400, w.Code)
}

func TestNotificationTemplateHandler_Create_DBError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db)
	h := NewNotificationTemplateHandler(svc)

	// Drop table to cause error
	db.Migrator().DropTable(&models.NotificationTemplate{})

	tmpl := models.NotificationTemplate{
		Name:   "Test",
		Config: `{"test": true}`,
	}
	body, _ := json.Marshal(tmpl)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/templates", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Create(c)

	assert.Equal(t, 500, w.Code)
}

func TestNotificationTemplateHandler_Update_BadJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db)
	h := NewNotificationTemplateHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "test-id"}}
	c.Request = httptest.NewRequest("PUT", "/templates/test-id", bytes.NewBufferString("invalid"))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Update(c)

	assert.Equal(t, 400, w.Code)
}

func TestNotificationTemplateHandler_Update_DBError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db)
	h := NewNotificationTemplateHandler(svc)

	// Drop table to cause error
	db.Migrator().DropTable(&models.NotificationTemplate{})

	tmpl := models.NotificationTemplate{
		Name:   "Test",
		Config: `{"test": true}`,
	}
	body, _ := json.Marshal(tmpl)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "test-id"}}
	c.Request = httptest.NewRequest("PUT", "/templates/test-id", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Update(c)

	assert.Equal(t, 500, w.Code)
}

func TestNotificationTemplateHandler_Delete_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db)
	h := NewNotificationTemplateHandler(svc)

	// Drop table to cause error
	db.Migrator().DropTable(&models.NotificationTemplate{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "test-id"}}

	h.Delete(c)

	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "failed to delete template")
}

func TestNotificationTemplateHandler_Preview_BadJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db)
	h := NewNotificationTemplateHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/templates/preview", bytes.NewBufferString("invalid"))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Preview(c)

	assert.Equal(t, 400, w.Code)
}

func TestNotificationTemplateHandler_Preview_TemplateNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db)
	h := NewNotificationTemplateHandler(svc)

	payload := map[string]interface{}{
		"template_id": "nonexistent",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/templates/preview", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Preview(c)

	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "template not found")
}

func TestNotificationTemplateHandler_Preview_WithStoredTemplate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db)
	h := NewNotificationTemplateHandler(svc)

	// Create a template
	tmpl := &models.NotificationTemplate{
		Name:   "Test",
		Config: `{"title": "{{.Title}}"}`,
	}
	require.NoError(t, svc.CreateTemplate(tmpl))

	payload := map[string]interface{}{
		"template_id": tmpl.ID,
		"data": map[string]interface{}{
			"Title": "Test Title",
		},
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/templates/preview", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Preview(c)

	assert.Equal(t, 200, w.Code)
}

func TestNotificationTemplateHandler_Preview_InvalidTemplate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db)
	h := NewNotificationTemplateHandler(svc)

	payload := map[string]interface{}{
		"template": "{{.Invalid",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/templates/preview", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Preview(c)

	assert.Equal(t, 400, w.Code)
}
