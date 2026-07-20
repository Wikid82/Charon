package handlers

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupAuthHandler(t *testing.T) (*AuthHandler, *gorm.DB) {
	dbName := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	require.NoError(t, err)
	_ = db.AutoMigrate(&models.User{}, &models.Setting{})

	cfg := config.Config{JWTSecret: "test-secret"}
	authService := services.NewAuthService(db, cfg)
	return NewAuthHandler(authService, nil), db
}

func TestAuthHandler_Login(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandler(t)

	// Create user
	user := &models.User{
		UUID:  uuid.NewString(),
		Email: "test@example.com",
		Name:  "Test User",
	}
	_ = user.SetPassword("password123")
	db.Create(user)

	r := gin.New()
	r.POST("/login", handler.Login)

	// Success
	body := map[string]string{
		"email":    "test@example.com",
		"password": "password123",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/login", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "token")
}

func TestSetSecureCookie_HTTPS_Strict(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest("POST", "https://example.com/login", http.NoBody)
	ctx.Request = req

	setSecureCookie(ctx, "auth_token", "abc", 60, nil)
	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	c := cookies[0]
	assert.True(t, c.Secure)
	assert.Equal(t, http.SameSiteStrictMode, c.SameSite)
}

func TestSetSecureCookie_HTTP_Lax(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest("POST", "http://192.0.2.10/login", http.NoBody)
	req.Header.Set("X-Forwarded-Proto", "http")
	ctx.Request = req

	setSecureCookie(ctx, "auth_token", "abc", 60, nil)
	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	c := cookies[0]
	assert.True(t, c.Secure)
	assert.Equal(t, http.SameSiteLaxMode, c.SameSite)
}

func TestSetSecureCookie_HTTP_Loopback_Insecure(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest("POST", "http://127.0.0.1:8080/login", http.NoBody)
	req.Host = "127.0.0.1:8080"
	req.RemoteAddr = "127.0.0.1:9999"
	req.Header.Set("X-Forwarded-Proto", "http")
	ctx.Request = req

	setSecureCookie(ctx, "auth_token", "abc", 60, nil)
	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	cookie := cookies[0]
	assert.False(t, cookie.Secure)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
}

func TestSetSecureCookie_ForwardedHTTPS_LocalhostForcesInsecure(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest("POST", "http://localhost:8080/login", http.NoBody)
	req.Host = "localhost:8080"
	req.RemoteAddr = "203.0.113.9:443"
	req.Header.Set("X-Forwarded-Proto", "https")
	ctx.Request = req

	setSecureCookie(ctx, "auth_token", "abc", 60, []string{"203.0.113.9/32"})
	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	cookie := cookies[0]
	assert.True(t, cookie.Secure)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
}

func TestSetSecureCookie_ForwardedHTTPS_LoopbackForcesInsecure(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest("POST", "http://127.0.0.1:8080/login", http.NoBody)
	req.Host = "127.0.0.1:8080"
	req.RemoteAddr = "203.0.113.9:443"
	req.Header.Set("X-Forwarded-Proto", "https")
	ctx.Request = req

	setSecureCookie(ctx, "auth_token", "abc", 60, []string{"203.0.113.9/32"})
	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	cookie := cookies[0]
	assert.True(t, cookie.Secure)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
}

func TestSetSecureCookie_ForwardedHostLocalhostForcesInsecure(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest("POST", "http://charon.local/login", http.NoBody)
	req.Host = "charon.internal:8080"
	req.RemoteAddr = "203.0.113.9:443"
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "localhost:8080")
	ctx.Request = req

	setSecureCookie(ctx, "auth_token", "abc", 60, []string{"203.0.113.9/32"})
	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	cookie := cookies[0]
	assert.True(t, cookie.Secure)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
}

// TestSetSecureCookie_OriginHeaderIgnored_NoLongerAffectsSecurity replaces
// the old TestSetSecureCookie_OriginLoopbackForcesInsecure (§13.4): Origin
// and Referer no longer factor into the cookie-security decision at all, so
// a spoofed loopback Origin over plain HTTP from a non-local, untrusted peer
// cannot downgrade Secure — the public fail-safe holds, for the same
// asserted values as before, but now via a different (accurate) mechanism.
func TestSetSecureCookie_OriginHeaderIgnored_NoLongerAffectsSecurity(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest("POST", "http://service.internal/login", http.NoBody)
	req.Host = "service.internal:8080"
	req.Header.Set("Origin", "http://127.0.0.1:8080")
	ctx.Request = req

	setSecureCookie(ctx, "auth_token", "abc", 60, nil)
	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	cookie := cookies[0]
	assert.True(t, cookie.Secure)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
}

func TestSetSecureCookie_HTTP_PrivateIP_Insecure(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest("POST", "http://192.168.1.50:8080/login", http.NoBody)
	req.Host = "192.168.1.50:8080"
	req.RemoteAddr = "192.168.1.50:9999"
	req.Header.Set("X-Forwarded-Proto", "http")
	ctx.Request = req

	setSecureCookie(ctx, "auth_token", "abc", 60, nil)
	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	cookie := cookies[0]
	assert.False(t, cookie.Secure)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
}

func TestSetSecureCookie_HTTP_10Network_Insecure(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest("POST", "http://10.0.0.5:8080/login", http.NoBody)
	req.Host = "10.0.0.5:8080"
	req.RemoteAddr = "10.0.0.5:9999"
	req.Header.Set("X-Forwarded-Proto", "http")
	ctx.Request = req

	setSecureCookie(ctx, "auth_token", "abc", 60, nil)
	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	cookie := cookies[0]
	assert.False(t, cookie.Secure)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
}

func TestSetSecureCookie_HTTP_172Network_Insecure(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest("POST", "http://172.16.0.1:8080/login", http.NoBody)
	req.Host = "172.16.0.1:8080"
	req.RemoteAddr = "172.16.0.1:9999"
	req.Header.Set("X-Forwarded-Proto", "http")
	ctx.Request = req

	setSecureCookie(ctx, "auth_token", "abc", 60, nil)
	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	cookie := cookies[0]
	assert.False(t, cookie.Secure)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
}

