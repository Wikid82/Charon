package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/security"
)

func forwardedTestContext(t *testing.T, remoteAddr string, headers map[string]string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "http://charon.example/api", http.NoBody)
	req.RemoteAddr = remoteAddr
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	c.Request = req
	return c
}

func TestTrustedPeer(t *testing.T) {
	trusted := security.NewTrustedProxyMatcher([]string{"10.0.0.0/8", "::1/128"})
	tests := []struct {
		name       string
		remoteAddr string
		matcher    security.TrustedProxyMatcher
		want       bool
	}{
		{name: "ipv4 with port inside range", remoteAddr: "10.1.2.3:4000", matcher: trusted, want: true},
		{name: "ipv4 without port", remoteAddr: "10.1.2.3", matcher: trusted, want: true},
		{name: "ipv4 outside range", remoteAddr: "192.0.2.1:80", matcher: trusted, want: false},
		{name: "bracketed ipv6 with port", remoteAddr: "[::1]:8080", matcher: trusted, want: true},
		{name: "bare ipv6 without port", remoteAddr: "::1", matcher: trusted, want: true},
		{name: "unparsable peer", remoteAddr: "not-an-address", matcher: trusted, want: false},
		{name: "empty peer", remoteAddr: "", matcher: trusted, want: false},
		{name: "zero matcher trusts nobody", remoteAddr: "10.1.2.3:4000", matcher: security.TrustedProxyMatcher{}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := forwardedTestContext(t, tt.remoteAddr, nil)
			assert.Equal(t, tt.want, TrustedPeer(c, tt.matcher))
		})
	}
}

func TestTrustedPeer_NilContextAndRequest(t *testing.T) {
	trusted := security.NewTrustedProxyMatcher([]string{"0.0.0.0/0"})
	assert.False(t, TrustedPeer(nil, trusted))
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	assert.False(t, TrustedPeer(c, trusted))
}

func TestForwardedRequest_UntrustedPeerIgnoresHeaders(t *testing.T) {
	c := forwardedTestContext(t, "192.0.2.10:5555", map[string]string{
		"X-Forwarded-Proto": "https",
		"X-Forwarded-Host":  "evil.example",
	})
	scheme, hosts := ForwardedRequest(c, security.NewTrustedProxyMatcher([]string{"10.0.0.0/8"}))
	assert.Empty(t, scheme)
	assert.Empty(t, hosts)
}

func TestForwardedRequest_TrustedPeerHonoursHeaders(t *testing.T) {
	c := forwardedTestContext(t, "10.0.0.2:443", map[string]string{
		"X-Forwarded-Proto": "HTTPS, http",
		"X-Forwarded-Host":  " public.example , , second.example ",
	})
	scheme, hosts := ForwardedRequest(c, security.NewTrustedProxyMatcher([]string{"10.0.0.0/8"}))
	assert.Equal(t, "https", scheme)
	assert.Equal(t, []string{"public.example", "second.example"}, hosts)
}

func TestForwardedRequest_VerifiedOriginWins(t *testing.T) {
	c := forwardedTestContext(t, "192.0.2.10:5555", map[string]string{
		"X-Forwarded-Proto": "http",
		"X-Forwarded-Host":  "ignored.example",
	})
	c.Set(requestOriginKey, RequestOrigin{Addr: "203.0.113.7", Peer: "127.0.0.1", Scheme: "https", Host: "charon.example"})
	scheme, hosts := ForwardedRequest(c, security.NewTrustedProxyMatcher([]string{"10.0.0.0/8"}))
	assert.Equal(t, "https", scheme)
	assert.Equal(t, []string{"charon.example"}, hosts)
}

func TestForwardedRequest_VerifiedOriginWithoutSchemeUsesTrustedProto(t *testing.T) {
	c := forwardedTestContext(t, "10.0.0.2:443", map[string]string{
		"X-Forwarded-Proto": "https",
	})
	c.Set(requestOriginKey, RequestOrigin{Addr: "203.0.113.7", Peer: "10.0.0.2"})
	scheme, hosts := ForwardedRequest(c, security.NewTrustedProxyMatcher([]string{"10.0.0.0/8"}))
	assert.Equal(t, "https", scheme)
	assert.Empty(t, hosts)
}

func TestForwardedRequest_EmptyProtoFromTrustedPeer(t *testing.T) {
	c := forwardedTestContext(t, "10.0.0.2:443", map[string]string{
		"X-Forwarded-Proto": ", https",
	})
	scheme, _ := ForwardedRequest(c, security.NewTrustedProxyMatcher([]string{"10.0.0.0/8"}))
	assert.Empty(t, scheme)
}

func TestForwardedRequest_NoHeadersNoOrigin(t *testing.T) {
	c := forwardedTestContext(t, "10.0.0.2:443", nil)
	scheme, hosts := ForwardedRequest(c, security.NewTrustedProxyMatcher([]string{"10.0.0.0/8"}))
	require.Empty(t, scheme)
	assert.Empty(t, hosts)
}
