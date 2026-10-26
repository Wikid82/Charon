package handlers

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

// newSecurityPinRouter serves the security config save, the enable and disable endpoints and the WAF exclusion
// endpoints for an admin caller over a fresh database.
func newSecurityPinRouter(t *testing.T) (*SecurityHandler, *gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}, &models.SecurityAudit{}, &models.SecurityDecision{}))

	h := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	t.Cleanup(h.Close)

	router := gin.New()
	require.NoError(t, router.SetTrustedProxies(nil))
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", uint(1))
		c.Next()
	})
	router.POST("/security/config", h.UpdateConfig)
	router.GET("/security/status", h.GetStatus)
	router.POST("/security/enable", h.Enable)
	router.POST("/security/disable", h.Disable)
	router.POST("/security/waf/exclusions", h.AddWAFExclusion)
	router.DELETE("/security/waf/exclusions/:rule_id", h.DeleteWAFExclusion)
	return h, router, db
}

// seedFullSecurityRow stores the default row with every column set to a non-zero value, so a
// save that touches one column can be checked against all the others.
func seedFullSecurityRow(t *testing.T, db *gorm.DB, enabled bool) models.SecurityConfig {
	t.Helper()
	require.NoError(t, db.Create(&models.SecurityConfig{
		Name:                models.DefaultSecurityConfigName,
		Enabled:             enabled,
		AdminWhitelist:      "192.0.2.0/24",
		CrowdSecMode:        "local",
		CrowdSecAPIURL:      "http://crowdsec.example:8080",
		WAFMode:             "monitor",
		WAFRulesSource:      "owasp-crs",
		WAFLearning:         true,
		WAFParanoiaLevel:    2,
		WAFExclusions:       `[{"rule_id":942100},{"rule_id":913100,"target":"ARGS:q"}]`,
		RateLimitMode:       "enabled",
		RateLimitEnable:     true,
		RateLimitBurst:      4,
		RateLimitRequests:   50,
		RateLimitWindowSec:  7,
		RateLimitBypassList: "198.51.100.0/24",
	}).Error)
	return loadDefaultSecurityRow(t, db)
}

func loadDefaultSecurityRow(t *testing.T, db *gorm.DB) models.SecurityConfig {
	t.Helper()
	var row models.SecurityConfig
	require.NoError(t, db.Where("name = ?", models.DefaultSecurityConfigName).First(&row).Error)
	return row
}

// assertRowKeptExcept asserts that every column of after equals before, except the named fields and the
// update timestamp, which a save always bumps.
func assertRowKeptExcept(t *testing.T, before, after models.SecurityConfig, changedFields ...string) {
	t.Helper()
	changed := make(map[string]bool, len(changedFields))
	for _, name := range changedFields {
		changed[name] = true
	}
	b := reflect.ValueOf(before)
	a := reflect.ValueOf(after)
	typ := b.Type()
	for i := 0; i < typ.NumField(); i++ {
		name := typ.Field(i).Name
		if changed[name] || name == "UpdatedAt" {
			continue
		}
		assert.Equal(t, b.Field(i).Interface(), a.Field(i).Interface(), "column %s changed", name)
	}
}

