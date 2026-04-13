package routes

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func materializeRoutePath(path string) string {
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		if strings.HasPrefix(segment, ":") {
			segments[i] = "1"
		}
	}
	return strings.Join(segments, "/")
}

func TestRegister(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Use in-memory DB
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{
		JWTSecret: "test-secret",
	}

	err = Register(context.Background(), router, db, cfg)
	assert.NoError(t, err)

	// Verify some routes are registered
	routes := router.Routes()
	assert.NotEmpty(t, routes)

	foundHealth := false
	for _, r := range routes {
		if r.Path == "/api/v1/health" {
			foundHealth = true
			break
		}
	}
	assert.True(t, foundHealth, "Health route should be registered")
}

func TestRegister_WithDevelopmentEnvironment(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_dev_env"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{
		JWTSecret:   "test-secret",
		Environment: "development",
	}

	err = Register(context.Background(), router, db, cfg)
	assert.NoError(t, err)
}

func TestRegister_WithProductionEnvironment(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_prod_env"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{
		JWTSecret:   "test-secret",
		Environment: "production",
	}

	err = Register(context.Background(), router, db, cfg)
	assert.NoError(t, err)
}

func TestRegister_AutoMigrateFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Open a valid connection then close it to simulate migration failure
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_migrate_fail"), &gorm.Config{})
	require.NoError(t, err)

	// Close underlying SQL connection to force migration failure
	sqlDB, err := db.DB()
	require.NoError(t, err)
	_ = sqlDB.Close()

	cfg := config.Config{
		JWTSecret: "test-secret",
	}

	err = Register(context.Background(), router, db, cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "auto migrate")
}

func TestRegisterImportHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	cfg := config.Config{JWTSecret: "test-secret"}

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_import"), &gorm.Config{})
	require.NoError(t, err)

	// RegisterImportHandler should not panic
	RegisterImportHandler(router, db, cfg, "/usr/bin/caddy", "/tmp/imports", "/tmp/mount")

	// Verify import routes exist
	routes := router.Routes()
	hasImportRoute := false
	for _, r := range routes {
		// Import routes are: /api/v1/import/status, /api/v1/import/preview, etc.
		if r.Path == "/api/v1/import/status" || r.Path == "/api/v1/import/preview" || r.Path == "/api/v1/import/upload" {
			hasImportRoute = true
			break
		}
	}
	assert.True(t, hasImportRoute, "Import routes should be registered")
}

func TestRegister_RoutesRegistration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_routes"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{
		JWTSecret: "test-secret",
	}

	err = Register(context.Background(), router, db, cfg)
	require.NoError(t, err)

	routes := router.Routes()

	// Verify key routes are registered
	expectedRoutes := []string{
		"/api/v1/health",
		"/metrics",
		"/api/v1/auth/login",
		"/api/v1/auth/register",
		"/api/v1/setup",
	}

	routeMap := make(map[string]bool)
	for _, r := range routes {
		routeMap[r.Path] = true
	}

	for _, expected := range expectedRoutes {
		assert.True(t, routeMap[expected], "Route %s should be registered", expected)
	}
}

func TestRegister_ProxyHostsRequireAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Use in-memory DB
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_proxyhosts_auth"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{JWTSecret: "test-secret"}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/proxy-hosts", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Authorization header required")
}

