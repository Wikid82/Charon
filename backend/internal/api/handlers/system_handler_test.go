package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getMyIP(t *testing.T, r *gin.Engine, remoteAddr string, headers map[string]string) MyIPResponse {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/myip", http.NoBody)
	req.RemoteAddr = remoteAddr
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var resp MyIPResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}

func newMyIPRouter(t *testing.T, trustedProxies []string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	require.NoError(t, r.SetTrustedProxies(trustedProxies))
	r.GET("/myip", NewSystemHandler().GetMyIP)
	return r
}

func TestGetMyIP_IgnoresClientSuppliedForwardingHeaders(t *testing.T) {
	r := newMyIPRouter(t, nil)

	for _, headers := range []map[string]string{
		{"CF-Connecting-IP": "5.6.7.8"},
		{"X-Real-IP": "8.8.8.8"},
		{"X-Forwarded-For": "9.9.9.9"},
		{"X-Forwarded-For": "9.9.9.9", "X-Real-IP": "8.8.8.8", "CF-Connecting-IP": "5.6.7.8"},
	} {
		resp := getMyIP(t, r, "7.7.7.7:9999", headers)
		assert.Equal(t, "7.7.7.7", resp.IP)
		assert.Equal(t, "direct", resp.Source)
	}
}

func TestGetMyIP_DirectConnection(t *testing.T) {
	r := newMyIPRouter(t, nil)
	resp := getMyIP(t, r, "7.7.7.7:9999", nil)
	assert.Equal(t, "7.7.7.7", resp.IP)
	assert.Equal(t, "direct", resp.Source)
}

func TestGetMyIP_HonorsTrustedProxyForwarding(t *testing.T) {
	r := newMyIPRouter(t, []string{"10.0.0.0/8"})

	resp := getMyIP(t, r, "10.0.0.2:4000", map[string]string{"X-Forwarded-For": "203.0.113.5"})
	assert.Equal(t, "203.0.113.5", resp.IP)
	assert.Equal(t, "forwarded", resp.Source)

	// Same header from a peer outside the trusted set is ignored.
	resp = getMyIP(t, r, "198.51.100.9:4000", map[string]string{"X-Forwarded-For": "203.0.113.5"})
	assert.Equal(t, "198.51.100.9", resp.IP)
	assert.Equal(t, "direct", resp.Source)
}
