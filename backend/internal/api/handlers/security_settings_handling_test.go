package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

// TestSecurityHandler_Disable_KeepsModuleColumns pins that the framework disable only changes the enabled flag of
// the default row and keeps the WAF and rate-limit columns as stored.
func TestSecurityHandler_Disable_KeepsModuleColumns(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{
		Name:               models.DefaultSecurityConfigName,
		Enabled:            true,
		WAFMode:            "block",
		RateLimitMode:      "enabled",
		RateLimitEnable:    true,
		RateLimitRequests:  50,
		RateLimitWindowSec: 60,
		RateLimitBurst:     20,
	}).Error)
	h := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	t.Cleanup(h.Close)

	w := serveDisable(t, h, "127.0.0.1:5000", `{}`)
	require.Equal(t, http.StatusOK, w.Code)

	var stored models.SecurityConfig
	require.NoError(t, db.Where("name = ?", models.DefaultSecurityConfigName).First(&stored).Error)
	assert.False(t, stored.Enabled)
	assert.Equal(t, "block", stored.WAFMode)
	assert.Equal(t, "enabled", stored.RateLimitMode)
	assert.True(t, stored.RateLimitEnable)
	assert.Equal(t, 50, stored.RateLimitRequests)
	assert.Equal(t, 60, stored.RateLimitWindowSec)
	assert.Equal(t, 20, stored.RateLimitBurst)
}

// Status contract: the status endpoint reports the state the proxy applied, not only the stored flags.

// TestSecurityStatus_ReportsAppliedState describes the additive status fields (enforced, unprotected host count,
// reason) being filled from the last enforcement report.
func TestSecurityStatus_ReportsAppliedState(t *testing.T) {
	h, _ := newStatusApplyFixture(t, models.SecurityConfig{
		Enabled:            true,
		WAFMode:            "block",
		RateLimitMode:      "enabled",
		RateLimitEnable:    true,
		RateLimitRequests:  100,
		RateLimitWindowSec: 60,
		RateLimitBurst:     20,
	}, noopCaddyClient{})
	mustApplyConfig(t, h)

	status := getSecurityStatus(t, h)
	waf := status["waf"].(map[string]any)
	assert.Equal(t, true, waf["enforced"])
	assert.Equal(t, float64(0), waf["unprotected_hosts"])
	assert.NotContains(t, waf, "reason", "a reason is reported only when a host is left out")

	rateLimit := status["rate_limit"].(map[string]any)
	assert.Equal(t, true, rateLimit["enforced"])
	assert.NotContains(t, rateLimit, "reason")
}

// TestSecurityStatus_KeyOnWithDisabledRowIsEnforced describes the WAF key on while the default row still holds the
// disabled mode: the key decides, so the handler is built and the status reports the WAF applied to every host.
func TestSecurityStatus_KeyOnWithDisabledRowIsEnforced(t *testing.T) {
	h, db := newStatusApplyFixture(t, models.SecurityConfig{Enabled: true, WAFMode: "disabled"}, noopCaddyClient{})
	setSettingValue(t, db, models.SecurityWAFEnabledSettingKey, "true")
	mustApplyConfig(t, h)

	waf := getSecurityStatus(t, h)["waf"].(map[string]any)
	assert.Equal(t, true, waf["enabled"], "the WAF key is on")
	assert.Equal(t, true, waf["enforced"])
	assert.Equal(t, float64(0), waf["unprotected_hosts"])
	assert.NotContains(t, waf, "reason")
}

// TestSecurityStatus_StaleEnforcementReportNotTrusted describes a stored enforcement report whose stamped flags do
// not match the handler's resolved flags (or that predates a failed apply): the enforced field is omitted.
func TestSecurityStatus_StaleEnforcementReportNotTrusted(t *testing.T) {
	row := models.SecurityConfig{
		Enabled:            true,
		WAFMode:            "block",
		RateLimitMode:      "enabled",
		RateLimitEnable:    true,
		RateLimitRequests:  100,
		RateLimitWindowSec: 60,
		RateLimitBurst:     20,
	}

	t.Run("no successful apply yet", func(t *testing.T) {
		h, _ := newStatusApplyFixture(t, row, noopCaddyClient{})
		status := getSecurityStatus(t, h)
		assert.NotContains(t, status["waf"], "enforced")
		assert.NotContains(t, status["waf"], "unprotected_hosts")
	})

	t.Run("module state changed after the apply", func(t *testing.T) {
		h, db := newStatusApplyFixture(t, row, noopCaddyClient{})
		mustApplyConfig(t, h)

		// WAF key turned off after the apply: the WAF fields are dropped, the unchanged rate limit keeps its fields.
		setSettingValue(t, db, models.SecurityWAFEnabledSettingKey, "false")
		status := getSecurityStatus(t, h)
		assert.Equal(t, false, status["waf"].(map[string]any)["enabled"])
		assert.NotContains(t, status["waf"], "enforced")
		assert.NotContains(t, status["waf"], "unprotected_hosts")
		assert.Equal(t, true, status["rate_limit"].(map[string]any)["enforced"])

		// Rate limit key turned off as well: its fields are dropped too.
		setSettingValue(t, db, models.SecurityRateLimitEnabledSettingKey, "false")
		status = getSecurityStatus(t, h)
		assert.NotContains(t, status["rate_limit"], "enforced")
	})

	t.Run("failed apply after a successful one", func(t *testing.T) {
		client := &switchableLoadClient{}
		h, _ := newStatusApplyFixture(t, row, client)
		mustApplyConfig(t, h)
		require.Equal(t, true, getSecurityStatus(t, h)["waf"].(map[string]any)["enforced"])

		client.loadErr = errors.New("caddy refused the config")
		require.Error(t, h.caddyManager.ApplyConfig(context.Background()))
		status := getSecurityStatus(t, h)
		assert.NotContains(t, status["waf"], "enforced")
		assert.NotContains(t, status["rate_limit"], "enforced")
	})

	t.Run("no caddy manager wired", func(t *testing.T) {
		db := OpenTestDB(t)
		require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}))
		require.NoError(t, db.Create(&row).Error)
		h := NewSecurityHandler(config.SecurityConfig{CerberusEnabled: true}, db, nil)
		t.Cleanup(h.Close)
		assert.NotContains(t, getSecurityStatus(t, h)["waf"], "enforced")
	})
}