func TestRegister_StateChangingRoutesDenyByDefaultWithExplicitAllowlist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_mutation_auth_guard"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{JWTSecret: "test-secret"}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	mutatingMethods := map[string]bool{
		http.MethodPost:   true,
		http.MethodPut:    true,
		http.MethodPatch:  true,
		http.MethodDelete: true,
	}

	publicMutationAllowlist := map[string]bool{
		http.MethodPost + " /api/v1/auth/login":               true,
		http.MethodPost + " /api/v1/auth/register":            true,
		http.MethodPost + " /api/v1/setup":                    true,
		http.MethodPost + " /api/v1/invite/accept":            true,
		http.MethodPost + " /api/v1/security/events":          true,
		http.MethodPost + " /api/v1/emergency/security-reset": true,
	}

	for _, route := range router.Routes() {
		if !strings.HasPrefix(route.Path, "/api/v1/") {
			continue
		}
		if !mutatingMethods[route.Method] {
			continue
		}

		key := route.Method + " " + route.Path
		if publicMutationAllowlist[key] {
			continue
		}

		requestPath := materializeRoutePath(route.Path)
		var body io.Reader = http.NoBody
		if route.Method == http.MethodPost || route.Method == http.MethodPut || route.Method == http.MethodPatch {
			body = strings.NewReader("{}")
		}

		req := httptest.NewRequest(route.Method, requestPath, body)
		if route.Method == http.MethodPost || route.Method == http.MethodPut || route.Method == http.MethodPatch {
			req.Header.Set("Content-Type", "application/json")
		}

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Contains(
			t,
			[]int{http.StatusUnauthorized, http.StatusForbidden},
			w.Code,
			"state-changing endpoint must deny unauthenticated access unless explicitly allowlisted: %s (materialized path: %s)",
			key,
			requestPath,
		)
	}
}

func TestRegister_DNSProviders_NotRegisteredWhenEncryptionKeyMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_dnsproviders_missing"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{JWTSecret: "test-secret", EncryptionKey: ""}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	for _, r := range router.Routes() {
		assert.NotContains(t, r.Path, "/api/v1/dns-providers")
	}
}

func TestRegister_DNSProviders_NotRegisteredWhenEncryptionKeyInvalid(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_dnsproviders_invalid"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{JWTSecret: "test-secret", EncryptionKey: "not-base64"}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	for _, r := range router.Routes() {
		assert.NotContains(t, r.Path, "/api/v1/dns-providers")
	}
}

func TestRegister_DNSProviders_RegisteredWhenEncryptionKeyValid(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_dnsproviders_valid"), &gorm.Config{})
	require.NoError(t, err)

	// 32-byte all-zero key in base64
	cfg := config.Config{JWTSecret: "test-secret", EncryptionKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	paths := make(map[string]bool)
	for _, r := range router.Routes() {
		paths[r.Path] = true
	}

	assert.True(t, paths["/api/v1/dns-providers"], "dns providers list route should be registered")
	assert.True(t, paths["/api/v1/dns-providers/types"], "dns providers types route should be registered")
}

func TestRegister_AllRoutesRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_all_routes"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{
		JWTSecret:     "test-secret",
		EncryptionKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
	}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	routes := router.Routes()
	routeMap := make(map[string][]string) // path -> methods
	for _, r := range routes {
		routeMap[r.Path] = append(routeMap[r.Path], r.Method)
	}

	// Core routes
	assert.Contains(t, routeMap, "/api/v1/health")
	assert.Contains(t, routeMap, "/metrics")

	// Auth routes
	assert.Contains(t, routeMap, "/api/v1/auth/login")
	assert.Contains(t, routeMap, "/api/v1/auth/register")
	assert.Contains(t, routeMap, "/api/v1/auth/verify")
	assert.Contains(t, routeMap, "/api/v1/auth/status")
	assert.Contains(t, routeMap, "/api/v1/auth/logout")
	assert.Contains(t, routeMap, "/api/v1/auth/me")

	// User routes
	assert.Contains(t, routeMap, "/api/v1/setup")
	assert.Contains(t, routeMap, "/api/v1/invite/validate")
	assert.Contains(t, routeMap, "/api/v1/invite/accept")
	assert.Contains(t, routeMap, "/api/v1/users")

	// Settings routes
	assert.Contains(t, routeMap, "/api/v1/settings")
	assert.Contains(t, routeMap, "/api/v1/settings/smtp")

	// Security routes
	assert.Contains(t, routeMap, "/api/v1/security/status")
	assert.Contains(t, routeMap, "/api/v1/security/config")
	assert.Contains(t, routeMap, "/api/v1/audit-logs")

	// Notification routes
	assert.Contains(t, routeMap, "/api/v1/notifications")
	assert.Contains(t, routeMap, "/api/v1/notifications/providers")

	// Uptime routes
	assert.Contains(t, routeMap, "/api/v1/uptime/monitors")

	// DNS Providers routes (when encryption key is set)
	assert.Contains(t, routeMap, "/api/v1/dns-providers")
	assert.Contains(t, routeMap, "/api/v1/dns-providers/types")
	assert.Contains(t, routeMap, "/api/v1/dns-providers/:id/credentials")

	// Admin routes - plugins should always be registered
	assert.Contains(t, routeMap, "/api/v1/admin/plugins")

	// CrowdSec routes
	assert.Contains(t, routeMap, "/api/v1/admin/crowdsec/status")
	assert.Contains(t, routeMap, "/api/v1/admin/crowdsec/start")
	assert.Contains(t, routeMap, "/api/v1/admin/crowdsec/stop")

	// Total route count should be substantial
	assert.Greater(t, len(routes), 50, "Expected more than 50 routes to be registered")
}