func TestSetSecureCookie_HTTPS_PrivateIP_Secure(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest("POST", "https://192.168.1.50:8080/login", http.NoBody)
	req.Host = "192.168.1.50:8080"
	req.RemoteAddr = "192.168.1.50:9999"
	req.Header.Set("X-Forwarded-Proto", "https")
	ctx.Request = req

	setSecureCookie(ctx, "auth_token", "abc", 60, nil)
	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	cookie := cookies[0]
	assert.True(t, cookie.Secure)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
}

func TestSetSecureCookie_HTTP_IPv6ULA_Insecure(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest("POST", "http://[fd12::1]:8080/login", http.NoBody)
	req.Host = "[fd12::1]:8080"
	req.RemoteAddr = "[fd12::1]:9999"
	req.Header.Set("X-Forwarded-Proto", "http")
	ctx.Request = req

	setSecureCookie(ctx, "auth_token", "abc", 60, nil)
	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	cookie := cookies[0]
	assert.False(t, cookie.Secure)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
}

func TestSetSecureCookie_HTTP_PublicIP_Secure(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest("POST", "http://203.0.113.5:8080/login", http.NoBody)
	req.Host = "203.0.113.5:8080"
	req.Header.Set("X-Forwarded-Proto", "http")
	ctx.Request = req

	setSecureCookie(ctx, "auth_token", "abc", 60, nil)
	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	cookie := cookies[0]
	assert.True(t, cookie.Secure)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
}

// TestSetSecureCookie_HTTP_TailscaleCGNAT_Insecure is the direct regression
// test for the reported bug: a plain-HTTP request from a Tailscale-assigned
// address (100.64.0.0/10, RFC 6598 carrier-grade NAT) must downgrade Secure
// to false so the browser actually persists the auth_token cookie, restoring
// the cookie-fallback auth path used by navigation-triggered downloads.
func TestSetSecureCookie_HTTP_TailscaleCGNAT_Insecure(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest("POST", "http://100.98.12.109:8787/login", http.NoBody)
	req.Host = "100.98.12.109:8787"
	req.RemoteAddr = "100.98.12.109:9999"
	req.Header.Set("X-Forwarded-Proto", "http")
	ctx.Request = req

	setSecureCookie(ctx, "auth_token", "abc", 60, nil)
	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	cookie := cookies[0]
	assert.False(t, cookie.Secure)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
}

// TestIsTrustedPeer table-drives isTrustedPeer's peer/allowlist matching,
// including the fail-safe behavior of security.IsIPInCIDRList when the
// configured list contains a malformed entry (docs/plans/current_spec.md
// §13.6, test #1 — the malformed-CIDR sub-cases were a Supervisor review
// correction folded in before implementation).
func TestIsTrustedPeer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		remoteAddr     string
		trustedProxies []string
		want           bool
	}{
		{
			name:           "remote addr inside configured CIDR",
			remoteAddr:     "203.0.113.9:1234",
			trustedProxies: []string{"203.0.113.0/24"},
			want:           true,
		},
		{
			name:           "remote addr outside configured CIDR",
			remoteAddr:     "198.51.100.9:1234",
			trustedProxies: []string{"203.0.113.0/24"},
			want:           false,
		},
		{
			name:           "empty trustedProxies always false",
			remoteAddr:     "203.0.113.9:1234",
			trustedProxies: nil,
			want:           false,
		},
		{
			name:           "malformed remote addr with no port",
			remoteAddr:     "not-an-address",
			trustedProxies: []string{"203.0.113.0/24"},
			want:           false,
		},
		{
			name:           "malformed remote addr garbage string",
			remoteAddr:     "!!!garbage!!!",
			trustedProxies: []string{"203.0.113.0/24"},
			want:           false,
		},
		{
			// A malformed CIDR entry earlier in the list must not break
			// matching of a valid entry that follows it — IsIPInCIDRList
			// skips (continue) a net.ParseCIDR error on just that entry.
			name:           "malformed CIDR entry does not break a later valid entry",
			remoteAddr:     "10.1.2.3:5555",
			trustedProxies: []string{"not-a-cidr", "10.0.0.0/8"},
			want:           true,
		},
		{
			// ...and the malformed entry must not be silently treated as
			// match-everything for an address outside every valid entry.
			name:           "malformed CIDR entry is not treated as match-everything",
			remoteAddr:     "198.51.100.9:5555",
			trustedProxies: []string{"not-a-cidr", "10.0.0.0/8"},
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			req := httptest.NewRequest(http.MethodGet, "http://example.com", http.NoBody)
			req.RemoteAddr = tt.remoteAddr
			ctx.Request = req

			var got bool
			assert.NotPanics(t, func() {
				got = isTrustedPeer(ctx, tt.trustedProxies)
			})
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestRequestScheme_ForgedForwardedProto_IgnoredFromUntrustedPeer proves a
// forged X-Forwarded-Proto from an untrusted (non-allowlisted) peer no
// longer flips the resolved scheme — the core fix of §13.
func TestRequestScheme_ForgedForwardedProto_IgnoredFromUntrustedPeer(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodGet, "http://example.com", http.NoBody)
	req.RemoteAddr = "198.51.100.9:1234"
	req.Header.Set("X-Forwarded-Proto", "https")
	ctx.Request = req

	assert.Equal(t, "http", requestScheme(ctx, nil))
}

// TestRequestScheme_ForwardedProto_HonoredFromTrustedPeer proves the
// legitimate trusted-reverse-proxy case still works when the peer address
// matches the configured allowlist.
func TestRequestScheme_ForwardedProto_HonoredFromTrustedPeer(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodGet, "http://example.com", http.NoBody)
	req.RemoteAddr = "198.51.100.9:1234"
	req.Header.Set("X-Forwarded-Proto", "https")
	ctx.Request = req

	assert.Equal(t, "https", requestScheme(ctx, []string{"198.51.100.9/32"}))
}

// TestIsLocalRequest_UntrustedPeer_IgnoresForwardedHost is the isLocalRequest
// companion to TestRequestScheme_ForgedForwardedProto_IgnoredFromUntrustedPeer.
func TestIsLocalRequest_UntrustedPeer_IgnoresForwardedHost(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodGet, "http://example.com", http.NoBody)
	req.RemoteAddr = "198.51.100.9:1234"
	req.Header.Set("X-Forwarded-Host", "localhost")
	ctx.Request = req

	assert.False(t, isLocalRequest(ctx, nil))
}

