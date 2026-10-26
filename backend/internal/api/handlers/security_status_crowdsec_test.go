package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

// crowdSecStatusFromGetStatus runs GetStatus against db with the given static config and returns the reported
// CrowdSec mode and enabled flag.
func crowdSecStatusFromGetStatus(t *testing.T, cfg config.SecurityConfig, db *gorm.DB) (mode string, enabled bool) {
	t.Helper()
	manager := caddy.NewManager(noopCaddyClient{}, db, t.TempDir(), "", false, config.SecurityConfig{})
	h := NewSecurityHandler(cfg, db, manager)
	t.Cleanup(h.Close)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/security/status", http.NoBody)
	h.GetStatus(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var body struct {
		CrowdSec struct {
			Mode    string `json:"mode"`
			Enabled bool   `json:"enabled"`
		} `json:"crowdsec"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body.CrowdSec.Mode, body.CrowdSec.Enabled
}

// A default row with an empty CrowdSec mode is off, even when the static config says local. The proxy config
// reads the same row and treats the empty mode as off, so the status must agree with it.
func TestGetStatus_EmptyRowCrowdSecModeIsOffEvenWhenStaticIsLocal(t *testing.T) {
	db := setupCrowdDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "sc-default", Name: models.DefaultSecurityConfigName, Enabled: true, CrowdSecMode: ""}).Error)

	mode, enabled := crowdSecStatusFromGetStatus(t, config.SecurityConfig{CrowdSecMode: "local"}, db)

	assert.Equal(t, "disabled", mode)
	assert.False(t, enabled)
}

// With no default row, the static config mode is the fallback, so local stays on.
func TestGetStatus_NoRowFallsBackToStaticLocal(t *testing.T) {
	db := setupCrowdDB(t)

	mode, enabled := crowdSecStatusFromGetStatus(t, config.SecurityConfig{CerberusEnabled: true, CrowdSecMode: "local"}, db)

	assert.Equal(t, "local", mode)
	assert.True(t, enabled)
}