// TestSecurityStatus_RoutesUseRealManagerReportAccessor describes the route wiring: the real caddy manager satisfies
// the optional report accessor the status handler type-asserts on.
func TestSecurityStatus_RoutesUseRealManagerReportAccessor(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}))
	manager := caddy.NewManager(noopCaddyClient{}, db, t.TempDir(), "", false, config.SecurityConfig{CerberusEnabled: true})

	// The routes pass the concrete manager through the nil-safe wrapper of NewSecurityHandlerWithDeps.
	h := NewSecurityHandlerWithDeps(config.SecurityConfig{CerberusEnabled: true}, db, manager, nil)
	t.Cleanup(h.Close)
	_, ok := h.caddyManager.(enforcementReporter)
	require.True(t, ok, "the real caddy manager must satisfy the report accessor")
}

// switchableLoadClient is a Caddy client whose Load fails while loadErr is set.
type switchableLoadClient struct {
	loadErr error
}

func (c *switchableLoadClient) Load(context.Context, *caddy.Config) error { return c.loadErr }
func (c *switchableLoadClient) Ping(context.Context) error                { return nil }
func (c *switchableLoadClient) GetConfig(context.Context) (*caddy.Config, error) {
	return &caddy.Config{}, nil
}

// ruleSetUUID is the uuid of the ruleset the status fixture creates; DELETE accepts it in place of the numeric id.
const ruleSetUUID = "9b2f7c1e-5d3a-4f6b-8c2d-1a0b9e8f7d6c"

// newStatusApplyFixture returns a security handler wired to a real caddy manager over a database that holds the given
// default row, one enabled host and one ruleset. The returned database is the one the handler and manager read.
func newStatusApplyFixture(t *testing.T, row models.SecurityConfig, client caddy.CaddyClient) (*SecurityHandler, *gorm.DB) {
	t.Helper()
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}))
	migrateCaddyApplyTables(t, db)

	row.Name = models.DefaultSecurityConfigName
	require.NoError(t, db.Create(&row).Error)
	require.NoError(t, db.Create(&models.SecurityRuleSet{
		UUID:    ruleSetUUID,
		Name:    "owasp-crs",
		Content: "SecRule ARGS \"@contains probe\" \"id:1000,phase:1,deny\"",
	}).Error)
	require.NoError(t, db.Create(&models.ProxyHost{
		UUID:          "status-host",
		DomainNames:   "status.example.com",
		ForwardScheme: "http",
		ForwardHost:   "upstream.internal",
		ForwardPort:   8080,
		Enabled:       true,
	}).Error)

	manager := caddy.NewManager(client, db, t.TempDir(), "", false, config.SecurityConfig{CerberusEnabled: true})
	h := NewSecurityHandler(config.SecurityConfig{CerberusEnabled: true}, db, manager)
	t.Cleanup(h.Close)
	return h, db
}

// mustApplyConfig runs the handler's manager apply and fails the test when it does not succeed.
func mustApplyConfig(t *testing.T, h *SecurityHandler) {
	t.Helper()
	require.NoError(t, h.caddyManager.ApplyConfig(context.Background()))
}

// setSettingValue writes a security setting, creating the row when it does not exist yet.
func setSettingValue(t *testing.T, db *gorm.DB, key, value string) {
	t.Helper()
	require.NoError(t, db.Where(models.Setting{Key: key}).
		Assign(models.Setting{Value: value, Category: "security", Type: "bool"}).
		FirstOrCreate(&models.Setting{}).Error)
}

// getSecurityStatus calls GET /security/status and returns the decoded body.
func getSecurityStatus(t *testing.T, h *SecurityHandler) map[string]any {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/security/status", h.GetStatus)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/security/status", http.NoBody))
	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body
}

// Save contract (POST /security/config): a mode maps to a module key only when it differs from the stored row.

// newModuleRouter serves the module writers and readers of one handler for an admin caller.
func newModuleRouter(t *testing.T, h *SecurityHandler) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	require.NoError(t, router.SetTrustedProxies(nil))
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", uint(1))
		c.Next()
	})
	router.GET("/security/config", h.GetConfig)
	router.POST("/security/config", h.UpdateConfig)
	router.GET("/security/status", h.GetStatus)
	router.POST("/security/waf/enable", h.EnableWAF)
	router.POST("/security/waf/disable", h.DisableWAF)
	router.POST("/security/rate-limit/enable", h.EnableRateLimit)
	router.POST("/security/rate-limit/disable", h.DisableRateLimit)
	router.POST("/security/break-glass", h.GenerateBreakGlass)
	router.DELETE("/security/rulesets/:id", h.DeleteRuleSet)
	return router
}