// TestIsLocalRequest_TrustedPeer_HonorsForwardedHost is the isLocalRequest
// companion to TestRequestScheme_ForwardedProto_HonoredFromTrustedPeer.
func TestIsLocalRequest_TrustedPeer_HonorsForwardedHost(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodGet, "http://example.com", http.NoBody)
	req.RemoteAddr = "198.51.100.9:1234"
	req.Header.Set("X-Forwarded-Host", "localhost")
	ctx.Request = req

	assert.True(t, isLocalRequest(ctx, []string{"198.51.100.9/32"}))
}

// TestIsLocalRequest_UntrustedPeer_UsesRawPeerIPNotHostHeader proves the
// Host-header-forgery half of the QA-identified gap is closed: a forged Host
// claiming loopback, from a peer that is genuinely public, is not local.
func TestIsLocalRequest_UntrustedPeer_UsesRawPeerIPNotHostHeader(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8080", http.NoBody)
	req.Host = "127.0.0.1:8080"
	req.RemoteAddr = "198.51.100.9:1234"
	ctx.Request = req

	assert.False(t, isLocalRequest(ctx, nil))
}

// TestIsLocalRequest_UntrustedPeer_DirectTailscaleAccess_StillWorks is the
// mandatory non-regression proof: the direct-access case §9 exists to
// support (no proxy at all, Tailscale/LAN) must not regress when no trusted
// proxy is configured. Uses the exact §9 bug-report IP.
func TestIsLocalRequest_UntrustedPeer_DirectTailscaleAccess_StillWorks(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodGet, "http://100.98.12.109:8787", http.NoBody)
	req.Host = "100.98.12.109:8787"
	req.RemoteAddr = "100.98.12.109:9999"
	ctx.Request = req

	assert.True(t, isLocalRequest(ctx, nil))
}

// TestSetSecureCookie_TrustedProxyHTTPS_PublicHost_Secure exercises the full
// setSecureCookie integration for a genuine trusted-proxy-terminated-HTTPS
// request to a public host — Secure/Strict, not via the local-network path.
func TestSetSecureCookie_TrustedProxyHTTPS_PublicHost_Secure(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest("POST", "http://admin.example.com/login", http.NoBody)
	req.RemoteAddr = "203.0.113.9:443"
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "admin.example.com")
	ctx.Request = req

	setSecureCookie(ctx, "auth_token", "abc", 60, []string{"203.0.113.9/32"})
	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	cookie := cookies[0]
	assert.True(t, cookie.Secure)
	assert.Equal(t, http.SameSiteStrictMode, cookie.SameSite)
}

// TestSetSecureCookie_UntrustedForgedHeaders_NoDowngrade is the single most
// important new test: full end-to-end reproduction and closure of the
// QA-identified adversarial scenario — a public, untrusted peer forging
// X-Forwarded-Proto/X-Forwarded-Host cannot downgrade its own cookie's
// Secure attribute.
func TestSetSecureCookie_UntrustedForgedHeaders_NoDowngrade(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest("POST", "http://public-charon.example.com/login", http.NoBody)
	req.RemoteAddr = "198.51.100.9:1234"
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "127.0.0.1")
	ctx.Request = req

	setSecureCookie(ctx, "auth_token", "abc", 60, nil)
	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	cookie := cookies[0]
	assert.True(t, cookie.Secure, "forged headers from an untrusted peer must never downgrade Secure")
}

// TestIsLocalRequest_OriginHeaderIgnored replaces the deleted "origin
// loopback" subtest of TestIsLocalRequest (§13.4): Origin no longer grants
// locality even when it claims loopback.
func TestIsLocalRequest_OriginHeaderIgnored(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodGet, "http://example.com", http.NoBody)
	req.Host = "example.com"
	req.Header.Set("Origin", "http://127.0.0.1:3000")
	ctx.Request = req

	assert.False(t, isLocalRequest(ctx, nil))
}

func TestIsProduction(t *testing.T) {
	t.Setenv("CHARON_ENV", "production")
	assert.True(t, isProduction())

	t.Setenv("CHARON_ENV", "prod")
	assert.True(t, isProduction())

	t.Setenv("CHARON_ENV", "development")
	assert.False(t, isProduction())
}

func TestRequestScheme(t *testing.T) {

	t.Run("forwarded proto first value wins", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		req := httptest.NewRequest("GET", "http://example.com", http.NoBody)
		req.RemoteAddr = "203.0.113.9:443"
		req.Header.Set("X-Forwarded-Proto", "HTTPS, http")
		ctx.Request = req

		assert.Equal(t, "https", requestScheme(ctx, []string{"203.0.113.9/32"}))
	})

	t.Run("tls request", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		req := httptest.NewRequest("GET", "https://example.com", http.NoBody)
		req.TLS = &tls.ConnectionState{}
		ctx.Request = req

		assert.Equal(t, "https", requestScheme(ctx, nil))
	})

	t.Run("url scheme fallback", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		req := httptest.NewRequest("GET", "http://example.com", http.NoBody)
		req.URL.Scheme = "HTTP"
		ctx.Request = req

		assert.Equal(t, "http", requestScheme(ctx, nil))
	})

	t.Run("default http fallback", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		req := httptest.NewRequest("GET", "/", http.NoBody)
		req.URL.Scheme = ""
		ctx.Request = req

		assert.Equal(t, "http", requestScheme(ctx, nil))
	})
}

func TestHostHelpers(t *testing.T) {
	t.Run("normalizeHost", func(t *testing.T) {
		assert.Equal(t, "", normalizeHost("   "))
		assert.Equal(t, "example.com", normalizeHost("example.com:8080"))
		assert.Equal(t, "::1", normalizeHost("[::1]:2020"))
		assert.Equal(t, "localhost", normalizeHost("localhost"))
	})

	t.Run("isLocalOrPrivateHost", func(t *testing.T) {
		assert.True(t, isLocalOrPrivateHost("localhost"))
		assert.True(t, isLocalOrPrivateHost("127.0.0.1"))
		assert.True(t, isLocalOrPrivateHost("::1"))
		assert.True(t, isLocalOrPrivateHost("192.168.1.50"))
		assert.True(t, isLocalOrPrivateHost("10.0.0.1"))
		assert.True(t, isLocalOrPrivateHost("172.16.0.1"))
		assert.True(t, isLocalOrPrivateHost("fd12::1"))
		assert.False(t, isLocalOrPrivateHost("203.0.113.5"))
		assert.False(t, isLocalOrPrivateHost("example.com"))
	})

	// Pins the 100.64.0.0/10 (RFC 6598 Tailscale/CGNAT) boundary so a future
	// edit can't silently widen or shrink the block.
	t.Run("isLocalOrPrivateHost tailscaleCGNAT boundary", func(t *testing.T) {
		assert.True(t, isLocalOrPrivateHost("100.64.0.1"), "100.64.0.1 is inside 100.64.0.0/10")
		assert.True(t, isLocalOrPrivateHost("100.127.255.254"), "100.127.255.254 is inside 100.64.0.0/10")
		assert.False(t, isLocalOrPrivateHost("100.63.255.255"), "100.63.255.255 is just below the block")
		assert.False(t, isLocalOrPrivateHost("100.128.0.1"), "100.128.0.1 is just above the block")
	})
}

