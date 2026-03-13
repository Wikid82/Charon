package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/internal/trace"
)

func setupNotificationCoverageDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := OpenTestDB(t)
	_ = db.AutoMigrate(&models.Notification{}, &models.NotificationProvider{}, &models.NotificationTemplate{})
	return db
}

func setAdminContext(c *gin.Context) {
	c.Set("role", "admin")
	c.Set("userID", uint(1))
}

// Notification Handler Tests

func TestNotificationHandler_List_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationHandler(svc)

	// Drop the table to cause error
	_ = db.Migrator().DropTable(&models.Notification{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	setAdminContext(c)
	setAdminContext(c)
	c.Request = httptest.NewRequest("GET", "/notifications", http.NoBody)

	h.List(c)

	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to list notifications")
}

func TestNotificationHandler_List_UnreadOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationHandler(svc)

	// Create some notifications
	_, _ = svc.Create(models.NotificationTypeInfo, "Test 1", "Message 1")
	_, _ = svc.Create(models.NotificationTypeInfo, "Test 2", "Message 2")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Request = httptest.NewRequest("GET", "/notifications?unread=true", http.NoBody)

	h.List(c)

	assert.Equal(t, 200, w.Code)
}

func TestNotificationHandler_MarkAsRead_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationHandler(svc)

	// Drop table to cause error
	_ = db.Migrator().DropTable(&models.Notification{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Params = gin.Params{{Key: "id", Value: "test-id"}}

	h.MarkAsRead(c)

	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to mark notification as read")
}

func TestNotificationHandler_MarkAllAsRead_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationHandler(svc)

	// Drop table to cause error
	_ = db.Migrator().DropTable(&models.Notification{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)

	h.MarkAllAsRead(c)

	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to mark all notifications as read")
}

// Notification Provider Handler Tests

func TestNotificationProviderHandler_List_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationProviderHandler(svc)

	// Drop table to cause error
	_ = db.Migrator().DropTable(&models.NotificationProvider{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)

	h.List(c)

	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to list providers")
}

func TestNotificationProviderHandler_Create_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationProviderHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Request = httptest.NewRequest("POST", "/providers", bytes.NewBufferString("invalid json"))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Create(c)

	assert.Equal(t, 400, w.Code)
}

func TestNotificationProviderHandler_Create_DBError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationProviderHandler(svc)

	// Drop table to cause error
	_ = db.Migrator().DropTable(&models.NotificationProvider{})

	provider := models.NotificationProvider{
		Name:     "Test",
		Type:     "discord",
		URL:      "https://discord.com/api/webhooks/123/abc",
		Template: "minimal",
	}
	body, _ := json.Marshal(provider)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Request = httptest.NewRequest("POST", "/providers", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Create(c)

	assert.Equal(t, 500, w.Code)
}

func TestNotificationProviderHandler_Create_InvalidTemplate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationProviderHandler(svc)

	provider := models.NotificationProvider{
		Name:     "Test",
		Type:     "discord",
		URL:      "https://discord.com/api/webhooks/123/abc",
		Template: "custom",
		Config:   "{{.Invalid", // Invalid template syntax
	}
	body, _ := json.Marshal(provider)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Request = httptest.NewRequest("POST", "/providers", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Create(c)

	assert.Equal(t, 400, w.Code)
}

func TestNotificationProviderHandler_Update_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationProviderHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Params = gin.Params{{Key: "id", Value: "test-id"}}
	c.Request = httptest.NewRequest("PUT", "/providers/test-id", bytes.NewBufferString("invalid"))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Update(c)

	assert.Equal(t, 400, w.Code)
}

func TestNotificationProviderHandler_Update_InvalidTemplate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationProviderHandler(svc)

	// Create a provider first
	provider := models.NotificationProvider{
		Name:     "Test",
		Type:     "discord",
		URL:      "https://discord.com/api/webhooks/123/abc",
		Template: "minimal",
	}
	require.NoError(t, svc.CreateProvider(&provider))

	// Update with invalid template
	provider.Template = "custom"
	provider.Config = "{{.Invalid" // Invalid
	body, _ := json.Marshal(provider)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Params = gin.Params{{Key: "id", Value: provider.ID}}
	c.Request = httptest.NewRequest("PUT", "/providers/"+provider.ID, bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Update(c)

	assert.Equal(t, 400, w.Code)
}

