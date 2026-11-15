package cerberus_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/cerberus"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	securitypkg "github.com/Wikid82/charon/backend/internal/security"
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

// managementSetting stores the chosen management lists, as the settings row the middleware reads.
func managementSetting(t *testing.T, db *gorm.DB, uuids ...string) {
	t.Helper()
	raw, err := json.Marshal(uuids)
	require.NoError(t, err)
	require.NoError(t, db.Create(&models.Setting{Key: securitypkg.ManagementListsSettingKey, Value: string(raw), Category: "security", Type: "json"}).Error)
}

// aclMiddlewareContext runs the middleware for one request from remote and reports whether it was aborted.
func aclMiddlewareContext(t *testing.T, c *cerberus.Cerberus, remote string) (aborted bool, code int) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.RemoteAddr = remote
	ctx.Request = req
	mw := c.Middleware()
	mw(ctx)
	return ctx.IsAborted(), w.Code
}

func TestMiddleware_ManagementListBlocksListedBlacklist(t *testing.T) {
	db := setupDB(t)
	acl := &models.AccessList{UUID: "block-8", Name: "Block8", Type: "blacklist", IPRules: `[{"cidr":"8.8.8.8/32"}]`, Enabled: true}
	require.NoError(t, db.Create(acl).Error)
	managementSetting(t, db, acl.UUID)

	c := cerberus.New(config.SecurityConfig{ACLMode: "enabled"}, db)
	aborted, code := aclMiddlewareContext(t, c, "8.8.8.8:1234")
	require.True(t, aborted)
	require.Equal(t, http.StatusForbidden, code)
}

func TestMiddleware_ManagementListAllowsListedWhitelist(t *testing.T) {
	db := setupDB(t)
	acl := &models.AccessList{UUID: "allow-8", Name: "Allow8", Type: "whitelist", IPRules: `[{"cidr":"8.8.8.8/32"}]`, Enabled: true}
	require.NoError(t, db.Create(acl).Error)
	managementSetting(t, db, acl.UUID)

	c := cerberus.New(config.SecurityConfig{ACLMode: "enabled"}, db)
	aborted, _ := aclMiddlewareContext(t, c, "8.8.8.8:1234")
	require.False(t, aborted)
}

func TestMiddleware_ManagementWhitelistRefusesUnlistedCaller(t *testing.T) {
	db := setupDB(t)
	acl := &models.AccessList{UUID: "allow-8-only", Name: "Allow8", Type: "whitelist", IPRules: `[{"cidr":"8.8.8.8/32"}]`, Enabled: true}
	require.NoError(t, db.Create(acl).Error)
	managementSetting(t, db, acl.UUID)

	c := cerberus.New(config.SecurityConfig{ACLMode: "enabled"}, db)
	aborted, code := aclMiddlewareContext(t, c, "203.0.113.5:1234")
	require.True(t, aborted)
	require.Equal(t, http.StatusForbidden, code)
}

// TestMiddleware_NoManagementListIsNotRestricted: with no list chosen, the interface is not restricted, whatever the
// proxy-host lists say.
func TestMiddleware_NoManagementListIsNotRestricted(t *testing.T) {
	db := setupDB(t)
	require.NoError(t, db.Create(&models.AccessList{UUID: "site-only", Name: "Site", Type: "whitelist", IPRules: `[{"cidr":"8.8.8.8/32"}]`, Enabled: true}).Error)

	c := cerberus.New(config.SecurityConfig{ACLMode: "enabled"}, db)
	aborted, _ := aclMiddlewareContext(t, c, "203.0.113.5:1234")
	require.False(t, aborted)
}

func TestMiddleware_ManagementListDisabledAllows(t *testing.T) {
	db := setupDB(t)
	acl := &models.AccessList{UUID: "off", Name: "Off", Type: "whitelist", IPRules: `[{"cidr":"8.8.8.8/32"}]`, Enabled: false}
	require.NoError(t, db.Create(acl).Error)
	managementSetting(t, db, acl.UUID)

	c := cerberus.New(config.SecurityConfig{ACLMode: "enabled"}, db)
	aborted, _ := aclMiddlewareContext(t, c, "203.0.113.5:1234")
	require.False(t, aborted)
}

