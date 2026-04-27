package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	glogger "gorm.io/gorm/logger"

	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/hecate"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

func openHecateTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: glogger.Default.LogMode(glogger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.TunnelConfig{}))
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func newHecateEncSvc(t *testing.T) *crypto.EncryptionService {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	svc, err := crypto.NewEncryptionService(base64.StdEncoding.EncodeToString(key))
	require.NoError(t, err)
	return svc
}

func newHecateTestSetup(t *testing.T) (*HecateHandler, *services.HecateService) {
	t.Helper()
	db := openHecateTestDB(t)
	encSvc := newHecateEncSvc(t)
	mgr := hecate.NewTunnelManager(db, encSvc)
	svc := services.NewHecateService(db, encSvc, mgr)
	return NewHecateHandler(svc), svc
}

// TestHecateHandler_GetStatus verifies status returns an array (empty or not).
func TestHecateHandler_GetStatus(t *testing.T) {
	h, _ := newHecateTestSetup(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/management/hecate/status", http.NoBody)

	h.GetStatus(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var result []any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.NotNil(t, result)
}

// TestHecateHandler_List_Empty verifies an empty list is returned when no configs exist.
func TestHecateHandler_List_Empty(t *testing.T) {
	h, _ := newHecateTestSetup(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/management/hecate/tunnels", http.NoBody)

	h.List(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var result []any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Empty(t, result)
}

// TestHecateHandler_Create_Success verifies a tunnel config is persisted.
func TestHecateHandler_Create_Success(t *testing.T) {
	h, _ := newHecateTestSetup(t)

	body := `{"name":"cf-tunnel","provider":"cloudflare","credentials":"{\"api_token\":\"tok\",\"account_id\":\"acc\"}"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/management/hecate/tunnels",
		bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Create(c)

	assert.Equal(t, http.StatusCreated, w.Code)
	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, "cf-tunnel", result["name"])
	assert.Equal(t, "cloudflare", result["provider"])
	// encrypted credentials must never appear in response
	assert.NotContains(t, result, "encrypted_credentials")
}

// TestHecateHandler_Create_MissingRequired verifies binding validation.
func TestHecateHandler_Create_MissingRequired(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"missing name", `{"provider":"cloudflare","credentials":"x"}`},
		{"missing provider", `{"name":"t","credentials":"x"}`},
		{"missing credentials", `{"name":"t","provider":"cloudflare"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := newHecateTestSetup(t)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/management/hecate/tunnels",
				bytes.NewBufferString(tc.body))
			c.Request.Header.Set("Content-Type", "application/json")

			h.Create(c)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

// TestHecateHandler_Get_Success verifies a single config is returned by UUID.
func TestHecateHandler_Get_Success(t *testing.T) {
	h, _ := newHecateTestSetup(t)

	// Create
	wC := httptest.NewRecorder()
	cC, _ := gin.CreateTestContext(wC)
	cC.Request = httptest.NewRequest(http.MethodPost, "/management/hecate/tunnels",
		bytes.NewBufferString(`{"name":"get-me","provider":"netbird","credentials":"creds"}`))
	cC.Request.Header.Set("Content-Type", "application/json")
	h.Create(cC)
	require.Equal(t, http.StatusCreated, wC.Code)

	var created map[string]any
	require.NoError(t, json.Unmarshal(wC.Body.Bytes(), &created))
	uuid := created["uuid"].(string)

	// Get
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/management/hecate/tunnels/"+uuid, http.NoBody)
	c.Params = gin.Params{{Key: "uuid", Value: uuid}}

	h.Get(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var got map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, uuid, got["uuid"])
}

// TestHecateHandler_Get_NotFound verifies 404 on unknown UUID.
func TestHecateHandler_Get_NotFound(t *testing.T) {
	h, _ := newHecateTestSetup(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/management/hecate/tunnels/nonexistent", http.NoBody)
	c.Params = gin.Params{{Key: "uuid", Value: "nonexistent-uuid"}}

	h.Get(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestHecateHandler_Update_Success verifies an existing config is updated.
func TestHecateHandler_Update_Success(t *testing.T) {
	h, _ := newHecateTestSetup(t)

	// Create
	wC := httptest.NewRecorder()
	cC, _ := gin.CreateTestContext(wC)
	cC.Request = httptest.NewRequest(http.MethodPost, "/management/hecate/tunnels",
		bytes.NewBufferString(`{"name":"upd-me","provider":"netbird","credentials":"orig"}`))
	cC.Request.Header.Set("Content-Type", "application/json")
	h.Create(cC)
	require.Equal(t, http.StatusCreated, wC.Code)

	var created map[string]any
	require.NoError(t, json.Unmarshal(wC.Body.Bytes(), &created))
	uuid := created["uuid"].(string)

	// Update
	updateBody := `{"name":"upd-me-v2","provider":"netbird","is_active":false}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/management/hecate/tunnels/"+uuid,
		bytes.NewBufferString(updateBody))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "uuid", Value: uuid}}

	h.Update(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestHecateHandler_Update_NotFound verifies 500 on missing UUID.
func TestHecateHandler_Update_NotFound(t *testing.T) {
	h, _ := newHecateTestSetup(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/management/hecate/tunnels/ghost",
		bytes.NewBufferString(`{"name":"x","provider":"netbird"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "uuid", Value: "ghost-uuid"}}

	h.Update(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestHecateHandler_Delete_Success verifies a config is removed.
func TestHecateHandler_Delete_Success(t *testing.T) {
	h, _ := newHecateTestSetup(t)

	// Create
	wC := httptest.NewRecorder()
	cC, _ := gin.CreateTestContext(wC)
	cC.Request = httptest.NewRequest(http.MethodPost, "/management/hecate/tunnels",
		bytes.NewBufferString(`{"name":"del-me","provider":"zerotier","credentials":"c"}`))
	cC.Request.Header.Set("Content-Type", "application/json")
	h.Create(cC)
	require.Equal(t, http.StatusCreated, wC.Code)

	var created map[string]any
	require.NoError(t, json.Unmarshal(wC.Body.Bytes(), &created))
	uuid := created["uuid"].(string)

	// Delete
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/management/hecate/tunnels/"+uuid, http.NoBody)
	c.Params = gin.Params{{Key: "uuid", Value: uuid}}

	h.Delete(c)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

// TestHecateHandler_RotateCredentials_Success verifies rotation on a non-active tunnel.
func TestHecateHandler_RotateCredentials_Success(t *testing.T) {
	h, _ := newHecateTestSetup(t)

	// Create non-active
	wC := httptest.NewRecorder()
	cC, _ := gin.CreateTestContext(wC)
	cC.Request = httptest.NewRequest(http.MethodPost, "/management/hecate/tunnels",
		bytes.NewBufferString(`{"name":"rot-me","provider":"tailscale","credentials":"old","is_active":false}`))
	cC.Request.Header.Set("Content-Type", "application/json")
	h.Create(cC)
	require.Equal(t, http.StatusCreated, wC.Code)

	var created map[string]any
	require.NoError(t, json.Unmarshal(wC.Body.Bytes(), &created))
	uuid := created["uuid"].(string)

	// Rotate
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/management/hecate/tunnels/"+uuid+"/rotate-credentials",
		bytes.NewBufferString(`{"credentials":"newcreds"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "uuid", Value: uuid}}

	h.RotateCredentials(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestHecateHandler_RotateCredentials_MissingBody verifies binding validation.
func TestHecateHandler_RotateCredentials_MissingBody(t *testing.T) {
	h, _ := newHecateTestSetup(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/management/hecate/tunnels/uuid/rotate-credentials",
		bytes.NewBufferString(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "uuid", Value: "uuid"}}

	h.RotateCredentials(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestHecateHandler_Start_NoTunnel verifies error when tunnel UUID is not managed.
func TestHecateHandler_Start_NoTunnel(t *testing.T) {
	h, _ := newHecateTestSetup(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/management/hecate/tunnels/none/start", http.NoBody)
	c.Params = gin.Params{{Key: "uuid", Value: "no-such-uuid"}}

	h.Start(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestHecateHandler_Stop_NoTunnel verifies graceful handling when tunnel is not running.
func TestHecateHandler_Stop_NoTunnel(t *testing.T) {
	h, _ := newHecateTestSetup(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/management/hecate/tunnels/none/stop", http.NoBody)
	c.Params = gin.Params{{Key: "uuid", Value: "no-such-uuid"}}

	h.Stop(c)

	// StopTunnel on unknown UUID is a no-op (no error)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestHecateHandler_ListCloudflareTunnels_NoProvider verifies 503 when no active provider.
func TestHecateHandler_ListCloudflareTunnels_NoProvider(t *testing.T) {
	h, _ := newHecateTestSetup(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/management/hecate/cloudflare/tunnels", http.NoBody)

	h.ListCloudflareTunnels(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestHecateHandler_GetCloudflaredConfig_NotFound verifies 404 on unknown UUID.
func TestHecateHandler_GetCloudflaredConfig_NotFound(t *testing.T) {
	h, _ := newHecateTestSetup(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/management/hecate/tunnels/x/config/cloudflared", http.NoBody)
	c.Params = gin.Params{{Key: "uuid", Value: "no-uuid"}}

	h.GetCloudflaredConfig(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestHecateHandler_GetCloudflaredConfig_WrongProvider verifies 400 for non-CF tunnel.
func TestHecateHandler_GetCloudflaredConfig_WrongProvider(t *testing.T) {
	h, _ := newHecateTestSetup(t)

	// Create a NetBird tunnel
	wC := httptest.NewRecorder()
	cC, _ := gin.CreateTestContext(wC)
	cC.Request = httptest.NewRequest(http.MethodPost, "/management/hecate/tunnels",
		bytes.NewBufferString(`{"name":"nb","provider":"netbird","credentials":"c"}`))
	cC.Request.Header.Set("Content-Type", "application/json")
	h.Create(cC)
	require.Equal(t, http.StatusCreated, wC.Code)

	var created map[string]any
	require.NoError(t, json.Unmarshal(wC.Body.Bytes(), &created))
	uuid := created["uuid"].(string)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/management/hecate/tunnels/"+uuid+"/config/cloudflared", http.NoBody)
	c.Params = gin.Params{{Key: "uuid", Value: uuid}}

	h.GetCloudflaredConfig(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestHecateHandler_GetCloudflaredConfig_Success verifies YAML is returned for a CF tunnel.
func TestHecateHandler_GetCloudflaredConfig_Success(t *testing.T) {
	h, _ := newHecateTestSetup(t)

	// Create a Cloudflare tunnel
	wC := httptest.NewRecorder()
	cC, _ := gin.CreateTestContext(wC)
	cC.Request = httptest.NewRequest(http.MethodPost, "/management/hecate/tunnels",
		bytes.NewBufferString(`{"name":"cf","provider":"cloudflare","credentials":"c"}`))
	cC.Request.Header.Set("Content-Type", "application/json")
	h.Create(cC)
	require.Equal(t, http.StatusCreated, wC.Code)

	var created map[string]any
	require.NoError(t, json.Unmarshal(wC.Body.Bytes(), &created))
	uuid := created["uuid"].(string)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/management/hecate/tunnels/"+uuid+"/config/cloudflared", http.NoBody)
	c.Params = gin.Params{{Key: "uuid", Value: uuid}}

	h.GetCloudflaredConfig(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/yaml")
}

// TestHecateHandler_ListTailscaleDevices_NoProvider verifies 503 when no active provider.
func TestHecateHandler_ListTailscaleDevices_NoProvider(t *testing.T) {
	h, _ := newHecateTestSetup(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/management/hecate/tailscale/devices", http.NoBody)

	h.ListTailscaleDevices(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestHecateHandler_SyncTailscale_NoProvider verifies 503 when no active provider.
func TestHecateHandler_SyncTailscale_NoProvider(t *testing.T) {
	h, _ := newHecateTestSetup(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/management/hecate/tailscale/sync", http.NoBody)

	h.SyncTailscale(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestHecateHandler_ListZeroTierNetworks_NoProvider verifies 503 when no active provider.
func TestHecateHandler_ListZeroTierNetworks_NoProvider(t *testing.T) {
	h, _ := newHecateTestSetup(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/management/hecate/zerotier/networks", http.NoBody)

	h.ListZeroTierNetworks(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestHecateHandler_ListZeroTierMembers_NoProvider verifies 503 when no active provider.
func TestHecateHandler_ListZeroTierMembers_NoProvider(t *testing.T) {
	h, _ := newHecateTestSetup(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/management/hecate/zerotier/networks/net1/members", http.NoBody)
	c.Params = gin.Params{{Key: "network_id", Value: "net1"}}

	h.ListZeroTierMembers(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestHecateHandler_ListNetBirdPeers_NoProvider verifies 503 when no active provider.
func TestHecateHandler_ListNetBirdPeers_NoProvider(t *testing.T) {
	h, _ := newHecateTestSetup(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/management/hecate/netbird/peers", http.NoBody)

	h.ListNetBirdPeers(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestHecateHandler_SyncNetBird_NoProvider verifies 503 when no active provider.
func TestHecateHandler_SyncNetBird_NoProvider(t *testing.T) {
	h, _ := newHecateTestSetup(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/management/hecate/netbird/sync", http.NoBody)

	h.SyncNetBird(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestHecateHandler_RegisterRoutes verifies all routes are registered.
func TestHecateHandler_RegisterRoutes(t *testing.T) {
	h, _ := newHecateTestSetup(t)

	r := gin.New()
	group := r.Group("/management")
	h.RegisterRoutes(group)

	routes := r.Routes()
	paths := make(map[string]bool)
	for _, route := range routes {
		paths[route.Method+" "+route.Path] = true
	}

	expected := []string{
		"GET /management/hecate/status",
		"GET /management/hecate/tunnels",
		"POST /management/hecate/tunnels",
		"GET /management/hecate/tunnels/:uuid",
		"PUT /management/hecate/tunnels/:uuid",
		"DELETE /management/hecate/tunnels/:uuid",
		"POST /management/hecate/tunnels/:uuid/start",
		"POST /management/hecate/tunnels/:uuid/stop",
		"POST /management/hecate/tunnels/:uuid/rotate-credentials",
		"GET /management/hecate/cloudflare/tunnels",
		"GET /management/hecate/tunnels/:uuid/config/cloudflared",
		"GET /management/hecate/tailscale/devices",
		"POST /management/hecate/tailscale/sync",
		"GET /management/hecate/zerotier/networks",
		"GET /management/hecate/zerotier/networks/:network_id/members",
		"GET /management/hecate/netbird/peers",
		"POST /management/hecate/netbird/sync",
	}

	for _, want := range expected {
		assert.True(t, paths[want], "missing route: %s", want)
	}
}

// TestHecateHandler_List_AfterCreate verifies the list grows after creation.
func TestHecateHandler_List_AfterCreate(t *testing.T) {
	h, _ := newHecateTestSetup(t)

	for i, provider := range []string{"cloudflare", "netbird"} {
		wC := httptest.NewRecorder()
		cC, _ := gin.CreateTestContext(wC)
		body := map[string]string{
			"name":        "tunnel-" + provider,
			"provider":    provider,
			"credentials": "creds",
		}
		bodyBytes, _ := json.Marshal(body)
		cC.Request = httptest.NewRequest(http.MethodPost, "/management/hecate/tunnels",
			bytes.NewBuffer(bodyBytes))
		cC.Request.Header.Set("Content-Type", "application/json")
		h.Create(cC)
		require.Equal(t, http.StatusCreated, wC.Code, "create %d failed", i)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/management/hecate/tunnels", http.NoBody)

	h.List(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var configs []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &configs))
	assert.Len(t, configs, 2)
}