func TestIsLocalRequest(t *testing.T) {

	t.Run("forwarded host list includes localhost", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		req := httptest.NewRequest("GET", "http://example.com", http.NoBody)
		req.Host = "example.com"
		req.RemoteAddr = "203.0.113.9:443"
		req.Header.Set("X-Forwarded-Host", "example.com, localhost:8080")
		ctx.Request = req

		assert.True(t, isLocalRequest(ctx, []string{"203.0.113.9/32"}))
	})

	// The "origin loopback" subtest was removed here per §13.4 (Origin/Referer
	// no longer factor into locality) — see the standalone
	// TestIsLocalRequest_OriginHeaderIgnored test below for its replacement.

	t.Run("non local request", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		req := httptest.NewRequest("GET", "http://example.com", http.NoBody)
		req.Host = "example.com"
		ctx.Request = req

		assert.False(t, isLocalRequest(ctx, nil))
	})
}

func TestClearSecureCookie(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "http://example.com/logout", http.NoBody)

	clearSecureCookie(ctx, "auth_token", nil)

	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	assert.Equal(t, "auth_token", cookies[0].Name)
	assert.Equal(t, -1, cookies[0].MaxAge)
	assert.True(t, cookies[0].Secure)
}

func TestAuthHandler_Login_Errors(t *testing.T) {
	t.Parallel()
	handler, _ := setupAuthHandler(t)
	r := gin.New()
	r.POST("/login", handler.Login)

	// 1. Invalid JSON
	req := httptest.NewRequest("POST", "/login", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 2. Invalid Credentials
	body := map[string]string{
		"email":    "nonexistent@example.com",
		"password": "wrong",
	}
	jsonBody, _ := json.Marshal(body)
	req = httptest.NewRequest("POST", "/login", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthHandler_Register(t *testing.T) {
	t.Parallel()
	handler, _ := setupAuthHandler(t)

	r := gin.New()
	r.POST("/register", handler.Register)

	body := map[string]string{
		"email":    "new@example.com",
		"password": "password123",
		"name":     "New User",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/register", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "new@example.com")
}

func TestAuthHandler_Register_Duplicate(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandler(t)
	db.Create(&models.User{UUID: uuid.NewString(), Email: "dup@example.com", Name: "Dup"})

	r := gin.New()
	r.POST("/register", handler.Register)

	body := map[string]string{
		"email":    "dup@example.com",
		"password": "password123",
		"name":     "Dup User",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/register", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestAuthHandler_Logout(t *testing.T) {
	t.Parallel()
	handler, _ := setupAuthHandler(t)

	r := gin.New()
	r.POST("/logout", handler.Logout)

	req := httptest.NewRequest("POST", "/logout", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Logged out")
	// Check cookie
	cookie := w.Result().Cookies()[0]
	assert.Equal(t, "auth_token", cookie.Name)
	assert.Equal(t, -1, cookie.MaxAge)
}

func TestAuthHandler_Me(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandler(t)

	// Create user that matches the middleware ID
	user := &models.User{
		UUID:  uuid.NewString(),
		Email: "me@example.com",
		Name:  "Me User",
		Role:  models.RoleAdmin,
	}
	db.Create(user)

	r := gin.New()
	// Simulate middleware
	r.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Set("role", user.Role)
		c.Next()
	})
	r.GET("/me", handler.Me)

	req := httptest.NewRequest("GET", "/me", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, float64(user.ID), resp["user_id"])
	assert.Equal(t, "admin", resp["role"])
	assert.Equal(t, "Me User", resp["name"])
	assert.Equal(t, "me@example.com", resp["email"])
}

func TestAuthHandler_Me_NotFound(t *testing.T) {
	t.Parallel()
	handler, _ := setupAuthHandler(t)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", uint(999)) // Non-existent ID
		c.Next()
	})
	r.GET("/me", handler.Me)

	req := httptest.NewRequest("GET", "/me", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAuthHandler_ChangePassword(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandler(t)

	// Create user
	user := &models.User{
		UUID:  uuid.NewString(),
		Email: "change@example.com",
		Name:  "Change User",
	}
	_ = user.SetPassword("oldpassword")
	db.Create(user)

	r := gin.New()
	// Simulate middleware
	r.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Next()
	})
	r.POST("/change-password", handler.ChangePassword)

	body := map[string]string{
		"old_password": "oldpassword",
		"new_password": "newpassword123",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/change-password", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Password updated successfully")

	// Verify password changed
	var updatedUser models.User
	db.First(&updatedUser, user.ID)
	assert.True(t, updatedUser.CheckPassword("newpassword123"))
}

func TestAuthHandler_ChangePassword_WrongOld(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandler(t)
	user := &models.User{UUID: uuid.NewString(), Email: "wrong@example.com"}
	_ = user.SetPassword("correct")
	db.Create(user)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Next()
	})
	r.POST("/change-password", handler.ChangePassword)

	body := map[string]string{
		"old_password": "wrong",
		"new_password": "newpassword",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/change-password", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthHandler_ChangePassword_Errors(t *testing.T) {
	t.Parallel()
	handler, _ := setupAuthHandler(t)
	r := gin.New()
	r.POST("/change-password", handler.ChangePassword)

	// 1. BindJSON error (checked before auth)
	req, _ := http.NewRequest("POST", "/change-password", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 2. Unauthorized (valid JSON but no user in context)
	body := map[string]string{
		"old_password": "oldpassword",
		"new_password": "newpassword123",
	}
	jsonBody, _ := json.Marshal(body)
	req, _ = http.NewRequest("POST", "/change-password", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// setupAuthHandlerWithDB creates an AuthHandler with DB access for forward auth tests
func setupAuthHandlerWithDB(t *testing.T) (*AuthHandler, *gorm.DB) {
	dbName := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	require.NoError(t, err)
	_ = db.AutoMigrate(&models.User{}, &models.Setting{}, &models.ProxyHost{})

	cfg := config.Config{JWTSecret: "test-secret"}
	authService := services.NewAuthService(db, cfg)
	return NewAuthHandlerWithDB(authService, db, nil), db
}

func TestNewAuthHandlerWithDB(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandlerWithDB(t)
	assert.NotNil(t, handler)
	assert.NotNil(t, handler.db)
	assert.NotNil(t, db)
}

func TestAuthHandler_Verify_NoCookie(t *testing.T) {
	t.Parallel()
	handler, _ := setupAuthHandlerWithDB(t)
	r := gin.New()
	r.GET("/verify", handler.Verify)

	req := httptest.NewRequest("GET", "/verify", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, "/login", w.Header().Get("X-Auth-Redirect"))
}

func TestAuthHandler_Verify_InvalidToken(t *testing.T) {
	t.Parallel()
	handler, _ := setupAuthHandlerWithDB(t)
	r := gin.New()
	r.GET("/verify", handler.Verify)

	req := httptest.NewRequest("GET", "/verify", http.NoBody)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: "invalid-token", Secure: true, HttpOnly: true})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthHandler_Verify_ValidToken(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandlerWithDB(t)

	// Create user
	user := &models.User{
		UUID:    uuid.NewString(),
		Email:   "test@example.com",
		Name:    "Test User",
		Role:    models.RoleUser,
		Enabled: true,
	}
	_ = user.SetPassword("password123")
	db.Create(user)

	// Generate token
	token, _ := handler.authService.GenerateToken(user)

	r := gin.New()
	r.GET("/verify", handler.Verify)

	req := httptest.NewRequest("GET", "/verify", http.NoBody)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token, Secure: true, HttpOnly: true})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "test@example.com", w.Header().Get("X-Forwarded-User"))
	assert.Equal(t, "user", w.Header().Get("X-Forwarded-Groups"))
}

