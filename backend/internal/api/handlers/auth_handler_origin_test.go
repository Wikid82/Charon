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

func cookieViaOrigin(t *testing.T, secret *selfhop.Secret, headers map[string]string) *http.Cookie {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	require.NoError(t, r.SetTrustedProxies(nil))
	r.Use(middleware.SelfHop(secret, security.TrustedProxyMatcher{}))
	r.GET("/cookie", func(c *gin.Context) {
		setSecureCookie(c, "auth_token", "v", 60, security.TrustedProxyMatcher{})
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/cookie", http.NoBody)
	req.RemoteAddr = "127.0.0.1:4000"
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	cookies := w.Result().Cookies()
	require.Len(t, cookies, 1)
	return cookies[0]
}

func TestSetSecureCookie_UsesVerifiedOrigin(t *testing.T) {
	secret, err := selfhop.NewSecret()
	require.NoError(t, err)
	hop := func(client string, extra map[string]string) map[string]string {
		h := map[string]string{selfhop.HeaderSecret: secret.Reveal(), selfhop.HeaderClient: client}
		for k, v := range extra {
			h[k] = v
		}
		return h
	}

	t.Run("direct loopback request over http keeps working", func(t *testing.T) {
		c := cookieViaOrigin(t, secret, nil)
		assert.False(t, c.Secure)
		assert.Equal(t, http.SameSiteLaxMode, c.SameSite)
	})

	t.Run("public client over http gets a secure cookie", func(t *testing.T) {
		c := cookieViaOrigin(t, secret, hop("198.51.100.23", map[string]string{"X-Forwarded-Proto": "http"}))
		assert.True(t, c.Secure)
		assert.Equal(t, http.SameSiteLaxMode, c.SameSite)
	})

	t.Run("public client over https gets a strict secure cookie", func(t *testing.T) {
		c := cookieViaOrigin(t, secret, hop("198.51.100.23", map[string]string{"X-Forwarded-Proto": "https"}))
		assert.True(t, c.Secure)
		assert.Equal(t, http.SameSiteStrictMode, c.SameSite)
	})

	t.Run("private client over http stays usable", func(t *testing.T) {
		c := cookieViaOrigin(t, secret, hop("192.168.1.50", map[string]string{"X-Forwarded-Proto": "http"}))
		assert.False(t, c.Secure)
		assert.Equal(t, http.SameSiteLaxMode, c.SameSite)
	})

	for name, client := range map[string]string{
		"zoned link-local client": "fe80::1%eth0",
		"unparsable client value": "not-an-ip",
		"empty client value":      "",
	} {
		t.Run(name+" is not treated as local", func(t *testing.T) {
			c := cookieViaOrigin(t, secret, hop(client, map[string]string{"X-Forwarded-Proto": "http"}))
			assert.True(t, c.Secure)
		})
	}

	t.Run("missing client header is not treated as local", func(t *testing.T) {
		c := cookieViaOrigin(t, secret, map[string]string{selfhop.HeaderSecret: secret.Reveal(), "X-Forwarded-Proto": "http"})
		assert.True(t, c.Secure)
	})

	t.Run("unverified proof does not change the result", func(t *testing.T) {
		c := cookieViaOrigin(t, secret, map[string]string{
			selfhop.HeaderSecret: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			selfhop.HeaderClient: "198.51.100.23",
		})
		assert.False(t, c.Secure)
	})
}
