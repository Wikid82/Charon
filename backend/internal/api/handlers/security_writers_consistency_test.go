package handlers

import (
	"net/http"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// wafExclusionsJSON is a stored exclusion list with one entry, as the exclusion writers keep it.
const wafExclusionsJSON = `[{"rule_id":942100,"description":"kept"}]`

// newApplyingSecurityPinRouter is newSecurityPinRouter with a proxy applier wired, so a refused apply can be simulated.
func newApplyingSecurityPinRouter(t *testing.T, applier CaddyConfigManager) (*gin.Engine, *gorm.DB) {
	t.Helper()
	h, router, db := newSecurityPinRouter(t)
	h.caddyManager = applier
	return router, db
}

// seedBreakGlassRow stores a default row whose break-glass hash matches token, with an allow-list that excludes the
// address the tests call from.
func seedBreakGlassRow(t *testing.T, db *gorm.DB, token string, enabled bool) models.SecurityConfig {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.MinCost)
	require.NoError(t, err)
	row := models.SecurityConfig{
		Name:           models.DefaultSecurityConfigName,
		Enabled:        enabled,
		AdminWhitelist: "203.0.113.0/24",
		BreakGlassHash: string(hash),
	}
	require.NoError(t, db.Create(&row).Error)
	return loadDefaultSecurityRow(t, db)
}

// TestSecurityWriters_EnableChecksAnswerTheirStatus describes the enable check on the stored row: a missing allow-list
// is a 400, a wrong break-glass token a 401, an address outside the allow-list a 403, and a token or an allow-listed
// address enables Cerberus.
func TestSecurityWriters_EnableChecksAnswerTheirStatus(t *testing.T) {
	tests := []struct {
		name       string
		whitelist  string
		token      string
		remoteAddr string
		body       string
		status     int
		enabled    bool
	}{
		{name: "no allow-list and no token", whitelist: "", remoteAddr: "192.0.2.10:5000", body: `{}`, status: http.StatusBadRequest},
		{name: "address outside the allow-list", whitelist: "203.0.113.0/24", remoteAddr: "192.0.2.10:5000", body: `{}`, status: http.StatusForbidden},
		{name: "address inside the allow-list range", whitelist: "192.0.2.0/24", remoteAddr: "192.0.2.10:5000", body: `{}`, status: http.StatusOK, enabled: true},
		{name: "wrong break-glass token", whitelist: "203.0.113.0/24", remoteAddr: "192.0.2.10:5000", body: `{"break_glass_token":"nope"}`, status: http.StatusUnauthorized},
		{name: "matching break-glass token", whitelist: "203.0.113.0/24", remoteAddr: "192.0.2.10:5000", body: `{"break_glass_token":"open-sesame"}`, status: http.StatusOK, enabled: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, db := newApplyingSecurityPinRouter(t, nil)
			row := seedBreakGlassRow(t, db, "open-sesame", false)
			if tt.whitelist != row.AdminWhitelist {
				row.AdminWhitelist = tt.whitelist
				require.NoError(t, db.Model(&models.SecurityConfig{}).Where("name = ?", models.DefaultSecurityConfigName).Update("admin_whitelist", tt.whitelist).Error)
			}

			w := serveSecurityPin(t, router, http.MethodPost, "/security/enable", tt.remoteAddr, tt.body)

			require.Equal(t, tt.status, w.Code, w.Body.String())
			assert.Equal(t, tt.enabled, loadDefaultSecurityRow(t, db).Enabled)
		})
	}
}

// TestSecurityWriters_EnableRefusedApplyRestoresTheRow describes a refused proxy apply after an enable: the answer is
// the generic apply failure and the stored row is back to its state before the enable.
func TestSecurityWriters_EnableRefusedApplyRestoresTheRow(t *testing.T) {
	applier := &recordingApplier{err: assert.AnError}
	router, db := newApplyingSecurityPinRouter(t, applier)
	before := seedFullSecurityRow(t, db, false)

	w := serveSecurityPin(t, router, http.MethodPost, "/security/enable", "192.0.2.10:5000", `{}`)

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), msgModuleApplyFailed)
	assert.Equal(t, 1, applier.count())
	assertRowKeptExcept(t, before, loadDefaultSecurityRow(t, db))
}

// TestSecurityWriters_EnableOnAbsentRowStoresRateLimitOff describes a first enable on an instance with no stored row: the
// row is created with rate limiting recorded as off, the same value every other first write stores.
func TestSecurityWriters_EnableOnAbsentRowStoresRateLimitOff(t *testing.T) {
	router, db := newApplyingSecurityPinRouter(t, nil)

	w := serveSecurityPin(t, router, http.MethodPost, "/security/enable", "192.0.2.10:5000", `{}`)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	row := loadDefaultSecurityRow(t, db)
	assert.True(t, row.Enabled)
	assert.Equal(t, "disabled", row.RateLimitMode)
}