// newModuleSettingsRouter serves the generic settings save and the bulk config patch for an admin caller.
func newModuleSettingsRouter(t *testing.T, db *gorm.DB, manager CaddyConfigManager) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	settings := NewSettingsHandlerWithDeps(db, manager, nil, nil, "")
	router := gin.New()
	require.NoError(t, router.SetTrustedProxies(nil))
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", uint(1))
		c.Next()
	})
	router.POST("/settings", settings.UpdateSetting)
	router.PATCH("/config", settings.PatchConfig)
	return router
}

// settingValue reads a settings key; found is false when the key is absent.
func settingValue(t *testing.T, db *gorm.DB, key string) (value string, found bool) {
	t.Helper()
	var setting models.Setting
	result := db.Where("key = ?", key).Limit(1).Find(&setting)
	require.NoError(t, result.Error)
	return setting.Value, result.RowsAffected > 0
}

// moduleStateView is the module part of the stored state: the module and framework keys and the default row columns
// that the module writers change. It is compared before and after a writer whose apply fails.
type moduleStateView struct {
	Keys                                  map[string]string
	WAFKey, RateLimitKey                  string
	Enabled                               bool
	WAFMode, WAFRulesSource               string
	RateLimitMode                         string
	RateLimitEnable                       bool
	RateLimitRequests, RateLimitWindowSec int
	RateLimitBurst                        int
	AdminWhitelist, RateLimitBypassList   string
}

// captureModuleStateView reads the module state. Missing keys are absent from Keys.
func captureModuleStateView(t *testing.T, db *gorm.DB) moduleStateView {
	t.Helper()
	view := moduleStateView{Keys: map[string]string{}}
	var settings []models.Setting
	require.NoError(t, db.Where("key LIKE ? OR key LIKE ?", "security.%", "feature.cerberus%").Find(&settings).Error)
	for _, s := range settings {
		view.Keys[s.Key] = s.Value
	}
	view.WAFKey = view.Keys[models.SecurityWAFEnabledSettingKey]
	view.RateLimitKey = view.Keys[models.SecurityRateLimitEnabledSettingKey]

	row := loadDefaultSecurityRow(t, db)
	view.Enabled = row.Enabled
	view.WAFMode = row.WAFMode
	view.WAFRulesSource = row.WAFRulesSource
	view.RateLimitMode = row.RateLimitMode
	view.RateLimitEnable = row.RateLimitEnable
	view.RateLimitRequests = row.RateLimitRequests
	view.RateLimitWindowSec = row.RateLimitWindowSec
	view.RateLimitBurst = row.RateLimitBurst
	view.AdminWhitelist = row.AdminWhitelist
	view.RateLimitBypassList = row.RateLimitBypassList
	return view
}

// TestSecurityConfigSave_ModeToKeyMapping describes the mapping table: block and monitor write the WAF key true,
// disabled writes false, and an empty mode deletes the key. Rate limit enable true or false writes the key,
// a mode-only save maps enabled to true, any other non-empty value to false, and an empty value deletes the key.
func TestSecurityConfigSave_ModeToKeyMapping(t *testing.T) {
	h, db := newStatusApplyFixture(t, models.SecurityConfig{Enabled: false, WAFMode: "disabled", RateLimitMode: "disabled"}, noopCaddyClient{})
	router := newModuleRouter(t, h)

	steps := []struct {
		name string
		body string
		key  string
		want string // "" means the key must be absent afterwards
	}{
		{"waf block writes true", `{"waf_mode":"block"}`, models.SecurityWAFEnabledSettingKey, "true"},
		{"waf monitor writes true", `{"waf_mode":"monitor"}`, models.SecurityWAFEnabledSettingKey, "true"},
		{"waf disabled writes false", `{"waf_mode":"disabled"}`, models.SecurityWAFEnabledSettingKey, "false"},
		{"waf empty deletes the key", `{"waf_mode":""}`, models.SecurityWAFEnabledSettingKey, ""},
		{"rate limit enable true writes true", `{"rate_limit_enable":true}`, models.SecurityRateLimitEnabledSettingKey, "true"},
		{"rate limit enable false writes false", `{"rate_limit_enable":false}`, models.SecurityRateLimitEnabledSettingKey, "false"},
		{"rate limit mode enabled writes true", `{"rate_limit_mode":"enabled"}`, models.SecurityRateLimitEnabledSettingKey, "true"},
		{"rate limit mode disabled writes false", `{"rate_limit_mode":"disabled"}`, models.SecurityRateLimitEnabledSettingKey, "false"},
		{"rate limit mode empty deletes the key", `{"rate_limit_mode":""}`, models.SecurityRateLimitEnabledSettingKey, ""},
	}
	for _, step := range steps {
		w := serveSecurityPin(t, router, http.MethodPost, "/security/config", "127.0.0.1:5000", step.body)
		require.Equal(t, http.StatusOK, w.Code, "%s: %s", step.name, w.Body.String())
		value, found := settingValue(t, db, step.key)
		if step.want == "" {
			assert.False(t, found, "%s: key %s should be deleted", step.name, step.key)
			continue
		}
		require.True(t, found, "%s: key %s should exist", step.name, step.key)
		assert.Equal(t, step.want, value, step.name)
	}
}

