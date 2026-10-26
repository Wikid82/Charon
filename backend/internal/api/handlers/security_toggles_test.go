package handlers

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/testutil/gormfault"
)

func setupToggleTest(t *testing.T) (*SecurityHandler, *gorm.DB) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}))

	// Create default SecurityConfig
	require.NoError(t, db.Create(&models.SecurityConfig{Name: "default", Enabled: true}).Error)

	cfg := config.SecurityConfig{}
	h := NewSecurityHandler(cfg, db, nil) // caddyManager nil to avoid reload logic
	// NewSecurityHandler starts a background audit-processing goroutine (via
	// NewSecurityService) that shares this shared-cache *gorm.DB. Same
	// concern as b9a46963: stop it via t.Cleanup so it can't outlive the test.
	t.Cleanup(h.Close)
	return h, db
}

func TestSecurityToggles(t *testing.T) {
	h, db := setupToggleTest(t)

	tests := []struct {
		name       string
		method     string
		path       string
		handler    gin.HandlerFunc
		settingKey string
		expectVal  string
		body       string
	}{
		// ACL
		{"EnableACL", "POST", "/api/v1/security/acl/enable", h.EnableACL, "security.acl.enabled", "true", ""},
		{"DisableACL", "POST", "/api/v1/security/acl/disable", h.DisableACL, "security.acl.enabled", "false", ""},
		// ACL Patch
		{"PatchACL_True", "PATCH", "/api/v1/security/acl", h.PatchACL, "security.acl.enabled", "true", `{"enabled": true}`},
		{"PatchACL_False", "PATCH", "/api/v1/security/acl", h.PatchACL, "security.acl.enabled", "false", `{"enabled": false}`},

		// WAF
		{"EnableWAF", "POST", "/api/v1/security/waf/enable", h.EnableWAF, "security.waf.enabled", "true", ""},
		{"DisableWAF", "POST", "/api/v1/security/waf/disable", h.DisableWAF, "security.waf.enabled", "false", ""},
		// WAF Patch
		{"PatchWAF_True", "PATCH", "/api/v1/security/waf", h.PatchWAF, "security.waf.enabled", "true", `{"enabled": true}`},
		{"PatchWAF_False", "PATCH", "/api/v1/security/waf", h.PatchWAF, "security.waf.enabled", "false", `{"enabled": false}`},

		// Cerberus
		{"EnableCerberus", "POST", "/api/v1/security/cerberus/enable", h.EnableCerberus, "feature.cerberus.enabled", "true", ""},
		{"DisableCerberus", "POST", "/api/v1/security/cerberus/disable", h.DisableCerberus, "feature.cerberus.enabled", "false", ""},

		// RateLimit
		{"EnableRateLimit", "POST", "/api/v1/security/rate-limit/enable", h.EnableRateLimit, "security.rate_limit.enabled", "true", ""},
		{"DisableRateLimit", "POST", "/api/v1/security/rate-limit/disable", h.DisableRateLimit, "security.rate_limit.enabled", "false", ""},
		// RateLimit Patch
		{"PatchRateLimit_True", "PATCH", "/api/v1/security/rate-limit", h.PatchRateLimit, "security.rate_limit.enabled", "true", `{"enabled": true}`},
		{"PatchRateLimit_False", "PATCH", "/api/v1/security/rate-limit", h.PatchRateLimit, "security.rate_limit.enabled", "false", `{"enabled": false}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			var req *http.Request
			if tc.body != "" {
				req, _ = http.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req, _ = http.NewRequest(tc.method, tc.path, http.NoBody)
			}

			c, _ := gin.CreateTestContext(w)
			c.Request = req
			// Mock Admin Role
			c.Set("role", "admin")

			tc.handler(c)

			require.Equal(t, http.StatusOK, w.Code)

			// Verify Setting
			var setting models.Setting
			err := db.Where("key = ?", tc.settingKey).First(&setting).Error
			assert.NoError(t, err)
			assert.Equal(t, tc.expectVal, setting.Value)

			if tc.expectVal == "true" && tc.settingKey != "feature.cerberus.enabled" {
				var cerberusSetting models.Setting
				err = db.Where("key = ?", "feature.cerberus.enabled").First(&cerberusSetting).Error
				assert.NoError(t, err)
				assert.Equal(t, "true", cerberusSetting.Value)
			}
		})
	}
}

// TestSecurityToggles_ModuleResponseAndFrameworkSwitch pins the response body of a module toggle and the
// framework switch: enabling a module turns the security framework on, disabling it leaves the framework on.
func TestSecurityToggles_ModuleResponseAndFrameworkSwitch(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}))
	require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, Enabled: false}).Error)
	h := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	t.Cleanup(h.Close)

	enable := toggleRequest(t, h.EnableRateLimit, http.MethodPost, "/api/v1/security/rate-limit/enable")
	require.Equal(t, http.StatusOK, enable.Code, enable.Body.String())
	assert.JSONEq(t, `{"success":true,"module":"security.rate_limit.enabled","enabled":true,"applied":true}`, enable.Body.String())

	var row models.SecurityConfig
	require.NoError(t, db.Where("name = ?", models.DefaultSecurityConfigName).First(&row).Error)
	assert.True(t, row.Enabled, "enabling a module turns the security framework on")

	disable := toggleRequest(t, h.DisableRateLimit, http.MethodPost, "/api/v1/security/rate-limit/disable")
	require.Equal(t, http.StatusOK, disable.Code, disable.Body.String())
	assert.JSONEq(t, `{"success":true,"module":"security.rate_limit.enabled","enabled":false,"applied":true}`, disable.Body.String())

	require.NoError(t, db.Where("name = ?", models.DefaultSecurityConfigName).First(&row).Error)
	assert.True(t, row.Enabled, "disabling a module leaves the security framework on")
}

// toggleRequest runs one admin toggle handler against a request with no body.
func toggleRequest(t *testing.T, handler gin.HandlerFunc, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, http.NoBody)
	c.Set("role", "admin")
	handler(c)
	return w
}

func TestSecurityToggles_Forbidden(t *testing.T) {
	h, _ := setupToggleTest(t)

	// Just test one endpoint to verify role check
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/security/acl/enable", http.NoBody)
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	// No role set

	h.EnableACL(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestPatchACL_InvalidBody(t *testing.T) {
	h, _ := setupToggleTest(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/api/v1/security/acl", strings.NewReader("invalid"))
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Set("role", "admin")

	h.PatchACL(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPatchWAF_InvalidBody(t *testing.T) {
	h, _ := setupToggleTest(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/api/v1/security/waf", strings.NewReader("invalid"))
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Set("role", "admin")

	h.PatchWAF(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPatchRateLimit_InvalidBody(t *testing.T) {
	h, _ := setupToggleTest(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/api/v1/security/rate-limit", strings.NewReader("invalid"))
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Set("role", "admin")

	h.PatchRateLimit(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPatchCrowdSec_InvalidBody(t *testing.T) {
	h, _ := setupToggleTest(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/api/v1/security/crowdsec", strings.NewReader("invalid"))
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Set("role", "admin")

	h.PatchCrowdSec(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestACLForbiddenIfIPNotWhitelisted(t *testing.T) {
	h, db := setupToggleTest(t)

	// Update config to have whitelist
	err := db.Model(&models.SecurityConfig{}).Where("name = ?", "default").Update("admin_whitelist", "10.0.0.1").Error
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/security/acl/enable", http.NoBody)
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Set("role", "admin")
	c.Request.RemoteAddr = "192.168.1.5:1234" // Different IP

	h.EnableACL(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestACLEnabledIfIPWhitelisted(t *testing.T) {
	h, db := setupToggleTest(t)

	// Update config to have whitelist
	err := db.Model(&models.SecurityConfig{}).Where("name = ?", "default").Update("admin_whitelist", "1.2.3.4").Error
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/security/acl/enable", http.NoBody)
	req.Header.Set("X-Forwarded-For", "1.2.3.4") // Trusted proxy simulation needed or direct RemoteAddr
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Request.RemoteAddr = "1.2.3.4:1234"
	c.Set("role", "admin")

	h.EnableACL(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSecurityToggles_RollbackSettingWhenApplyFails(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}))
	require.NoError(t, db.Create(&models.SecurityConfig{Name: "default", Enabled: true}).Error)
	require.NoError(t, db.Create(&models.Setting{Key: "security.waf.enabled", Value: "false", Category: "security", Type: "bool"}).Error)

	manager := caddy.NewManager(
		caddy.NewClient("http://127.0.0.1:65535"),
		db,
		t.TempDir(),
		t.TempDir(),
		false,
		config.SecurityConfig{},
	)
	h := NewSecurityHandler(config.SecurityConfig{}, db, manager)
	t.Cleanup(h.Close)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/api/v1/security/waf", strings.NewReader(`{"enabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Set("role", "admin")

	h.PatchWAF(c)

	require.Equal(t, http.StatusInternalServerError, w.Code)

	var setting models.Setting
	require.NoError(t, db.Where("key = ?", "security.waf.enabled").First(&setting).Error)
	assert.Equal(t, "false", setting.Value)
}

// newFailingApplyWAFToggle builds a WAF toggle whose Caddy apply always fails, so the handler must roll back.
// The WAF setting exists as false and the default row is off, so the rollback has state to restore.
func newFailingApplyWAFToggle(t *testing.T) (*SecurityHandler, *gorm.DB) {
	t.Helper()
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}))
	require.NoError(t, db.Create(&models.SecurityConfig{Name: "default", Enabled: false}).Error)
	require.NoError(t, db.Create(&models.Setting{Key: "security.waf.enabled", Value: "false", Category: "security", Type: "bool"}).Error)

	manager := caddy.NewManager(
		caddy.NewClient("http://127.0.0.1:65535"),
		db,
		t.TempDir(),
		t.TempDir(),
		false,
		config.SecurityConfig{},
	)
	h := NewSecurityHandler(config.SecurityConfig{}, db, manager)
	t.Cleanup(h.Close)
	return h, db
}

