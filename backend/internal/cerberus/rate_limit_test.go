package cerberus

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/ratelimit"
	"github.com/Wikid82/charon/backend/internal/services"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func newTestClock() *testClock {
	return &testClock{now: time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)}
}

func (tc *testClock) Now() time.Time {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	return tc.now
}

func (tc *testClock) Advance(d time.Duration) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.now = tc.now.Add(d)
}

func setupRateLimitTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:rate_limit_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Setting{}))
	return db
}

func enabledLimitConfig(requests, windowSec, burst int) config.SecurityConfig {
	return config.SecurityConfig{
		RateLimitMode:      "enabled",
		RateLimitRequests:  requests,
		RateLimitWindowSec: windowSec,
		RateLimitBurst:     burst,
	}
}

// newLimitedRouter builds a router with optional pre-middleware (e.g. auth context) and the
// Cerberus limiter, serving 200 on every given path for GET and POST.
func newLimitedRouter(cerb *Cerberus, pre []gin.HandlerFunc, paths ...string) *gin.Engine {
	r := gin.New()
	r.Use(pre...)
	r.Use(cerb.RateLimitMiddleware())
	ok := func(c *gin.Context) { c.Status(http.StatusOK) }
	for _, p := range paths {
		r.GET(p, ok)
		r.POST(p, ok)
	}
	return r
}

func serve(r *gin.Engine, method, path, remoteAddr string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, http.NoBody)
	req.RemoteAddr = remoteAddr
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func setRole(role string, userID uint) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("role", role)
		c.Set("userID", userID)
		c.Next()
	}
}

func TestCerberusRateLimitMiddleware_DisabledAllowsTraffic(t *testing.T) {
	cerb := New(config.SecurityConfig{RateLimitMode: "disabled"}, nil)
	r := newLimitedRouter(cerb, nil, "/")
	for i := 0; i < 3; i++ {
		assert.Equal(t, http.StatusOK, serve(r, http.MethodGet, "/", "10.0.0.1:1234", nil).Code)
	}
}

func TestCerberusRateLimitMiddleware_EnabledByConfigSetsRetryAfter(t *testing.T) {
	cerb := New(enabledLimitConfig(1, 60, 1), nil)
	r := newLimitedRouter(cerb, nil, "/")

	assert.Equal(t, http.StatusOK, serve(r, http.MethodGet, "/", "10.0.0.1:1234", nil).Code)
	w := serve(r, http.MethodGet, "/", "10.0.0.1:1234", nil)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, "60", w.Header().Get("Retry-After"))
	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, ratelimit.TooManyRequestsMessage, body["error"])
}

func TestCerberusRateLimitMiddleware_RefillsWithInjectedClock(t *testing.T) {
	clk := newTestClock()
	cerb := New(enabledLimitConfig(1, 10, 1), nil)
	cerb.now = clk.Now
	r := newLimitedRouter(cerb, nil, "/")

	assert.Equal(t, http.StatusOK, serve(r, http.MethodGet, "/", "10.0.0.1:1234", nil).Code)
	assert.Equal(t, http.StatusTooManyRequests, serve(r, http.MethodGet, "/", "10.0.0.1:1234", nil).Code)
	clk.Advance(10 * time.Second)
	assert.Equal(t, http.StatusOK, serve(r, http.MethodGet, "/", "10.0.0.1:1234", nil).Code)
}

func TestCerberusRateLimitMiddleware_DifferentClientsIndependent(t *testing.T) {
	cerb := New(enabledLimitConfig(1, 60, 1), nil)
	r := newLimitedRouter(cerb, nil, "/")
	assert.Equal(t, http.StatusOK, serve(r, http.MethodGet, "/", "10.0.0.1:1234", nil).Code)
	assert.Equal(t, http.StatusOK, serve(r, http.MethodGet, "/", "10.0.0.2:1234", nil).Code)
}

func TestCerberusRateLimitMiddleware_IPv6SlashSixtyFourShared(t *testing.T) {
	cerb := New(enabledLimitConfig(1, 60, 1), nil)
	r := newLimitedRouter(cerb, nil, "/")
	assert.Equal(t, http.StatusOK, serve(r, http.MethodGet, "/", "[2001:db8:1:2::1]:1234", nil).Code)
	assert.Equal(t, http.StatusTooManyRequests, serve(r, http.MethodGet, "/", "[2001:db8:1:2::ffff]:1234", nil).Code)
	assert.Equal(t, http.StatusOK, serve(r, http.MethodGet, "/", "[2001:db8:1:3::1]:1234", nil).Code)
}

