package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

// getJSONBody serves one GET request to router as role and decodes the JSON body into a generic map.
func getJSONBody(t *testing.T, router *gin.Engine, path string) map[string]any {
	t.Helper()
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, http.NoBody))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body
}

// TestGetConfig_AdminOnlyFieldsAreHiddenFromStandardUsers describes the security config read: an administrator sees the
// allow-list, the bypass list and the CrowdSec address; any other caller gets the flags in their place.
func TestGetConfig_AdminOnlyFieldsAreHiddenFromStandardUsers(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))
	require.NoError(t, db.Create(&models.SecurityConfig{ //nolint:gosec // G101: test fixture, not a credential
		Name: models.DefaultSecurityConfigName, AdminWhitelist: "203.0.113.0/24", RateLimitBypassList: "198.51.100.0/24",
		CrowdSecAPIURL: "http://crowdsec.example:8080", WAFMode: "block",
	}).Error)
	h := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	t.Cleanup(h.Close)

	admin := roleStampRouter("admin")
	admin.GET("/security/config", h.GetConfig)
	adminCfg := getJSONBody(t, admin, "/security/config")["config"].(map[string]any)
	assert.Equal(t, "203.0.113.0/24", adminCfg["admin_whitelist"])
	assert.Equal(t, "198.51.100.0/24", adminCfg["rate_limit_bypass_list"])
	assert.Equal(t, "http://crowdsec.example:8080", adminCfg["crowdsec_api_url"])
	assert.Equal(t, "block", adminCfg["waf_mode"])

	user := roleStampRouter("user")
	user.GET("/security/config", h.GetConfig)
	userCfg := getJSONBody(t, user, "/security/config")["config"].(map[string]any)
	for _, hidden := range []string{"admin_whitelist", "rate_limit_bypass_list", "crowdsec_api_url"} {
		assert.NotContains(t, userCfg, hidden)
	}
	assert.Equal(t, true, userCfg["has_admin_whitelist"])
	assert.Equal(t, true, userCfg["has_rate_limit_bypass_list"])
	assert.Equal(t, "block", userCfg["waf_mode"], "other fields are still answered")
}

// TestGetConfig_NoRowAnswersNullForEveryRole keeps the answer for a missing row unchanged.
func TestGetConfig_NoRowAnswersNullForEveryRole(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))
	h := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	t.Cleanup(h.Close)
	user := roleStampRouter("user")
	user.GET("/security/config", h.GetConfig)

	body := getJSONBody(t, user, "/security/config")

	assert.Contains(t, body, "config")
	assert.Nil(t, body["config"])
}

// TestListRuleSets_ContentAndSourceAreHiddenFromStandardUsers describes the rule set read: an administrator sees the rule
// text and the download address; any other caller sees the flags and the non-blank line count instead.
func TestListRuleSets_ContentAndSourceAreHiddenFromStandardUsers(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityRuleSet{}))
	require.NoError(t, db.Create(&models.SecurityRuleSet{
		Name: "custom", SourceURL: "https://rules.example.com/custom.conf",
		Content: "SecRule ARGS \"a\" \"id:1,deny\"\n\n   \nSecRule ARGS \"b\" \"id:2,deny\"\n",
	}).Error)
	h := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	t.Cleanup(h.Close)

	admin := roleStampRouter("admin")
	admin.GET("/security/rulesets", h.ListRuleSets)
	adminSet := getJSONBody(t, admin, "/security/rulesets")["rulesets"].([]any)[0].(map[string]any)
	assert.Contains(t, adminSet["content"], "id:2")
	assert.Equal(t, "https://rules.example.com/custom.conf", adminSet["source_url"])

	user := roleStampRouter("user")
	user.GET("/security/rulesets", h.ListRuleSets)
	userSet := getJSONBody(t, user, "/security/rulesets")["rulesets"].([]any)[0].(map[string]any)
	assert.NotContains(t, userSet, "content")
	assert.NotContains(t, userSet, "source_url")
	assert.Equal(t, true, userSet["has_content"])
	assert.Equal(t, float64(2), userSet["content_line_count"], "blank lines are not counted")
	assert.Equal(t, "custom", userSet["name"])
}

