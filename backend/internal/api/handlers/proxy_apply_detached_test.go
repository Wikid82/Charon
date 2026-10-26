package handlers

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

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

func TestProxyApplyDetached_SecurityToggleRequestCancelled(t *testing.T) {
	h, _ := setupToggleTest(t)
	applier := &recordingApplier{}
	h.caddyManager = applier

	c, w := proxyApplyAdminContext(http.MethodPost, "/api/v1/security/acl/enable", nil, cancelledRequestContext())
	h.EnableACL(c)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	requireDetachedApply(t, applier)
}