// patchWAFEnable sends the enable request through PatchWAF and returns the recorded response.
func patchWAFEnable(t *testing.T, h *SecurityHandler) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/api/v1/security/waf", strings.NewReader(`{"enabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Set("role", "admin")
	h.PatchWAF(c)
	return w
}

// captureLogrusOutput routes the logrus standard logger, which the security handlers write to, into a buffer
// for the rest of the test.
func captureLogrusOutput(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	previous := log.StandardLogger().Out
	log.SetOutput(buf)
	t.Cleanup(func() { log.SetOutput(previous) })
	return buf
}

// When the apply fails and the default row cannot be restored either, the failure is logged and the response is
// still a 500 with the setting rolled back. Statements on security_configs before the restore are the framework
// enable only, so the second update is the restore.
func TestSecurityToggles_LogsWhenDefaultRowRestoreFailsAfterApplyFailure(t *testing.T) {
	h, db := newFailingApplyWAFToggle(t)
	gormfault.InjectNth(t, db, gormfault.Update, "security_configs", 2, nil)
	logs := captureLogrusOutput(t)

	w := patchWAFEnable(t, h)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, logs.String(), "Failed to restore security config after security module toggle apply failure")
	var setting models.Setting
	require.NoError(t, db.Where("key = ?", "security.waf.enabled").First(&setting).Error)
	assert.Equal(t, "false", setting.Value)
}

