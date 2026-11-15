package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/security"
	"github.com/Wikid82/charon/backend/internal/security/selfhop"
)

// wsOriginContext builds a gin context for a WebSocket handshake from peer with the given Origin and headers.
func wsOriginContext(host, peer, origin string, headers map[string]string) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest(http.MethodGet, "/ws", http.NoBody)
	req.Host = host
	req.RemoteAddr = peer
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	c.Request = req
	return c
}

var (
	wsUntrustedPeer = "192.0.2.10:5555"
	wsTrustedPeer   = "10.0.0.2:5555"
	wsTrustedList   = security.NewTrustedProxyMatcher([]string{"10.0.0.0/8"})
)

func TestIsAllowedWebSocketOrigin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		origin         string
		host           string
		peer           string
		publicOrigin   string
		xForwardedHost string
		want           bool
	}{
		{"empty origin allows request", "", "example.com", wsUntrustedPeer, "", "", true},
		{"invalid URL origin rejects", "://bad-url", "example.com", wsUntrustedPeer, "", "", false},
		{"matching host allows", "http://example.com", "example.com", wsUntrustedPeer, "", "", true},
		{"non-matching host rejects", "http://evil.com", "example.com", wsUntrustedPeer, "", "", false},
		{"origin with port matching", "http://example.com:8080", "example.com:8080", wsUntrustedPeer, "", "", true},
		{"origin with port non-matching", "http://example.com:9090", "example.com:8080", wsUntrustedPeer, "", "", false},
		{"forwarded host from untrusted peer is ignored", "http://proxy.example.com", "backend.internal", wsUntrustedPeer, "", "proxy.example.com", false},
		{"forwarded host from trusted peer allows", "http://proxy.example.com", "backend.internal", wsTrustedPeer, "", "proxy.example.com", true},
		{"forwarded host from trusted peer uses first entry", "http://proxy.example.com", "backend.internal", wsTrustedPeer, "", "proxy.example.com, other.internal", true},
		{"forwarded host from trusted peer non-matching rejects", "http://evil.com", "backend.internal", wsTrustedPeer, "", "proxy.example.com", false},
		{"public origin allowed when Host is the inner address", "https://charon.example.com", "charon.internal:8080", wsUntrustedPeer, "https://charon.example.com", "", true},
		{"public origin match ignores host case", "https://CHARON.example.com", "charon.internal:8080", wsUntrustedPeer, "https://charon.example.com", "", true},
		{"public origin scheme must match", "http://charon.example.com", "charon.internal:8080", wsUntrustedPeer, "https://charon.example.com", "", false},
		{"other origin refused even with public origin set", "https://evil.example", "charon.internal:8080", wsUntrustedPeer, "https://charon.example.com", "", false},
		{"public origin refused when none is configured", "https://charon.example.com", "charon.internal:8080", wsUntrustedPeer, "", "", false},
		{"host match still allowed with public origin set", "http://charon.internal:8080", "charon.internal:8080", wsUntrustedPeer, "https://charon.example.com", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			headers := map[string]string{}
			if tc.xForwardedHost != "" {
				headers["X-Forwarded-Host"] = tc.xForwardedHost
			}
			c := wsOriginContext(tc.host, tc.peer, tc.origin, headers)
			assert.Equal(t, tc.want, isAllowedWebSocketOrigin(c, tc.publicOrigin, wsTrustedList),
				"origin=%q host=%q peer=%q xfh=%q", tc.origin, tc.host, tc.peer, tc.xForwardedHost)
		})
	}
}

// staticPublicOrigin is a PublicOriginSource that always reports the same origin.
type staticPublicOrigin string

func (s staticPublicOrigin) TrustedOrigin() string { return string(s) }

func TestUpgradeWebSocket_UsesPublicOriginSource(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const public = "https://charon.example.com"
	tests := []struct {
		name       string
		origin     string
		source     PublicOriginSource
		wantDenied bool
	}{
		{"public origin from the source passes the origin check", public, staticPublicOrigin(public), false},
		{"other origin refused even with a source", "https://evil.example", staticPublicOrigin(public), true},
		{"nil source trusts no public origin", public, nil, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Not a real handshake: once the origin check passes, the upgrade itself fails for another reason.
			c := wsOriginContext("charon.internal:8080", wsUntrustedPeer, tc.origin, nil)

			conn, _, err := upgradeWebSocket(c, tc.source, wsTrustedList, nil)
			require.Error(t, err)
			assert.Nil(t, conn)
			if tc.wantDenied {
				assert.ErrorIs(t, err, errWebSocketOriginDenied)
				assert.Equal(t, http.StatusForbidden, c.Writer.Status())
			} else {
				assert.NotErrorIs(t, err, errWebSocketOriginDenied)
			}
		})
	}
}

func TestIsAllowedWebSocketOrigin_VerifiedHopHost(t *testing.T) {
	secret, err := selfhop.NewSecret()
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	var allowed bool
	r := gin.New()
	r.Use(middleware.SelfHop(secret, security.TrustedProxyMatcher{}))
	r.GET("/ws", func(c *gin.Context) {
		allowed = isAllowedWebSocketOrigin(c, "", security.TrustedProxyMatcher{})
	})

	req := httptest.NewRequest(http.MethodGet, "/ws", http.NoBody)
	req.RemoteAddr = "127.0.0.1:4000"
	req.Host = "backend.internal"
	req.Header.Set("Origin", "https://public.example")
	req.Header.Set("X-Forwarded-Host", "public.example")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set(selfhop.HeaderSecret, secret.Reveal())
	req.Header.Set(selfhop.HeaderClient, "198.51.100.23")
	r.ServeHTTP(httptest.NewRecorder(), req)

	assert.True(t, allowed, "a verified hop supplies the host the client addressed")
}

func TestWebSocketHandlers_WithOriginPolicy(t *testing.T) {
	src := staticPublicOrigin("https://charon.example.com")
	assert.Equal(t, src, NewLogsWSHandler(nil).WithOriginPolicy(src, wsTrustedList).publicOrigins)
	assert.Equal(t, src, NewCerberusLogsHandler(nil, nil).WithOriginPolicy(src, wsTrustedList).publicOrigins)
	assert.Equal(t, src, NewHecateWSHandler(nil, nil).WithOriginPolicy(src, wsTrustedList).publicOrigins)
	assert.Equal(t, src, NewStatsHandler(nil).WithOriginPolicy(src, wsTrustedList).publicOrigins)

	h := NewLogsWSHandler(nil).WithOriginPolicy(src, wsTrustedList)
	assert.Equal(t, wsTrustedList, h.trustedProxies)
}
