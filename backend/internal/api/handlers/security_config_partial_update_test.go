package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

// A body that carries only the three rate-limit numbers changes only those columns.
func TestUpdateConfig_RateLimitNumbersKeepOtherColumns(t *testing.T) {
	_, router, db := newSecurityPinRouter(t)
	before := seedFullSecurityRow(t, db, true)
	statusBefore := serveSecurityStatus(t, router)

	w := serveSecurityPin(t, router, http.MethodPost, "/security/config", "192.0.2.10:5000",
		`{"rate_limit_requests":20,"rate_limit_burst":4,"rate_limit_window_sec":30}`)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	after := loadDefaultSecurityRow(t, db)
	assert.Equal(t, 20, after.RateLimitRequests)
	assert.Equal(t, 4, after.RateLimitBurst)
	assert.Equal(t, 30, after.RateLimitWindowSec)
	assertRowKeptExcept(t, before, after, "RateLimitRequests", "RateLimitBurst", "RateLimitWindowSec")
	assert.Equal(t, statusBefore, serveSecurityStatus(t, router), "the module status is unchanged")
}

// A body that carries only the name and the allow-list changes only the allow-list.
func TestUpdateConfig_AllowListOnlyKeepsOtherColumns(t *testing.T) {
	_, router, db := newSecurityPinRouter(t)
	before := seedFullSecurityRow(t, db, true)
	statusBefore := serveSecurityStatus(t, router)

	w := serveSecurityPin(t, router, http.MethodPost, "/security/config", "192.0.2.10:5000",
		`{"name":"default","admin_whitelist":"198.51.100.0/24"}`)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	after := loadDefaultSecurityRow(t, db)
	assert.Equal(t, "198.51.100.0/24", after.AdminWhitelist)
	assertRowKeptExcept(t, before, after, "AdminWhitelist")
	assert.Equal(t, statusBefore, serveSecurityStatus(t, router), "the module status is unchanged")
}

// An explicit empty value clears the stored allow-list, an explicit false is stored as false, and an explicit
// zero stores zero. Each is applied on its own.
func TestUpdateConfig_ExplicitZeroValuesAreApplied(t *testing.T) {
	cases := []struct {
		name string
		body string
		set  func(row *models.SecurityConfig)
	}{
		{"empty allow-list clears it", `{"admin_whitelist":""}`, func(r *models.SecurityConfig) { r.AdminWhitelist = "" }},
		{"false turns enabled off", `{"enabled":false}`, func(r *models.SecurityConfig) { r.Enabled = false }},
		{"zero stores zero", `{"rate_limit_requests":0}`, func(r *models.SecurityConfig) { r.RateLimitRequests = 0 }},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			_, router, db := newSecurityPinRouter(t)
			before := seedFullSecurityRow(t, db, true)

			w := serveSecurityPin(t, router, http.MethodPost, "/security/config", "192.0.2.10:5000", c.body)

			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			want := before
			c.set(&want)
			assertRowKeptExcept(t, want, loadDefaultSecurityRow(t, db))
		})
	}
}

// An absent key and a null key both keep the stored value.
func TestUpdateConfig_NullValuesKeepStoredValues(t *testing.T) {
	_, router, db := newSecurityPinRouter(t)
	before := seedFullSecurityRow(t, db, true)

	w := serveSecurityPin(t, router, http.MethodPost, "/security/config", "192.0.2.10:5000",
		`{"admin_whitelist":null,"enabled":null,"rate_limit_requests":null,"waf_mode":null}`)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assertRowKeptExcept(t, before, loadDefaultSecurityRow(t, db))
}

// A body with no updatable key changes no column and does not bump updated_at.
func TestUpdateConfig_BodyWithoutUpdatableKeyChangesNothing(t *testing.T) {
	for _, body := range []string{`{}`, `{"name":"default"}`, `{"crowdsec_mode":"disabled"}`} {
		body := body
		t.Run(body, func(t *testing.T) {
			_, router, db := newSecurityPinRouter(t)
			before := seedFullSecurityRow(t, db, true)

			w := serveSecurityPin(t, router, http.MethodPost, "/security/config", "192.0.2.10:5000", body)

			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			after := loadDefaultSecurityRow(t, db)
			assert.Equal(t, before, after, "no column changes and updated_at is not bumped")
		})
	}
}