// TestSecurityWriters_ExclusionApplyFailureRestoresTheStoredList describes a refused proxy apply after an exclusion add
// or delete: the answer is the generic apply failure and the stored exclusion list is put back.
func TestSecurityWriters_ExclusionApplyFailureRestoresTheStoredList(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "add", method: http.MethodPost, path: "/security/waf/exclusions", body: `{"rule_id":941100}`},
		{name: "delete", method: http.MethodDelete, path: "/security/waf/exclusions/942100", body: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applier := &recordingApplier{err: assert.AnError}
			router, db := newApplyingSecurityPinRouter(t, applier)
			seedFullSecurityRow(t, db, true)
			require.NoError(t, db.Model(&models.SecurityConfig{}).Where("name = ?", models.DefaultSecurityConfigName).Update("waf_exclusions", wafExclusionsJSON).Error)
			before := loadDefaultSecurityRow(t, db)

			w := serveSecurityPin(t, router, tt.method, tt.path, "192.0.2.10:5000", tt.body)

			require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
			assert.Contains(t, w.Body.String(), msgModuleApplyFailed)
			assert.Equal(t, 1, applier.count())
			assertRowKeptExcept(t, before, loadDefaultSecurityRow(t, db))
		})
	}
}

// TestSecurityWriters_ExclusionAnswersDuplicateAndNotFound describes the refusals that come from the stored list: a
// duplicate add is a 409, and a delete with no match is a 404 that leaves the row alone.
func TestSecurityWriters_ExclusionAnswersDuplicateAndNotFound(t *testing.T) {
	router, db := newApplyingSecurityPinRouter(t, nil)
	seedFullSecurityRow(t, db, true)
	require.NoError(t, db.Model(&models.SecurityConfig{}).Where("name = ?", models.DefaultSecurityConfigName).Update("waf_exclusions", wafExclusionsJSON).Error)
	before := loadDefaultSecurityRow(t, db)

	duplicate := serveSecurityPin(t, router, http.MethodPost, "/security/waf/exclusions", "192.0.2.10:5000", `{"rule_id":942100}`)
	require.Equal(t, http.StatusConflict, duplicate.Code, duplicate.Body.String())

	missing := serveSecurityPin(t, router, http.MethodDelete, "/security/waf/exclusions/111111", "192.0.2.10:5000", "")
	require.Equal(t, http.StatusNotFound, missing.Code, missing.Body.String())

	assertRowKeptExcept(t, before, loadDefaultSecurityRow(t, db))
}

// TestSecurityWriters_UnreadableStoredExclusionsAnswer500 describes a delete when the stored list is not valid JSON: the
// delete refuses with the parse failure and writes nothing.
func TestSecurityWriters_UnreadableStoredExclusionsAnswer500(t *testing.T) {
	router, db := newApplyingSecurityPinRouter(t, nil)
	seedFullSecurityRow(t, db, true)
	require.NoError(t, db.Model(&models.SecurityConfig{}).Where("name = ?", models.DefaultSecurityConfigName).Update("waf_exclusions", "not-json").Error)
	before := loadDefaultSecurityRow(t, db)

	w := serveSecurityPin(t, router, http.MethodDelete, "/security/waf/exclusions/942100", "192.0.2.10:5000", "")

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "failed to parse exclusions")
	assertRowKeptExcept(t, before, loadDefaultSecurityRow(t, db))
}

// TestSecurityWriters_ConcurrentModeChangeSurvivesExclusionAdd describes two writers on different columns of the default
// row: a mode change made while an exclusion is added survives, and so does the exclusion.
func TestSecurityWriters_ConcurrentModeChangeSurvivesExclusionAdd(t *testing.T) {
	router, db := newApplyingSecurityPinRouter(t, nil)
	seedFullSecurityRow(t, db, true)

	// Production runs the pool on one connection; the test does the same, so the two writers queue on it.
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Another config writer takes the same write lock, so the two writes do not interleave on the connection.
		assert.NoError(t, services.WithWriteLock(db, &models.SecurityConfig{}, func(tx *gorm.DB) error {
			return tx.Model(&models.SecurityConfig{}).Where("name = ?", models.DefaultSecurityConfigName).Update("waf_mode", "monitor").Error
		}))
	}()
	w := serveSecurityPin(t, router, http.MethodPost, "/security/waf/exclusions", "192.0.2.10:5000", `{"rule_id":941100}`)
	wg.Wait()

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	after := loadDefaultSecurityRow(t, db)
	assert.Equal(t, "monitor", after.WAFMode, "the mode change is kept")
	assert.Contains(t, after.WAFExclusions, "941100", "the exclusion is kept")
}

// TestSecurityWriters_RuleSetApplyFailureAnswersTheGenericError describes a refused proxy apply after a rule set save:
// the answer is the generic apply failure, not the text of the apply error.
func TestSecurityWriters_RuleSetApplyFailureAnswersTheGenericError(t *testing.T) {
	h, router, db := newSecurityPinRouter(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityRuleSet{}))
	h.caddyManager = &recordingApplier{err: assert.AnError}
	router.POST("/security/rulesets", h.UpsertRuleSet)

	body := `{"name":"custom","content":"SecRule ARGS \"x\" \"id:1,deny\""}`
	w := serveSecurityPin(t, router, http.MethodPost, "/security/rulesets", "192.0.2.10:5000", body)

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), msgModuleApplyFailed)
	assert.NotContains(t, w.Body.String(), assert.AnError.Error())
}