func TestCerberusRateLimitMiddleware_EmergencyBypass(t *testing.T) {
	cerb := New(enabledLimitConfig(1, 60, 1), nil)
	bypass := func(c *gin.Context) {
		c.Set(middleware.EmergencyBypassContextKey, true)
		c.Next()
	}
	r := newLimitedRouter(cerb, []gin.HandlerFunc{bypass}, "/")
	for i := 0; i < 3; i++ {
		assert.Equal(t, http.StatusOK, serve(r, http.MethodGet, "/", "10.0.0.1:1234", nil).Code)
	}
}

func TestCerberusRateLimitMiddleware_NonBoolBypassDoesNotPanic(t *testing.T) {
	cerb := New(enabledLimitConfig(1, 60, 1), nil)
	bogus := func(c *gin.Context) {
		c.Set(middleware.EmergencyBypassContextKey, "yes")
		c.Next()
	}
	r := newLimitedRouter(cerb, []gin.HandlerFunc{bogus}, "/")
	assert.NotPanics(t, func() {
		assert.Equal(t, http.StatusOK, serve(r, http.MethodGet, "/", "10.0.0.1:1234", nil).Code)
		assert.Equal(t, http.StatusTooManyRequests, serve(r, http.MethodGet, "/", "10.0.0.1:1234", nil).Code)
	})
}

func TestCerberusRateLimitMiddleware_EnabledBySetting(t *testing.T) {
	db := setupRateLimitTestDB(t)
	for k, v := range map[string]string{
		"security.rate_limit.enabled":  "true",
		"security.rate_limit.requests": "1",
		"security.rate_limit.window":   "1",
		"security.rate_limit.burst":    "1",
	} {
		require.NoError(t, db.Create(&models.Setting{Key: k, Value: v}).Error)
	}
	cerb := New(config.SecurityConfig{RateLimitMode: "disabled"}, db)
	r := newLimitedRouter(cerb, nil, "/")
	assert.Equal(t, http.StatusOK, serve(r, http.MethodGet, "/", "10.0.0.1:1234", nil).Code)
	assert.Equal(t, http.StatusTooManyRequests, serve(r, http.MethodGet, "/", "10.0.0.1:1234", nil).Code)
}

func TestCerberusRateLimitMiddleware_OverridesConfigWithSettings(t *testing.T) {
	db := setupRateLimitTestDB(t)
	for k, v := range map[string]string{
		"security.rate_limit.enabled":  "true",
		"security.rate_limit.requests": "1",
		"security.rate_limit.window":   "1",
		"security.rate_limit.burst":    "1",
	} {
		require.NoError(t, db.Create(&models.Setting{Key: k, Value: v}).Error)
	}
	cerb := New(enabledLimitConfig(10, 10, 10), db)
	r := newLimitedRouter(cerb, nil, "/")
	assert.Equal(t, http.StatusOK, serve(r, http.MethodGet, "/", "10.0.0.1:1234", nil).Code)
	assert.Equal(t, http.StatusTooManyRequests, serve(r, http.MethodGet, "/", "10.0.0.1:1234", nil).Code)
}

func TestCerberusRateLimitMiddleware_InvalidSettingsIgnored(t *testing.T) {
	db := setupRateLimitTestDB(t)
	for k, v := range map[string]string{
		"security.rate_limit.requests": "abc",
		"security.rate_limit.window":   "-5",
		"security.rate_limit.burst":    "0",
	} {
		require.NoError(t, db.Create(&models.Setting{Key: k, Value: v}).Error)
	}
	cerb := New(enabledLimitConfig(2, 60, 2), db)
	requests, window, burst := cerb.effectiveRateLimit()
	assert.Equal(t, []int{2, 60, 2}, []int{requests, window, burst})
}

func TestCerberusRateLimitMiddleware_SettingsDisableOverride(t *testing.T) {
	db := setupRateLimitTestDB(t)
	require.NoError(t, db.Create(&models.Setting{Key: "security.rate_limit.enabled", Value: "false"}).Error)
	cerb := New(enabledLimitConfig(1, 60, 1), db)
	r := newLimitedRouter(cerb, nil, "/")
	for i := 0; i < 3; i++ {
		assert.Equal(t, http.StatusOK, serve(r, http.MethodGet, "/", "10.0.0.1:1234", nil).Code)
	}
}