func TestAuthHandler_Verify_BearerToken(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandlerWithDB(t)

	user := &models.User{
		UUID:    uuid.NewString(),
		Email:   "bearer@example.com",
		Name:    "Bearer User",
		Role:    models.RoleAdmin,
		Enabled: true,
	}
	_ = user.SetPassword("password123")
	db.Create(user)

	token, _ := handler.authService.GenerateToken(user)

	r := gin.New()
	r.GET("/verify", handler.Verify)

	req := httptest.NewRequest("GET", "/verify", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "bearer@example.com", w.Header().Get("X-Forwarded-User"))
}

func TestAuthHandler_Verify_DisabledUser(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandlerWithDB(t)

	user := &models.User{
		UUID:  uuid.NewString(),
		Email: "disabled@example.com",
		Name:  "Disabled User",
		Role:  models.RoleUser,
	}
	_ = user.SetPassword("password123")
	db.Create(user)
	// Explicitly disable after creation to bypass GORM's default:true behavior
	db.Model(user).Update("enabled", false)

	token, _ := handler.authService.GenerateToken(user)

	r := gin.New()
	r.GET("/verify", handler.Verify)

	req := httptest.NewRequest("GET", "/verify", http.NoBody)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token, Secure: true, HttpOnly: true})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthHandler_Verify_ForwardAuthDenied(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandlerWithDB(t)

	// Create proxy host with forward auth enabled
	proxyHost := &models.ProxyHost{
		UUID:               uuid.NewString(),
		Name:               "Protected App",
		DomainNames:        "app.example.com",
		ForwardAuthEnabled: true,
		Enabled:            true,
	}
	db.Create(proxyHost)

	// Create user with deny_all permission
	user := &models.User{
		UUID:           uuid.NewString(),
		Email:          "denied@example.com",
		Name:           "Denied User",
		Role:           models.RoleUser,
		Enabled:        true,
		PermissionMode: models.PermissionModeDenyAll,
	}
	_ = user.SetPassword("password123")
	db.Create(user)

	token, _ := handler.authService.GenerateToken(user)

	r := gin.New()
	r.GET("/verify", handler.Verify)

	req := httptest.NewRequest("GET", "/verify", http.NoBody)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token, Secure: true, HttpOnly: true})
	req.Header.Set("X-Forwarded-Host", "app.example.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAuthHandler_VerifyStatus_NotAuthenticated(t *testing.T) {
	t.Parallel()
	handler, _ := setupAuthHandlerWithDB(t)
	r := gin.New()
	r.GET("/status", handler.VerifyStatus)

	req := httptest.NewRequest("GET", "/status", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, false, resp["authenticated"])
}

func TestAuthHandler_VerifyStatus_InvalidToken(t *testing.T) {
	t.Parallel()
	handler, _ := setupAuthHandlerWithDB(t)
	r := gin.New()
	r.GET("/status", handler.VerifyStatus)

	req := httptest.NewRequest("GET", "/status", http.NoBody)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: "invalid", Secure: true, HttpOnly: true})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, false, resp["authenticated"])
}

func TestAuthHandler_VerifyStatus_Authenticated(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandlerWithDB(t)

	user := &models.User{
		UUID:    uuid.NewString(),
		Email:   "status@example.com",
		Name:    "Status User",
		Role:    models.RoleUser,
		Enabled: true,
	}
	_ = user.SetPassword("password123")
	db.Create(user)

	token, _ := handler.authService.GenerateToken(user)

	r := gin.New()
	r.GET("/status", handler.VerifyStatus)

	req := httptest.NewRequest("GET", "/status", http.NoBody)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token, Secure: true, HttpOnly: true})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, true, resp["authenticated"])
	userObj := resp["user"].(map[string]any)
	assert.Equal(t, "status@example.com", userObj["email"])
}