// When the apply fails and the settings snapshot cannot be restored, the failure is logged and the response is
// still a 500. The second settings update is the restore of the WAF setting (the first is the toggle write).
func TestSecurityToggles_LogsWhenSettingsRestoreFailsAfterApplyFailure(t *testing.T) {
	h, db := newFailingApplyWAFToggle(t)
	gormfault.InjectNth(t, db, gormfault.Update, "settings", 2, nil)
	logs := captureLogrusOutput(t)

	w := patchWAFEnable(t, h)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, logs.String(), "Failed to restore settings after security module toggle apply failure")
}

// fakeCrowdsecLifecycle stands in for the CrowdSec lifecycle. It records the calls and answers with scripted results.
type fakeCrowdsecLifecycle struct {
	startRes   CrowdsecStartResult
	startErr   error
	stopRes    CrowdsecStopResult
	stopErr    error
	startCalls int
	stopCalls  int
}

func (f *fakeCrowdsecLifecycle) StartCrowdSec(_ context.Context) (CrowdsecStartResult, error) {
	f.startCalls++
	return f.startRes, f.startErr
}

func (f *fakeCrowdsecLifecycle) StopCrowdSec(_ context.Context) (CrowdsecStopResult, error) {
	f.stopCalls++
	return f.stopRes, f.stopErr
}

func (f *fakeCrowdsecLifecycle) ResetCrowdSecState(write func() error) error { return write() }