// TestSecurityConfigSave_EchoLeavesModulesUnchanged describes the echo save: reading the full row, editing one
// unrelated field and posting the row back leaves every module key and mode as it was.
func TestSecurityConfigSave_EchoLeavesModulesUnchanged(t *testing.T) {
	h, db := newStatusApplyFixture(t, models.SecurityConfig{
		Enabled:            true,
		WAFMode:            "block",
		RateLimitMode:      "enabled",
		RateLimitEnable:    true,
		RateLimitRequests:  100,
		RateLimitWindowSec: 60,
		RateLimitBurst:     20,
	}, noopCaddyClient{})
	// The modules were switched off from the UI: the keys say false while the row still holds the modes.
	setSettingValue(t, db, models.SecurityWAFEnabledSettingKey, "false")
	setSettingValue(t, db, models.SecurityRateLimitEnabledSettingKey, "false")
	router := newModuleRouter(t, h)

	get := serveSecurityPin(t, router, http.MethodGet, "/security/config", "127.0.0.1:5000", "")
	require.Equal(t, http.StatusOK, get.Code)
	var body struct {
		Config map[string]any `json:"config"`
	}
	require.NoError(t, json.Unmarshal(get.Body.Bytes(), &body))
	body.Config["crowdsec_api_url"] = "http://crowdsec.example:9090"
	echo, err := json.Marshal(body.Config)
	require.NoError(t, err)

	before := captureModuleStateView(t, db)
	w := serveSecurityPin(t, router, http.MethodPost, "/security/config", "127.0.0.1:5000", string(echo))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	after := captureModuleStateView(t, db)

	assert.Equal(t, "false", after.WAFKey, "an echo must not turn the WAF on")
	assert.Equal(t, "false", after.RateLimitKey, "an echo must not turn the rate limit on")
	assert.Equal(t, before, after, "an echo save changes no module state")
}

// TestSecurityConfigSave_EnableStartsFramework describes a key-on write also enabling the security framework,
// unless the same request turns the module off.
func TestSecurityConfigSave_EnableStartsFramework(t *testing.T) {
	t.Run("key on starts the framework", func(t *testing.T) {
		h, db := newStatusApplyFixture(t, models.SecurityConfig{Enabled: false, WAFMode: "disabled"}, noopCaddyClient{})
		w := serveSecurityPin(t, newModuleRouter(t, h), http.MethodPost, "/security/config", "127.0.0.1:5000", `{"waf_mode":"block"}`)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())

		assert.True(t, loadDefaultSecurityRow(t, db).Enabled)
		cerberus, _ := settingValue(t, db, models.CerberusFeatureSettingKey)
		assert.Equal(t, "true", cerberus)
	})

	t.Run("enabled false in the same request keeps the framework off", func(t *testing.T) {
		h, db := newStatusApplyFixture(t, models.SecurityConfig{Enabled: false, WAFMode: "disabled"}, noopCaddyClient{})
		w := serveSecurityPin(t, newModuleRouter(t, h), http.MethodPost, "/security/config", "127.0.0.1:5000", `{"waf_mode":"block","enabled":false}`)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())

		assert.False(t, loadDefaultSecurityRow(t, db).Enabled)
		value, found := settingValue(t, db, models.SecurityWAFEnabledSettingKey)
		assert.True(t, found)
		assert.Equal(t, "true", value, "the WAF key is still written")
		_, cerberusFound := settingValue(t, db, models.CerberusFeatureSettingKey)
		assert.False(t, cerberusFound, "the framework switch is not written")
	})
}