func TestAuthHandler_VerifyStatus_DisabledUser(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandlerWithDB(t)

	user := &models.User{
		UUID:  uuid.NewString(),
		Email: "disabled2@example.com",
		Name:  "Disabled User 2",
		Role:  models.RoleUser,
	}
	_ = user.SetPassword("password123")
	db.Create(user)
	// Explicitly disable after creation to bypass GORM's default:true behavior
	db.Model(user).Update("enabled", false)

	token, _ := handler.authService.GenerateToken(user)

	r := gin.New()
	r.GET("/status", handler.VerifyStatus)

	req := httptest.NewRequest("GET", "/status", http.NoBody)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token, Secure: true, HttpOnly: true})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, false, resp["authenticated"])
}

func TestAuthHandler_GetAccessibleHosts_Unauthorized(t *testing.T) {
	t.Parallel()
	handler, _ := setupAuthHandlerWithDB(t)
	r := gin.New()
	r.GET("/hosts", handler.GetAccessibleHosts)

	req := httptest.NewRequest("GET", "/hosts", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthHandler_GetAccessibleHosts_AllowAll(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandlerWithDB(t)

	// Create proxy hosts
	host1 := &models.ProxyHost{UUID: uuid.NewString(), Name: "Host 1", DomainNames: "host1.example.com", Enabled: true}
	host2 := &models.ProxyHost{UUID: uuid.NewString(), Name: "Host 2", DomainNames: "host2.example.com", Enabled: true}
	db.Create(host1)
	db.Create(host2)

	user := &models.User{
		UUID:           uuid.NewString(),
		Email:          "allowall@example.com",
		Name:           "Allow All User",
		Role:           models.RoleUser,
		Enabled:        true,
		PermissionMode: models.PermissionModeAllowAll,
	}
	db.Create(user)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Next()
	})
	r.GET("/hosts", handler.GetAccessibleHosts)

	req := httptest.NewRequest("GET", "/hosts", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	hosts := resp["hosts"].([]any)
	assert.Len(t, hosts, 2)
}

func TestAuthHandler_GetAccessibleHosts_DenyAll(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandlerWithDB(t)

	// Create proxy hosts
	host1 := &models.ProxyHost{UUID: uuid.NewString(), Name: "Host 1", DomainNames: "host1.example.com", Enabled: true}
	db.Create(host1)

	user := &models.User{
		UUID:           uuid.NewString(),
		Email:          "denyall@example.com",
		Name:           "Deny All User",
		Role:           models.RoleUser,
		Enabled:        true,
		PermissionMode: models.PermissionModeDenyAll,
	}
	db.Create(user)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Next()
	})
	r.GET("/hosts", handler.GetAccessibleHosts)

	req := httptest.NewRequest("GET", "/hosts", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	hosts := resp["hosts"].([]any)
	assert.Len(t, hosts, 0)
}

func TestAuthHandler_GetAccessibleHosts_PermittedHosts(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandlerWithDB(t)

	// Create proxy hosts
	host1 := &models.ProxyHost{UUID: uuid.NewString(), Name: "Host 1", DomainNames: "host1.example.com", Enabled: true}
	host2 := &models.ProxyHost{UUID: uuid.NewString(), Name: "Host 2", DomainNames: "host2.example.com", Enabled: true}
	db.Create(host1)
	db.Create(host2)

	user := &models.User{
		UUID:           uuid.NewString(),
		Email:          "permitted@example.com",
		Name:           "Permitted User",
		Role:           models.RoleUser,
		Enabled:        true,
		PermissionMode: models.PermissionModeDenyAll,
		PermittedHosts: []models.ProxyHost{*host1}, // Only host1
	}
	db.Create(user)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Next()
	})
	r.GET("/hosts", handler.GetAccessibleHosts)

	req := httptest.NewRequest("GET", "/hosts", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	hosts := resp["hosts"].([]any)
	assert.Len(t, hosts, 1)
}

func TestAuthHandler_GetAccessibleHosts_UserNotFound(t *testing.T) {
	t.Parallel()
	handler, _ := setupAuthHandlerWithDB(t)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", uint(99999))
		c.Next()
	})
	r.GET("/hosts", handler.GetAccessibleHosts)

	req := httptest.NewRequest("GET", "/hosts", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAuthHandler_CheckHostAccess_Unauthorized(t *testing.T) {
	t.Parallel()
	handler, _ := setupAuthHandlerWithDB(t)
	r := gin.New()
	r.GET("/hosts/:hostId/access", handler.CheckHostAccess)

	req := httptest.NewRequest("GET", "/hosts/1/access", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthHandler_CheckHostAccess_InvalidHostID(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandlerWithDB(t)

	user := &models.User{UUID: uuid.NewString(), Email: "check@example.com", Enabled: true}
	db.Create(user)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Next()
	})
	r.GET("/hosts/:hostId/access", handler.CheckHostAccess)

	req := httptest.NewRequest("GET", "/hosts/invalid/access", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthHandler_CheckHostAccess_Allowed(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandlerWithDB(t)

	host := &models.ProxyHost{UUID: uuid.NewString(), Name: "Test Host", DomainNames: "test.example.com", Enabled: true}
	db.Create(host)

	user := &models.User{
		UUID:           uuid.NewString(),
		Email:          "checkallowed@example.com",
		Enabled:        true,
		PermissionMode: models.PermissionModeAllowAll,
	}
	db.Create(user)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Next()
	})
	r.GET("/hosts/:hostId/access", handler.CheckHostAccess)

	req := httptest.NewRequest("GET", "/hosts/1/access", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, true, resp["can_access"])
}

func TestAuthHandler_CheckHostAccess_Denied(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandlerWithDB(t)

	host := &models.ProxyHost{UUID: uuid.NewString(), Name: "Protected Host", DomainNames: "protected.example.com", Enabled: true}
	db.Create(host)

	user := &models.User{
		UUID:           uuid.NewString(),
		Email:          "checkdenied@example.com",
		Enabled:        true,
		PermissionMode: models.PermissionModeDenyAll,
	}
	db.Create(user)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Next()
	})
	r.GET("/hosts/:hostId/access", handler.CheckHostAccess)

	req := httptest.NewRequest("GET", "/hosts/1/access", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, false, resp["can_access"])
}