func TestCerberusRateLimitMiddleware_DefaultsWhenUnset(t *testing.T) {
	cerb := New(config.SecurityConfig{RateLimitMode: "enabled"}, nil)
	requests, window, burst := cerb.effectiveRateLimit()
	assert.Equal(t, []int{100, 60, 20}, []int{requests, window, burst})
}

func TestCerberusRateLimitMiddleware_WindowFallback(t *testing.T) {
	cerb := New(enabledLimitConfig(1, 0, 1), nil)
	_, window, _ := cerb.effectiveRateLimit()
	assert.Equal(t, 60, window)
	r := newLimitedRouter(cerb, nil, "/")
	assert.Equal(t, http.StatusOK, serve(r, http.MethodGet, "/", "10.0.0.1:1234", nil).Code)
	assert.Equal(t, http.StatusTooManyRequests, serve(r, http.MethodGet, "/", "10.0.0.1:1234", nil).Code)
}

func TestCerberusRateLimitMiddleware_SettingsReconfigureResetsBuckets(t *testing.T) {
	db := setupRateLimitTestDB(t)
	require.NoError(t, db.Create(&models.Setting{Key: "security.rate_limit.burst", Value: "1"}).Error)
	cerb := New(enabledLimitConfig(1, 60, 1), db)
	r := newLimitedRouter(cerb, nil, "/")
	assert.Equal(t, http.StatusOK, serve(r, http.MethodGet, "/", "10.0.0.1:1234", nil).Code)
	assert.Equal(t, http.StatusTooManyRequests, serve(r, http.MethodGet, "/", "10.0.0.1:1234", nil).Code)

	require.NoError(t, db.Model(&models.Setting{}).Where("key = ?", "security.rate_limit.burst").Update("value", "3").Error)
	cerb.InvalidateCache()
	for i := 0; i < 3; i++ {
		assert.Equal(t, http.StatusOK, serve(r, http.MethodGet, "/", "10.0.0.1:1234", nil).Code, "request %d", i)
	}
	assert.Equal(t, http.StatusTooManyRequests, serve(r, http.MethodGet, "/", "10.0.0.1:1234", nil).Code)
}

func TestCerberusRateLimitMiddleware_ValidatedAdminExempt(t *testing.T) {
	cerb := New(enabledLimitConfig(1, 60, 1), nil)
	r := newLimitedRouter(cerb, []gin.HandlerFunc{setRole("admin", 1)},
		"/api/v1/security/status", "/api/v1/settings", "/api/v1/config", "/api/v1/config/x")
	for _, p := range []string{"/api/v1/security/status", "/api/v1/settings", "/api/v1/config", "/api/v1/config/x"} {
		for i := 0; i < 3; i++ {
			assert.Equal(t, http.StatusOK, serve(r, http.MethodPost, p, "10.0.0.1:1234", nil).Code, p)
		}
	}
}

func TestCerberusRateLimitMiddleware_AdminNonControlPlanePathStillLimited(t *testing.T) {
	cerb := New(enabledLimitConfig(1, 60, 1), nil)
	r := newLimitedRouter(cerb, []gin.HandlerFunc{setRole("admin", 1)}, "/api/v1/users")
	assert.Equal(t, http.StatusOK, serve(r, http.MethodGet, "/api/v1/users", "10.0.0.1:1234", nil).Code)
	assert.Equal(t, http.StatusTooManyRequests, serve(r, http.MethodGet, "/api/v1/users", "10.0.0.1:1234", nil).Code)
}

func TestCerberusRateLimitMiddleware_AdminWithoutUserIDIsLimited(t *testing.T) {
	cerb := New(enabledLimitConfig(1, 60, 1), nil)
	r := newLimitedRouter(cerb, []gin.HandlerFunc{setRole("admin", 0)}, "/api/v1/settings")
	assert.Equal(t, http.StatusOK, serve(r, http.MethodGet, "/api/v1/settings", "10.0.0.1:1234", nil).Code)
	assert.Equal(t, http.StatusTooManyRequests, serve(r, http.MethodGet, "/api/v1/settings", "10.0.0.1:1234", nil).Code)
}