func TestNotificationProviderHandler_Update_DBError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationProviderHandler(svc)

	// Drop table to cause error
	_ = db.Migrator().DropTable(&models.NotificationProvider{})

	provider := models.NotificationProvider{
		Name:     "Test",
		Type:     "discord",
		URL:      "https://discord.com/api/webhooks/123/abc",
		Template: "minimal",
	}
	body, _ := json.Marshal(provider)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Params = gin.Params{{Key: "id", Value: "test-id"}}
	c.Request = httptest.NewRequest("PUT", "/providers/test-id", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Update(c)

	assert.Equal(t, 500, w.Code)
}

func TestNotificationProviderHandler_Delete_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationProviderHandler(svc)

	// Drop table to cause error
	_ = db.Migrator().DropTable(&models.NotificationProvider{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Params = gin.Params{{Key: "id", Value: "test-id"}}

	h.Delete(c)

	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to delete provider")
}

func TestNotificationProviderHandler_Test_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationProviderHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Request = httptest.NewRequest("POST", "/providers/test", bytes.NewBufferString("invalid"))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Test(c)

	assert.Equal(t, 400, w.Code)
}

func TestNotificationProviderHandler_Test_RejectsClientSuppliedGotifyToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationProviderHandler(svc)

	payload := map[string]any{
		"type":  "gotify",
		"url":   "https://gotify.example/message",
		"token": "super-secret-client-token",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Set(string(trace.RequestIDKey), "req-token-reject-1")
	c.Request = httptest.NewRequest(http.MethodPost, "/providers/test", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Test(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "TOKEN_WRITE_ONLY", resp["code"])
	assert.Equal(t, "validation", resp["category"])
	assert.Equal(t, "Gotify token is accepted only on provider create/update", resp["error"])
	assert.Equal(t, "req-token-reject-1", resp["request_id"])
	assert.NotContains(t, w.Body.String(), "super-secret-client-token")
}

func TestNotificationProviderHandler_Test_RejectsGotifyTokenWithWhitespace(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationProviderHandler(svc)

	payload := map[string]any{
		"type":  "gotify",
		"token": "   secret-with-space   ",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Request = httptest.NewRequest(http.MethodPost, "/providers/test", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Test(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "TOKEN_WRITE_ONLY")
	assert.NotContains(t, w.Body.String(), "secret-with-space")
}

func TestClassifyProviderTestFailure_NilError(t *testing.T) {
	code, category, message := classifyProviderTestFailure(nil)

	assert.Equal(t, "PROVIDER_TEST_FAILED", code)
	assert.Equal(t, "dispatch", category)
	assert.Equal(t, "Provider test failed", message)
}

func TestClassifyProviderTestFailure_DefaultStatusCode(t *testing.T) {
	code, category, message := classifyProviderTestFailure(errors.New("provider returned status 500"))

	assert.Equal(t, "PROVIDER_TEST_REMOTE_REJECTED", code)
	assert.Equal(t, "dispatch", category)
	assert.Contains(t, message, "HTTP 500")
}

func TestClassifyProviderTestFailure_GenericError(t *testing.T) {
	code, category, message := classifyProviderTestFailure(errors.New("something completely unexpected"))

	assert.Equal(t, "PROVIDER_TEST_FAILED", code)
	assert.Equal(t, "dispatch", category)
	assert.Equal(t, "Provider test failed", message)
}

func TestClassifyProviderTestFailure_InvalidDiscordWebhookURL(t *testing.T) {
	code, category, message := classifyProviderTestFailure(errors.New("invalid discord webhook url"))

	assert.Equal(t, "PROVIDER_TEST_URL_INVALID", code)
	assert.Equal(t, "validation", category)
	assert.Contains(t, message, "Provider URL")
}

func TestClassifyProviderTestFailure_URLValidation(t *testing.T) {
	code, category, message := classifyProviderTestFailure(errors.New("destination URL validation failed"))

	assert.Equal(t, "PROVIDER_TEST_URL_INVALID", code)
	assert.Equal(t, "validation", category)
	assert.Contains(t, message, "Provider URL")
}

func TestClassifyProviderTestFailure_AuthRejected(t *testing.T) {
	code, category, message := classifyProviderTestFailure(errors.New("failed to send webhook: provider returned status 401"))

	assert.Equal(t, "PROVIDER_TEST_AUTH_REJECTED", code)
	assert.Equal(t, "dispatch", category)
	assert.Contains(t, message, "rejected authentication")
}

func TestClassifyProviderTestFailure_EndpointNotFound(t *testing.T) {
	code, category, message := classifyProviderTestFailure(errors.New("failed to send webhook: provider returned status 404"))

	assert.Equal(t, "PROVIDER_TEST_ENDPOINT_NOT_FOUND", code)
	assert.Equal(t, "dispatch", category)
	assert.Contains(t, message, "endpoint was not found")
}

func TestClassifyProviderTestFailure_UnreachableEndpoint(t *testing.T) {
	code, category, message := classifyProviderTestFailure(errors.New("failed to send webhook: outbound request failed"))

	assert.Equal(t, "PROVIDER_TEST_UNREACHABLE", code)
	assert.Equal(t, "dispatch", category)
	assert.Contains(t, message, "Could not reach provider endpoint")
}

func TestClassifyProviderTestFailure_DNSLookupFailed(t *testing.T) {
	code, category, message := classifyProviderTestFailure(errors.New("failed to send webhook: outbound request failed: dns lookup failed"))

	assert.Equal(t, "PROVIDER_TEST_DNS_FAILED", code)
	assert.Equal(t, "dispatch", category)
	assert.Contains(t, message, "DNS lookup failed")
}

func TestClassifyProviderTestFailure_ConnectionRefused(t *testing.T) {
	code, category, message := classifyProviderTestFailure(errors.New("failed to send webhook: outbound request failed: connection refused"))

	assert.Equal(t, "PROVIDER_TEST_CONNECTION_REFUSED", code)
	assert.Equal(t, "dispatch", category)
	assert.Contains(t, message, "refused the connection")
}

func TestClassifyProviderTestFailure_Timeout(t *testing.T) {
	code, category, message := classifyProviderTestFailure(errors.New("failed to send webhook: outbound request failed: request timed out"))

	assert.Equal(t, "PROVIDER_TEST_TIMEOUT", code)
	assert.Equal(t, "dispatch", category)
	assert.Contains(t, message, "timed out")
}

func TestClassifyProviderTestFailure_TLSHandshakeFailed(t *testing.T) {
	code, category, message := classifyProviderTestFailure(errors.New("failed to send webhook: outbound request failed: tls handshake failed"))

	assert.Equal(t, "PROVIDER_TEST_TLS_FAILED", code)
	assert.Equal(t, "dispatch", category)
	assert.Contains(t, message, "TLS handshake failed")
}

func TestNotificationProviderHandler_Templates(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationProviderHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)

	h.Templates(c)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "minimal")
	assert.Contains(t, w.Body.String(), "detailed")
	assert.Contains(t, w.Body.String(), "custom")
}

func TestNotificationProviderHandler_Preview_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationProviderHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Request = httptest.NewRequest("POST", "/providers/preview", bytes.NewBufferString("invalid"))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Preview(c)

	assert.Equal(t, 400, w.Code)
}

