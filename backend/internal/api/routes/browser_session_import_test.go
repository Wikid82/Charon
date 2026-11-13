package routes

import (
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

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// The import routes accept admin cookie sessions, so they carry the origin check. A cross-site cookie POST is refused
// before the handler, while a bearer client and the configured public origin still reach it.
func TestRegisterImportHandler_CookieSessionsCheckOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(isolatedMemoryDSN(t)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.ImportSession{}, &models.ProxyHost{}))

	cfg := config.Config{JWTSecret: "test-secret"}
	admin := &models.User{UUID: uuid.NewString(), APIKey: uuid.NewString(), Email: "admin-import-origin@example.com", Role: models.RoleAdmin, Enabled: true}
	require.NoError(t, db.Create(admin).Error)
	token, err := services.NewAuthService(db, cfg).GenerateToken(admin)
	require.NoError(t, err)

	publicOrigin := "https://charon.example.com"
	setting := &originSettingSource{origin: publicOrigin, ok: false}
	guard := middleware.NewBrowserOriginGuard(setting.source)

	router := gin.New()
	RegisterImportHandler(router, db, cfg, "echo", t.TempDir(), "/import/Caddyfile", guard)

	send := func(method, path string, cookie bool, headers map[string]string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader("{}"))
		req.Host = "charon.internal:8080"
		if cookie {
			req.AddCookie(&http.Cookie{Name: "auth_token", Value: token, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	crossSite := map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": "https://evil.example"}

	for _, path := range []string{"/api/v1/import/upload", "/api/v1/import/npm/upload", "/api/v1/import/json/commit"} {
		t.Run(path+" cross-site cookie refused", func(t *testing.T) {
			assertCrossOriginRefused(t, send(http.MethodPost, path, true, crossSite), "https://evil.example")
		})
	}

	bearer := send(http.MethodDelete, "/api/v1/import/cancel", false, map[string]string{"Authorization": "Bearer " + token, "Sec-Fetch-Site": "cross-site", "Origin": "https://evil.example"})
	assert.NotContains(t, bearer.Body.String(), "cross_origin_refused")

	// The public origin is refused until the setting is saved and the guard refreshed, then it is allowed.
	publicHeaders := map[string]string{"Origin": publicOrigin}
	assertCrossOriginRefused(t, send(http.MethodDelete, "/api/v1/import/cancel", true, publicHeaders), publicOrigin)
	setting.ok = true
	require.NoError(t, guard.Refresh())
	assert.NotContains(t, send(http.MethodDelete, "/api/v1/import/cancel", true, publicHeaders).Body.String(), "cross_origin_refused")
}

// originSettingSource is a mutable TrustedOriginSource for tests.
type originSettingSource struct {
	origin string
	ok     bool
}

func (s *originSettingSource) source() (string, bool) { return s.origin, s.ok }