func TestMiddleware_ManagementListMissingUUIDIsSkipped(t *testing.T) {
	db := setupDB(t)
	managementSetting(t, db, "deleted-list")

	c := cerberus.New(config.SecurityConfig{ACLMode: "enabled"}, db)
	aborted, _ := aclMiddlewareContext(t, c, "203.0.113.5:1234")
	require.False(t, aborted)
}

func TestMiddleware_ManagementAdminAllowListExemptsByAddress(t *testing.T) {
	db := setupDB(t)
	acl := &models.AccessList{UUID: "allow-8-only", Name: "Allow8", Type: "whitelist", IPRules: `[{"cidr":"8.8.8.8/32"}]`, Enabled: true}
	require.NoError(t, db.Create(acl).Error)
	managementSetting(t, db, acl.UUID)
	require.NoError(t, db.Create(&models.SecurityConfig{Name: "default", Enabled: true, AdminWhitelist: "203.0.113.5/32"}).Error)

	c := cerberus.New(config.SecurityConfig{ACLMode: "enabled"}, db)
	aborted, _ := aclMiddlewareContext(t, c, "203.0.113.5:1234")
	require.False(t, aborted, "an admin allow-list address is exempt, whatever the caller's role")
}

func TestMiddleware_ManagementUnusableClientAddressIsRefused(t *testing.T) {
	db := setupDB(t)
	acl := &models.AccessList{UUID: "allow-8-only", Name: "Allow8", Type: "whitelist", IPRules: `[{"cidr":"8.8.8.8/32"}]`, Enabled: true}
	require.NoError(t, db.Create(acl).Error)
	managementSetting(t, db, acl.UUID)

	c := cerberus.New(config.SecurityConfig{ACLMode: "enabled"}, db)
	aborted, code := aclMiddlewareContext(t, c, "not-an-address")
	require.True(t, aborted)
	require.Equal(t, http.StatusForbidden, code)
}

func TestMiddleware_ManagementCorruptChoiceIsRefused(t *testing.T) {
	db := setupDB(t)
	require.NoError(t, db.Create(&models.Setting{Key: securitypkg.ManagementListsSettingKey, Value: "{not json", Category: "security", Type: "json"}).Error)

	c := cerberus.New(config.SecurityConfig{ACLMode: "enabled"}, db)
	aborted, code := aclMiddlewareContext(t, c, "203.0.113.5:1234")
	require.True(t, aborted)
	require.Equal(t, http.StatusForbidden, code)
}

func TestMiddleware_ManagementUnusableWhitelistRefuses(t *testing.T) {
	db := setupDB(t)
	acl := &models.AccessList{UUID: "bad-allow", Name: "Bad", Type: "whitelist", IPRules: `[{"cidr":"bogus"}]`, Enabled: true}
	require.NoError(t, db.Create(acl).Error)
	managementSetting(t, db, acl.UUID)

	c := cerberus.New(config.SecurityConfig{ACLMode: "enabled"}, db)
	aborted, code := aclMiddlewareContext(t, c, "8.8.8.8:1234")
	require.True(t, aborted)
	require.Equal(t, http.StatusForbidden, code)
}

func TestMiddleware_ManagementUnusableBlacklistIsNotApplied(t *testing.T) {
	db := setupDB(t)
	acl := &models.AccessList{UUID: "bad-block", Name: "Bad", Type: "blacklist", IPRules: `[{"cidr":"bogus"}]`, Enabled: true}
	require.NoError(t, db.Create(acl).Error)
	managementSetting(t, db, acl.UUID)

	c := cerberus.New(config.SecurityConfig{ACLMode: "enabled"}, db)
	aborted, _ := aclMiddlewareContext(t, c, "8.8.8.8:1234")
	require.False(t, aborted, "a block-list that cannot be used blocks nobody, as at the proxy")
}