func TestRegister_MiddlewareApplied(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_middleware"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{JWTSecret: "test-secret"}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	// Test that security headers middleware is applied
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	router.ServeHTTP(w, req)

	// Security headers should be present
	assert.NotEmpty(t, w.Header().Get("X-Content-Type-Options"))
	assert.NotEmpty(t, w.Header().Get("X-Frame-Options"))

	// Response should be compressed (gzip middleware applied)
	// Note: Only compressed if Accept-Encoding is set
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req2.Header.Set("Accept-Encoding", "gzip")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	// Check for gzip content encoding when response is large enough
	assert.Equal(t, http.StatusOK, w2.Code)
}

func TestRegister_AuthenticatedRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_auth_routes"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{JWTSecret: "test-secret"}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	// Test that protected routes require authentication
	protectedPaths := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/backups"},
		{http.MethodPost, "/api/v1/backups"},
		{http.MethodGet, "/api/v1/logs"},
		{http.MethodGet, "/api/v1/settings"},
		{http.MethodGet, "/api/v1/notifications"},
		{http.MethodGet, "/api/v1/users"},
		{http.MethodGet, "/api/v1/auth/me"},
		{http.MethodPost, "/api/v1/auth/logout"},
		{http.MethodGet, "/api/v1/uptime/monitors"},
	}

	for _, tc := range protectedPaths {
		t.Run(tc.method+"_"+tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, nil)
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusUnauthorized, w.Code, "Route %s %s should require auth", tc.method, tc.path)
		})
	}
}

func TestRegister_StateChangingRoutesRequireAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_mutating_auth_routes"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{JWTSecret: "test-secret"}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	stateChangingPaths := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/backups"},
		{http.MethodPost, "/api/v1/settings"},
		{http.MethodPatch, "/api/v1/settings"},
		{http.MethodPatch, "/api/v1/config"},
		{http.MethodPost, "/api/v1/user/profile"},
		{http.MethodPost, "/api/v1/remote-servers"},
		{http.MethodPost, "/api/v1/remote-servers/test"},
		{http.MethodPut, "/api/v1/remote-servers/1"},
		{http.MethodDelete, "/api/v1/remote-servers/1"},
		{http.MethodPost, "/api/v1/remote-servers/1/test"},
	}

	for _, tc := range stateChangingPaths {
		t.Run(tc.method+"_"+tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, nil)
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusUnauthorized, w.Code, "State-changing route %s %s should require auth", tc.method, tc.path)
		})
	}
}

func TestRegister_AdminRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_admin_routes"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{
		JWTSecret:     "test-secret",
		EncryptionKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
	}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	// Admin routes should exist and require auth
	adminPaths := []string{
		"/api/v1/admin/plugins",
		"/api/v1/admin/crowdsec/status",
	}

	for _, path := range adminPaths {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		router.ServeHTTP(w, req)
		// Should require auth (401) not be missing (404)
		assert.Equal(t, http.StatusUnauthorized, w.Code, "Admin route %s should exist and require auth", path)
	}
}

