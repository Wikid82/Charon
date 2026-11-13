package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/orthrus"
)

// stubLocalNetworks returns a fixed host view for handler tests.
func stubLocalNetworks() ([]orthrus.LocalNetwork, netip.Addr, orthrus.BindSource) {
	return []orthrus.LocalNetwork{
		{Interface: "eth0", Address: "172.18.0.4", CIDR: "172.18.0.0/16", Primary: true},
	}, netip.MustParseAddr("172.18.0.4"), orthrus.BindSourceDefaultRoute
}

func TestOrthrusHandler_LocalNetworks_ReturnsDetectedNetworks(t *testing.T) {
	h, _ := newOrthrusTestSetup(t)
	h.localNetworks = stubLocalNetworks

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/management/orthrus/local-networks", http.NoBody)
	h.LocalNetworks(c)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, map[string]any{
		"address": "172.18.0.4",
		"source":  "default_route",
	}, resp["auto"])
	assert.Equal(t, []any{
		map[string]any{
			"interface": "eth0",
			"address":   "172.18.0.4",
			"cidr":      "172.18.0.0/16",
			"primary":   true,
		},
	}, resp["networks"])
}

func TestOrthrusHandler_LocalNetworks_EmptyListIsArray(t *testing.T) {
	h, _ := newOrthrusTestSetup(t)
	h.localNetworks = func() ([]orthrus.LocalNetwork, netip.Addr, orthrus.BindSource) {
		return nil, netip.MustParseAddr("127.0.0.1"), orthrus.BindSourceLoopbackFallback
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/management/orthrus/local-networks", http.NoBody)
	h.LocalNetworks(c)

	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"auto":{"address":"127.0.0.1","source":"loopback_fallback"},"networks":[]}`, w.Body.String())
}

func TestOrthrusHandler_LocalNetworks_RegisteredOnAdminOnly(t *testing.T) {
	h, _ := newOrthrusTestSetup(t)

	r := gin.New()
	read := r.Group("/read")
	admin := r.Group("/admin")
	h.RegisterRoutes(read, admin)

	paths := map[string]bool{}
	for _, route := range r.Routes() {
		paths[route.Method+" "+route.Path] = true
	}
	assert.True(t, paths["GET /admin/orthrus/local-networks"])
	assert.False(t, paths["GET /read/orthrus/local-networks"])
}

func TestOrthrusHandler_PatchAgent_ExternalProxyBind(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantCode int
		wantBind string
	}{
		{name: "automatic", body: `{"external_proxy_bind":""}`, wantCode: http.StatusOK, wantBind: ""},
		{name: "all interfaces", body: `{"external_proxy_bind":"0.0.0.0"}`, wantCode: http.StatusOK, wantBind: "0.0.0.0"},
		{name: "subnet", body: `{"external_proxy_bind":"172.18.0.9/16"}`, wantCode: http.StatusOK, wantBind: "172.18.0.0/16"},
		{name: "invalid", body: `{"external_proxy_bind":"localhost"}`, wantCode: http.StatusBadRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := newOrthrusTestSetup(t)
			uuid := provisionForBindTest(t, h)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPatch, "/management/orthrus/agents/"+uuid,
				bytes.NewBufferString(tc.body))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Params = gin.Params{{Key: "uuid", Value: uuid}}
			h.Patch(c)

			require.Equal(t, tc.wantCode, w.Code)
			var resp map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			if tc.wantCode == http.StatusOK {
				assert.Equal(t, tc.wantBind, resp["external_proxy_bind"])
				return
			}
			assert.Equal(t, "Listen address must be automatic, all interfaces, an address or a subnet", resp["error"])
			assert.Equal(t, "external_proxy_bind_invalid", resp["error_code"])
		})
	}
}

func TestOrthrusHandler_GetProxyStatus_IncludesBindFields(t *testing.T) {
	h, _ := newOrthrusTestSetup(t)
	h.SetProxyResolver(&mockProxyResolver{ok: true, status: orthrus.ExternalProxyStatus{
		ConfiguredPort: 23750,
		ActivePort:     23750,
		BoundAddress:   "172.18.0.4:23750",
		Active:         true,
		ActiveBind:     "172.18.0.0/16",
		BindMode:       orthrus.BindModeSubnet,
		BindFallback:   false,
	}})
	uuid := provisionForBindTest(t, h)
	setBind(t, h, uuid, "172.18.0.0/16")

	resp := getProxyStatusForBindTest(t, h, uuid)
	assert.Equal(t, "172.18.0.0/16", resp["configured_bind"])
	assert.Equal(t, "172.18.0.0/16", resp["active_bind"])
	assert.Equal(t, "subnet", resp["bind_mode"])
	assert.Equal(t, false, resp["bind_fallback"])
	assert.Equal(t, "172.18.0.4:23750", resp["bind_address"])
}

func TestOrthrusHandler_GetProxyStatus_BindFallbackReported(t *testing.T) {
	h, _ := newOrthrusTestSetup(t)
	h.SetProxyResolver(&mockProxyResolver{ok: true, status: orthrus.ExternalProxyStatus{
		Active:       true,
		ActivePort:   23750,
		BoundAddress: "127.0.0.1:23750",
		ActiveBind:   "",
		BindMode:     orthrus.BindModeAuto,
		BindFallback: true,
	}})
	uuid := provisionForBindTest(t, h)

	resp := getProxyStatusForBindTest(t, h, uuid)
	assert.Equal(t, "auto", resp["bind_mode"])
	assert.Equal(t, true, resp["bind_fallback"])
	assert.Equal(t, "", resp["configured_bind"])
}

func TestOrthrusHandler_GetProxyStatus_OfflineBindFieldsEmpty(t *testing.T) {
	h, _ := newOrthrusTestSetup(t)
	uuid := provisionForBindTest(t, h)

	resp := getProxyStatusForBindTest(t, h, uuid)
	assert.Equal(t, "", resp["active_bind"])
	assert.Equal(t, "", resp["bind_mode"])
	assert.Equal(t, false, resp["bind_fallback"])
	assert.Equal(t, "", resp["configured_bind"])
}

// provisionForBindTest creates one agent and returns its UUID.
func provisionForBindTest(t *testing.T, h *OrthrusHandler) string {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/management/orthrus/agents",
		bytes.NewBufferString(`{"name":"bind-agent"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	h.Provision(c)
	require.Equal(t, http.StatusCreated, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body["agent"].(map[string]any)["uuid"].(string)
}

func setBind(t *testing.T, h *OrthrusHandler, uuid, bind string) {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"external_proxy_bind": bind})
	require.NoError(t, err)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPatch, "/management/orthrus/agents/"+uuid, bytes.NewBuffer(payload))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "uuid", Value: uuid}}
	h.Patch(c)
	require.Equal(t, http.StatusOK, w.Code)
}

func getProxyStatusForBindTest(t *testing.T, h *OrthrusHandler, uuid string) map[string]any {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/management/orthrus/agents/"+uuid+"/proxy-status", http.NoBody)
	c.Params = gin.Params{{Key: "uuid", Value: uuid}}
	h.GetProxyStatus(c)
	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}