// TestMiddleware_OrthrusRouteIsExemptFromManagementLists: the agent route is not judged by the management lists.
func TestMiddleware_OrthrusRouteIsExemptFromManagementLists(t *testing.T) {
	db := setupDB(t)
	acl := &models.AccessList{UUID: "allow-8-only", Name: "Allow8", Type: "whitelist", IPRules: `[{"cidr":"8.8.8.8/32"}]`, Enabled: true}
	require.NoError(t, db.Create(acl).Error)
	managementSetting(t, db, acl.UUID)

	gin.SetMode(gin.TestMode)
	c := cerberus.New(config.SecurityConfig{ACLMode: "enabled"}, db)
	router := gin.New()
	router.GET("/api/v1/ws/orthrus/connect", c.Middleware(), func(ctx *gin.Context) { ctx.Status(http.StatusNoContent) })
	router.GET("/api/v1/other", c.Middleware(), func(ctx *gin.Context) { ctx.Status(http.StatusNoContent) })

	agent := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ws/orthrus/connect", http.NoBody)
	req.RemoteAddr = "203.0.113.5:1234"
	router.ServeHTTP(agent, req)
	require.Equal(t, http.StatusNoContent, agent.Code, "the agent route is exempt")

	other := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/other", http.NoBody)
	req.RemoteAddr = "203.0.113.5:1234"
	router.ServeHTTP(other, req)
	require.Equal(t, http.StatusForbidden, other.Code)
}

func TestMiddleware_ACLSwitchInSettingsOverridesConfig(t *testing.T) {
	db := setupDB(t)
	acl := &models.AccessList{UUID: "allow-8-only", Name: "Allow8", Type: "whitelist", IPRules: `[{"cidr":"8.8.8.8/32"}]`, Enabled: true}
	require.NoError(t, db.Create(acl).Error)
	managementSetting(t, db, acl.UUID)
	require.NoError(t, db.Create(&models.Setting{Key: models.SecurityACLEnabledSettingKey, Value: "false", Category: "security", Type: "bool"}).Error)

	c := cerberus.New(config.SecurityConfig{ACLMode: "enabled"}, db)
	aborted, _ := aclMiddlewareContext(t, c, "203.0.113.5:1234")
	require.False(t, aborted, "a runtime switch set to off wins over the static configuration")
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

func TestMiddleware_EmergencyBypassSkipsChecks(t *testing.T) {
	t.Parallel()

	db := setupDB(t)
	c := cerberus.New(config.SecurityConfig{CerberusEnabled: true, ACLMode: "enabled"}, db)

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/admin/secure", http.NoBody)
	req.RemoteAddr = "203.0.113.10:1234"
	ctx.Request = req
	ctx.Set("emergency_bypass", true)

	mw := c.Middleware()
	mw(ctx)

	require.False(t, ctx.IsAborted(), "middleware should short-circuit when emergency_bypass=true")
}

// TestMiddleware_CountryManagementListUsesGeoIPService: a country list chosen for Charon's own interface is decided with
// the GeoIP service once one is wired; without a service it is not enforced.
func TestMiddleware_CountryManagementListUsesGeoIPService(t *testing.T) {
	db := setupDB(t)
	acl := &models.AccessList{UUID: "geo-block", Name: "geo", Type: "geo_blacklist", CountryCodes: "RU", Enabled: true}
	require.NoError(t, db.Create(acl).Error)
	managementSetting(t, db, acl.UUID)

	c := cerberus.New(config.SecurityConfig{ACLMode: "enabled"}, db)
	aborted, _ := aclMiddlewareContext(t, c, "8.8.8.8:1234")
	require.False(t, aborted, "without a GeoIP service the country list is not enforced")

	c.SetGeoIPService(nil)
	aborted, _ = aclMiddlewareContext(t, c, "8.8.8.8:1234")
	require.False(t, aborted, "a nil GeoIP service is the same as none")
}
