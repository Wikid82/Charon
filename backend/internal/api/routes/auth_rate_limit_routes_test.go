package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

type throttledApp struct {
	t      *testing.T
	router *gin.Engine
	db     *gorm.DB
	auth   *services.AuthService
}

func newThrottledApp(t *testing.T, cfg config.Config) *throttledApp {
	t.Helper()
	gin.SetMode(gin.TestMode)
	if cfg.JWTSecret == "" {
		cfg.JWTSecret = "test-secret"
	}
	db, err := gorm.Open(sqlite.Open(isolatedMemoryDSN(t)), &gorm.Config{})
	require.NoError(t, err)
	router := gin.New()
	require.NoError(t, router.SetTrustedProxies(nil))
	require.NoError(t, Register(context.Background(), router, db, cfg))
	return &throttledApp{t: t, router: router, db: db, auth: services.NewAuthService(db, cfg)}
}

func withAuthBudget(login, session int) config.Config {
	return config.Config{Security: config.SecurityConfig{AuthRateLimit: config.AuthRateLimitConfig{
		LoginRequests: login, LoginWindowSec: 600, SessionRequests: session, SessionWindowSec: 60,
	}}}
}

func (a *throttledApp) createUser(role models.UserRole) (*models.User, string) {
	a.t.Helper()
	u := &models.User{UUID: uuid.NewString(), APIKey: uuid.NewString(), Email: uuid.NewString() + "@example.com", Role: role, Enabled: true}
	require.NoError(a.t, u.SetPassword("correct-password"))
	require.NoError(a.t, a.db.Create(u).Error)
	token, err := a.auth.GenerateToken(u)
	require.NoError(a.t, err)
	return u, token
}

func (a *throttledApp) do(method, path, token, body string, headers map[string]string) *httptest.ResponseRecorder {
	var r io.Reader = http.NoBody
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	req.RemoteAddr = "198.51.100.7:4000"
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	a.router.ServeHTTP(w, req)
	return w
}

func (a *throttledApp) login(email, password string) *httptest.ResponseRecorder {
	return a.do(http.MethodPost, "/api/v1/auth/login", "", fmt.Sprintf(`{"email":%q,"password":%q}`, email, password), nil)
}

func assertGeneric429(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	require.Equal(t, http.StatusTooManyRequests, w.Code, "body: %s", w.Body.String())
	assert.NotEmpty(t, w.Header().Get("Retry-After"))
	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]string{"error": ratelimit.TooManyRequestsMessage}, body)
}

func TestRegister_AuthRoutesHaveExplicitRateLimitClass(t *testing.T) {
	app := newThrottledApp(t, config.Config{})
	table := middleware.AuthRouteClasses()

	registered := map[string]bool{}
	for _, r := range app.router.Routes() {
		if strings.HasPrefix(r.Path, "/api/v1/auth/") || r.Path == "/api/v1/auth" {
			key := r.Method + " " + r.Path
			registered[key] = true
			_, ok := table[key]
			assert.True(t, ok, "route %s must be classified in middleware.authRouteClasses", key)
		}
	}
	for key := range table {
		assert.True(t, registered[key], "classified route %s is not registered", key)
	}
}

func TestRegister_LoginThrottledBeforeHandler(t *testing.T) {
	app := newThrottledApp(t, config.Config{}) // production defaults: 10 per 600s
	start := time.Now()
	for i := 0; i < 10; i++ {
		w := app.login(fmt.Sprintf("probe-%d@example.com", i), "wrong")
		require.Equal(t, http.StatusUnauthorized, w.Code, "attempt %d", i+1)
	}

	victim, _ := app.createUser(models.RoleUser)
	w := app.login(victim.Email, "wrong")
	assertGeneric429(t, w)
	assertRetryAfterNear(t, w, 60, start)
	assert.NotContains(t, w.Body.String(), victim.Email)

	var stored models.User
	require.NoError(t, app.db.First(&stored, victim.ID).Error)
	assert.Zero(t, stored.FailedLoginAttempts, "a throttled request never reaches account bookkeeping")
}

func TestRegister_RefreshThrottledAsSession(t *testing.T) {
	app := newThrottledApp(t, withAuthBudget(100, 2))
	assert.Equal(t, http.StatusUnauthorized, app.do(http.MethodPost, "/api/v1/auth/refresh", "", "", nil).Code)
	assert.Equal(t, http.StatusUnauthorized, app.do(http.MethodPost, "/api/v1/auth/refresh", "", "", nil).Code)
	assertGeneric429(t, app.do(http.MethodPost, "/api/v1/auth/refresh", "", "", nil))
}