// TestSecurityConfigSave_WafRulesSourceValidation describes the rules source cases: empty, an existing ruleset
// name, an http(s) URL, the stored value on an unrelated save (accepted), and an unknown name (422
// waf_rules_source_unknown).
func TestSecurityConfigSave_WafRulesSourceValidation(t *testing.T) {
	h, db := newStatusApplyFixture(t, models.SecurityConfig{Enabled: true, WAFMode: "block"}, noopCaddyClient{})
	router := newModuleRouter(t, h)

	accepted := []string{
		`{"waf_rules_source":"owasp-crs"}`,
		`{"waf_rules_source":"https://rules.example.com/crs.zip"}`,
		`{"waf_rules_source":""}`,
	}
	for _, body := range accepted {
		w := serveSecurityPin(t, router, http.MethodPost, "/security/config", "127.0.0.1:5000", body)
		require.Equal(t, http.StatusOK, w.Code, "%s: %s", body, w.Body.String())
	}

	w := serveSecurityPin(t, router, http.MethodPost, "/security/config", "127.0.0.1:5000", `{"waf_rules_source":"missing-set"}`)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
	assert.Equal(t, codeWAFRulesSourceUnknown, errorCodeOf(t, w))
	assert.NotEqual(t, "missing-set", loadDefaultSecurityRow(t, db).WAFRulesSource)

	// A value stored before this check existed is kept by an unrelated save and by a save that repeats it.
	require.NoError(t, db.Model(&models.SecurityConfig{}).Where("name = ?", models.DefaultSecurityConfigName).Update("waf_rules_source", "legacy-set").Error)
	w = serveSecurityPin(t, router, http.MethodPost, "/security/config", "127.0.0.1:5000", `{"waf_paranoia_level":2}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = serveSecurityPin(t, router, http.MethodPost, "/security/config", "127.0.0.1:5000", `{"waf_rules_source":"legacy-set"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

// errorCodeOf returns the error_code of a coded response body.
func errorCodeOf(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	code, _ := body["error_code"].(string)
	return code
}

// TestSecurityConfigSave_WafModeAndKeyValueValidation describes validation: waf_mode outside the allowed set,
// and non-boolean values for the module keys, answer 400.
func TestSecurityConfigSave_WafModeAndKeyValueValidation(t *testing.T) {
	h, db := newStatusApplyFixture(t, models.SecurityConfig{Enabled: true, WAFMode: "block"}, noopCaddyClient{})
	before := captureModuleStateView(t, db)

	w := serveSecurityPin(t, newModuleRouter(t, h), http.MethodPost, "/security/config", "127.0.0.1:5000", `{"waf_mode":"foo"}`)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Equal(t, codeInvalidWAFMode, errorCodeOf(t, w))
	assert.Equal(t, before, captureModuleStateView(t, db), "a refused save writes nothing")

	settings := newModuleSettingsRouter(t, db, h.caddyManager)
	for _, body := range []string{
		`{"key":"security.waf.enabled","value":"yes"}`,
		`{"key":"security.rate_limit.enabled","value":""}`,
	} {
		w = serveSecurityPin(t, settings, http.MethodPost, "/settings", "127.0.0.1:5000", body)
		require.Equal(t, http.StatusBadRequest, w.Code, "%s: %s", body, w.Body.String())
	}
	w = serveSecurityPin(t, settings, http.MethodPatch, "/config", "127.0.0.1:5000", `{"security.waf.enabled":"maybe"}`)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

	assert.Equal(t, before, captureModuleStateView(t, db), "refused module values write nothing")
}

// TestRuleSetDelete_InUseReturnsConflict describes deleting the ruleset named by the rules source: 409
// waf_ruleset_in_use, checked in the same write-locked transaction as the delete.
func TestRuleSetDelete_InUseReturnsConflict(t *testing.T) {
	h, db := newStatusApplyFixture(t, models.SecurityConfig{Enabled: true, WAFMode: "block", WAFRulesSource: "owasp-crs"}, noopCaddyClient{})
	router := newModuleRouter(t, h)

	w := serveSecurityPin(t, router, http.MethodDelete, "/security/rulesets/"+ruleSetUUID, "127.0.0.1:5000", "")
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Equal(t, codeWAFRulesetInUse, errorCodeOf(t, w))
	var kept int64
	require.NoError(t, db.Model(&models.SecurityRuleSet{}).Where("uuid = ?", ruleSetUUID).Count(&kept).Error)
	assert.Equal(t, int64(1), kept, "the ruleset in use is not deleted")

	// Once the rules source names another value, the same ruleset deletes by uuid.
	require.NoError(t, db.Model(&models.SecurityConfig{}).Where("name = ?", models.DefaultSecurityConfigName).Update("waf_rules_source", "").Error)
	w = serveSecurityPin(t, router, http.MethodDelete, "/security/rulesets/"+ruleSetUUID, "127.0.0.1:5000", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// The numeric id keeps working, and an unknown uuid answers 404.
	extra := models.SecurityRuleSet{UUID: "numeric-ruleset", Name: "extra", Content: "SecRule ARGS \"@contains y\" \"id:2,deny\""}
	require.NoError(t, db.Create(&extra).Error)
	w = serveSecurityPin(t, router, http.MethodDelete, "/security/rulesets/"+strconv.FormatUint(uint64(extra.ID), 10), "127.0.0.1:5000", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = serveSecurityPin(t, router, http.MethodDelete, "/security/rulesets/00000000-0000-4000-8000-000000000000", "127.0.0.1:5000", "")
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

// Start-up reconciliation: a WAF key that is on with a disabled or empty row moves the row to monitor mode once.

// TestSecurityConfigReconcile_KeyOnRowDisabledBecomesMonitor describes the start-up step that sets the row to
// monitor and reports the count of changes (the caller logs one Info line with it).
func TestSecurityConfigReconcile_KeyOnRowDisabledBecomesMonitor(t *testing.T) {
	cases := []struct {
		name    string
		mode    string
		key     string // "" leaves the key absent
		wantRow string
		changed int64
	}{
		{"key on, row disabled", "disabled", "true", "monitor", 1},
		{"key on, row empty", "", "true", "monitor", 1},
		{"key on, row already blocking", "block", "true", "block", 0},
		{"key off, row disabled", "disabled", "false", "disabled", 0},
		{"key absent, row disabled", "disabled", "", "disabled", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			db := OpenTestDB(t)
			require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}))
			require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, WAFMode: c.mode}).Error)
			if c.key != "" {
				setSettingValue(t, db, models.SecurityWAFEnabledSettingKey, c.key)
			}

			changed, err := models.ReconcileWAFDetectionMode(db)
			require.NoError(t, err)
			assert.Equal(t, c.changed, changed)
			assert.Equal(t, c.wantRow, loadDefaultSecurityRow(t, db).WAFMode)
		})
	}
}

// TestSecurityConfigReconcile_IsIdempotent describes running the start-up step twice: the second run changes nothing.
func TestSecurityConfigReconcile_IsIdempotent(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}))
	require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, WAFMode: "disabled"}).Error)
	setSettingValue(t, db, models.SecurityWAFEnabledSettingKey, "true")

	first, err := models.ReconcileWAFDetectionMode(db)
	require.NoError(t, err)
	assert.Equal(t, int64(1), first)
	second, err := models.ReconcileWAFDetectionMode(db)
	require.NoError(t, err)
	assert.Zero(t, second)
	assert.Equal(t, "monitor", loadDefaultSecurityRow(t, db).WAFMode)
}

