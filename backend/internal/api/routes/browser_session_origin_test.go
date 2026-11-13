package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

const (
	browserSessionPublicOrigin = "https://charon.example.com"
	browserSessionInnerHost    = "charon.internal:8080"
)

// browserSessionFixture is a router built by Register with an admin account and its cookie and bearer tokens.
type browserSessionFixture struct {
	router *gin.Engine
	admin  string
}

func newBrowserSessionFixture(t *testing.T) *browserSessionFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	db, err := gorm.Open(sqlite.Open(isolatedMemoryDSN(t)), &gorm.Config{})
	require.NoError(t, err)
	cfg := config.Config{JWTSecret: "test-secret"}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	admin := &models.User{UUID: uuid.NewString(), APIKey: uuid.NewString(), Email: "admin-browser-session@example.com", Role: models.RoleAdmin, Enabled: true}
	require.NoError(t, db.Create(admin).Error)
	token, err := services.NewAuthService(db, cfg).GenerateToken(admin)
	require.NoError(t, err)
	return &browserSessionFixture{router: router, admin: token}
}

// send runs one request. A cookie session is sent as the auth_token cookie; bearer sends an Authorization header.
func (f *browserSessionFixture) send(method, path string, cookie bool, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader("{}"))
	req.Host = browserSessionInnerHost
	req.Header.Set("Content-Type", "application/json")
	if cookie {
		req.AddCookie(&http.Cookie{Name: "auth_token", Value: f.admin, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, req)
	return w
}

func (f *browserSessionFixture) sendBearer(method, path string, headers map[string]string) *httptest.ResponseRecorder {
	h := map[string]string{"Authorization": "Bearer " + f.admin}
	for k, v := range headers {
		h[k] = v
	}
	return f.send(method, path, false, h)
}

// A cookie session's cross-origin refusal has one fixed body. The body never echoes the Origin the browser sent.
func assertCrossOriginRefused(t *testing.T, w *httptest.ResponseRecorder, origin string) {
	t.Helper()
	require.Equal(t, http.StatusForbidden, w.Code)
	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]string{"error": "Cross-origin request refused", "error_code": "cross_origin_refused"}, body)
	assert.NotContains(t, w.Body.String(), origin)
}

func TestRegister_ProtectedCookieMutationsRefuseSameSiteAndCrossSite(t *testing.T) {
	f := newBrowserSessionFixture(t)

	// Each path is in a different protected subgroup: self-service, management, management admin, security admin and
	// security headers admin. The origin check runs before the handler, so a refused request changes nothing.
	paths := []string{
		"/api/v1/user/profile",
		"/api/v1/changelog/ack",
		"/api/v1/users",
		"/api/v1/settings",
		"/api/v1/security/config",
		"/api/v1/security/headers/presets/apply",
	}
	for _, path := range paths {
		for _, site := range []struct{ fetchSite, origin string }{
			{"same-site", "https://app.charon.test"},
			{"cross-site", "https://evil.example"},
		} {
			t.Run(path+" "+site.fetchSite, func(t *testing.T) {
				w := f.send(http.MethodPost, path, true, map[string]string{"Sec-Fetch-Site": site.fetchSite, "Origin": site.origin})
				assertCrossOriginRefused(t, w, site.origin)
			})
		}
	}
}

func TestRegister_ProtectedSafeMethodsAreNotRefusedAcrossSites(t *testing.T) {
	f := newBrowserSessionFixture(t)

	w := f.send(http.MethodGet, "/api/v1/user/profile", true, map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": "https://evil.example"})
	assert.NotEqual(t, http.StatusForbidden, w.Code)
	assert.NotContains(t, w.Body.String(), "cross_origin_refused")
}

func TestRegister_ProtectedNonBrowserAndHeaderClientsAreNotRefused(t *testing.T) {
	f := newBrowserSessionFixture(t)

	// A cookie client that sends no browser fetch metadata, and a bearer client that is cross-site on paper, both reach
	// the handler. The changelog acknowledgement is a harmless self-service write, not a security control check.
	noHeaders := f.send(http.MethodPost, "/api/v1/changelog/ack", true, nil)
	assert.NotContains(t, noHeaders.Body.String(), "cross_origin_refused")

	bearer := f.sendBearer(http.MethodPost, "/api/v1/changelog/ack", map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": "https://evil.example"})
	assert.NotContains(t, bearer.Body.String(), "cross_origin_refused")
}

func TestRegister_ProtectedProxiedOriginFollowsConfiguredPublicURL(t *testing.T) {
	f := newBrowserSessionFixture(t)
	publicOrigin := map[string]string{"Origin": browserSessionPublicOrigin}

	// Before the public address is configured, a page on that origin is refused: the proxy rewrote Host.
	before := f.send(http.MethodPost, "/api/v1/changelog/ack", true, publicOrigin)
	assertCrossOriginRefused(t, before, browserSessionPublicOrigin)

	// Saving the setting refreshes the guard. The write uses a bearer header, which the origin check passes.
	require.Equal(t, http.StatusOK, f.sendJSON(t, `{"key":"app.public_url","value":"`+browserSessionPublicOrigin+`"}`).Code)

	after := f.send(http.MethodPost, "/api/v1/changelog/ack", true, publicOrigin)
	assert.NotContains(t, after.Body.String(), "cross_origin_refused")

	// Only the configured origin is trusted; another origin is still refused.
	other := f.send(http.MethodPost, "/api/v1/changelog/ack", true, map[string]string{"Origin": "https://evil.example"})
	assertCrossOriginRefused(t, other, "https://evil.example")
}

// sendJSON posts a body with a bearer token, so the settings write itself is not refused by the origin check.
func (f *browserSessionFixture) sendJSON(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/settings", strings.NewReader(body))
	req.Host = browserSessionInnerHost
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+f.admin)
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, req)
	return w
}

func TestRegister_ProtectedWebSocketUpgradeFromOtherSiteIsRefused(t *testing.T) {
	f := newBrowserSessionFixture(t)
	upgrade := map[string]string{"Connection": "Upgrade", "Upgrade": "websocket"}

	cross := map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": "https://evil.example"}
	for k, v := range upgrade {
		cross[k] = v
	}
	assertCrossOriginRefused(t, f.send(http.MethodGet, "/api/v1/logs/live", true, cross), "https://evil.example")

	same := map[string]string{"Sec-Fetch-Site": "same-origin"}
	for k, v := range upgrade {
		same[k] = v
	}
	assert.NotContains(t, f.send(http.MethodGet, "/api/v1/logs/live", true, same).Body.String(), "cross_origin_refused")
}