func TestAuthHandler_Logout_InvalidatesBearerSession(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandler(t)

	user := &models.User{
		UUID:    uuid.NewString(),
		Email:   "logout-session@example.com",
		Name:    "Logout Session",
		Role:    models.RoleAdmin,
		Enabled: true,
	}
	_ = user.SetPassword("password123")
	require.NoError(t, db.Create(user).Error)

	r := gin.New()
	r.POST("/auth/login", handler.Login)
	protected := r.Group("/")
	protected.Use(middleware.AuthMiddleware(handler.authService))
	protected.POST("/auth/logout", handler.Logout)
	protected.GET("/auth/me", handler.Me)

	loginBody, _ := json.Marshal(map[string]string{
		"email":    "logout-session@example.com",
		"password": "password123",
	})
	loginReq := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRes := httptest.NewRecorder()
	r.ServeHTTP(loginRes, loginReq)
	require.Equal(t, http.StatusOK, loginRes.Code)

	var loginPayload map[string]string
	require.NoError(t, json.Unmarshal(loginRes.Body.Bytes(), &loginPayload))
	token := loginPayload["token"]
	require.NotEmpty(t, token)

	meReq := httptest.NewRequest(http.MethodGet, "/auth/me", http.NoBody)
	meReq.Header.Set("Authorization", "Bearer "+token)
	meRes := httptest.NewRecorder()
	r.ServeHTTP(meRes, meReq)
	require.Equal(t, http.StatusOK, meRes.Code)

	logoutReq := httptest.NewRequest(http.MethodPost, "/auth/logout", http.NoBody)
	logoutReq.Header.Set("Authorization", "Bearer "+token)
	logoutRes := httptest.NewRecorder()
	r.ServeHTTP(logoutRes, logoutReq)
	require.Equal(t, http.StatusOK, logoutRes.Code)

	meAfterLogoutReq := httptest.NewRequest(http.MethodGet, "/auth/me", http.NoBody)
	meAfterLogoutReq.Header.Set("Authorization", "Bearer "+token)
	meAfterLogoutRes := httptest.NewRecorder()
	r.ServeHTTP(meAfterLogoutRes, meAfterLogoutReq)
	require.Equal(t, http.StatusUnauthorized, meAfterLogoutRes.Code)
}

// TestAuthHandler_Logout_InvalidatesSessionBeforeClearingCookie documents the
// §9.7 known-limitation mitigation (docs/plans/current_spec.md): Logout calls
// h.authService.InvalidateSessions(userID) (auth_handler.go:214) before
// clearSecureCookie (auth_handler.go:221). If the logout request arrives over
// a local/Tailscale plain-HTTP origin, setSecureCookie's clearing cookie is
// non-Secure, which the browser may refuse to apply on top of an
// earlier-Secure cookie (RFC 6265bis "Leave Secure Cookies Alone"), leaving a
// stale client-side cookie. This test proves that scenario is inert: session
// invalidation happens server-side regardless, so the stale/original token is
// rejected on the very next request even if the client-side cookie clear
// silently failed.
func TestAuthHandler_Logout_InvalidatesSessionBeforeClearingCookie(t *testing.T) {
	t.Parallel()
	handler, db := setupAuthHandler(t)

	user := &models.User{
		UUID:    uuid.NewString(),
		Email:   "cross-scheme-logout@example.com",
		Name:    "Cross Scheme Logout",
		Role:    models.RoleUser,
		Enabled: true,
	}
	require.NoError(t, user.SetPassword("password123"))
	require.NoError(t, db.Create(user).Error)

	token, err := handler.authService.GenerateToken(user)
	require.NoError(t, err)

	r := gin.New()
	protected := r.Group("/")
	protected.Use(middleware.AuthMiddleware(handler.authService))
	protected.POST("/auth/logout", handler.Logout)

	// Simulate an admin logging out over the instance's local/Tailscale
	// plain-HTTP address (the reported bug's origin) after an earlier HTTPS
	// session — this is exactly the request shape that makes
	// clearSecureCookie emit a non-Secure cookie.
	logoutReq := httptest.NewRequest(http.MethodPost, "/auth/logout", http.NoBody)
	logoutReq.Host = "100.98.12.109:8787"
	logoutReq.RemoteAddr = "100.98.12.109:9999"
	logoutReq.Header.Set("Authorization", "Bearer "+token)
	logoutReq.Header.Set("X-Forwarded-Proto", "http")
	logoutRes := httptest.NewRecorder()
	r.ServeHTTP(logoutRes, logoutReq)
	require.Equal(t, http.StatusOK, logoutRes.Code)

	// The clearing cookie itself is non-Secure per the local/HTTP downgrade...
	cookies := logoutRes.Result().Cookies()
	require.Len(t, cookies, 1)
	assert.False(t, cookies[0].Secure)

	// ...but InvalidateSessions already ran server-side (bumping
	// session_version) before that cookie was ever written, so the pre-logout
	// token is rejected regardless of whether the browser actually applies the
	// (possibly-refused) clearing cookie — the stale cookie is inert, not a
	// live session.
	var reloaded models.User
	require.NoError(t, db.First(&reloaded, user.ID).Error)
	assert.Equal(t, user.SessionVersion+1, reloaded.SessionVersion, "InvalidateSessions must have run and bumped session_version")

	_, _, err = handler.authService.AuthenticateToken(token)
	assert.Error(t, err, "the pre-logout token must be rejected once InvalidateSessions has run, regardless of the clearing cookie's Secure attribute")
}

func TestAuthHandler_Me_RequiresUserContext(t *testing.T) {
	t.Parallel()
	handler, _ := setupAuthHandler(t)

	r := gin.New()
	r.GET("/me", handler.Me)

	req := httptest.NewRequest(http.MethodGet, "/me", http.NoBody)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	assert.Equal(t, http.StatusUnauthorized, res.Code)
}