func serveSecurityPin(t *testing.T, router *gin.Engine, method, path, remoteAddr, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = remoteAddr
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestSecurityConfigSave_EnableChangesOnlyEnabledColumn(t *testing.T) {
	_, router, db := newSecurityPinRouter(t)
	before := seedFullSecurityRow(t, db, false)

	// The caller's address is inside the stored allow-list, so the enable passes its check.
	w := serveSecurityPin(t, router, http.MethodPost, "/security/enable", "192.0.2.10:5000", `{}`)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	after := loadDefaultSecurityRow(t, db)
	assert.True(t, after.Enabled)
	assertRowKeptExcept(t, before, after, "Enabled")
}

func TestSecurityConfigSave_DisableFromLocalhostChangesOnlyEnabledColumn(t *testing.T) {
	_, router, db := newSecurityPinRouter(t)
	before := seedFullSecurityRow(t, db, true)

	w := serveSecurityPin(t, router, http.MethodPost, "/security/disable", "127.0.0.1:5000", `{}`)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	after := loadDefaultSecurityRow(t, db)
	assert.False(t, after.Enabled)
	assertRowKeptExcept(t, before, after, "Enabled")
}

func TestSecurityConfigSave_DisableWithBreakGlassTokenChangesOnlyEnabledColumn(t *testing.T) {
	h, router, db := newSecurityPinRouter(t)
	seedFullSecurityRow(t, db, true)
	token, err := h.svc.GenerateBreakGlassToken(models.DefaultSecurityConfigName)
	require.NoError(t, err)
	// Read the row after the token is stored, so the stored break-glass column is part of the baseline.
	before := loadDefaultSecurityRow(t, db)

	w := serveSecurityPin(t, router, http.MethodPost, "/security/disable", "198.51.100.7:5000", `{"break_glass_token":"`+token+`"}`)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	after := loadDefaultSecurityRow(t, db)
	assert.False(t, after.Enabled)
	assertRowKeptExcept(t, before, after, "Enabled")
}

func TestSecurityConfigSave_AddWAFExclusionChangesOnlyExclusions(t *testing.T) {
	_, router, db := newSecurityPinRouter(t)
	before := seedFullSecurityRow(t, db, true)

	w := serveSecurityPin(t, router, http.MethodPost, "/security/waf/exclusions", "192.0.2.10:5000", `{"rule_id":920350}`)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	after := loadDefaultSecurityRow(t, db)
	assert.Contains(t, after.WAFExclusions, "920350")
	assert.Contains(t, after.WAFExclusions, "942100", "existing exclusions are kept")
	assertRowKeptExcept(t, before, after, "WAFExclusions")
}

func TestSecurityConfigSave_DeleteWAFExclusionChangesOnlyExclusions(t *testing.T) {
	_, router, db := newSecurityPinRouter(t)
	before := seedFullSecurityRow(t, db, true)

	w := serveSecurityPin(t, router, http.MethodDelete, "/security/waf/exclusions/942100", "192.0.2.10:5000", "")

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	after := loadDefaultSecurityRow(t, db)
	assert.NotContains(t, after.WAFExclusions, "942100")
	assert.Contains(t, after.WAFExclusions, "913100", "other exclusions are kept")
	assertRowKeptExcept(t, before, after, "WAFExclusions")
}

func TestSecurityConfigSave_SyncAdminWhitelistChangesOnlyAllowList(t *testing.T) {
	_, _, db := newSecurityPinRouter(t)
	before := seedFullSecurityRow(t, db, true)
	settings := NewSettingsHandler(db)

	require.NoError(t, settings.syncAdminWhitelistWithDB(db, "203.0.113.0/24"))

	after := loadDefaultSecurityRow(t, db)
	assert.Equal(t, "203.0.113.0/24", after.AdminWhitelist)
	assertRowKeptExcept(t, before, after, "AdminWhitelist")
}

func TestSecurityConfigSave_FullRowPayloadStoresEveryKey(t *testing.T) {
	_, router, db := newSecurityPinRouter(t)
	before := seedFullSecurityRow(t, db, true)

	// Every key the endpoint accepts. crowdsec_mode differs from the stored value and must not be stored.
	body := `{"name":"default","enabled":false,"admin_whitelist":"203.0.113.0/24","crowdsec_mode":"disabled",` +
		`"crowdsec_api_url":"http://crowdsec.example:9090","waf_mode":"block","waf_rules_source":"custom-rules",` +
		`"waf_learning":false,"waf_paranoia_level":3,"waf_exclusions":"[]","rate_limit_mode":"enabled",` +
		`"rate_limit_enable":true,"rate_limit_burst":9,"rate_limit_requests":90,"rate_limit_window_sec":11,` +
		`"rate_limit_bypass_list":"198.51.100.7/32"}`

	w := serveSecurityPin(t, router, http.MethodPost, "/security/config", "192.0.2.10:5000", body)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"crowdsec_mode":"local"`, "the 200 body shows the stored mode")
	after := loadDefaultSecurityRow(t, db)
	assert.False(t, after.Enabled)
	assert.Equal(t, "203.0.113.0/24", after.AdminWhitelist)
	assert.Equal(t, "local", after.CrowdSecMode)
	assert.Equal(t, "http://crowdsec.example:9090", after.CrowdSecAPIURL)
	assert.Equal(t, "block", after.WAFMode)
	assert.Equal(t, "custom-rules", after.WAFRulesSource)
	assert.False(t, after.WAFLearning)
	assert.Equal(t, 3, after.WAFParanoiaLevel)
	assert.Equal(t, "[]", after.WAFExclusions)
	assert.Equal(t, "enabled", after.RateLimitMode)
	assert.True(t, after.RateLimitEnable)
	assert.Equal(t, 9, after.RateLimitBurst)
	assert.Equal(t, 90, after.RateLimitRequests)
	assert.Equal(t, 11, after.RateLimitWindowSec)
	assert.Equal(t, "198.51.100.7/32", after.RateLimitBypassList)
	assertRowKeptExcept(t, before, after, "Enabled", "AdminWhitelist", "CrowdSecAPIURL", "WAFMode",
		"WAFRulesSource", "WAFLearning", "WAFParanoiaLevel", "WAFExclusions", "RateLimitMode",
		"RateLimitEnable", "RateLimitBurst", "RateLimitRequests", "RateLimitWindowSec", "RateLimitBypassList")
}

func TestSecurityConfigSave_InvalidAllowListWritesNothing(t *testing.T) {
	_, router, db := newSecurityPinRouter(t)
	before := seedFullSecurityRow(t, db, true)

	w := serveSecurityPin(t, router, http.MethodPost, "/security/config", "192.0.2.10:5000",
		`{"name":"default","admin_whitelist":"not-a-cidr","waf_mode":"block"}`)

	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assertRowKeptExcept(t, before, loadDefaultSecurityRow(t, db))
}

func TestSecurityConfigSave_InvalidCrowdSecModeWritesNothing(t *testing.T) {
	_, router, db := newSecurityPinRouter(t)
	before := seedFullSecurityRow(t, db, true)

	w := serveSecurityPin(t, router, http.MethodPost, "/security/config", "192.0.2.10:5000",
		`{"name":"default","crowdsec_mode":"external","waf_mode":"block"}`)

	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "invalid crowdsec mode: external")
	assertRowKeptExcept(t, before, loadDefaultSecurityRow(t, db))
}