func TestNotificationProviderHandler_Preview_WithData(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationProviderHandler(svc)

	payload := map[string]any{
		"template": "minimal",
		"data": map[string]any{
			"Title":   "Custom Title",
			"Message": "Custom Message",
		},
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Request = httptest.NewRequest("POST", "/providers/preview", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Preview(c)

	assert.Equal(t, 200, w.Code)
}

func TestNotificationProviderHandler_Preview_InvalidTemplate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationProviderHandler(svc)

	payload := map[string]any{
		"template": "custom",
		"config":   "{{.Invalid",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Request = httptest.NewRequest("POST", "/providers/preview", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Preview(c)

	assert.Equal(t, 400, w.Code)
}

// Notification Template Handler Tests

func TestNotificationTemplateHandler_List_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationTemplateHandler(svc)

	// Drop table to cause error
	_ = db.Migrator().DropTable(&models.NotificationTemplate{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)

	h.List(c)

	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "failed to list templates")
}

func TestNotificationTemplateHandler_Create_BadJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationTemplateHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Request = httptest.NewRequest("POST", "/templates", bytes.NewBufferString("invalid"))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Create(c)

	assert.Equal(t, 400, w.Code)
}

func TestNotificationTemplateHandler_Create_DBError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationTemplateHandler(svc)

	// Drop table to cause error
	_ = db.Migrator().DropTable(&models.NotificationTemplate{})

	tmpl := models.NotificationTemplate{
		Name:   "Test",
		Config: `{"test": true}`,
	}
	body, _ := json.Marshal(tmpl)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Request = httptest.NewRequest("POST", "/templates", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Create(c)

	assert.Equal(t, 500, w.Code)
}

func TestNotificationTemplateHandler_Update_BadJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationTemplateHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Params = gin.Params{{Key: "id", Value: "test-id"}}
	c.Request = httptest.NewRequest("PUT", "/templates/test-id", bytes.NewBufferString("invalid"))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Update(c)

	assert.Equal(t, 400, w.Code)
}

