package handlers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	glogger "gorm.io/gorm/logger"

	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/hecate"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"

	cfprovider "github.com/Wikid82/charon/backend/internal/hecate/providers/cloudflare"
	nbprovider "github.com/Wikid82/charon/backend/internal/hecate/providers/netbird"
	tsprovider "github.com/Wikid82/charon/backend/internal/hecate/providers/tailscale"
	ztprovider "github.com/Wikid82/charon/backend/internal/hecate/providers/zerotier"
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

// TestHecateHandler_Update_NotFound verifies 404 on missing UUID.
func TestHecateHandler_Update_NotFound(t *testing.T) {
	h, _ := newHecateTestSetup(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/management/hecate/tunnels/ghost",
		bytes.NewBufferString(`{"name":"x","provider":"netbird"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "uuid", Value: "ghost-uuid"}}

	h.Update(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
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

// TestHecateHandler_Start_NoTunnel verifies 404 when tunnel UUID does not exist in DB.
func TestHecateHandler_Start_NoTunnel(t *testing.T) {
	h, _ := newHecateTestSetup(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/management/hecate/tunnels/none/start", http.NoBody)
	c.Params = gin.Params{{Key: "uuid", Value: "no-such-uuid"}}

	h.Start(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
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

// ---- Mock providers for coverage tests ----

// testNopProvider implements hecate.TunnelProvider and always succeeds.
type testNopProvider struct{}

func (p *testNopProvider) Name() string                  { return "nop" }
func (p *testNopProvider) Status() hecate.TunnelState    { return hecate.TunnelStateConnected }
func (p *testNopProvider) Start(_ context.Context) error { return nil }
func (p *testNopProvider) Stop() error                   { return nil }
func (p *testNopProvider) GetAddress() string            { return "" }

// testErrStopProvider implements hecate.TunnelProvider but fails on Stop.
type testErrStopProvider struct{}

func (p *testErrStopProvider) Name() string                  { return "errstop" }
func (p *testErrStopProvider) Status() hecate.TunnelState    { return hecate.TunnelStateConnected }
func (p *testErrStopProvider) Start(_ context.Context) error { return nil }
func (p *testErrStopProvider) Stop() error                   { return fmt.Errorf("intentional stop error") }
func (p *testErrStopProvider) GetAddress() string            { return "" }

// newHecateTestSetupWithDB is identical to newHecateTestSetup but also returns
// the raw *gorm.DB so tests can close it to trigger DB error paths.
func newHecateTestSetupWithDB(t *testing.T) (*HecateHandler, *services.HecateService, *gorm.DB) {
	t.Helper()
	db := openHecateTestDB(t)
	encSvc := newHecateEncSvc(t)
	mgr := hecate.NewTunnelManager(db, encSvc)
	svc := services.NewHecateService(db, encSvc, mgr)
	return NewHecateHandler(svc), svc, db
}

// startRunningTunnel registers factory, creates a tunnel via the handler, and
// starts it in the manager so GetProviderByType returns a live provider instance.
func startRunningTunnel(t *testing.T, h *HecateHandler, svc *services.HecateService, provider models.TunnelProviderType, factory hecate.ProviderFactory) string {
	t.Helper()
	svc.GetManager().RegisterFactory(provider, factory)

	wC := httptest.NewRecorder()
	cC, _ := gin.CreateTestContext(wC)
	body := fmt.Sprintf(`{"name":"running","provider":"%s","credentials":"{}"}`, provider)
	cC.Request = httptest.NewRequest(http.MethodPost, "/management/hecate/tunnels", bytes.NewBufferString(body))
	cC.Request.Header.Set("Content-Type", "application/json")
	h.Create(cC)
	require.Equal(t, http.StatusCreated, wC.Code, "startRunningTunnel: Create failed: %s", wC.Body.String())

	var created map[string]any
	require.NoError(t, json.Unmarshal(wC.Body.Bytes(), &created))
	tunnelUUID := created["uuid"].(string)
	require.NoError(t, svc.GetManager().StartTunnel(tunnelUUID))
	return tunnelUUID
}

// ---- Coverage tests: error paths in HecateHandler ----

func TestHecateHandler_List_DBError(t *testing.T) {
	h, _, db := newHecateTestSetupWithDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/management/hecate/tunnels", http.NoBody)
	h.List(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHecateHandler_Create_ServiceError(t *testing.T) {
	h, _, db := newHecateTestSetupWithDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	body := `{"name":"x","provider":"cloudflare","credentials":"creds"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/management/hecate/tunnels", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	h.Create(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHecateHandler_Update_BindingError(t *testing.T) {
	h, _ := newHecateTestSetup(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/management/hecate/tunnels/uuid", bytes.NewBufferString(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "uuid", Value: "some-uuid"}}
	h.Update(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHecateHandler_Delete_ServiceError(t *testing.T) {
	h, _, db := newHecateTestSetupWithDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/management/hecate/tunnels/uuid", http.NoBody)
	c.Params = gin.Params{{Key: "uuid", Value: "some-uuid"}}
	h.Delete(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHecateHandler_Stop_RunningTunnelStopError(t *testing.T) {
	h, svc := newHecateTestSetup(t)
	errFactory := hecate.ProviderFactory(func(_ *models.TunnelConfig, _ string) (hecate.TunnelProvider, error) {
		return &testErrStopProvider{}, nil
	})
	tunnelUUID := startRunningTunnel(t, h, svc, models.ProviderCloudflare, errFactory)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/management/hecate/tunnels/"+tunnelUUID+"/stop", http.NoBody)
	c.Params = gin.Params{{Key: "uuid", Value: tunnelUUID}}
	h.Stop(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHecateHandler_RotateCredentials_ServiceError(t *testing.T) {
	h, svc := newHecateTestSetup(t)
	errFactory := hecate.ProviderFactory(func(_ *models.TunnelConfig, _ string) (hecate.TunnelProvider, error) {
		return &testErrStopProvider{}, nil
	})
	tunnelUUID := startRunningTunnel(t, h, svc, models.ProviderCloudflare, errFactory)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"credentials":"newcreds"}`
	c.Request = httptest.NewRequest(http.MethodPost, "/management/hecate/tunnels/"+tunnelUUID+"/rotate-credentials", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "uuid", Value: tunnelUUID}}
	h.RotateCredentials(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ---- Coverage tests: "unexpected provider type" in provider query handlers ----

func TestHecateHandler_ListCloudflareTunnels_WrongProviderType(t *testing.T) {
	h, svc := newHecateTestSetup(t)
	nopFactory := hecate.ProviderFactory(func(_ *models.TunnelConfig, _ string) (hecate.TunnelProvider, error) {
		return &testNopProvider{}, nil
	})
	startRunningTunnel(t, h, svc, models.ProviderCloudflare, nopFactory)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/management/hecate/cloudflare/tunnels", http.NoBody)
	h.ListCloudflareTunnels(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHecateHandler_ListTailscaleDevices_WrongProviderType(t *testing.T) {
	h, svc := newHecateTestSetup(t)
	nopFactory := hecate.ProviderFactory(func(_ *models.TunnelConfig, _ string) (hecate.TunnelProvider, error) {
		return &testNopProvider{}, nil
	})
	startRunningTunnel(t, h, svc, models.ProviderTailscale, nopFactory)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/management/hecate/tailscale/devices", http.NoBody)
	h.ListTailscaleDevices(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHecateHandler_SyncTailscale_WrongProviderType(t *testing.T) {
	h, svc := newHecateTestSetup(t)
	nopFactory := hecate.ProviderFactory(func(_ *models.TunnelConfig, _ string) (hecate.TunnelProvider, error) {
		return &testNopProvider{}, nil
	})
	startRunningTunnel(t, h, svc, models.ProviderTailscale, nopFactory)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/management/hecate/tailscale/sync", http.NoBody)
	h.SyncTailscale(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHecateHandler_ListZeroTierNetworks_WrongProviderType(t *testing.T) {
	h, svc := newHecateTestSetup(t)
	nopFactory := hecate.ProviderFactory(func(_ *models.TunnelConfig, _ string) (hecate.TunnelProvider, error) {
		return &testNopProvider{}, nil
	})
	startRunningTunnel(t, h, svc, models.ProviderZeroTier, nopFactory)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/management/hecate/zerotier/networks", http.NoBody)
	h.ListZeroTierNetworks(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHecateHandler_ListZeroTierMembers_WrongProviderType(t *testing.T) {
	h, svc := newHecateTestSetup(t)
	nopFactory := hecate.ProviderFactory(func(_ *models.TunnelConfig, _ string) (hecate.TunnelProvider, error) {
		return &testNopProvider{}, nil
	})
	startRunningTunnel(t, h, svc, models.ProviderZeroTier, nopFactory)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/management/hecate/zerotier/networks/net1/members", http.NoBody)
	c.Params = gin.Params{{Key: "network_id", Value: "net1"}}
	h.ListZeroTierMembers(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHecateHandler_ListNetBirdPeers_WrongProviderType(t *testing.T) {
	h, svc := newHecateTestSetup(t)
	nopFactory := hecate.ProviderFactory(func(_ *models.TunnelConfig, _ string) (hecate.TunnelProvider, error) {
		return &testNopProvider{}, nil
	})
	startRunningTunnel(t, h, svc, models.ProviderNetBird, nopFactory)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/management/hecate/netbird/peers", http.NoBody)
	h.ListNetBirdPeers(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHecateHandler_SyncNetBird_WrongProviderType(t *testing.T) {
	h, svc := newHecateTestSetup(t)
	nopFactory := hecate.ProviderFactory(func(_ *models.TunnelConfig, _ string) (hecate.TunnelProvider, error) {
		return &testNopProvider{}, nil
	})
	startRunningTunnel(t, h, svc, models.ProviderNetBird, nopFactory)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/management/hecate/netbird/sync", http.NoBody)
	h.SyncNetBird(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ---- Coverage tests: HecateWSHandler ----

func TestHecateWSHandler_New(t *testing.T) {
	_, svc := newHecateTestSetup(t)
	tracker := services.NewWebSocketTracker()
	wsHandler := NewHecateWSHandler(svc, tracker)
	require.NotNil(t, wsHandler)

	wsHandlerNoTracker := NewHecateWSHandler(svc, nil)
	require.NotNil(t, wsHandlerNoTracker)
}

func TestHecateWSHandler_StreamLogs_TunnelNotFound(t *testing.T) {
	_, svc := newHecateTestSetup(t)
	wsHandler := NewHecateWSHandler(svc, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/ws/tunnels/nonexistent/logs", http.NoBody)
	c.Params = gin.Params{{Key: "uuid", Value: "nonexistent-uuid"}}
	wsHandler.StreamLogs(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHecateWSHandler_StreamLogs_UpgradeAndStream(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h, svc := newHecateTestSetup(t)
	nopFactory := hecate.ProviderFactory(func(_ *models.TunnelConfig, _ string) (hecate.TunnelProvider, error) {
		return &testNopProvider{}, nil
	})
	tunnelUUID := startRunningTunnel(t, h, svc, models.ProviderCloudflare, nopFactory)

	tracker := services.NewWebSocketTracker()
	wsHandler := NewHecateWSHandler(svc, tracker)

	r := gin.New()
	r.GET("/ws/tunnels/:uuid/logs", wsHandler.StreamLogs)

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	wsURL := toWebSocketURL(srv.URL) + "/ws/tunnels/" + tunnelUUID + "/logs"
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)

	waitFor(t, 2*time.Second, func() bool {
		return tracker.GetCount() == 1
	})

	require.NoError(t, conn.Close())

	waitFor(t, 2*time.Second, func() bool {
		return tracker.GetCount() == 0
	})
}

// ---- Coverage tests: new paths added for coverage improvement ----

// testErrStartProvider implements hecate.TunnelProvider but fails on Start.
type testErrStartProvider struct{}

func (p *testErrStartProvider) Name() string               { return "errstart" }
func (p *testErrStartProvider) Status() hecate.TunnelState { return hecate.TunnelStateStopped }
func (p *testErrStartProvider) Start(_ context.Context) error {
	return fmt.Errorf("intentional start error")
}
func (p *testErrStartProvider) Stop() error        { return nil }
func (p *testErrStartProvider) GetAddress() string { return "" }

// TestHecateHandler_Start_StartError verifies 500 when the provider Start fails.
func TestHecateHandler_Start_StartError(t *testing.T) {
	h, svc, db := newHecateTestSetupWithDB(t)

	// Create a tunnel in DB via the handler.
	wC := httptest.NewRecorder()
	cC, _ := gin.CreateTestContext(wC)
	cC.Request = httptest.NewRequest(http.MethodPost, "/management/hecate/tunnels",
		bytes.NewBufferString(`{"name":"err-start","provider":"netbird","credentials":"c"}`))
	cC.Request.Header.Set("Content-Type", "application/json")
	h.Create(cC)
	require.Equal(t, http.StatusCreated, wC.Code)

	var created map[string]any
	require.NoError(t, json.Unmarshal(wC.Body.Bytes(), &created))
	tunnelUUID := created["uuid"].(string)

	// Register a factory that returns an error-starting provider.
	svc.GetManager().RegisterFactory(models.ProviderNetBird, hecate.ProviderFactory(func(_ *models.TunnelConfig, _ string) (hecate.TunnelProvider, error) {
		return &testErrStartProvider{}, nil
	}))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/management/hecate/tunnels/"+tunnelUUID+"/start", http.NoBody)
	c.Params = gin.Params{{Key: "uuid", Value: tunnelUUID}}
	h.Start(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	_ = db
}

// TestHecateHandler_RotateCredentials_NotFound verifies 404 on a missing UUID.
func TestHecateHandler_RotateCredentials_NotFound(t *testing.T) {
	h, _ := newHecateTestSetup(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/management/hecate/tunnels/ghost/rotate-credentials",
		bytes.NewBufferString(`{"credentials":"newcreds"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "uuid", Value: "ghost-uuid-404"}}
	h.RotateCredentials(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestHecateHandler_ListZeroTierNetworks_NilClient verifies 503 when the ZeroTier
// client has not been initialized (provider not yet started).
func TestHecateHandler_ListZeroTierNetworks_NilClient(t *testing.T) {
	h, svc := newHecateTestSetup(t)

	cfg := &models.TunnelConfig{UUID: "zt-net-uuid", Provider: models.ProviderZeroTier}
	ztProv, err := ztprovider.NewZeroTierProvider(cfg, `{"api_token":"test"}`)
	require.NoError(t, err)
	svc.GetManager().RegisterProvider("zt-net-uuid", ztProv)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/management/hecate/zerotier/networks", http.NoBody)
	h.ListZeroTierNetworks(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestHecateHandler_ListZeroTierMembers_NilClient verifies 503 when the ZeroTier
// client has not been initialized.
func TestHecateHandler_ListZeroTierMembers_NilClient(t *testing.T) {
	h, svc := newHecateTestSetup(t)

	cfg := &models.TunnelConfig{UUID: "zt-mem-uuid", Provider: models.ProviderZeroTier}
	ztProv, err := ztprovider.NewZeroTierProvider(cfg, `{"api_token":"test"}`)
	require.NoError(t, err)
	svc.GetManager().RegisterProvider("zt-mem-uuid", ztProv)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/management/hecate/zerotier/networks/net1/members", http.NoBody)
	c.Params = gin.Params{{Key: "network_id", Value: "net1"}}
	h.ListZeroTierMembers(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestHecateHandler_ListNetBirdPeers_NilClient verifies 503 when the NetBird
// client has not been initialized.
func TestHecateHandler_ListNetBirdPeers_NilClient(t *testing.T) {
	h, svc := newHecateTestSetup(t)

	cfg := &models.TunnelConfig{UUID: "nb-peers-uuid", Provider: models.ProviderNetBird}
	nbProv, err := nbprovider.NewNetBirdProvider(cfg, `{"access_token":"test"}`)
	require.NoError(t, err)
	svc.GetManager().RegisterProvider("nb-peers-uuid", nbProv)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/management/hecate/netbird/peers", http.NoBody)
	h.ListNetBirdPeers(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestHecateHandler_SyncNetBird_NilClient verifies 503 when the NetBird
// client has not been initialized.
func TestHecateHandler_SyncNetBird_NilClient(t *testing.T) {
	h, svc := newHecateTestSetup(t)

	cfg := &models.TunnelConfig{UUID: "nb-sync-uuid", Provider: models.ProviderNetBird}
	nbProv, err := nbprovider.NewNetBirdProvider(cfg, `{"access_token":"test"}`)
	require.NoError(t, err)
	svc.GetManager().RegisterProvider("nb-sync-uuid", nbProv)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/management/hecate/netbird/sync", http.NoBody)
	h.SyncNetBird(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestHecateHandler_ListCloudflareTunnels_ContextCancel verifies error handling
// when the request context is cancelled before the Cloudflare API responds.
func TestHecateHandler_ListCloudflareTunnels_ContextCancel(t *testing.T) {
	h, svc := newHecateTestSetup(t)

	cfg := &models.TunnelConfig{UUID: "cf-cancel-uuid", Provider: models.ProviderCloudflare}
	cfProv, err := cfprovider.NewCloudflareProvider(cfg, `{"api_token":"test","account_id":"acc","tunnel_token":"tok"}`)
	require.NoError(t, err)
	svc.GetManager().RegisterProvider("cf-cancel-uuid", cfProv)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/management/hecate/cloudflare/tunnels", http.NoBody)
	c.Request = req.WithContext(ctx)
	h.ListCloudflareTunnels(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestHecateHandler_ListTailscaleDevices_ContextCancel verifies error handling
// when the request context is cancelled before the Tailscale API responds.
func TestHecateHandler_ListTailscaleDevices_ContextCancel(t *testing.T) {
	h, svc := newHecateTestSetup(t)

	cfg := &models.TunnelConfig{UUID: "ts-cancel-uuid", Provider: models.ProviderTailscale}
	tsProv, err := tsprovider.NewTailscaleProvider(cfg, `{"api_key":"test","tailnet":"test.ts.net"}`)
	require.NoError(t, err)
	svc.GetManager().RegisterProvider("ts-cancel-uuid", tsProv)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/management/hecate/tailscale/devices", http.NoBody)
	c.Request = req.WithContext(ctx)
	h.ListTailscaleDevices(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestHecateHandler_ListTailscaleDevices_Success verifies a successful Tailscale
// device listing when the API returns a valid response.
func TestHecateHandler_ListTailscaleDevices_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"devices": []any{}})
	}))
	t.Cleanup(srv.Close)

	h, svc := newHecateTestSetup(t)
	cfg := &models.TunnelConfig{UUID: "ts-ok-uuid", Provider: models.ProviderTailscale}
	tsProv, err := tsprovider.NewTailscaleProvider(cfg, `{"api_key":"test","tailnet":"test"}`)
	require.NoError(t, err)
	tsProv.GetClient().SetBaseURL(srv.URL)
	svc.GetManager().RegisterProvider("ts-ok-uuid", tsProv)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/management/hecate/tailscale/devices", http.NoBody)
	h.ListTailscaleDevices(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestHecateHandler_SyncTailscale_ContextCancel verifies error handling
// when the request context is cancelled before the Tailscale sync API responds.
func TestHecateHandler_SyncTailscale_ContextCancel(t *testing.T) {
	h, svc := newHecateTestSetup(t)

	cfg := &models.TunnelConfig{UUID: "ts-sync-cancel-uuid", Provider: models.ProviderTailscale}
	tsProv, err := tsprovider.NewTailscaleProvider(cfg, `{"api_key":"test","tailnet":"test.ts.net"}`)
	require.NoError(t, err)
	svc.GetManager().RegisterProvider("ts-sync-cancel-uuid", tsProv)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/management/hecate/tailscale/sync", http.NoBody)
	c.Request = req.WithContext(ctx)
	h.SyncTailscale(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestHecateHandler_SyncTailscale_Success verifies a successful Tailscale sync
// when the API returns a valid response.
func TestHecateHandler_SyncTailscale_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"devices": []any{}})
	}))
	t.Cleanup(srv.Close)

	h, svc := newHecateTestSetup(t)
	cfg := &models.TunnelConfig{UUID: "ts-sync-ok-uuid", Provider: models.ProviderTailscale}
	tsProv, err := tsprovider.NewTailscaleProvider(cfg, `{"api_key":"test","tailnet":"test"}`)
	require.NoError(t, err)
	tsProv.GetClient().SetBaseURL(srv.URL)
	svc.GetManager().RegisterProvider("ts-sync-ok-uuid", tsProv)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/management/hecate/tailscale/sync", http.NoBody)
	h.SyncTailscale(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestHecateWSHandler_StreamLogs_UpgradeError verifies that a plain HTTP GET
// (without WebSocket upgrade headers) triggers the upgrade-error path.
func TestHecateWSHandler_StreamLogs_UpgradeError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h, svc := newHecateTestSetup(t)
	nopFactory := hecate.ProviderFactory(func(_ *models.TunnelConfig, _ string) (hecate.TunnelProvider, error) {
		return &testNopProvider{}, nil
	})
	tunnelUUID := startRunningTunnel(t, h, svc, models.ProviderCloudflare, nopFactory)

	wsHandler := NewHecateWSHandler(svc, nil)
	r := gin.New()
	r.GET("/ws/tunnels/:uuid/logs", wsHandler.StreamLogs)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	// Plain HTTP GET — no WebSocket upgrade headers — causes upgrader.Upgrade to fail.
	resp, err := http.Get(srv.URL + "/ws/tunnels/" + tunnelUUID + "/logs") //nolint:noctx
	require.NoError(t, err)
	_ = resp.Body.Close()
	// The handler returns after logging the upgrade error; HTTP status is irrelevant here.
}

// TestHecateWSHandler_StreamLogs_SubClose verifies the branch where the ring
// buffer subscription channel is closed while a client is connected.
func TestHecateWSHandler_StreamLogs_SubClose(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h, svc := newHecateTestSetup(t)
	nopFactory := hecate.ProviderFactory(func(_ *models.TunnelConfig, _ string) (hecate.TunnelProvider, error) {
		return &testNopProvider{}, nil
	})
	tunnelUUID := startRunningTunnel(t, h, svc, models.ProviderCloudflare, nopFactory)

	buf, err := svc.GetManager().GetLogBuffer(tunnelUUID)
	require.NoError(t, err)

	// Pre-populate buffer so the replay loop executes (covers lines 65-68).
	buf.Write("pre-connect line")

	wsHandler := NewHecateWSHandler(svc, nil)
	r := gin.New()
	r.GET("/ws/tunnels/:uuid/logs", wsHandler.StreamLogs)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	wsURL := toWebSocketURL(srv.URL) + "/ws/tunnels/" + tunnelUUID + "/logs"
	conn, resp, dialErr := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, dialErr)
	defer resp.Body.Close()
	require.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
	t.Cleanup(func() { _ = conn.Close() })

	// Write a live line so the sub case fires with ok=true (covers lines 90-96 assignment).
	buf.Write("live line")

	// Give the handler goroutine a moment to process the live line.
	waitFor(t, 2*time.Second, func() bool {
		_, _, readErr := conn.ReadMessage()
		return readErr == nil
	})

	// Close the buffer → sub channel closes → handler returns via !ok branch.
	buf.Close()

	// Wait for the WS connection to be terminated by the handler.
	waitFor(t, 2*time.Second, func() bool {
		conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond)) //nolint:errcheck
		_, _, readErr := conn.ReadMessage()
		return readErr != nil
	})
}

// TestHecateHandler_Start_Success verifies 200 when StartTunnel succeeds.
func TestHecateHandler_Start_Success(t *testing.T) {
	h, svc, _ := newHecateTestSetupWithDB(t)

	wC := httptest.NewRecorder()
	cC, _ := gin.CreateTestContext(wC)
	cC.Request = httptest.NewRequest(http.MethodPost, "/management/hecate/tunnels",
		bytes.NewBufferString(`{"name":"start-ok","provider":"netbird","credentials":"c"}`))
	cC.Request.Header.Set("Content-Type", "application/json")
	h.Create(cC)
	require.Equal(t, http.StatusCreated, wC.Code)

	var created map[string]any
	require.NoError(t, json.Unmarshal(wC.Body.Bytes(), &created))
	tunnelUUID := created["uuid"].(string)

	svc.GetManager().RegisterFactory(models.ProviderNetBird, hecate.ProviderFactory(func(_ *models.TunnelConfig, _ string) (hecate.TunnelProvider, error) {
		return &testNopProvider{}, nil
	}))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/management/hecate/tunnels/"+tunnelUUID+"/start", http.NoBody)
	c.Params = gin.Params{{Key: "uuid", Value: tunnelUUID}}
	h.Start(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestHecateHandler_Update_ServiceError verifies 500 when the service Update
// returns a non-NotFound error (e.g., database closed).
func TestHecateHandler_Update_ServiceError(t *testing.T) {
	h, _, db := newHecateTestSetupWithDB(t)

	wC := httptest.NewRecorder()
	cC, _ := gin.CreateTestContext(wC)
	cC.Request = httptest.NewRequest(http.MethodPost, "/management/hecate/tunnels",
		bytes.NewBufferString(`{"name":"upd-err","provider":"cloudflare","credentials":"c"}`))
	cC.Request.Header.Set("Content-Type", "application/json")
	h.Create(cC)
	require.Equal(t, http.StatusCreated, wC.Code)

	var created map[string]any
	require.NoError(t, json.Unmarshal(wC.Body.Bytes(), &created))
	tunnelUUID := created["uuid"].(string)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPatch, "/management/hecate/tunnels/"+tunnelUUID,
		bytes.NewBufferString(`{"name":"new-name","provider":"cloudflare"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "uuid", Value: tunnelUUID}}
	h.Update(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