func TestAuthHandler_HelperFunctions(t *testing.T) {
	t.Parallel()

	t.Run("requestScheme prefers forwarded proto", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		req := httptest.NewRequest(http.MethodGet, "http://example.com", http.NoBody)
		req.RemoteAddr = "203.0.113.9:443"
		req.Header.Set("X-Forwarded-Proto", "HTTPS, http")
		ctx.Request = req
		assert.Equal(t, "https", requestScheme(ctx, []string{"203.0.113.9/32"}))
	})

	t.Run("requestScheme uses tls when forwarded proto missing", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		req := httptest.NewRequest(http.MethodGet, "http://example.com", http.NoBody)
		req.TLS = &tls.ConnectionState{}
		ctx.Request = req
		assert.Equal(t, "https", requestScheme(ctx, nil))
	})

	t.Run("requestScheme uses request url scheme when available", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		req := httptest.NewRequest(http.MethodGet, "http://example.com", http.NoBody)
		req.URL.Scheme = "HTTP"
		ctx.Request = req
		assert.Equal(t, "http", requestScheme(ctx, nil))
	})

	t.Run("requestScheme defaults to http when request url is nil", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		req := httptest.NewRequest(http.MethodGet, "http://example.com", http.NoBody)
		req.URL = nil
		ctx.Request = req
		assert.Equal(t, "http", requestScheme(ctx, nil))
	})

	t.Run("normalizeHost strips brackets and port", func(t *testing.T) {
		assert.Equal(t, "::1", normalizeHost("[::1]:443"))
		assert.Equal(t, "example.com", normalizeHost("example.com:8080"))
	})

	t.Run("isLocalOrPrivateHost and isLocalRequest", func(t *testing.T) {
		assert.True(t, isLocalOrPrivateHost("localhost"))
		assert.True(t, isLocalOrPrivateHost("127.0.0.1"))
		assert.False(t, isLocalOrPrivateHost("example.com"))

		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		req := httptest.NewRequest(http.MethodGet, "http://service.internal", http.NoBody)
		req.Host = "service.internal:8080"
		req.RemoteAddr = "203.0.113.9:443"
		req.Header.Set("X-Forwarded-Host", "example.com, localhost:8080")
		ctx.Request = req
		assert.True(t, isLocalRequest(ctx, []string{"203.0.113.9/32"}))
	})
}

func TestAuthHandler_Refresh(t *testing.T) {
	t.Parallel()

	handler, db := setupAuthHandler(t)

	user := &models.User{UUID: uuid.NewString(), Email: "refresh@example.com", Name: "Refresh User", Role: models.RoleUser, Enabled: true}
	require.NoError(t, user.SetPassword("password123"))
	require.NoError(t, db.Create(user).Error)

	r := gin.New()
	r.POST("/refresh", func(c *gin.Context) {
		c.Set("userID", user.ID)
		handler.Refresh(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/refresh", http.NoBody)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	assert.Equal(t, http.StatusOK, res.Code)
	assert.Contains(t, res.Body.String(), "token")
	cookies := res.Result().Cookies()
	assert.NotEmpty(t, cookies)
}

func TestAuthHandler_Refresh_Unauthorized(t *testing.T) {
	t.Parallel()

	handler, _ := setupAuthHandler(t)
	r := gin.New()
	r.POST("/refresh", handler.Refresh)

	req := httptest.NewRequest(http.MethodPost, "/refresh", http.NoBody)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	assert.Equal(t, http.StatusUnauthorized, res.Code)
}

func TestAuthHandler_Register_BadRequest(t *testing.T) {
	t.Parallel()

	handler, _ := setupAuthHandler(t)
	r := gin.New()
	r.POST("/register", handler.Register)

	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	assert.Equal(t, http.StatusBadRequest, res.Code)
}

func TestAuthHandler_Logout_InvalidateSessionsFailure(t *testing.T) {
	t.Parallel()

	handler, _ := setupAuthHandler(t)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", uint(999999))
		c.Next()
	})
	r.POST("/logout", handler.Logout)

	req := httptest.NewRequest(http.MethodPost, "/logout", http.NoBody)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	assert.Equal(t, http.StatusInternalServerError, res.Code)
	assert.Contains(t, res.Body.String(), "Failed to invalidate session")
}

func TestAuthHandler_Verify_UsesOriginalHostFallback(t *testing.T) {
	t.Parallel()

	handler, db := setupAuthHandlerWithDB(t)

	proxyHost := &models.ProxyHost{
		UUID:               uuid.NewString(),
		Name:               "Original Host App",
		DomainNames:        "original-host.example.com",
		ForwardAuthEnabled: true,
		Enabled:            true,
	}
	require.NoError(t, db.Create(proxyHost).Error)

	user := &models.User{
		UUID:           uuid.NewString(),
		Email:          "originalhost@example.com",
		Name:           "Original Host User",
		Role:           models.RoleUser,
		Enabled:        true,
		PermissionMode: models.PermissionModeAllowAll,
	}
	require.NoError(t, user.SetPassword("password123"))
	require.NoError(t, db.Create(user).Error)

	token, err := handler.authService.GenerateToken(user)
	require.NoError(t, err)

	r := gin.New()
	r.GET("/verify", handler.Verify)

	req := httptest.NewRequest(http.MethodGet, "/verify", http.NoBody)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token, Secure: true, HttpOnly: true})
	req.Header.Set("X-Original-Host", "original-host.example.com")
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	assert.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, "originalhost@example.com", res.Header().Get("X-Forwarded-User"))
}

func TestAuthHandler_GetAccessibleHosts_DatabaseUnavailable(t *testing.T) {
	t.Parallel()

	handler, _ := setupAuthHandler(t)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", uint(1))
		c.Next()
	})
	r.GET("/hosts", handler.GetAccessibleHosts)

	req := httptest.NewRequest(http.MethodGet, "/hosts", http.NoBody)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	assert.Equal(t, http.StatusInternalServerError, res.Code)
	assert.Contains(t, res.Body.String(), "Database not available")
}

func TestAuthHandler_CheckHostAccess_DatabaseUnavailable(t *testing.T) {
	t.Parallel()

	handler, _ := setupAuthHandler(t)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", uint(1))
		c.Next()
	})
	r.GET("/hosts/:hostId/access", handler.CheckHostAccess)

	req := httptest.NewRequest(http.MethodGet, "/hosts/1/access", http.NoBody)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	assert.Equal(t, http.StatusInternalServerError, res.Code)
	assert.Contains(t, res.Body.String(), "Database not available")
}

func TestAuthHandler_CheckHostAccess_UserNotFound(t *testing.T) {
	t.Parallel()

	handler, _ := setupAuthHandlerWithDB(t)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", uint(999999))
		c.Next()
	})
	r.GET("/hosts/:hostId/access", handler.CheckHostAccess)

	req := httptest.NewRequest(http.MethodGet, "/hosts/1/access", http.NoBody)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	assert.Equal(t, http.StatusNotFound, res.Code)
	assert.Contains(t, res.Body.String(), "User not found")
}