func (f *fakeCrowdsecLifecycle) StopAfterReset(error) {}

// newCrowdsecSwitchHandler builds a security handler whose CrowdSec switches drive fake.
func newCrowdsecSwitchHandler(t *testing.T, fake *fakeCrowdsecLifecycle) (*SecurityHandler, *gorm.DB) {
	t.Helper()
	h, db := setupToggleTest(t)
	return h.WithCrowdsecLifecycle(fake), db
}

// crowdsecSwitchRequest runs one CrowdSec switch as role with the given body.
func crowdsecSwitchRequest(t *testing.T, handler gin.HandlerFunc, method, body, role string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, "/api/v1/security/crowdsec", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	if role != "" {
		c.Set("role", role)
	}
	handler(c)
	return w
}

// crowdsecSettingWritten reports whether the security.crowdsec.enabled setting row exists.
func crowdsecSettingWritten(t *testing.T, db *gorm.DB) bool {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(&models.Setting{}).Where("key = ?", "security.crowdsec.enabled").Count(&count).Error)
	return count > 0
}

// TestSecurityToggles_CrowdSecLifecycle pins the CrowdSec switches: each one starts or stops the engine through the
// lifecycle, and the 200 body carries the engine fields next to the existing keys.
func TestSecurityToggles_CrowdSecLifecycle(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		body       string
		handler    func(*SecurityHandler) gin.HandlerFunc
		start      CrowdsecStartResult
		stop       CrowdsecStopResult
		wantStart  int
		wantStop   int
		wantStatus int
		wantBody   string
	}{
		{
			name:       "EnableCrowdSec starts the engine",
			method:     http.MethodPost,
			handler:    func(h *SecurityHandler) gin.HandlerFunc { return h.EnableCrowdSec },
			start:      CrowdsecStartResult{Status: "started", LAPIReady: true, ProxyApplied: true},
			wantStart:  1,
			wantStatus: http.StatusOK,
			wantBody:   `{"success":true,"module":"security.crowdsec.enabled","enabled":true,"applied":true,"status":"started","lapi_ready":true}`,
		},
		{
			name:       "PatchCrowdSec true reports an engine that already runs",
			method:     http.MethodPatch,
			body:       `{"enabled": true}`,
			handler:    func(h *SecurityHandler) gin.HandlerFunc { return h.PatchCrowdSec },
			start:      CrowdsecStartResult{Status: "already_running", PID: 42, LAPIReady: true, ProxyApplied: true},
			wantStart:  1,
			wantStatus: http.StatusOK,
			wantBody:   `{"success":true,"module":"security.crowdsec.enabled","enabled":true,"applied":true,"status":"already_running","lapi_ready":true}`,
		},
		{
			name:       "DisableCrowdSec stops the engine",
			method:     http.MethodPost,
			handler:    func(h *SecurityHandler) gin.HandlerFunc { return h.DisableCrowdSec },
			stop:       CrowdsecStopResult{ProxyApplied: true},
			wantStop:   1,
			wantStatus: http.StatusOK,
			wantBody:   `{"success":true,"module":"security.crowdsec.enabled","enabled":false,"applied":true,"status":"stopped"}`,
		},
		{
			name:       "PatchCrowdSec false reports a proxy apply that did not take",
			method:     http.MethodPatch,
			body:       `{"enabled": false}`,
			handler:    func(h *SecurityHandler) gin.HandlerFunc { return h.PatchCrowdSec },
			stop:       CrowdsecStopResult{ProxyApplied: false},
			wantStop:   1,
			wantStatus: http.StatusOK,
			wantBody:   `{"success":true,"module":"security.crowdsec.enabled","enabled":false,"applied":false,"status":"stopped"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeCrowdsecLifecycle{startRes: tc.start, stopRes: tc.stop}
			h, db := newCrowdsecSwitchHandler(t, fake)

			w := crowdsecSwitchRequest(t, tc.handler(h), tc.method, tc.body, "admin")

			require.Equal(t, tc.wantStatus, w.Code, w.Body.String())
			assert.JSONEq(t, tc.wantBody, w.Body.String())
			assert.Equal(t, tc.wantStart, fake.startCalls)
			assert.Equal(t, tc.wantStop, fake.stopCalls)
			assert.False(t, crowdsecSettingWritten(t, db), "the handler writes nothing itself; the lifecycle owns the setting")
		})
	}
}

func TestSecurityToggles_CrowdSecEnableDuringJobIsRefused(t *testing.T) {
	fake := &fakeCrowdsecLifecycle{startErr: &CrowdsecBusyError{JobID: "job-7"}}
	h, db := newCrowdsecSwitchHandler(t, fake)

	w := crowdsecSwitchRequest(t, h.EnableCrowdSec, http.MethodPost, "", "admin")

	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.JSONEq(t, `{"error":"CrowdSec is busy installing a preset; try again when it finishes","error_code":"crowdsec_busy","job_id":"job-7"}`, w.Body.String())
	assert.False(t, crowdsecSettingWritten(t, db), "a refused enable writes nothing")
}

func TestSecurityToggles_CrowdSecFailedStartAnswers500(t *testing.T) {
	fake := &fakeCrowdsecLifecycle{startErr: fmt.Errorf("%w: exec failed", errCrowdsecStart)}
	h, db := newCrowdsecSwitchHandler(t, fake)

	w := crowdsecSwitchRequest(t, h.EnableCrowdSec, http.MethodPost, "", "admin")

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	assert.JSONEq(t, `{"error":"failed to start CrowdSec"}`, w.Body.String())
	assert.False(t, crowdsecSettingWritten(t, db))
}

func TestSecurityToggles_CrowdSecFailedStopAnswers500(t *testing.T) {
	fake := &fakeCrowdsecLifecycle{stopErr: fmt.Errorf("%w: pid still alive", errCrowdsecStop)}
	h, _ := newCrowdsecSwitchHandler(t, fake)

	w := crowdsecSwitchRequest(t, h.DisableCrowdSec, http.MethodPost, "", "admin")

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	assert.JSONEq(t, `{"error":"failed to stop CrowdSec"}`, w.Body.String())
}

func TestSecurityToggles_CrowdSecApplyFailureAnswers200NotApplied(t *testing.T) {
	fake := &fakeCrowdsecLifecycle{startRes: CrowdsecStartResult{Status: "started", LAPIReady: true, ProxyApplied: false}}
	h, _ := newCrowdsecSwitchHandler(t, fake)

	w := crowdsecSwitchRequest(t, h.EnableCrowdSec, http.MethodPost, "", "admin")

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.JSONEq(t, `{"success":true,"module":"security.crowdsec.enabled","enabled":true,"applied":false,"status":"started","lapi_ready":true}`, w.Body.String())
	assert.Equal(t, 1, fake.startCalls)
}

// A non-admin is refused before the body is read, so a malformed body gets 403, not 400.
func TestSecurityToggles_CrowdSecNonAdminBadBodyIsForbidden(t *testing.T) {
	fake := &fakeCrowdsecLifecycle{}
	h, db := newCrowdsecSwitchHandler(t, fake)

	w := crowdsecSwitchRequest(t, h.PatchCrowdSec, http.MethodPatch, "invalid", "user")

	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	assert.JSONEq(t, `{"error":"admin privileges required","error_code":"permissions_admin_only"}`, w.Body.String())
	assert.Equal(t, 0, fake.startCalls+fake.stopCalls)
	assert.False(t, crowdsecSettingWritten(t, db))
}

// Without a lifecycle the CrowdSec switches cannot drive the engine, so they answer 503 and write nothing.
func TestSecurityToggles_CrowdSecWithoutLifecycleAnswers503(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		body    string
		handler func(*SecurityHandler) gin.HandlerFunc
	}{
		{"EnableCrowdSec", http.MethodPost, "", func(h *SecurityHandler) gin.HandlerFunc { return h.EnableCrowdSec }},
		{"DisableCrowdSec", http.MethodPost, "", func(h *SecurityHandler) gin.HandlerFunc { return h.DisableCrowdSec }},
		{"PatchCrowdSec", http.MethodPatch, `{"enabled": true}`, func(h *SecurityHandler) gin.HandlerFunc { return h.PatchCrowdSec }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, db := setupToggleTest(t)

			w := crowdsecSwitchRequest(t, tc.handler(h), tc.method, tc.body, "admin")

			require.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
			assert.JSONEq(t, `{"error":"CrowdSec is not available on this server.","error_code":"crowdsec_unavailable"}`, w.Body.String())
			assert.False(t, crowdsecSettingWritten(t, db))
		})
	}
}
