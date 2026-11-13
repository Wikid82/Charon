package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/services"
)

// provisionForRotate creates an agent through the handler and returns its UUID
// and the one-time key.
func provisionForRotate(t *testing.T, h *OrthrusHandler, name string) (uuid, authKey string) {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/management/orthrus/agents",
		bytes.NewBufferString(`{"name":"`+name+`"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	h.Provision(c)
	require.Equal(t, http.StatusCreated, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	uuid = body["agent"].(map[string]any)["uuid"].(string)
	authKey = body["auth_key"].(string)
	return uuid, authKey
}

func callRotateKey(t *testing.T, h *OrthrusHandler, uuid string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/management/orthrus/agents/"+uuid+"/rotate-key", http.NoBody)
	c.Params = gin.Params{{Key: "uuid", Value: uuid}}
	h.RotateKey(c)
	return w
}

func TestOrthrusHandler_RotateKey_ReturnsNewKeyOnce(t *testing.T) {
	h, _ := newOrthrusTestSetup(t)
	uuid, oldKey := provisionForRotate(t, h, "rotate-handler")

	w := callRotateKey(t, h, uuid)

	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	newKey, ok := body["auth_key"].(string)
	require.True(t, ok, "response must carry auth_key")
	assert.True(t, strings.HasPrefix(newKey, "ch_orthrus_"))
	assert.NotEqual(t, oldKey, newKey)
	assert.Len(t, body, 1, "response must contain only auth_key")
}

func TestOrthrusHandler_RotateKey_SetsNoStore(t *testing.T) {
	h, _ := newOrthrusTestSetup(t)
	uuid, _ := provisionForRotate(t, h, "no-store")

	w := callRotateKey(t, h, uuid)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
}

func TestOrthrusHandler_RotateKey_UnknownAgentReturnsNotFound(t *testing.T) {
	h, _ := newOrthrusTestSetup(t)

	w := callRotateKey(t, h, "missing-agent")

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "agent not found", body["error"])
	_, hasKey := body["auth_key"]
	assert.False(t, hasKey)
}

func TestOrthrusHandler_RotateKey_ServiceErrorReturnsInternalError(t *testing.T) {
	h, db := newOrthrusTestSetup(t)
	uuid, _ := provisionForRotate(t, h, "db-broken")

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	w := callRotateKey(t, h, uuid)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	_, hasKey := body["auth_key"]
	assert.False(t, hasKey)
}

func TestOrthrusHandler_RotateKey_AuditEntryOmitsKey(t *testing.T) {
	h, securityService := newOrthrusTestSetupWithSecurity(t)
	uuid, _ := provisionForRotate(t, h, "audited-agent")

	w := callRotateKey(t, h, uuid)
	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	newKey := body["auth_key"]

	securityService.Flush()
	entries, total, err := securityService.ListAuditLogs(services.AuditLogFilter{
		ResourceUUID: uuid,
	}, 1, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)

	entry := entries[0]
	assert.Equal(t, "orthrus_key_rotated", entry.Action)
	assert.Equal(t, "orthrus_agent", entry.EventCategory)
	assert.Equal(t, uuid, entry.ResourceUUID)
	assert.Contains(t, entry.Details, "audited-agent")
	assert.NotContains(t, entry.Details, newKey)
	assert.NotContains(t, entry.Details, "ch_orthrus_")
}

func TestOrthrusHandler_RotateKey_NoAuditOnFailure(t *testing.T) {
	h, securityService := newOrthrusTestSetupWithSecurity(t)

	w := callRotateKey(t, h, "missing-agent")
	require.Equal(t, http.StatusNotFound, w.Code)

	securityService.Flush()
	_, total, err := securityService.ListAuditLogs(services.AuditLogFilter{
		EventCategory: "orthrus_agent",
	}, 1, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 0, total)
}

func TestOrthrusHandler_RegisterRoutes_RotateKeyIsAdminOnly(t *testing.T) {
	h, _ := newOrthrusTestSetup(t)

	r := gin.New()
	read := r.Group("/read")
	admin := r.Group("/admin")
	h.RegisterRoutes(read, admin)

	paths := make(map[string]bool)
	for _, route := range r.Routes() {
		paths[route.Method+" "+route.Path] = true
	}
	assert.True(t, paths["POST /admin/orthrus/agents/:uuid/rotate-key"])
	assert.False(t, paths["POST /read/orthrus/agents/:uuid/rotate-key"])
}