func TestNotificationTemplateHandler_Update_DBError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationTemplateHandler(svc)

	// Drop table to cause error
	_ = db.Migrator().DropTable(&models.NotificationTemplate{})

	tmpl := models.NotificationTemplate{
		Name:   "Test",
		Config: `{"test": true}`,
	}
	body, _ := json.Marshal(tmpl)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Params = gin.Params{{Key: "id", Value: "test-id"}}
	c.Request = httptest.NewRequest("PUT", "/templates/test-id", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Update(c)

	assert.Equal(t, 500, w.Code)
}

func TestNotificationTemplateHandler_Delete_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationTemplateHandler(svc)

	// Drop table to cause error
	_ = db.Migrator().DropTable(&models.NotificationTemplate{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Params = gin.Params{{Key: "id", Value: "test-id"}}

	h.Delete(c)

	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "failed to delete template")
}

func TestNotificationTemplateHandler_Preview_BadJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationTemplateHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Request = httptest.NewRequest("POST", "/templates/preview", bytes.NewBufferString("invalid"))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Preview(c)

	assert.Equal(t, 400, w.Code)
}

func TestNotificationTemplateHandler_Preview_TemplateNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationTemplateHandler(svc)

	payload := map[string]any{
		"template_id": "nonexistent",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Request = httptest.NewRequest("POST", "/templates/preview", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Preview(c)

	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "template not found")
}

