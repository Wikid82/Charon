package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	glogger "gorm.io/gorm/logger"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/orthrus"
	"github.com/Wikid82/charon/backend/internal/services"
)

func openOrthrusTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: glogger.Default.LogMode(glogger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.OrthrusAgent{}))
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func newOrthrusTestSetup(t *testing.T) (*OrthrusHandler, *gorm.DB) {
	t.Helper()
	dir := t.TempDir()
	caPath := filepath.Join(dir, "ca")
	require.NoError(t, os.MkdirAll(caPath, 0o700))

	db := openOrthrusTestDB(t)
	ca, err := orthrus.NewInternalCA(caPath)
	require.NoError(t, err)
	srv, err := orthrus.NewOrthrusServer(db, ca)
	require.NoError(t, err)

	svc := services.NewOrthrusService(db, srv)
	return NewOrthrusHandler(svc), db
}

func TestOrthrusHandler_List_Empty(t *testing.T) {
	h, _ := newOrthrusTestSetup(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/management/orthrus/agents", http.NoBody)

	h.List(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var result []any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Empty(t, result)
}

func TestOrthrusHandler_Provision_Success(t *testing.T) {
	h, _ := newOrthrusTestSetup(t)

	body := `{"name":"test-agent"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/management/orthrus/agents",
		bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Provision(c)

	assert.Equal(t, http.StatusCreated, w.Code)
	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))

	assert.Contains(t, result, "agent")
	assert.Contains(t, result, "auth_key")

	authKey, ok := result["auth_key"].(string)
	require.True(t, ok)
	assert.NotEmpty(t, authKey)
	assert.Contains(t, authKey, "ch_orthrus_")

	agent, ok := result["agent"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "test-agent", agent["name"])
	// auth_key_hash must never appear in response
	assert.NotContains(t, agent, "auth_key_hash")
	// numeric id must never appear in response
	assert.NotContains(t, agent, "id")
}

func TestOrthrusHandler_Provision_MissingName(t *testing.T) {
	h, _ := newOrthrusTestSetup(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/management/orthrus/agents",
		bytes.NewBufferString(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Provision(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestOrthrusHandler_Get_Success(t *testing.T) {
	h, _ := newOrthrusTestSetup(t)

	// Provision first
	svc := services.NewOrthrusService(openOrthrusTestDB(t), nil)
	_ = svc // We'll use h.svc directly; the DB is the same

	wProv := httptest.NewRecorder()
	cProv, _ := gin.CreateTestContext(wProv)
	cProv.Request = httptest.NewRequest(http.MethodPost, "/management/orthrus/agents",
		bytes.NewBufferString(`{"name":"get-target"}`))
	cProv.Request.Header.Set("Content-Type", "application/json")
	h.Provision(cProv)
	require.Equal(t, http.StatusCreated, wProv.Code)

	var provisioned map[string]any
	require.NoError(t, json.Unmarshal(wProv.Body.Bytes(), &provisioned))
	agent := provisioned["agent"].(map[string]any)
	agentUUID := agent["uuid"].(string)

	// Now Get
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/management/orthrus/agents/"+agentUUID, http.NoBody)
	c.Params = gin.Params{{Key: "uuid", Value: agentUUID}}

	h.Get(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var got map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, agentUUID, got["uuid"])
	assert.NotContains(t, got, "auth_key_hash")
}

func TestOrthrusHandler_Get_NotFound(t *testing.T) {
	h, _ := newOrthrusTestSetup(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/management/orthrus/agents/nonexistent", http.NoBody)
	c.Params = gin.Params{{Key: "uuid", Value: "nonexistent-uuid"}}

	h.Get(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestOrthrusHandler_Delete_Success(t *testing.T) {
	h, _ := newOrthrusTestSetup(t)

	// Provision
	wProv := httptest.NewRecorder()
	cProv, _ := gin.CreateTestContext(wProv)
	cProv.Request = httptest.NewRequest(http.MethodPost, "/management/orthrus/agents",
		bytes.NewBufferString(`{"name":"delete-me"}`))
	cProv.Request.Header.Set("Content-Type", "application/json")
	h.Provision(cProv)
	require.Equal(t, http.StatusCreated, wProv.Code)

	var provisioned map[string]any
	require.NoError(t, json.Unmarshal(wProv.Body.Bytes(), &provisioned))
	agentUUID := provisioned["agent"].(map[string]any)["uuid"].(string)

	// Delete
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/management/orthrus/agents/"+agentUUID, http.NoBody)
	c.Params = gin.Params{{Key: "uuid", Value: agentUUID}}

	h.Delete(c)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestOrthrusHandler_Delete_NonExistent(t *testing.T) {
	h, _ := newOrthrusTestSetup(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/management/orthrus/agents/gone", http.NoBody)
	c.Params = gin.Params{{Key: "uuid", Value: "gone-uuid"}}

	h.Delete(c)

	// GORM soft-delete on non-existent record is a no-op (no error), 204 expected
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestOrthrusHandler_Revoke_Success(t *testing.T) {
	h, _ := newOrthrusTestSetup(t)

	// Provision
	wProv := httptest.NewRecorder()
	cProv, _ := gin.CreateTestContext(wProv)
	cProv.Request = httptest.NewRequest(http.MethodPost, "/management/orthrus/agents",
		bytes.NewBufferString(`{"name":"revoke-me"}`))
	cProv.Request.Header.Set("Content-Type", "application/json")
	h.Provision(cProv)
	require.Equal(t, http.StatusCreated, wProv.Code)

	var provisioned map[string]any
	require.NoError(t, json.Unmarshal(wProv.Body.Bytes(), &provisioned))
	agentUUID := provisioned["agent"].(map[string]any)["uuid"].(string)

	// Revoke
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/management/orthrus/agents/"+agentUUID+"/revoke", http.NoBody)
	c.Params = gin.Params{{Key: "uuid", Value: agentUUID}}

	h.Revoke(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, "revoked", result["message"])
}

func TestOrthrusHandler_Revoke_NotFound(t *testing.T) {
	h, _ := newOrthrusTestSetup(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/management/orthrus/agents/none/revoke", http.NoBody)
	c.Params = gin.Params{{Key: "uuid", Value: "none-uuid"}}

	h.Revoke(c)

	// GORM Update on non-existent record is a no-op (no error); revoke still succeeds
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestOrthrusHandler_GetInstallSnippets_Success(t *testing.T) {
	h, _ := newOrthrusTestSetup(t)

	// Provision
	wProv := httptest.NewRecorder()
	cProv, _ := gin.CreateTestContext(wProv)
	cProv.Request = httptest.NewRequest(http.MethodPost, "/management/orthrus/agents",
		bytes.NewBufferString(`{"name":"snippet-agent"}`))
	cProv.Request.Header.Set("Content-Type", "application/json")
	h.Provision(cProv)
	require.Equal(t, http.StatusCreated, wProv.Code)

	var provisioned map[string]any
	require.NoError(t, json.Unmarshal(wProv.Body.Bytes(), &provisioned))
	agentUUID := provisioned["agent"].(map[string]any)["uuid"].(string)

	// GetInstallSnippets with X-Charon-URL header
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/management/orthrus/agents/"+agentUUID+"/snippets", http.NoBody)
	c.Request.Header.Set("X-Charon-URL", "https://charon.example.com")
	c.Params = gin.Params{{Key: "uuid", Value: agentUUID}}

	h.GetInstallSnippets(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var snippets map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &snippets))

	// All snippet keys should be present
	assert.Contains(t, snippets, "docker_compose")
	assert.Contains(t, snippets, "systemd")
	assert.Contains(t, snippets, "tarball")
	assert.Contains(t, snippets, "homebrew")
	assert.Contains(t, snippets, "kubernetes_daemonset")

	// auth_key must never appear in snippets (only placeholder)
	for _, v := range snippets {
		assert.NotContains(t, v.(string), "ch_orthrus_")
		assert.Contains(t, v.(string), "<AUTH_KEY>")
	}
}

func TestOrthrusHandler_GetInstallSnippets_FallbackURL(t *testing.T) {
	h, _ := newOrthrusTestSetup(t)

	wProv := httptest.NewRecorder()
	cProv, _ := gin.CreateTestContext(wProv)
	cProv.Request = httptest.NewRequest(http.MethodPost, "/management/orthrus/agents",
		bytes.NewBufferString(`{"name":"fallback-agent"}`))
	cProv.Request.Header.Set("Content-Type", "application/json")
	h.Provision(cProv)
	require.Equal(t, http.StatusCreated, wProv.Code)

	var provisioned map[string]any
	require.NoError(t, json.Unmarshal(wProv.Body.Bytes(), &provisioned))
	agentUUID := provisioned["agent"].(map[string]any)["uuid"].(string)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/management/orthrus/agents/"+agentUUID+"/snippets", http.NoBody)
	c.Request.Host = "localhost:8080"
	c.Params = gin.Params{{Key: "uuid", Value: agentUUID}}

	h.GetInstallSnippets(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestOrthrusHandler_PatchAgent_NameOnly(t *testing.T) {
	h, _ := newOrthrusTestSetup(t)

	wProv := httptest.NewRecorder()
	cProv, _ := gin.CreateTestContext(wProv)
	cProv.Request = httptest.NewRequest(http.MethodPost, "/management/orthrus/agents",
		bytes.NewBufferString(`{"name":"original-name"}`))
	cProv.Request.Header.Set("Content-Type", "application/json")
	h.Provision(cProv)
	require.Equal(t, http.StatusCreated, wProv.Code)

	var provisioned map[string]any
	require.NoError(t, json.Unmarshal(wProv.Body.Bytes(), &provisioned))
	agentUUID := provisioned["agent"].(map[string]any)["uuid"].(string)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPatch, "/management/orthrus/agents/"+agentUUID,
		bytes.NewBufferString(`{"name":"new-name"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "uuid", Value: agentUUID}}

	h.Patch(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, "new-name", result["name"])
	assert.Equal(t, agentUUID, result["uuid"])
}

func TestOrthrusHandler_PatchAgent_TunnelFields(t *testing.T) {
	h, _ := newOrthrusTestSetup(t)

	wProv := httptest.NewRecorder()
	cProv, _ := gin.CreateTestContext(wProv)
	cProv.Request = httptest.NewRequest(http.MethodPost, "/management/orthrus/agents",
		bytes.NewBufferString(`{"name":"tunnel-agent"}`))
	cProv.Request.Header.Set("Content-Type", "application/json")
	h.Provision(cProv)
	require.Equal(t, http.StatusCreated, wProv.Code)

	var provisioned map[string]any
	require.NoError(t, json.Unmarshal(wProv.Body.Bytes(), &provisioned))
	agentUUID := provisioned["agent"].(map[string]any)["uuid"].(string)

	body := `{"hecate_tunnel_uuid":"tunnel-x","device_id":"peer-y","resolved_address":"10.0.0.1:8080"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPatch, "/management/orthrus/agents/"+agentUUID,
		bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "uuid", Value: agentUUID}}

	h.Patch(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, "tunnel-x", result["hecate_tunnel_uuid"])
	assert.Equal(t, "peer-y", result["device_id"])
	assert.Equal(t, "10.0.0.1:8080", result["resolved_address"])
	assert.Equal(t, "tunnel-agent", result["name"])
}

func TestOrthrusHandler_PatchAgent_EmptyBody(t *testing.T) {
	h, _ := newOrthrusTestSetup(t)

	wProv := httptest.NewRecorder()
	cProv, _ := gin.CreateTestContext(wProv)
	cProv.Request = httptest.NewRequest(http.MethodPost, "/management/orthrus/agents",
		bytes.NewBufferString(`{"name":"unchanged-agent"}`))
	cProv.Request.Header.Set("Content-Type", "application/json")
	h.Provision(cProv)
	require.Equal(t, http.StatusCreated, wProv.Code)

	var provisioned map[string]any
	require.NoError(t, json.Unmarshal(wProv.Body.Bytes(), &provisioned))
	agentUUID := provisioned["agent"].(map[string]any)["uuid"].(string)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPatch, "/management/orthrus/agents/"+agentUUID,
		bytes.NewBufferString(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "uuid", Value: agentUUID}}

	h.Patch(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, "unchanged-agent", result["name"])
	assert.Equal(t, agentUUID, result["uuid"])
}

func TestOrthrusHandler_PatchAgent_UnknownUUID(t *testing.T) {
	h, _ := newOrthrusTestSetup(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPatch, "/management/orthrus/agents/nonexistent",
		bytes.NewBufferString(`{"name":"any-name"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "uuid", Value: "nonexistent-uuid"}}

	h.Patch(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestOrthrusHandler_GetInstallSnippets_NotFound(t *testing.T) {
	h, _ := newOrthrusTestSetup(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/management/orthrus/agents/none/snippets", http.NoBody)
	c.Params = gin.Params{{Key: "uuid", Value: "none-uuid"}}

	h.GetInstallSnippets(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestOrthrusHandler_List_AfterProvision(t *testing.T) {
	h, _ := newOrthrusTestSetup(t)

	// Provision two agents
	for _, name := range []string{"agent-a", "agent-b"} {
		wP := httptest.NewRecorder()
		cP, _ := gin.CreateTestContext(wP)
		body := `{"name":"` + name + `"}`
		cP.Request = httptest.NewRequest(http.MethodPost, "/management/orthrus/agents",
			bytes.NewBufferString(body))
		cP.Request.Header.Set("Content-Type", "application/json")
		h.Provision(cP)
		require.Equal(t, http.StatusCreated, wP.Code)
	}

	// List
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/management/orthrus/agents", http.NoBody)

	h.List(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var agents []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &agents))
	assert.Len(t, agents, 2)
	for _, a := range agents {
		assert.NotContains(t, a, "auth_key_hash", "auth_key_hash must never appear in list response")
		assert.NotContains(t, a, "auth_key", "auth_key must never appear in list response")
		assert.NotContains(t, a, "id", "numeric id must never appear in list response")
	}
}

func TestOrthrusHandler_RegisterRoutes(t *testing.T) {
	h, _ := newOrthrusTestSetup(t)

	r := gin.New()
	group := r.Group("/management")
	h.RegisterRoutes(group)

	routes := r.Routes()
	paths := make(map[string]bool)
	for _, route := range routes {
		paths[route.Method+" "+route.Path] = true
	}

	assert.True(t, paths["GET /management/orthrus/agents"])
	assert.True(t, paths["POST /management/orthrus/agents"])
	assert.True(t, paths["GET /management/orthrus/agents/:uuid"])
	assert.True(t, paths["PATCH /management/orthrus/agents/:uuid"])
	assert.True(t, paths["DELETE /management/orthrus/agents/:uuid"])
	assert.True(t, paths["POST /management/orthrus/agents/:uuid/revoke"])
	assert.True(t, paths["GET /management/orthrus/agents/:uuid/snippets"])
}

func TestOrthrusHandler_List_InternalError(t *testing.T) {
	h, db := newOrthrusTestSetup(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	_ = sqlDB.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/management/orthrus/agents", http.NoBody)
	h.List(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestOrthrusHandler_Provision_InternalError(t *testing.T) {
	h, db := newOrthrusTestSetup(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	_ = sqlDB.Close()

	body, _ := json.Marshal(map[string]string{"name": "agent-x"})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/management/orthrus/agents", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	h.Provision(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestOrthrusHandler_Delete_InternalError(t *testing.T) {
	h, db := newOrthrusTestSetup(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	_ = sqlDB.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/management/orthrus/agents/uuid-x", http.NoBody)
	c.Params = gin.Params{{Key: "uuid", Value: "uuid-x"}}
	h.Delete(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestOrthrusHandler_Revoke_InternalError(t *testing.T) {
	h, db := newOrthrusTestSetup(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	_ = sqlDB.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/management/orthrus/agents/uuid-x/revoke", http.NoBody)
	c.Params = gin.Params{{Key: "uuid", Value: "uuid-x"}}
	h.Revoke(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
