package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/config"
)

func loginProtectionRouter(t *testing.T, cfg config.AuthRateLimitConfig, trusted []string) (*gin.Engine, *middleware.AuthRateLimiter) {
	t.Helper()
	lim, err := middleware.NewAuthRateLimiter(cfg, trusted)
	require.NoError(t, err)
	r := gin.New()
	require.NoError(t, r.SetTrustedProxies(trusted))
	r.GET("/api/v1/security/login-protection", NewLoginProtectionHandler(lim).Get)
	return r, lim
}

func getLoginProtection(t *testing.T, r *gin.Engine, remote string, headers map[string]string) map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/security/login-protection", http.NoBody)
	req.RemoteAddr = remote
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body
}

func TestLoginProtectionHandler_Get_Fields(t *testing.T) {
	r, _ := loginProtectionRouter(t, config.AuthRateLimitConfig{}, []string{"172.18.0.5"})
	body := getLoginProtection(t, r, "203.0.113.7:1234", nil)

	assert.Equal(t, true, body["enabled"])
	assert.Equal(t, map[string]any{"requests": float64(10), "window_seconds": float64(600)}, body["login"])
	assert.Equal(t, map[string]any{"requests": float64(60), "window_seconds": float64(60)}, body["session"])
	assert.Equal(t, float64(1), body["trusted_proxy_count"])
	assert.Equal(t, "203.0.113.7", body["caller_client_key"])
	assert.Equal(t, "public", body["caller_client_scope"])

	obs, ok := body["untrusted_forwarded_headers"].(map[string]any)
	require.True(t, ok)
	for _, scope := range []string{"local", "public"} {
		rec, ok := obs[scope].(map[string]any)
		require.True(t, ok, scope)
		assert.Equal(t, float64(0), rec["count"])
		assert.Nil(t, rec["last_seen"], "last_seen is null while count is 0")
		assert.Equal(t, "", rec["last_peer"])
		assert.Equal(t, "", rec["last_peer_scope"])
	}
}

func TestLoginProtectionHandler_Get_CallerKeyEchoesTrustedForwardedFor(t *testing.T) {
	r, _ := loginProtectionRouter(t, config.AuthRateLimitConfig{}, []string{"172.18.0.0/16"})
	body := getLoginProtection(t, r, "172.18.0.5:4000", map[string]string{"X-Forwarded-For": "198.18.4.5"})
	assert.Equal(t, "198.18.4.5", body["caller_client_key"])
	assert.Equal(t, "public", body["caller_client_scope"])

	body = getLoginProtection(t, r, "172.18.0.5:4000", nil)
	assert.Equal(t, "172.18.0.5", body["caller_client_key"])
	assert.Equal(t, "private", body["caller_client_scope"])
}

func TestLoginProtectionHandler_Get_CallerKeyIgnoresUntrustedForwardedFor(t *testing.T) {
	r, _ := loginProtectionRouter(t, config.AuthRateLimitConfig{}, nil)
	body := getLoginProtection(t, r, "127.0.0.1:4000", map[string]string{"X-Forwarded-For": "198.18.4.5"})
	assert.Equal(t, "127.0.0.1", body["caller_client_key"])
	assert.Equal(t, "loopback", body["caller_client_scope"])

	body = getLoginProtection(t, r, "[2001:db8:1:2::99]:4000", nil)
	assert.Equal(t, "2001:db8:1:2::/64", body["caller_client_key"])
	assert.Equal(t, "public", body["caller_client_scope"])
}

func TestLoginProtectionHandler_Get_DisabledAndObservations(t *testing.T) {
	r, lim := loginProtectionRouter(t, config.AuthRateLimitConfig{Disabled: true}, nil)

	// Feed the detector through the limiter's own middleware.
	probe := gin.New()
	require.NoError(t, probe.SetTrustedProxies(nil))
	probe.GET("/api/v1/auth/me", lim.Middleware(), func(c *gin.Context) { c.Status(http.StatusOK) })
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", http.NoBody)
	req.RemoteAddr = "172.18.0.5:1"
	req.Header.Set("X-Forwarded-For", "198.18.0.1")
	probe.ServeHTTP(httptest.NewRecorder(), req)

	body := getLoginProtection(t, r, "10.0.0.2:1", nil)
	assert.Equal(t, false, body["enabled"])
	local := body["untrusted_forwarded_headers"].(map[string]any)["local"].(map[string]any)
	assert.Equal(t, float64(1), local["count"])
	assert.NotNil(t, local["last_seen"])
	assert.Equal(t, "172.18.0.5", local["last_peer"])
	assert.Equal(t, "private", local["last_peer_scope"])
}
