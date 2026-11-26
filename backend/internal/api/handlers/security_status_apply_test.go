package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

// TestLatestConfigApplyState_MessageIsOnlyForAdministrators checks the stored apply message is returned to
// administrators and left out for any other caller.
func TestLatestConfigApplyState_MessageIsOnlyForAdministrators(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.CaddyConfig{}))
	require.NoError(t, db.Create(&models.CaddyConfig{
		AppliedAt: time.Now(), Success: false, ErrorMsg: "stored apply text",
	}).Error)

	admin := latestConfigApplyState(db, true)
	assert.Equal(t, "stored apply text", admin["error_msg"])
	assert.Equal(t, "failed", admin["status"])
	assert.Equal(t, true, admin["available"])

	other := latestConfigApplyState(db, false)
	assert.NotContains(t, other, "error_msg")
	assert.Equal(t, "failed", other["status"])
	assert.Equal(t, false, other["success"])
	assert.Equal(t, true, other["available"])
	assert.Contains(t, other, "applied_at")
}

// TestSecurityStatus_ConfigApplyMessageDependsOnRole serves the status route as each role.
func TestSecurityStatus_ConfigApplyMessageDependsOnRole(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.CaddyConfig{}, &models.SecurityConfig{}, &models.Setting{}))
	require.NoError(t, db.Create(&models.CaddyConfig{AppliedAt: time.Now(), ErrorMsg: "stored apply text"}).Error)
	h := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	t.Cleanup(h.Close)

	for _, tc := range []struct {
		role    string
		visible bool
	}{{"admin", true}, {"user", false}} {
		router := roleStampRouter(tc.role)
		router.GET("/security/status", h.GetStatus)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/security/status", http.NoBody))
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())

		var body struct {
			ConfigApply map[string]any `json:"config_apply"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		if tc.visible {
			assert.Equal(t, "stored apply text", body.ConfigApply["error_msg"], tc.role)
		} else {
			assert.NotContains(t, body.ConfigApply, "error_msg", tc.role)
		}
		assert.Equal(t, true, body.ConfigApply["available"], tc.role)
	}
}