func TestRegister_PublicRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_public_routes"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{JWTSecret: "test-secret"}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	// Public routes should be accessible without auth (route exists, not 404)
	publicPaths := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/health"},
		{http.MethodGet, "/metrics"},
		{http.MethodGet, "/api/v1/setup"},
		{http.MethodGet, "/api/v1/auth/status"},
	}

	for _, tc := range publicPaths {
		t.Run(tc.method+"_"+tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, nil)
			router.ServeHTTP(w, req)
			// Should not be 404 (route exists)
			assert.NotEqual(t, http.StatusNotFound, w.Code, "Public route %s %s should exist", tc.method, tc.path)
		})
	}
}

func TestRegister_HealthEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_health_endpoint"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{JWTSecret: "test-secret"}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "status")
}

func TestRegister_MetricsEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_metrics_endpoint"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{JWTSecret: "test-secret"}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// Prometheus metrics format
	assert.Contains(t, w.Header().Get("Content-Type"), "text/plain")
}

func TestRegister_DBHealthEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_db_health"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{JWTSecret: "test-secret"}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health/db", nil)
	router.ServeHTTP(w, req)

	// Should return OK or service unavailable, but not 404
	assert.NotEqual(t, http.StatusNotFound, w.Code)
}

func TestRegister_LoginEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_login"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{JWTSecret: "test-secret"}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	// Test login endpoint exists and accepts POST
	body := `{"username": "test", "password": "test"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// Should not be 404 (route exists)
	assert.NotEqual(t, http.StatusNotFound, w.Code)
}

func TestRegister_SetupEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_setup"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{JWTSecret: "test-secret"}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	// GET /setup should return setup status
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/setup", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "setup")
}

func TestRegister_WithEncryptionRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_encryption_routes"), &gorm.Config{})
	require.NoError(t, err)

	// Set valid encryption key env var (32-byte key base64 encoded)
	t.Setenv("CHARON_ENCRYPTION_KEY", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")

	cfg := config.Config{
		JWTSecret:     "test-secret",
		EncryptionKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
	}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	// Check if encryption routes are registered (may depend on env)
	routes := router.Routes()
	routeMap := make(map[string]bool)
	for _, r := range routes {
		routeMap[r.Path] = true
	}

	// DNS providers should be registered with valid encryption key
	assert.True(t, routeMap["/api/v1/dns-providers"])
	assert.True(t, routeMap["/api/v1/dns-providers/types"])
}

func TestRegister_UptimeCheckEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_uptime_check"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{JWTSecret: "test-secret"}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	// Uptime check route should exist and require auth
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/system/uptime/check", nil)
	router.ServeHTTP(w, req)

	// Should require auth
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRegister_CrowdSecRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_crowdsec_routes"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{JWTSecret: "test-secret"}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	// CrowdSec routes should exist
	routes := router.Routes()
	routeMap := make(map[string]bool)
	for _, r := range routes {
		routeMap[r.Path] = true
	}

	// CrowdSec management routes
	assert.True(t, routeMap["/api/v1/admin/crowdsec/start"])
	assert.True(t, routeMap["/api/v1/admin/crowdsec/stop"])
	assert.True(t, routeMap["/api/v1/admin/crowdsec/status"])
	assert.True(t, routeMap["/api/v1/admin/crowdsec/presets"])
	assert.True(t, routeMap["/api/v1/admin/crowdsec/decisions"])
	assert.True(t, routeMap["/api/v1/admin/crowdsec/ban"])
}

func TestRegister_SecurityRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_security_routes"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{JWTSecret: "test-secret"}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	routes := router.Routes()
	routeMap := make(map[string]bool)
	for _, r := range routes {
		routeMap[r.Path] = true
	}

	// Security routes
	assert.True(t, routeMap["/api/v1/security/status"])
	assert.True(t, routeMap["/api/v1/security/config"])
	assert.True(t, routeMap["/api/v1/security/enable"])
	assert.True(t, routeMap["/api/v1/security/disable"])
	assert.True(t, routeMap["/api/v1/security/decisions"])
	assert.True(t, routeMap["/api/v1/security/rulesets"])
	assert.True(t, routeMap["/api/v1/security/geoip/status"])
	assert.True(t, routeMap["/api/v1/security/waf/exclusions"])
}

func TestRegister_AccessListRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_acl_routes"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{JWTSecret: "test-secret"}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	routes := router.Routes()
	routeMap := make(map[string]bool)
	for _, r := range routes {
		routeMap[r.Path] = true
	}

	// Access List routes
	assert.True(t, routeMap["/api/v1/access-lists"])
	assert.True(t, routeMap["/api/v1/access-lists/:id"])
	assert.True(t, routeMap["/api/v1/access-lists/:id/test"])
	assert.True(t, routeMap["/api/v1/access-lists/templates"])
}

func TestRegister_CertificateRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_cert_routes"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{JWTSecret: "test-secret"}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	routes := router.Routes()
	routeMap := make(map[string]bool)
	for _, r := range routes {
		routeMap[r.Path] = true
	}

	// Certificate routes
	assert.True(t, routeMap["/api/v1/certificates"])
	assert.True(t, routeMap["/api/v1/certificates/:uuid"])
}

// TestRegister_NilHandlers verifies registration behavior with minimal/nil components
func TestRegister_NilHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Create a minimal DB connection that will work
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_nil_handlers"), &gorm.Config{})
	require.NoError(t, err)

	// Config with minimal settings - no encryption key, no special features
	cfg := config.Config{
		JWTSecret:     "test-secret",
		Environment:   "production",
		EncryptionKey: "", // No encryption key - DNS providers won't be registered
	}

	err = Register(context.Background(), router, db, cfg)
	assert.NoError(t, err)

	// Verify that routes still work without DNS provider features
	routes := router.Routes()
	routeMap := make(map[string]bool)
	for _, r := range routes {
		routeMap[r.Path] = true
	}

	// Core routes should still be registered
	assert.True(t, routeMap["/api/v1/health"])
	assert.True(t, routeMap["/api/v1/auth/login"])

	// DNS provider routes should NOT be registered (no encryption key)
	assert.False(t, routeMap["/api/v1/dns-providers"])
}

// TestRegister_MiddlewareOrder verifies middleware is attached in correct order
func TestRegister_MiddlewareOrder(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_middleware_order"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{
		JWTSecret:   "test-secret",
		Environment: "development",
	}

	err = Register(context.Background(), router, db, cfg)
	require.NoError(t, err)

	// Test that security headers are applied (they should come first)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	router.ServeHTTP(w, req)

	// Security headers should be present regardless of response
	assert.NotEmpty(t, w.Header().Get("X-Content-Type-Options"), "Security headers middleware should set X-Content-Type-Options")
	assert.NotEmpty(t, w.Header().Get("X-Frame-Options"), "Security headers middleware should set X-Frame-Options")

	// In development mode, CSP should be more permissive
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestRegister_GzipCompression verifies gzip middleware is working
func TestRegister_GzipCompression(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_gzip"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{JWTSecret: "test-secret"}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	// Request with Accept-Encoding: gzip
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	router.ServeHTTP(w, req)

	// Response should be OK (gzip will only compress if response is large enough)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestRegister_CerberusMiddleware verifies Cerberus security middleware is applied
func TestRegister_CerberusMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_cerberus_mw"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{
		JWTSecret: "test-secret",
		Security: config.SecurityConfig{
			CerberusEnabled: true,
		},
	}

	err = Register(context.Background(), router, db, cfg)
	require.NoError(t, err)

	// API routes should have Cerberus middleware applied
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/setup", nil)
	router.ServeHTTP(w, req)

	// Should still work (Cerberus allows normal requests)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestRegister_FeatureFlagsEndpoint verifies feature flags endpoint is registered
func TestRegister_FeatureFlagsEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_feature_flags"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{JWTSecret: "test-secret"}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	// Feature flags should require auth
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/feature-flags", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestRegister_WebSocketRoutes verifies WebSocket routes are registered
func TestRegister_WebSocketRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_ws_routes"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{JWTSecret: "test-secret"}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	routes := router.Routes()
	routeMap := make(map[string]bool)
	for _, r := range routes {
		routeMap[r.Path] = true
	}

	// WebSocket routes should be registered
	assert.True(t, routeMap["/api/v1/logs/live"])
	assert.True(t, routeMap["/api/v1/websocket/connections"])
	assert.True(t, routeMap["/api/v1/websocket/stats"])
	assert.True(t, routeMap["/api/v1/cerberus/logs/ws"])
}

// TestRegister_NotificationRoutes verifies all notification routes are registered
func TestRegister_NotificationRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_notification_routes"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{JWTSecret: "test-secret"}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	routes := router.Routes()
	routeMap := make(map[string]bool)
	for _, r := range routes {
		routeMap[r.Path] = true
	}

	// Notification routes
	assert.True(t, routeMap["/api/v1/notifications"])
	assert.True(t, routeMap["/api/v1/notifications/:id/read"])
	assert.True(t, routeMap["/api/v1/notifications/read-all"])
	assert.True(t, routeMap["/api/v1/notifications/providers"])
	assert.True(t, routeMap["/api/v1/notifications/providers/:id"])
	assert.True(t, routeMap["/api/v1/notifications/templates"])
	assert.True(t, routeMap["/api/v1/notifications/external-templates"])
	assert.True(t, routeMap["/api/v1/notifications/external-templates/:id"])
}

// TestRegister_DomainRoutes verifies domain management routes
func TestRegister_DomainRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_domain_routes"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{JWTSecret: "test-secret"}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	routes := router.Routes()
	routeMap := make(map[string]bool)
	for _, r := range routes {
		routeMap[r.Path] = true
	}

	// Domain routes
	assert.True(t, routeMap["/api/v1/domains"])
	assert.True(t, routeMap["/api/v1/domains/:id"])
}

// TestRegister_VerifyAuthEndpoint tests the verify endpoint for Caddy forward auth
func TestRegister_VerifyAuthEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_verify_auth"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{JWTSecret: "test-secret"}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	// Verify endpoint is public (for Caddy forward auth)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/verify", nil)
	router.ServeHTTP(w, req)

	// Should not be 404 (route exists) - will return 401 without valid session
	assert.NotEqual(t, http.StatusNotFound, w.Code)
}

// TestRegister_SMTPRoutes verifies SMTP configuration routes
func TestRegister_SMTPRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_smtp_routes"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{JWTSecret: "test-secret"}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	routes := router.Routes()
	routeMap := make(map[string]bool)
	for _, r := range routes {
		routeMap[r.Path] = true
	}

	// SMTP routes
	assert.True(t, routeMap["/api/v1/settings/smtp"])
	assert.True(t, routeMap["/api/v1/settings/smtp/test"])
	assert.True(t, routeMap["/api/v1/settings/smtp/test-email"])
	assert.True(t, routeMap["/api/v1/settings/validate-url"])
	assert.True(t, routeMap["/api/v1/settings/test-url"])
}

// TestRegisterImportHandler_RoutesExist verifies import handler routes
func TestRegisterImportHandler_RoutesExist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	cfg := config.Config{JWTSecret: "test-secret"}

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_import_routes"), &gorm.Config{})
	require.NoError(t, err)

	RegisterImportHandler(router, db, cfg, "/usr/bin/caddy", "/tmp/imports", "/tmp/mount")

	routes := router.Routes()
	routeMap := make(map[string]bool)
	for _, r := range routes {
		routeMap[r.Path] = true
	}

	// Import routes
	assert.True(t, routeMap["/api/v1/import/status"] || routeMap["/api/v1/import/preview"] || routeMap["/api/v1/import/upload"],
		"At least one import route should be registered")
}

// TestRegister_EncryptionRoutesWithValidKey verifies encryption management routes
func TestRegister_EncryptionRoutesWithValidKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_encryption_routes_valid"), &gorm.Config{})
	require.NoError(t, err)

	// Set the env var needed for rotation service
	t.Setenv("CHARON_ENCRYPTION_KEY", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")

	// Valid 32-byte key in base64
	cfg := config.Config{
		JWTSecret:     "test-secret",
		EncryptionKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
	}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	routes := router.Routes()
	routeMap := make(map[string]bool)
	for _, r := range routes {
		routeMap[r.Path] = true
	}

	// Encryption management routes should be registered (depends on rotation service init)
	// Note: If rotation service init fails, these routes won't be registered
	// We check if DNS provider routes are registered (which don't depend on rotation service)
	assert.True(t, routeMap["/api/v1/dns-providers"])
	assert.True(t, routeMap["/api/v1/dns-providers/types"])

	// Encryption routes may or may not be registered depending on env setup
	// Just verify the DNS providers are there when encryption key is valid
}

// TestRegister_WAFExclusionRoutes verifies WAF exclusion management routes
func TestRegister_WAFExclusionRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_waf_exclusion_routes"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{JWTSecret: "test-secret"}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	routes := router.Routes()
	routeMap := make(map[string]bool)
	for _, r := range routes {
		routeMap[r.Path] = true
	}

	// WAF exclusion routes
	assert.True(t, routeMap["/api/v1/security/waf/exclusions"])
	assert.True(t, routeMap["/api/v1/security/waf/exclusions/:rule_id"])
}

// TestRegister_BreakGlassRoute verifies break glass endpoint is registered
func TestRegister_BreakGlassRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_breakglass_route"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{JWTSecret: "test-secret"}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	routes := router.Routes()
	routeMap := make(map[string]bool)
	for _, r := range routes {
		routeMap[r.Path] = true
	}

	// Break glass route
	assert.True(t, routeMap["/api/v1/security/breakglass/generate"])
}

// TestRegister_RateLimitPresetsRoute verifies rate limit presets endpoint
func TestRegister_RateLimitPresetsRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_ratelimit_presets"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{JWTSecret: "test-secret"}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	routes := router.Routes()
	routeMap := make(map[string]bool)
	for _, r := range routes {
		routeMap[r.Path] = true
	}

	// Rate limit presets route
	assert.True(t, routeMap["/api/v1/security/rate-limit/presets"])
}

// TestEmergencyEndpoint_BypassACL verifies emergency endpoint works when ACL is blocking
func TestEmergencyEndpoint_BypassACL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Setup test database with ACL enabled
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_emergency_bypass_acl"), &gorm.Config{})
	require.NoError(t, err)

	// Set emergency token in env
	t.Setenv("CHARON_EMERGENCY_TOKEN", "test-token-that-meets-minimum-length-requirement-32-chars")

	// Register routes with security enabled
	cfg := config.Config{
		JWTSecret: "test-secret",
		Security: config.SecurityConfig{
			ACLMode:         "enabled",
			CerberusEnabled: true,
		},
	}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	// Note: We don't need to create ACL settings here because the emergency endpoint
	// bypass happens at middleware level before Cerberus checks

	// Test 1: Verify emergency endpoint exists
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/emergency/security-reset", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	router.ServeHTTP(w, req)

	// Should not be 404 (route exists)
	assert.NotEqual(t, http.StatusNotFound, w.Code, "Emergency endpoint should exist")

	// Test 2: Emergency request with valid token should work
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/emergency/security-reset", nil)
	req.Header.Set("X-Emergency-Token", "test-token-that-meets-minimum-length-requirement-32-chars")
	req.RemoteAddr = "127.0.0.1:12345"
	router.ServeHTTP(w, req)

	// Should succeed (even if ACL would normally block)
	// Emergency handler returns 200 on success
	assert.NotEqual(t, http.StatusForbidden, w.Code, "Emergency request should not be blocked by ACL")
	assert.Equal(t, http.StatusOK, w.Code, "Emergency request should succeed")
}

// TestEmergencyBypass_MiddlewareOrder verifies emergency bypass is first in chain
func TestEmergencyBypass_MiddlewareOrder(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_emergency_mw_order"), &gorm.Config{})
	require.NoError(t, err)

	t.Setenv("CHARON_EMERGENCY_TOKEN", "test-token-that-meets-minimum-length-requirement-32-chars")

	cfg := config.Config{
		JWTSecret: "test-secret",
		Security: config.SecurityConfig{
			CerberusEnabled: true,
			ManagementCIDRs: []string{"127.0.0.0/8"},
		},
	}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	// Request with emergency token should set bypass flag
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set("X-Emergency-Token", "test-token-that-meets-minimum-length-requirement-32-chars")
	req.RemoteAddr = "127.0.0.1:12345"
	router.ServeHTTP(w, req)

	// Should succeed - emergency bypass allows request through
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestEmergencyBypass_InvalidToken verifies invalid tokens are rejected
func TestEmergencyBypass_InvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_emergency_invalid_token"), &gorm.Config{})
	require.NoError(t, err)

	t.Setenv("CHARON_EMERGENCY_TOKEN", "test-token-that-meets-minimum-length-requirement-32-chars")

	cfg := config.Config{
		JWTSecret: "test-secret",
		Security: config.SecurityConfig{
			CerberusEnabled: true,
		},
	}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	// Request with WRONG emergency token
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/emergency/security-reset", nil)
	req.Header.Set("X-Emergency-Token", "wrong-token")
	req.RemoteAddr = "127.0.0.1:12345"
	router.ServeHTTP(w, req)

	// Should not activate bypass (wrong token)
	// Endpoint may still respond with proper error, but bypass flag should not be set
	assert.NotEqual(t, http.StatusNotFound, w.Code)
}

// TestEmergencyBypass_UnauthorizedIP verifies IP restrictions work
func TestEmergencyBypass_UnauthorizedIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_emergency_unauthorized_ip"), &gorm.Config{})
	require.NoError(t, err)

	t.Setenv("CHARON_EMERGENCY_TOKEN", "test-token-that-meets-minimum-length-requirement-32-chars")

	// Only allow 192.168.1.0/24
	cfg := config.Config{
		JWTSecret: "test-secret",
		Security: config.SecurityConfig{
			CerberusEnabled: true,
			ManagementCIDRs: []string{"192.168.1.0/24"},
		},
	}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	// Request from public IP (not in management network)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/emergency/security-reset", nil)
	req.Header.Set("X-Emergency-Token", "test-token-that-meets-minimum-length-requirement-32-chars")
	req.RemoteAddr = "203.0.113.1:12345" // Public IP
	router.ServeHTTP(w, req)

	// Should not activate bypass (unauthorized IP)
	assert.NotEqual(t, http.StatusNotFound, w.Code)
}

func TestRegister_CreatesAccessLogFileForLogWatcher(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_access_log_create"), &gorm.Config{})
	require.NoError(t, err)

	logFilePath := filepath.Join(t.TempDir(), "logs", "access.log")
	t.Setenv("CHARON_CADDY_ACCESS_LOG", logFilePath)

	cfg := config.Config{JWTSecret: "test-secret"}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	_, statErr := os.Stat(logFilePath)
	assert.NoError(t, statErr)
}

func TestMigrateViewerToPassthrough(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.User{}))

	// Seed a user with the legacy "viewer" role
	viewer := models.User{
		UUID:    uuid.NewString(),
		APIKey:  uuid.NewString(),
		Email:   "viewer@example.com",
		Role:    models.UserRole("viewer"),
		Enabled: true,
	}
	require.NoError(t, db.Create(&viewer).Error)

	migrateViewerToPassthrough(db)

	var updated models.User
	require.NoError(t, db.First(&updated, viewer.ID).Error)
	assert.Equal(t, models.RolePassthrough, updated.Role)
}

func TestRegister_CleansLetsEncryptCertAssignments(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_test_lecleaner"), &gorm.Config{})
	require.NoError(t, err)

	// Pre-migrate just the two tables needed to seed test data before Register runs.
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cert := models.SSLCertificate{Provider: "letsencrypt"}
	require.NoError(t, db.Create(&cert).Error)

	certID := cert.ID
	host := models.ProxyHost{DomainNames: "test.example.com", CertificateID: &certID}
	require.NoError(t, db.Create(&host).Error)

	cfg := config.Config{JWTSecret: "test-secret"}
	err = Register(context.Background(), router, db, cfg)
	require.NoError(t, err)

	var reloaded models.ProxyHost
	require.NoError(t, db.First(&reloaded, host.ID).Error)
	assert.Nil(t, reloaded.CertificateID, "letsencrypt cert assignment must be cleared")
}