func TestCerberusRateLimitMiddleware_SegmentAwarePrefix(t *testing.T) {
	cerb := New(enabledLimitConfig(1, 60, 1), nil)
	r := newLimitedRouter(cerb, []gin.HandlerFunc{setRole("admin", 1)}, "/api/v1/settingsx", "/api/v1/configuration")
	for _, p := range []string{"/api/v1/settingsx", "/api/v1/configuration"} {
		addr := "10.0.0." + fmt.Sprint(len(p)) + ":1234"
		assert.Equal(t, http.StatusOK, serve(r, http.MethodGet, p, addr, nil).Code, p)
		assert.Equal(t, http.StatusTooManyRequests, serve(r, http.MethodGet, p, addr, nil).Code, p)
	}
}

func TestCerberusRateLimitMiddleware_DecodedRawPathForAdmin(t *testing.T) {
	cerb := New(enabledLimitConfig(1, 60, 1), nil)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/security%2Frules", http.NoBody)
	req.URL.Path = "/api/v1/security%2Frules"
	req.URL.RawPath = "/api/v1/security%2Frules"
	ctx.Request = req
	ctx.Set("role", "admin")
	ctx.Set("userID", uint(1))
	assert.True(t, cerb.isAdminSecurityControlPlaneRequest(ctx))

	ctx.Request.URL.RawPath = "/api/v1/security%zz"
	ctx.Request.URL.Path = "/api/v1/security/rules"
	assert.True(t, cerb.isAdminSecurityControlPlaneRequest(ctx), "undecodable raw path falls back to Path")
}