// TestListRuleSets_EmptyListStaysAnArrayForStandardUsers describes an empty list: the answer is [] and not null.
func TestListRuleSets_EmptyListStaysAnArrayForStandardUsers(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityRuleSet{}))
	h := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	t.Cleanup(h.Close)
	user := roleStampRouter("user")
	user.GET("/security/rulesets", h.ListRuleSets)

	body := getJSONBody(t, user, "/security/rulesets")

	assert.Equal(t, []any{}, body["rulesets"])
}

// TestGetSettings_AdminOnlyKeysAreHiddenFromStandardUsers describes the settings read: an administrator sees the allow-list
// and the mail server details; any other caller sees <key>.has_value in their place and every other key is unchanged.
func TestGetSettings_AdminOnlyKeysAreHiddenFromStandardUsers(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}))
	for _, s := range []models.Setting{
		{Key: "security.admin_whitelist", Value: "10.0.0.0/8", Category: "security", Type: "string"},
		{Key: "smtp_host", Value: "mail.example.com", Category: "smtp", Type: "string"},
		{Key: "smtp_port", Value: "587", Category: "smtp", Type: "string"},
		{Key: "smtp_username", Value: "", Category: "smtp", Type: "string"},
		{Key: "smtp_from_address", Value: "noreply@example.com", Category: "smtp", Type: "string"},
		{Key: "smtp_encryption", Value: "starttls", Category: "smtp", Type: "string"},
		{Key: "app.name", Value: "Charon", Category: "app", Type: "string"},
	} {
		require.NoError(t, db.Create(&s).Error)
	}
	h := NewSettingsHandler(db)

	admin := roleStampRouter("admin")
	admin.GET("/settings", h.GetSettings)
	adminBody := getJSONBody(t, admin, "/settings")
	assert.Equal(t, "mail.example.com", adminBody["smtp_host"])
	assert.Equal(t, "10.0.0.0/8", adminBody["security.admin_whitelist"])

	user := roleStampRouter("user")
	user.GET("/settings", h.GetSettings)
	userBody := getJSONBody(t, user, "/settings")
	for _, key := range adminOnlySettingKeys {
		assert.NotContains(t, userBody, key)
	}
	assert.Equal(t, true, userBody["smtp_host.has_value"])
	assert.Equal(t, true, userBody["smtp_port.has_value"])
	assert.Equal(t, false, userBody["smtp_username.has_value"], "a blank value has no value")
	assert.Equal(t, "Charon", userBody["app.name"])
}

// TestSettingsResponseMap_MasksSecretsForEveryRole describes the shared map: a sensitive key is masked whether or not the
// caller is an administrator.
func TestSettingsResponseMap_MasksSecretsForEveryRole(t *testing.T) {
	settings := []models.Setting{{Key: "smtp_password", Value: "hunter2"}}
	for _, admin := range []bool{true, false} {
		body := settingsResponseMap(settings, admin)
		assert.Equal(t, "********", body["smtp_password"])
		assert.Equal(t, true, body["smtp_password.has_secret"])
	}
}

// TestCountNonBlankLines describes the line count the WAF page shows for a rule set.
func TestCountNonBlankLines(t *testing.T) {
	assert.Equal(t, 0, countNonBlankLines(""))
	assert.Equal(t, 2, countNonBlankLines("a\n\n   \nb\n"))
	assert.Equal(t, 1, countNonBlankLines("only"))
}

// TestPatchConfig_ReplyMasksSecrets describes the bulk settings reply: a secret written by the save is answered masked,
// with the has_secret flag, as GET /settings answers it.
func TestPatchConfig_ReplyMasksSecrets(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}))
	h := NewSettingsHandler(db)
	admin := roleStampRouter("admin")
	admin.PATCH("/config", h.PatchConfig)

	w := serveJSON(t, admin, http.MethodPatch, "/config", `{"smtp_password":"hunter2","app":{"name":"Charon"}}`)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "********", body["smtp_password"])
	assert.Equal(t, true, body["smtp_password.has_secret"])
	assert.NotContains(t, w.Body.String(), "hunter2")
	assert.Equal(t, "Charon", body["app.name"])
}
