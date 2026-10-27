package handlers

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// proxyApplyBound is the longest a request-triggered proxy apply may run.
const proxyApplyBound = 30 * time.Second

// requireDetachedApply checks that the single apply a caller made saw a live, bounded context.
func requireDetachedApply(t *testing.T, applier *recordingApplier) {
	t.Helper()
	require.Len(t, applier.seen, 1, "the caller must apply the proxy configuration once")
	seen := applier.seen[0]
	require.NoError(t, seen.ctxErr, "the apply must not inherit the client's cancellation")
	require.True(t, seen.hasDeadline, "the apply must be bounded")
	require.LessOrEqual(t, time.Until(seen.deadline), proxyApplyBound)
}

// cancelledRequestContext returns a request context that is already cancelled, as after a client disconnect.
func cancelledRequestContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

// proxyApplyAdminContext builds an admin gin context whose request carries reqCtx.
func proxyApplyAdminContext(method, path string, body []byte, reqCtx context.Context) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(method, path, bytes.NewReader(body)).WithContext(reqCtx)
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Set("role", "admin")
	c.Set("userID", uint(1))
	return c, w
}

func TestProxyApplyDetached_SettingsUpdateSettingRequestCancelled(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}))
	applier := &recordingApplier{}
	handler := NewSettingsHandlerWithDeps(db, applier, nil, nil, "")

	body := []byte(`{"key":"security.waf.enabled","value":"true"}`)
	c, w := proxyApplyAdminContext(http.MethodPost, "/settings", body, cancelledRequestContext())
	handler.UpdateSetting(c)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	requireDetachedApply(t, applier)
}

func TestProxyApplyDetached_SettingsPatchConfigRequestCancelled(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}))
	applier := &recordingApplier{}
	handler := NewSettingsHandlerWithDeps(db, applier, nil, nil, "")

	body := []byte(`{"security":{"waf":{"enabled":"true"}}}`)
	c, w := proxyApplyAdminContext(http.MethodPatch, "/config", body, cancelledRequestContext())
	handler.PatchConfig(c)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	requireDetachedApply(t, applier)
}

func TestProxyApplyDetached_EmergencyResetRequestCancelled(t *testing.T) {
	db := setupEmergencyTestDB(t)
	applier := &recordingApplier{}
	handler := NewEmergencyHandlerWithDeps(db, applier, nil)
	t.Cleanup(handler.Close)
	router := setupEmergencyRouter(handler)

	validToken := "this-is-a-valid-emergency-token-with-32-chars-minimum"
	require.NoError(t, os.Setenv(EmergencyTokenEnvVar, validToken))
	defer func() { require.NoError(t, os.Unsetenv(EmergencyTokenEnvVar)) }()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/emergency/security-reset", http.NoBody).WithContext(cancelledRequestContext())
	req.Header.Set(EmergencyTokenHeader, validToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	requireDetachedApply(t, applier)
}

func TestProxyApplyDetached_SecurityUpdateConfigRequestCancelled(t *testing.T) {
	h, db := setupToggleTest(t)
	applier := &recordingApplier{}
	h.caddyManager = applier
	require.NoError(t, db.Where("name = ?", models.DefaultSecurityConfigName).Delete(&models.SecurityConfig{}).Error)

	body := []byte(`{"name":"default","enabled":true}`)
	c, w := proxyApplyAdminContext(http.MethodPost, "/security/config", body, cancelledRequestContext())
	h.UpdateConfig(c)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	requireDetachedApply(t, applier)
}

// securityApplyHandler returns a security handler whose apply is recorded by the returned applier.
func securityApplyHandler(t *testing.T) (*SecurityHandler, *gorm.DB, *recordingApplier) {
	t.Helper()
	h, db := setupToggleTest(t)
	applier := &recordingApplier{}
	h.caddyManager = applier
	return h, db, applier
}

func TestProxyApplyDetached_SecurityUpsertRuleSetRequestCancelled(t *testing.T) {
	h, db, applier := securityApplyHandler(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityRuleSet{}))

	c, w := proxyApplyAdminContext(http.MethodPost, "/security/rulesets", []byte(`{"name":"ruleset-a"}`), cancelledRequestContext())
	h.UpsertRuleSet(c)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	requireDetachedApply(t, applier)
}

func TestProxyApplyDetached_SecurityDeleteRuleSetRequestCancelled(t *testing.T) {
	h, db, applier := securityApplyHandler(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityRuleSet{}))
	ruleSet := models.SecurityRuleSet{Name: "ruleset-a"}
	require.NoError(t, db.Create(&ruleSet).Error)

	c, w := proxyApplyAdminContext(http.MethodDelete, "/security/rulesets", nil, cancelledRequestContext())
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(ruleSet.ID), 10)}}
	h.DeleteRuleSet(c)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	requireDetachedApply(t, applier)
}

func TestProxyApplyDetached_SecurityEnableRequestCancelled(t *testing.T) {
	h, db, applier := securityApplyHandler(t)
	// The request comes from 192.0.2.1 (httptest's default RemoteAddr), so whitelist that address.
	require.NoError(t, db.Model(&models.SecurityConfig{}).Where("name = ?", "default").Update("admin_whitelist", "192.0.2.1/32").Error)

	c, w := proxyApplyAdminContext(http.MethodPost, "/security/enable", []byte(`{}`), cancelledRequestContext())
	h.Enable(c)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	requireDetachedApply(t, applier)
}

func TestProxyApplyDetached_SecurityDisableRequestCancelled(t *testing.T) {
	h, _, applier := securityApplyHandler(t)

	// Disable without a break-glass token is allowed only from localhost.
	c, w := proxyApplyAdminContext(http.MethodPost, "/security/disable", []byte(`{}`), cancelledRequestContext())
	c.Request.RemoteAddr = "127.0.0.1:5000"
	h.Disable(c)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	requireDetachedApply(t, applier)
}

func TestProxyApplyDetached_SecurityAddWAFExclusionRequestCancelled(t *testing.T) {
	h, _, applier := securityApplyHandler(t)

	c, w := proxyApplyAdminContext(http.MethodPost, "/security/waf/exclusions", []byte(`{"rule_id":942100}`), cancelledRequestContext())
	h.AddWAFExclusion(c)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	requireDetachedApply(t, applier)
}

func TestProxyApplyDetached_SecurityDeleteWAFExclusionRequestCancelled(t *testing.T) {
	h, db, applier := securityApplyHandler(t)
	require.NoError(t, db.Model(&models.SecurityConfig{}).Where("name = ?", "default").Update("waf_exclusions", `[{"rule_id":942100}]`).Error)

	c, w := proxyApplyAdminContext(http.MethodDelete, "/security/waf/exclusions/942100", nil, cancelledRequestContext())
	c.Params = gin.Params{{Key: "rule_id", Value: "942100"}}
	h.DeleteWAFExclusion(c)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	requireDetachedApply(t, applier)
}

func TestProxyApplyDetached_SecurityToggleRequestCancelled(t *testing.T) {
	h, _ := setupToggleTest(t)
	applier := &recordingApplier{}
	h.caddyManager = applier

	c, w := proxyApplyAdminContext(http.MethodPost, "/api/v1/security/acl/enable", nil, cancelledRequestContext())
	h.EnableACL(c)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	requireDetachedApply(t, applier)
}