// Regression sequences from the module state (reset, then save or toggle).

// resetModuleState writes the state the emergency reset leaves: both module keys false and the default row disabled.
func resetModuleState(t *testing.T, db *gorm.DB) {
	t.Helper()
	setSettingValue(t, db, models.SecurityWAFEnabledSettingKey, "false")
	setSettingValue(t, db, models.SecurityRateLimitEnabledSettingKey, "false")
	require.NoError(t, db.Model(&models.SecurityConfig{}).Where("name = ?", models.DefaultSecurityConfigName).
		Updates(map[string]any{"enabled": false, "waf_mode": "disabled", "rate_limit_mode": "disabled", "rate_limit_enable": false}).Error)
}

// TestSecurityModuleKey_ResetThenBlockSaveBuildsHandler describes the sequence: emergency reset, then a config save
// with enabled true and mode block. The status reports the WAF on and the WAF handler is built.
func TestSecurityModuleKey_ResetThenBlockSaveBuildsHandler(t *testing.T) {
	h, db := newStatusApplyFixture(t, models.SecurityConfig{Enabled: true, WAFMode: "block"}, noopCaddyClient{})
	resetModuleState(t, db)
	mustApplyConfig(t, h)

	w := serveSecurityPin(t, newModuleRouter(t, h), http.MethodPost, "/security/config", "127.0.0.1:5000", `{"enabled":true,"waf_mode":"block"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	waf := getSecurityStatus(t, h)["waf"].(map[string]any)
	assert.Equal(t, true, waf["enabled"])
	assert.Equal(t, true, waf["enforced"])
}

// TestSecurityModuleKey_ToggleAfterResetBuildsWAFHandler describes the sequence: emergency reset, then the WAF toggle
// on. The status reports the WAF on and the WAF handler is built.
func TestSecurityModuleKey_ToggleAfterResetBuildsWAFHandler(t *testing.T) {
	h, db := newStatusApplyFixture(t, models.SecurityConfig{Enabled: true, WAFMode: "block"}, noopCaddyClient{})
	resetModuleState(t, db)
	mustApplyConfig(t, h)

	w := serveSecurityPin(t, newModuleRouter(t, h), http.MethodPost, "/security/waf/enable", "127.0.0.1:5000", `{}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	assert.Equal(t, "block", loadDefaultSecurityRow(t, db).WAFMode, "the toggle mirrors the WAF into the row")
	waf := getSecurityStatus(t, h)["waf"].(map[string]any)
	assert.Equal(t, true, waf["enabled"])
	assert.Equal(t, true, waf["enforced"])
}

// TestSecurityModuleKey_ToggleAfterResetRateLimitHalf describes the rate-limit half of the toggle sequence: after the
// reset, the rate-limit toggle on with seeded zero thresholds writes the visible thresholds and builds the handler.
func TestSecurityModuleKey_ToggleAfterResetRateLimitHalf(t *testing.T) {
	t.Skip("enabled in commit 5 of this PR: unset rate limit values are filled on enable")
}

// Writer contracts: every writer that changes the module keys or the row keeps both stores consistent.

// TestSettingsPatchConfig_RunsUnderWriteLock describes the bulk settings patch running under the shared write lock:
// the first statement on the settings table is the write-lock statement, before any settings row is written.
func TestSettingsPatchConfig_RunsUnderWriteLock(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}))
	recorder := &sqlRecorder{}
	db.Logger = recorder

	router := newModuleSettingsRouter(t, db, nil)
	w := serveSecurityPin(t, router, http.MethodPatch, "/config", "127.0.0.1:5000", `{"security.waf.enabled":"true"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	first := recorder.firstStatementOn("settings")
	require.NotEmpty(t, first)
	assert.Contains(t, first, "1 = 0", "the first settings statement takes the write lock")
}

// sqlRecorder is a gorm logger that keeps every executed statement, in order.
type sqlRecorder struct {
	mu   sync.Mutex
	sqls []string
}

// LogMode returns the recorder itself; the level does not change what is recorded.
func (r *sqlRecorder) LogMode(gormlogger.LogLevel) gormlogger.Interface { return r }

// Info does nothing: only statements are recorded.
func (r *sqlRecorder) Info(context.Context, string, ...any) {}

// Warn does nothing: only statements are recorded.
func (r *sqlRecorder) Warn(context.Context, string, ...any) {}

// Error does nothing: only statements are recorded.
func (r *sqlRecorder) Error(context.Context, string, ...any) {}

// Trace records the statement that ran.
func (r *sqlRecorder) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	sql, _ := fc()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sqls = append(r.sqls, sql)
}

// firstStatementOn returns the first recorded statement that names the table, or "" when none did.
func (r *sqlRecorder) firstStatementOn(table string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, sql := range r.sqls {
		if strings.Contains(sql, "`"+table+"`") {
			return sql
		}
	}
	return ""
}

// TestSingleConnectionPool_ModuleWritersDoNotDeadlock describes the toggle, settings, config save and patch writers
// running against a pool with one connection: each completes without waiting on itself.
func TestSingleConnectionPool_ModuleWritersDoNotDeadlock(t *testing.T) {
	h, db := newStatusApplyFixture(t, models.SecurityConfig{Enabled: false, WAFMode: "disabled"}, noopCaddyClient{})
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	module := newModuleRouter(t, h)
	settings := newModuleSettingsRouter(t, db, h.caddyManager)
	writers := []struct {
		name   string
		router *gin.Engine
		method string
		path   string
		body   string
	}{
		{"toggle", module, http.MethodPost, "/security/waf/enable", `{}`},
		{"settings", settings, http.MethodPost, "/settings", `{"key":"security.rate_limit.enabled","value":"false"}`},
		{"config save", module, http.MethodPost, "/security/config", `{"waf_mode":"monitor"}`},
		{"patch", settings, http.MethodPatch, "/config", `{"security.waf.enabled":"false"}`},
	}
	for _, writer := range writers {
		done := make(chan *httptest.ResponseRecorder, 1)
		go func() {
			done <- serveSecurityPin(t, writer.router, writer.method, writer.path, "127.0.0.1:5000", writer.body)
		}()
		select {
		case w := <-done:
			require.Equal(t, http.StatusOK, w.Code, "%s: %s", writer.name, w.Body.String())
		case <-time.After(10 * time.Second):
			t.Fatalf("%s writer waited on the single connection it already holds", writer.name)
		}
	}
}

// TestModuleWriters_RestorePreImageOnApplyFailure describes each writer (settings update, settings patch, module
// toggle, config save) restoring the stored row columns and module keys to their pre-image and answering 500
// when the proxy apply fails.
func TestModuleWriters_RestorePreImageOnApplyFailure(t *testing.T) {
	writers := []struct {
		name        string
		method      string
		path        string
		body        string
		viaSettings bool
	}{
		{"toggle", http.MethodPost, "/security/waf/enable", `{}`, false},
		{"settings update", http.MethodPost, "/settings", `{"key":"security.waf.enabled","value":"true"}`, true},
		{"settings patch", http.MethodPatch, "/config", `{"security.waf.enabled":"true","security.rate_limit.enabled":"true"}`, true},
		{"config save", http.MethodPost, "/security/config", `{"enabled":true,"waf_mode":"block","rate_limit_enable":true,"admin_whitelist":"203.0.113.0/24"}`, false},
	}
	for _, writer := range writers {
		t.Run(writer.name, func(t *testing.T) {
			client := &switchableLoadClient{}
			h, db := newStatusApplyFixture(t, models.SecurityConfig{Enabled: false, WAFMode: "disabled", AdminWhitelist: "192.0.2.0/24"}, client)
			before := captureModuleStateView(t, db)

			client.loadErr = errors.New("caddy refused the config")
			router := newModuleRouter(t, h)
			if writer.viaSettings {
				router = newModuleSettingsRouter(t, db, h.caddyManager)
			}
			w := serveSecurityPin(t, router, writer.method, writer.path, "127.0.0.1:5000", writer.body)
			require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())

			assert.Equal(t, before, captureModuleStateView(t, db), "the pre-image is restored after the failed apply")
		})
	}
}

// TestModuleWriters_EchoAndKeyContractSweep describes one table over the writers of the module keys and the row
// module columns, asserting the echo rule and the key contract for each.
func TestModuleWriters_EchoAndKeyContractSweep(t *testing.T) {
	const (
		wafKey = models.SecurityWAFEnabledSettingKey
		rlKey  = models.SecurityRateLimitEnabledSettingKey
	)
	type writerCase struct {
		name    string
		run     func(t *testing.T, h *SecurityHandler, db *gorm.DB)
		wantWAF string // "" means absent
		wantRL  string
	}
	post := func(path, body string) func(t *testing.T, h *SecurityHandler, db *gorm.DB) {
		return func(t *testing.T, h *SecurityHandler, _ *gorm.DB) {
			w := serveSecurityPin(t, newModuleRouter(t, h), http.MethodPost, path, "127.0.0.1:5000", body)
			require.Equal(t, http.StatusOK, w.Code, "%s %s: %s", path, body, w.Body.String())
		}
	}
	settingsWrite := func(key, value string) func(t *testing.T, h *SecurityHandler, db *gorm.DB) {
		return func(t *testing.T, h *SecurityHandler, db *gorm.DB) {
			router := newModuleSettingsRouter(t, db, h.caddyManager)
			w := serveSecurityPin(t, router, http.MethodPost, "/settings", "127.0.0.1:5000", `{"key":"`+key+`","value":"`+value+`"}`)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		}
	}
	patchWrite := func(body string) func(t *testing.T, h *SecurityHandler, db *gorm.DB) {
		return func(t *testing.T, h *SecurityHandler, db *gorm.DB) {
			router := newModuleSettingsRouter(t, db, h.caddyManager)
			w := serveSecurityPin(t, router, http.MethodPatch, "/config", "127.0.0.1:5000", body)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		}
	}
	echoSave := func(t *testing.T, h *SecurityHandler, _ *gorm.DB) {
		router := newModuleRouter(t, h)
		get := serveSecurityPin(t, router, http.MethodGet, "/security/config", "127.0.0.1:5000", "")
		require.Equal(t, http.StatusOK, get.Code)
		var body struct {
			Config map[string]any `json:"config"`
		}
		require.NoError(t, json.Unmarshal(get.Body.Bytes(), &body))
		echo, err := json.Marshal(body.Config)
		require.NoError(t, err)
		w := serveSecurityPin(t, router, http.MethodPost, "/security/config", "127.0.0.1:5000", string(echo))
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	}

	// The base state is WAF key "true" and rate-limit key "false"; a case states the value each key holds afterwards.
	cases := []writerCase{
		{"toggle waf on", post("/security/waf/enable", `{}`), "true", "false"},
		{"toggle waf off", post("/security/waf/disable", `{}`), "false", "false"},
		{"toggle rate limit on", post("/security/rate-limit/enable", `{}`), "true", "true"},
		{"toggle rate limit off", post("/security/rate-limit/disable", `{}`), "true", "false"},
		{"settings waf on", settingsWrite(wafKey, "true"), "true", "false"},
		{"settings waf off", settingsWrite(wafKey, "false"), "false", "false"},
		{"settings rate limit on", settingsWrite(rlKey, "true"), "true", "true"},
		{"patch waf on", patchWrite(`{"security.waf.enabled":"true"}`), "true", "false"},
		{"patch rate limit off", patchWrite(`{"security.rate_limit.enabled":"false"}`), "true", "false"},
		{"config save block keeps the stored block", post("/security/config", `{"waf_mode":"block"}`), "true", "false"},
		{"config save disabled", post("/security/config", `{"waf_mode":"disabled"}`), "false", "false"},
		{"config save empty deletes the waf key", post("/security/config", `{"waf_mode":""}`), "", "false"},
		{"config save rate limit enable", post("/security/config", `{"rate_limit_enable":true}`), "true", "true"},
		{"config save rate limit mode equal to stored", post("/security/config", `{"rate_limit_mode":"disabled"}`), "true", "false"},
		{"echo save keeps keys", echoSave, "true", "false"},
		{"break-glass keeps keys", post("/security/break-glass", `{}`), "true", "false"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h, db := newStatusApplyFixture(t, models.SecurityConfig{Enabled: true, WAFMode: "block", RateLimitMode: "disabled"}, noopCaddyClient{})
			setSettingValue(t, db, wafKey, "true")
			setSettingValue(t, db, rlKey, "false")
			c.run(t, h, db)

			view := captureModuleStateView(t, db)
			assertKeyState(t, view.WAFKey, c.wantWAF, "waf key")
			assertKeyState(t, view.RateLimitKey, c.wantRL, "rate limit key")
		})
	}

	t.Run("emergency reset leaves the keys and the row in the same off state", func(t *testing.T) {
		db := setupEmergencyTestDB(t)
		handler := NewEmergencyHandler(db)
		t.Cleanup(handler.Close)
		router := setupEmergencyRouter(handler)
		token := "this-is-a-valid-emergency-token-with-32-chars-minimum"
		require.NoError(t, os.Setenv(EmergencyTokenEnvVar, token))
		t.Cleanup(func() { _ = os.Unsetenv(EmergencyTokenEnvVar) })
		require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, Enabled: true, WAFMode: "block", RateLimitMode: "enabled", RateLimitEnable: true}).Error)
		setSettingValue(t, db, wafKey, "true")
		setSettingValue(t, db, rlKey, "true")

		req := httptest.NewRequest(http.MethodPost, "/api/v1/emergency/security-reset", http.NoBody)
		req.Header.Set(EmergencyTokenHeader, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())

		view := captureModuleStateView(t, db)
		assert.Equal(t, "false", view.WAFKey)
		assert.Equal(t, "false", view.RateLimitKey)
		assert.Equal(t, "disabled", view.WAFMode, "the row says off, as the keys do")
		assert.Equal(t, "disabled", view.RateLimitMode)
	})

	t.Run("seed keeps the keys", func(t *testing.T) {
		db := OpenTestDB(t)
		require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}))
		setSettingValue(t, db, wafKey, "true")
		_, err := models.SeedDefaultSecurityConfig(db)
		require.NoError(t, err)
		value, _ := settingValue(t, db, wafKey)
		assert.Equal(t, "true", value)
	})
}

// assertKeyState checks a module key: want "" means absent, otherwise the stored value.
func assertKeyState(t *testing.T, got, want, label string) {
	t.Helper()
	if want == "" {
		assert.Empty(t, got, "%s should be absent", label)
		return
	}
	assert.Equal(t, want, got, label)
}

// Rate limit thresholds: enabling fills only the missing values and reports unset values.

// TestRateLimitEnable_FillsMissingThresholds describes enabling the rate limit on a row with zero thresholds: the
// standard values 100 requests, 60 seconds and burst 20 are written in the same transaction.
func TestRateLimitEnable_FillsMissingThresholds(t *testing.T) {
	t.Skip("enabled in commit 5 of this PR: unset rate limit values are filled on enable")
}

// TestRateLimitEnable_KeepsSuppliedThresholds describes enabling with requests 50 and window 0: 50 is kept and the
// window is filled with 60.
func TestRateLimitEnable_KeepsSuppliedThresholds(t *testing.T) {
	t.Skip("enabled in commit 5 of this PR: unset rate limit values are filled on enable")
}

// TestRateLimitDisable_WritesNothing describes disabling the rate limit: no threshold is written.
func TestRateLimitDisable_WritesNothing(t *testing.T) {
	t.Skip("enabled in commit 5 of this PR: unset rate limit values are filled on enable")
}

// TestRateLimitAlreadyEnabledZeroes_ReportsUnset describes a row that is already enabled with zero thresholds: no
// write happens at apply time and the status reports thresholds_unset.
func TestRateLimitAlreadyEnabledZeroes_ReportsUnset(t *testing.T) {
	t.Skip("enabled in commit 5 of this PR: unset rate limit values are filled on enable")
}