func TestRegister_SessionBudget(t *testing.T) {
	app := newThrottledApp(t, withAuthBudget(100, 3))
	start := time.Now()
	for i := 0; i < 3; i++ {
		assert.NotEqual(t, http.StatusTooManyRequests, app.do(http.MethodGet, "/api/v1/auth/status", "", "", nil).Code)
	}
	w := app.do(http.MethodGet, "/api/v1/auth/status", "", "", nil)
	assertGeneric429(t, w)
	assertRetryAfterNear(t, w, 20, start)
	assert.Equal(t, http.StatusUnauthorized, app.login("nobody@example.com", "x").Code, "the login budget is separate")
}

func TestRegister_ChangePasswordSharesLoginBudget(t *testing.T) {
	app := newThrottledApp(t, withAuthBudget(3, 60))
	_, token := app.createUser(models.RoleUser)
	change := `{"old_password":"wrong","new_password":"new-password-123"}`

	assert.NotEqual(t, http.StatusTooManyRequests, app.login("nobody@example.com", "x").Code)
	assert.NotEqual(t, http.StatusTooManyRequests, app.do(http.MethodPost, "/api/v1/auth/change-password", token, change, nil).Code)
	assert.NotEqual(t, http.StatusTooManyRequests, app.login("nobody@example.com", "x").Code)
	assertGeneric429(t, app.do(http.MethodPost, "/api/v1/auth/change-password", token, change, nil))
	assertGeneric429(t, app.login("nobody@example.com", "x"))
}

func TestRegister_ProfileEmailChangeSharesLoginBudget(t *testing.T) {
	app := newThrottledApp(t, withAuthBudget(2, 60))
	user, token := app.createUser(models.RoleUser)
	emailChange := `{"name":"N","email":"changed@example.com","current_password":"wrong"}`

	assert.Equal(t, http.StatusUnauthorized, app.do(http.MethodPost, "/api/v1/user/profile", token, emailChange, nil).Code)
	assert.Equal(t, http.StatusUnauthorized, app.login("nobody@example.com", "x").Code)
	assertGeneric429(t, app.do(http.MethodPost, "/api/v1/user/profile", token, emailChange, nil))

	nameOnly := fmt.Sprintf(`{"name":"Renamed","email":%q}`, user.Email)
	assert.Equal(t, http.StatusOK, app.do(http.MethodPost, "/api/v1/user/profile", token, nameOnly, nil).Code,
		"a name-only update verifies no password and is not throttled")
}

func TestRegister_CertificateKeyExportSharesLoginBudget(t *testing.T) {
	app := newThrottledApp(t, withAuthBudget(1, 60))
	_, adminToken := app.createUser(models.RoleAdmin)

	assert.Equal(t, http.StatusUnauthorized, app.login("nobody@example.com", "x").Code)
	path := "/api/v1/certificates/" + uuid.NewString() + "/export"
	assertGeneric429(t, app.do(http.MethodPost, path, adminToken, `{"format":"pem","include_key":true,"password":"x"}`, nil))
	assert.NotEqual(t, http.StatusTooManyRequests,
		app.do(http.MethodPost, path, adminToken, `{"format":"pem","include_key":false}`, nil).Code)
}

func TestRegister_ExemptAuthRoutesNeverThrottled(t *testing.T) {
	app := newThrottledApp(t, withAuthBudget(1, 1))
	_, token := app.createUser(models.RoleUser)
	app.login("nobody@example.com", "x")
	assertGeneric429(t, app.login("nobody@example.com", "x"))
	app.do(http.MethodPost, "/api/v1/auth/refresh", token, "", nil)
	assertGeneric429(t, app.do(http.MethodPost, "/api/v1/auth/refresh", token, "", nil))

	for _, r := range [][2]string{
		{http.MethodGet, "/api/v1/auth/me"},
		{http.MethodGet, "/api/v1/auth/verify"},
		{http.MethodGet, "/api/v1/auth/accessible-hosts"},
		{http.MethodGet, "/api/v1/auth/check-host/1"},
	} {
		for i := 0; i < 15; i++ {
			assert.NotEqual(t, http.StatusTooManyRequests, app.do(r[0], r[1], token, "", nil).Code, r[1])
		}
	}
	assert.Equal(t, http.StatusOK, app.do(http.MethodGet, "/api/v1/auth/me", token, "", nil).Code)
	assert.Equal(t, http.StatusOK, app.do(http.MethodPost, "/api/v1/auth/logout", token, "", nil).Code)
}