func TestNotificationTemplateHandler_Preview_WithStoredTemplate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationTemplateHandler(svc)

	// Create a template
	tmpl := &models.NotificationTemplate{
		Name:   "Test",
		Config: `{"title": "{{.Title}}"}`,
	}
	require.NoError(t, svc.CreateTemplate(tmpl))

	payload := map[string]any{
		"template_id": tmpl.ID,
		"data": map[string]any{
			"Title": "Test Title",
		},
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Request = httptest.NewRequest("POST", "/templates/preview", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Preview(c)

	assert.Equal(t, 200, w.Code)
}

func TestNotificationTemplateHandler_Preview_InvalidTemplate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationTemplateHandler(svc)

	payload := map[string]any{
		"template": "{{.Invalid",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Request = httptest.NewRequest("POST", "/templates/preview", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Preview(c)

	assert.Equal(t, 400, w.Code)
}

func TestNotificationProviderHandler_Preview_TokenWriteOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationProviderHandler(svc)

	payload := map[string]any{
		"template": "minimal",
		"token":    "secret-token-value",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Request = httptest.NewRequest("POST", "/providers/preview", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Preview(c)

	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "TOKEN_WRITE_ONLY")
}

func TestNotificationProviderHandler_Update_TypeChangeRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationProviderHandler(svc)

	existing := models.NotificationProvider{
		ID:   "update-type-test",
		Name: "Discord Provider",
		Type: "discord",
		URL:  "https://discord.com/api/webhooks/123/abc",
	}
	require.NoError(t, db.Create(&existing).Error)

	payload := map[string]any{
		"name": "Changed Type Provider",
		"type": "gotify",
		"url":  "https://gotify.example.com",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Params = gin.Params{{Key: "id", Value: "update-type-test"}}
	c.Request = httptest.NewRequest("PUT", "/providers/update-type-test", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Update(c)

	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "PROVIDER_TYPE_IMMUTABLE")
}

func TestNotificationProviderHandler_Test_MissingProviderID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationProviderHandler(svc)

	payload := map[string]any{
		"type": "discord",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Request = httptest.NewRequest("POST", "/providers/test", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Test(c)

	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "MISSING_PROVIDER_ID")
}

func TestNotificationProviderHandler_Test_ProviderNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationProviderHandler(svc)

	payload := map[string]any{
		"type": "discord",
		"id":   "nonexistent-provider",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Request = httptest.NewRequest("POST", "/providers/test", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Test(c)

	assert.Equal(t, 404, w.Code)
	assert.Contains(t, w.Body.String(), "PROVIDER_NOT_FOUND")
}

func TestNotificationProviderHandler_Test_EmptyProviderURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationProviderHandler(svc)

	existing := models.NotificationProvider{
		ID:   "empty-url-test",
		Name: "Empty URL Provider",
		Type: "discord",
		URL:  "",
	}
	require.NoError(t, db.Create(&existing).Error)

	payload := map[string]any{
		"type": "discord",
		"id":   "empty-url-test",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Request = httptest.NewRequest("POST", "/providers/test", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Test(c)

	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "PROVIDER_CONFIG_MISSING")
}

func TestIsProviderValidationError_Comprehensive(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		expect bool
	}{
		{"nil", nil, false},
		{"invalid_custom_template", errors.New("invalid custom template: missing field"), true},
		{"rendered_template", errors.New("rendered template exceeds maximum"), true},
		{"failed_to_parse", errors.New("failed to parse template: unexpected end"), true},
		{"failed_to_render", errors.New("failed to render template: missing key"), true},
		{"invalid_discord_webhook", errors.New("invalid Discord webhook URL"), true},
		{"unrelated_error", errors.New("database connection failed"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expect, isProviderValidationError(tc.err))
		})
	}
}

func TestNotificationProviderHandler_Update_UnsupportedType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationProviderHandler(svc)

	existing := models.NotificationProvider{
		ID:   "unsupported-type",
		Name: "Custom Provider",
		Type: "pushover",
		URL:  "https://pushover.example.com/test",
	}
	require.NoError(t, db.Create(&existing).Error)

	payload := map[string]any{
		"name": "Updated Pushover Provider",
		"url":  "https://pushover.example.com/updated",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Params = gin.Params{{Key: "id", Value: "unsupported-type"}}
	c.Request = httptest.NewRequest("PUT", "/providers/unsupported-type", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Update(c)

	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "UNSUPPORTED_PROVIDER_TYPE")
}

func TestNotificationProviderHandler_Update_GotifyKeepsExistingToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationProviderHandler(svc)

	existing := models.NotificationProvider{
		ID:    "gotify-keep-token",
		Name:  "Gotify Provider",
		Type:  "gotify",
		URL:   "https://gotify.example.com",
		Token: "existing-secret-token",
	}
	require.NoError(t, db.Create(&existing).Error)

	payload := map[string]any{
		"name":     "Updated Gotify",
		"url":      "https://gotify.example.com/new",
		"template": "minimal",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Params = gin.Params{{Key: "id", Value: "gotify-keep-token"}}
	c.Request = httptest.NewRequest("PUT", "/providers/gotify-keep-token", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Update(c)

	assert.Equal(t, 200, w.Code)

	var updated models.NotificationProvider
	require.NoError(t, db.Where("id = ?", "gotify-keep-token").First(&updated).Error)
	assert.Equal(t, "existing-secret-token", updated.Token)
}

func TestNotificationProviderHandler_Test_ReadDBError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupNotificationCoverageDB(t)
	svc := services.NewNotificationService(db, nil)
	h := NewNotificationProviderHandler(svc)

	_ = db.Migrator().DropTable(&models.NotificationProvider{})

	payload := map[string]any{
		"type": "discord",
		"id":   "some-provider",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Request = httptest.NewRequest("POST", "/providers/test", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Test(c)

	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "PROVIDER_READ_FAILED")
}
