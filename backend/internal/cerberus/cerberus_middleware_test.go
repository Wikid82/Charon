package cerberus_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/cerberus"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupDB(t *testing.T) *gorm.DB {
	dsn := fmt.Sprintf("file:cerberus_middleware_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.AccessList{}, &models.AccessListRule{}, &models.SecurityConfig{}))
	return db
}

// TestMiddleware_WAFEnabledTracksMetrics tests that the cerberus middleware tracks WAF metrics
// when WAF mode is enabled. Note: Actual WAF blocking is handled by Coraza at the Caddy layer,
// not by this middleware. The middleware only provides metrics tracking and ACL enforcement.
func TestMiddleware_WAFEnabledTracksMetrics(t *testing.T) {
	db := setupDB(t)
	cfg := config.SecurityConfig{WAFMode: "block"}
	c := cerberus.New(cfg, db)

	// Setup gin context
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	// Create a request - this middleware no longer blocks on payload content
	// because Coraza handles WAF at the Caddy layer
	req := httptest.NewRequest(http.MethodGet, "/?q=test", http.NoBody)
	req.RequestURI = "/?q=test"
	ctx.Request = req

	// call middleware
	mw := c.Middleware()
	mw(ctx)

	// Middleware should pass through - it only tracks metrics now
	// WAF blocking happens at Caddy/Coraza layer
	require.False(t, ctx.IsAborted(), "cerberus middleware should not block - WAF is handled by Coraza at Caddy layer")
}

func TestMiddleware_ACLBlocksClientIP(t *testing.T) {
	db := setupDB(t)
	cfg := config.SecurityConfig{ACLMode: "enabled"}
	// Create an ACL that blocks 8.8.8.8
	ruleJSON := `[ { "cidr": "8.8.8.8/32", "description": "block" } ]`
	acl := &models.AccessList{Name: "Block8", Type: "blacklist", IPRules: ruleJSON, Enabled: true}
	require.NoError(t, db.Create(acl).Error)

	c := cerberus.New(cfg, db)

	// Setup gin context with remote address 8.8.8.8
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.RemoteAddr = "8.8.8.8:1234"
	ctx.Request = req

	mw := c.Middleware()
	mw(ctx)

	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestMiddleware_ACLAllowsClientIP(t *testing.T) {
	db := setupDB(t)
	cfg := config.SecurityConfig{ACLMode: "enabled"}
	// Create a whitelist that allows 8.8.8.8
	ruleJSON := `[ { "cidr": "8.8.8.8/32", "description": "allow" } ]`
	acl := &models.AccessList{Name: "Allow8", Type: "whitelist", IPRules: ruleJSON, Enabled: true}
	require.NoError(t, db.Create(acl).Error)

	c := cerberus.New(cfg, db)

	// Setup gin context with remote address 8.8.8.8 (allowed)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.RemoteAddr = "8.8.8.8:1234"
	ctx.Request = req

	mw := c.Middleware()
	mw(ctx)
	// Should not block - middleware did not abort
	require.False(t, ctx.IsAborted())
}

func TestMiddleware_ACLDefaultDenyWhenNoLists(t *testing.T) {
	db := setupDB(t)
	cfg := config.SecurityConfig{ACLMode: "enabled"}

	c := cerberus.New(cfg, db)

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.RemoteAddr = "203.0.113.5:1234"
	ctx.Request = req

	mw := c.Middleware()
	mw(ctx)

	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestMiddleware_ACLAdminWhitelistBypass(t *testing.T) {
	db := setupDB(t)
	cfg := config.SecurityConfig{ACLMode: "enabled"}

	whitelist := "203.0.113.5/32"
	require.NoError(t, db.Create(&models.SecurityConfig{Name: "default", Enabled: true, AdminWhitelist: whitelist}).Error)

	c := cerberus.New(cfg, db)

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Set("role", "admin")
	ctx.Set("userID", uint(1))
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.RemoteAddr = "203.0.113.5:1234"
	ctx.Request = req

	mw := c.Middleware()
	mw(ctx)

	require.False(t, ctx.IsAborted())
}

func TestMiddleware_ACLAdminWhitelistBypass_RequiresAuthenticatedAdmin(t *testing.T) {
	db := setupDB(t)
	cfg := config.SecurityConfig{ACLMode: "enabled"}

	whitelist := "203.0.113.5/32"
	require.NoError(t, db.Create(&models.SecurityConfig{Name: "default", Enabled: true, AdminWhitelist: whitelist}).Error)

	c := cerberus.New(cfg, db)

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.RemoteAddr = "203.0.113.5:1234"
	ctx.Request = req

	mw := c.Middleware()
	mw(ctx)

	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestMiddleware_NotEnabledSkips(t *testing.T) {
	db := setupDB(t)
	// All modes disabled by default
	cfg := config.SecurityConfig{}
	c := cerberus.New(cfg, db)

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.RemoteAddr = "1.2.3.4:1234"
	ctx.Request = req

	mw := c.Middleware()
	mw(ctx)
	require.False(t, ctx.IsAborted())
}

func TestMiddleware_WAFPassesWithNoPayload(t *testing.T) {
	db := setupDB(t)
	cfg := config.SecurityConfig{WAFMode: "block"}
	c := cerberus.New(cfg, db)

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/?q=safe", http.NoBody)
	req.RequestURI = "/?q=safe"
	ctx.Request = req

	mw := c.Middleware()
	mw(ctx)
	require.False(t, ctx.IsAborted())
}

func TestMiddleware_WAFMonitorLogsButDoesNotBlock(t *testing.T) {
	db := setupDB(t)
	cfg := config.SecurityConfig{WAFMode: "monitor"}
	c := cerberus.New(cfg, db)

	// Test 1: suspicious payload in monitor mode should NOT block
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/?q=<script>", http.NoBody)
	req.RequestURI = "/?q=<script>"
	ctx.Request = req

	mw := c.Middleware()
	mw(ctx)
	require.False(t, ctx.IsAborted(), "monitor mode should not block suspicious payload")

	// Test 2: safe query in monitor mode should also pass
	w2 := httptest.NewRecorder()
	ctx2, _ := gin.CreateTestContext(w2)
	req2 := httptest.NewRequest(http.MethodGet, "/?q=safe", http.NoBody)
	req2.RequestURI = "/?q=safe"
	ctx2.Request = req2

	mw2 := c.Middleware()
	mw2(ctx2)
	require.False(t, ctx2.IsAborted(), "monitor mode should not block safe payload")
}

func TestMiddleware_ACLDisabledDoesNotBlock(t *testing.T) {
	db := setupDB(t)
	cfg := config.SecurityConfig{ACLMode: "enabled"}
	// Create a disabled ACL that would block 8.8.8.8 (but it's disabled)
	ruleJSON := `[ { "cidr": "8.8.8.8/32", "description": "block" } ]`
	acl := &models.AccessList{Name: "Block8_Disabled", Type: "blacklist", IPRules: ruleJSON, Enabled: false}
	require.NoError(t, db.Create(acl).Error)

	c := cerberus.New(cfg, db)

	// Setup gin context with remote address 8.8.8.8
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Set("role", "admin")
	ctx.Set("userID", uint(1))
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.RemoteAddr = "8.8.8.8:1234"
	ctx.Request = req

	mw := c.Middleware()
	mw(ctx)
	// Disabled ACL should not block
	require.False(t, ctx.IsAborted())
}