// authServiceWithUsers returns an AuthService plus valid tokens for an admin and a regular user.
func authServiceWithUsers(t *testing.T) (authSvc *services.AuthService, adminTok, userTok string) {
	t.Helper()
	dsn := fmt.Sprintf("file:rate_limit_auth_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.User{}))
	svc := services.NewAuthService(db, config.Config{JWTSecret: "test-secret"})

	admin := models.User{UUID: "admin-uuid", APIKey: "admin-key", Email: "admin@example.com", Role: models.RoleAdmin, Enabled: true}
	user := models.User{UUID: "user-uuid", APIKey: "user-key", Email: "user@example.com", Role: models.RoleUser, Enabled: true}
	require.NoError(t, db.Create(&admin).Error)
	require.NoError(t, db.Create(&user).Error)
	adminToken, err := svc.GenerateToken(&admin)
	require.NoError(t, err)
	userToken, err := svc.GenerateToken(&user)
	require.NoError(t, err)
	return svc, adminToken, userToken
}

func TestCerberusRateLimitMiddleware_UnvalidatedBearerIsLimited(t *testing.T) {
	svc, _, _ := authServiceWithUsers(t)
	cerb := New(enabledLimitConfig(1, 60, 1), nil)
	r := newLimitedRouter(cerb, []gin.HandlerFunc{middleware.OptionalAuth(svc)}, "/api/v1/settings")
	h := map[string]string{"Authorization": "Bearer not-a-valid-token"}
	assert.Equal(t, http.StatusOK, serve(r, http.MethodPost, "/api/v1/settings", "10.0.0.1:1234", h).Code)
	assert.Equal(t, http.StatusTooManyRequests, serve(r, http.MethodPost, "/api/v1/settings", "10.0.0.1:1234", h).Code)
}

func TestCerberusRateLimitMiddleware_ValidNonAdminBearerIsLimited(t *testing.T) {
	svc, _, userToken := authServiceWithUsers(t)
	cerb := New(enabledLimitConfig(1, 60, 1), nil)
	r := newLimitedRouter(cerb, []gin.HandlerFunc{middleware.OptionalAuth(svc)}, "/api/v1/security/status")
	h := map[string]string{"Authorization": "Bearer " + userToken}
	assert.Equal(t, http.StatusOK, serve(r, http.MethodGet, "/api/v1/security/status", "10.0.0.1:1234", h).Code)
	assert.Equal(t, http.StatusTooManyRequests, serve(r, http.MethodGet, "/api/v1/security/status", "10.0.0.1:1234", h).Code)
}

func TestCerberusRateLimitMiddleware_ValidatedAdminBearerExempt(t *testing.T) {
	svc, adminToken, _ := authServiceWithUsers(t)
	cerb := New(enabledLimitConfig(1, 60, 1), nil)
	r := newLimitedRouter(cerb, []gin.HandlerFunc{middleware.OptionalAuth(svc)}, "/api/v1/security/status")
	h := map[string]string{"Authorization": "Bearer " + adminToken}
	for i := 0; i < 3; i++ {
		assert.Equal(t, http.StatusOK, serve(r, http.MethodGet, "/api/v1/security/status", "10.0.0.1:1234", h).Code)
	}
}

// captureLogger injects a request-scoped logger backed by a test hook.
func captureLogger() (gin.HandlerFunc, *logtest.Hook) {
	base, hook := logtest.NewNullLogger()
	base.SetLevel(logrus.DebugLevel)
	return func(c *gin.Context) {
		c.Set("logger", logrus.NewEntry(base))
		c.Next()
	}, hook
}

func countLevel(hook *logtest.Hook, level logrus.Level) int {
	n := 0
	for _, e := range hook.AllEntries() {
		if e.Level == level {
			n++
		}
	}
	return n
}

func TestCerberusRateLimitMiddleware_PerEpisodeWarnCapped(t *testing.T) {
	clk := newTestClock()
	cerb := New(enabledLimitConfig(1, 60, 1), nil)
	cerb.now = clk.Now
	capture, hook := captureLogger()
	r := newLimitedRouter(cerb, []gin.HandlerFunc{capture}, "/")

	// One client, many denials: one WARN for the episode, the rest DEBUG.
	serve(r, http.MethodGet, "/", "10.0.0.1:1234", nil)
	for i := 0; i < 5; i++ {
		serve(r, http.MethodGet, "/", "10.0.0.1:1234", nil)
	}
	assert.Equal(t, 1, countLevel(hook, logrus.WarnLevel))
	assert.Equal(t, 4, countLevel(hook, logrus.DebugLevel))

	// 30 distinct clients each start an episode: the global cap limits WARNs to its burst.
	hook.Reset()
	for i := 0; i < 30; i++ {
		addr := fmt.Sprintf("10.1.0.%d:1234", i)
		serve(r, http.MethodGet, "/", addr, nil)
		serve(r, http.MethodGet, "/", addr, nil)
	}
	assert.Equal(t, rateLimitWarnBurst-1, countLevel(hook, logrus.WarnLevel))

	// After the cap refills, the next WARN reports how many episodes were suppressed.
	hook.Reset()
	clk.Advance(time.Minute)
	serve(r, http.MethodGet, "/", "10.2.0.1:1234", nil)
	serve(r, http.MethodGet, "/", "10.2.0.1:1234", nil)
	warns := 0
	for _, e := range hook.AllEntries() {
		if e.Level != logrus.WarnLevel {
			continue
		}
		warns++
		assert.Equal(t, uint64(30-(rateLimitWarnBurst-1)), e.Data["suppressed"])
		assert.Equal(t, "10.2.0.1", e.Data["client"])
		assert.Equal(t, 60, e.Data["retry_after_seconds"])
		for _, v := range e.Data {
			assert.NotContains(t, fmt.Sprint(v), "Bearer")
		}
	}
	assert.Equal(t, 1, warns)
}

func TestCerberusRateLimitMiddleware_NoGoroutineLeak(t *testing.T) {
	cerb := New(enabledLimitConfig(1, 60, 1), nil)
	before := runtime.NumGoroutine()
	for i := 0; i < 50; i++ {
		_ = cerb.RateLimitMiddleware()
	}
	assert.LessOrEqual(t, runtime.NumGoroutine(), before, "limiters must not start goroutines")
}

func TestCerberusRateLimitMiddleware_NoRequestDataInBody(t *testing.T) {
	cerb := New(enabledLimitConfig(1, 60, 1), nil)
	r := newLimitedRouter(cerb, nil, "/")
	serve(r, http.MethodGet, "/?user=probe-marker", "10.0.0.1:1234", nil)
	w := serve(r, http.MethodGet, "/?user=probe-marker", "10.0.0.1:1234", nil)
	require.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.False(t, strings.Contains(w.Body.String(), "probe-marker"))
}

func TestCerberusRateLimitMiddleware_NonAdminRoleOnControlPlaneIsLimited(t *testing.T) {
	cerb := New(enabledLimitConfig(1, 60, 1), nil)
	r := newLimitedRouter(cerb, []gin.HandlerFunc{setRole("user", 5)}, "/api/v1/settings")
	assert.Equal(t, http.StatusOK, serve(r, http.MethodGet, "/api/v1/settings", "10.0.0.1:1234", nil).Code)
	assert.Equal(t, http.StatusTooManyRequests, serve(r, http.MethodGet, "/api/v1/settings", "10.0.0.1:1234", nil).Code)
}
