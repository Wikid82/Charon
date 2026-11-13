package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsAllowedWebSocketOrigin_PublicOrigin(t *testing.T) {
	const public = "https://charon.example.com"
	tests := []struct {
		name   string
		origin string
		host   string
		public string
		want   bool
	}{
		{"public origin allowed when Host is the inner address", public, "charon.internal:8080", public, true},
		{"public origin match ignores host case", "https://CHARON.example.com", "charon.internal:8080", public, true},
		{"public origin scheme must match", "http://charon.example.com", "charon.internal:8080", public, false},
		{"other origin refused even with public origin set", "https://evil.example", "charon.internal:8080", public, false},
		{"public origin refused when none is configured", public, "charon.internal:8080", "", false},
		{"host match still allowed with public origin set", "http://charon.internal:8080", "charon.internal:8080", public, true},
		{"no Origin header allowed", "", "charon.internal:8080", public, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/ws", http.NoBody)
			req.Host = tc.host
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			assert.Equal(t, tc.want, isAllowedWebSocketOrigin(req, tc.public))
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
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			// Not a real handshake: once the origin check passes, the upgrade itself fails for another reason.
			c.Request = httptest.NewRequest(http.MethodGet, "/ws", http.NoBody)
			c.Request.Host = "charon.internal:8080"
			c.Request.Header.Set("Origin", tc.origin)

			conn, err := upgradeWebSocket(c, tc.source)
			require.Error(t, err)
			assert.Nil(t, conn)
			if tc.wantDenied {
				assert.ErrorIs(t, err, errWebSocketOriginDenied)
				assert.Equal(t, http.StatusForbidden, w.Code)
			} else {
				assert.NotErrorIs(t, err, errWebSocketOriginDenied)
			}
		})
	}
}

func TestWebSocketHandlers_WithPublicOriginSource(t *testing.T) {
	src := staticPublicOrigin("https://charon.example.com")
	assert.Equal(t, src, NewLogsWSHandler(nil).WithPublicOriginSource(src).publicOrigins)
	assert.Equal(t, src, NewCerberusLogsHandler(nil, nil).WithPublicOriginSource(src).publicOrigins)
	assert.Equal(t, src, NewHecateWSHandler(nil, nil).WithPublicOriginSource(src).publicOrigins)
	assert.Equal(t, src, NewStatsHandler(nil).WithPublicOriginSource(src).publicOrigins)
}