func TestRegister_AuthThrottleIndependentOfCerberusToggle(t *testing.T) {
	for name, sec := range map[string]config.SecurityConfig{
		"cerberus limiter disabled": {RateLimitMode: "disabled"},
		"cerberus limiter enabled":  {RateLimitMode: "enabled", RateLimitRequests: 1000, RateLimitWindowSec: 60, RateLimitBurst: 1000},
	} {
		t.Run(name, func(t *testing.T) {
			sec.AuthRateLimit = config.AuthRateLimitConfig{LoginRequests: 2}
			app := newThrottledApp(t, config.Config{Security: sec})
			app.login("nobody@example.com", "x")
			app.login("nobody@example.com", "x")
			assertGeneric429(t, app.login("nobody@example.com", "x"))
		})
	}
}

func TestRegister_AuthThrottleDisabledByConfig(t *testing.T) {
	cfg := withAuthBudget(1, 1)
	cfg.Security.AuthRateLimit.Disabled = true
	app := newThrottledApp(t, cfg)
	for i := 0; i < 15; i++ {
		assert.Equal(t, http.StatusUnauthorized, app.login("nobody@example.com", "x").Code)
		assert.NotEqual(t, http.StatusTooManyRequests, app.do(http.MethodGet, "/api/v1/auth/status", "", "", nil).Code)
	}
}

func TestRegister_EmergencyEndpointsNeverThrottled(t *testing.T) {
	app := newThrottledApp(t, withAuthBudget(1, 1))
	app.login("nobody@example.com", "x")
	assertGeneric429(t, app.login("nobody@example.com", "x"))
	for i := 0; i < 15; i++ {
		assert.NotEqual(t, http.StatusTooManyRequests, app.do(http.MethodPost, "/api/v1/emergency/security-reset", "", "", nil).Code)
		assert.NotEqual(t, http.StatusTooManyRequests, app.do(http.MethodGet, "/api/v1/emergency/token/status", "", "", nil).Code)
	}
}

func TestRegister_EmergencyBypassSkipsAuthThrottle(t *testing.T) {
	const token = "test-token-that-meets-minimum-length-requirement-32-chars"
	t.Setenv("CHARON_EMERGENCY_TOKEN", token)
	app := newThrottledApp(t, withAuthBudget(1, 1))
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"nobody@example.com","password":"x"}`))
		req.RemoteAddr = "127.0.0.1:4000"
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Emergency-Token", token)
		w := httptest.NewRecorder()
		app.router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	}
}

func TestRegister_LoginProtectionEndpointAdminOnly(t *testing.T) {
	app := newThrottledApp(t, config.Config{})
	_, userToken := app.createUser(models.RoleUser)
	_, adminToken := app.createUser(models.RoleAdmin)

	assert.Equal(t, http.StatusUnauthorized, app.do(http.MethodGet, "/api/v1/security/login-protection", "", "", nil).Code)
	assert.Equal(t, http.StatusForbidden, app.do(http.MethodGet, "/api/v1/security/login-protection", userToken, "", nil).Code)

	w := app.do(http.MethodGet, "/api/v1/security/login-protection", adminToken, "", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, true, body["enabled"])
	assert.Equal(t, "198.51.100.7", body["caller_client_key"])
	assert.Contains(t, body, "untrusted_forwarded_headers")
}

func TestRegister_LoginProtectionEchoesTrustedForwardedFor(t *testing.T) {
	cfg := config.Config{JWTSecret: "test-secret", Security: config.SecurityConfig{TrustedProxies: []string{"198.51.100.0/24"}}}
	app := newThrottledApp(t, cfg)
	require.NoError(t, app.router.SetTrustedProxies(cfg.Security.TrustedProxies))
	_, adminToken := app.createUser(models.RoleAdmin)

	w := app.do(http.MethodGet, "/api/v1/security/login-protection", adminToken, "", map[string]string{"X-Forwarded-For": "198.18.7.7"})
	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "198.18.7.7", body["caller_client_key"])
	assert.Equal(t, float64(1), body["trusted_proxy_count"])
}

// assertRetryAfterNear checks Retry-After against the nominal wait, allowing
// for the token refill that happens in real time between the start of the
// burst and the rejection. The allowance is the measured elapsed time (rounded
// up, plus one second for header rounding), so a stalled runner widens the
// window in exactly the direction the clock moved, while a header off by more
// than the measured elapsed time still fails. The header never exceeds nominal.
func assertRetryAfterNear(t *testing.T, w *httptest.ResponseRecorder, nominal int, start time.Time) {
	t.Helper()
	elapsed := int(math.Ceil(time.Since(start).Seconds()))
	got, err := strconv.Atoi(w.Header().Get("Retry-After"))
	require.NoError(t, err)
	assert.LessOrEqual(t, got, nominal)
	assert.GreaterOrEqual(t, got, nominal-elapsed-1)
}