// The 200 body is the stored row, and the stored CrowdSec mode is the one shown.
func TestUpdateConfig_ResponseIsTheStoredRow(t *testing.T) {
	_, router, db := newSecurityPinRouter(t)
	seedFullSecurityRow(t, db, true)

	w := serveSecurityPin(t, router, http.MethodPost, "/security/config", "192.0.2.10:5000",
		`{"name":"default","waf_mode":"block"}`)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body struct {
		Config models.SecurityConfig `json:"config"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	stored := loadDefaultSecurityRow(t, db)
	assert.Equal(t, stored.UUID, body.Config.UUID)
	assert.Equal(t, "block", body.Config.WAFMode)
	assert.Equal(t, "local", body.Config.CrowdSecMode)
}

// With no stored row, the save creates one from the defaults. The client cannot choose the UUID or the creation time.
func TestUpdateConfig_CreatePathUsesDefaultsAndServerFields(t *testing.T) {
	_, router, db := newSecurityPinRouter(t)

	w := serveSecurityPin(t, router, http.MethodPost, "/security/config", "192.0.2.10:5000",
		`{"admin_whitelist":"192.0.2.0/24","uuid":"client-supplied-uuid","created_at":"2000-01-01T00:00:00Z"}`)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	stored := loadDefaultSecurityRow(t, db)
	assert.Equal(t, "192.0.2.0/24", stored.AdminWhitelist)
	assert.Equal(t, "disabled", stored.RateLimitMode)
	assert.Equal(t, 1, stored.WAFParanoiaLevel)
	assert.NotEmpty(t, stored.UUID)
	assert.NotEqual(t, "client-supplied-uuid", stored.UUID)
	assert.NotEqual(t, 2000, stored.CreatedAt.Year())
}

// Invalid values answer 400 and write nothing.
func TestUpdateConfig_InvalidPayloadsWriteNothing(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"malformed JSON", `{"waf_mode":`},
		{"wrong type", `{"enabled":"yes","waf_mode":"block"}`},
		{"invalid allow-list", `{"admin_whitelist":"not-a-cidr","waf_mode":"block"}`},
		{"invalid crowdsec mode", `{"crowdsec_mode":"external","waf_mode":"block"}`},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			_, router, db := newSecurityPinRouter(t)
			before := seedFullSecurityRow(t, db, true)

			w := serveSecurityPin(t, router, http.MethodPost, "/security/config", "192.0.2.10:5000", c.body)

			require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
			assertRowKeptExcept(t, before, loadDefaultSecurityRow(t, db))
		})
	}
}

// A caller without the admin role is refused before anything is read or written.
func TestUpdateConfig_NonAdminIsRefused(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}))
	before := seedFullSecurityRow(t, db, true)
	h := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	t.Cleanup(h.Close)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "user")
		c.Set("userID", uint(2))
		c.Next()
	})
	router.POST("/security/config", h.UpdateConfig)

	w := serveSecurityPin(t, router, http.MethodPost, "/security/config", "192.0.2.10:5000", `{"waf_mode":"block"}`)

	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	assertRowKeptExcept(t, before, loadDefaultSecurityRow(t, db))
}

// When the proxy apply fails, the save answers 500 and the pre-image is restored: the stored row and the module keys
// are the ones the save found, not the refused values.
func TestUpdateConfig_ApplyFailureRestoresStoredStateAndAnswers500(t *testing.T) {
	h, router, db := newSecurityPinRouter(t)
	before := seedFullSecurityRow(t, db, true)
	applier := &recordingApplier{err: errors.New("apply failed")}
	h.caddyManager = applier
	logs := captureLogrusOutput(t)

	w := serveSecurityPin(t, router, http.MethodPost, "/security/config", "192.0.2.10:5000",
		`{"name":"default","waf_mode":"block"}`)

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "Failed to reload configuration")
	assert.Equal(t, 1, applier.count(), "the proxy is applied once")
	assert.Contains(t, logs.String(), "failed to apply security config changes to Caddy")
	assertRowKeptExcept(t, before, loadDefaultSecurityRow(t, db))
}

// serveSecurityStatus returns the module status body, so a save can be checked against it.
func serveSecurityStatus(t *testing.T, router *gin.Engine) string {
	t.Helper()
	return serveSecurityPin(t, router, http.MethodGet, "/security/status", "192.0.2.10:5000", "").Body.String()
}
