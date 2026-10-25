package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// Stop must write the default row even when a row with a lower ID exists under another name.
func TestCrowdsecHandler_Stop_WritesDefaultRowNotLowerIDOtherRow(t *testing.T) {
	t.Parallel()
	db := setupCrowdDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-other", Name: "other", CrowdSecMode: "local"}).Error)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-default", Name: models.DefaultSecurityConfigName, CrowdSecMode: "local"}).Error)

	fe := &fakeExec{started: true}
	h := newTestCrowdsecHandler(t, db, fe, "/bin/false", t.TempDir())
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/stop", http.NoBody))
	require.Equal(t, http.StatusOK, w.Code)

	defaultCfg, err := models.FindDefaultSecurityConfig(db)
	require.NoError(t, err)
	assert.Equal(t, "disabled", defaultCfg.CrowdSecMode)

	var other models.SecurityConfig
	require.NoError(t, db.Where("name = ?", "other").First(&other).Error)
	assert.Equal(t, "local", other.CrowdSecMode, "rows under other names are not touched")
}

func TestSetDefaultSecurityConfigState_SeedsMissingRow(t *testing.T) {
	t.Parallel()
	db := setupCrowdDB(t)

	cfg, err := setDefaultSecurityConfigState(db, map[string]any{"crowd_sec_mode": "local", "enabled": true})
	require.NoError(t, err)
	assert.Equal(t, models.DefaultSecurityConfigName, cfg.Name)
	assert.Equal(t, "local", cfg.CrowdSecMode)
	assert.True(t, cfg.Enabled)

	var count int64
	require.NoError(t, db.Model(&models.SecurityConfig{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestSetDefaultSecurityConfigState_WritesOnlyNamedColumns(t *testing.T) {
	t.Parallel()
	db := setupCrowdDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-default", Name: models.DefaultSecurityConfigName, WAFMode: "block"}).Error)

	_, err := setDefaultSecurityConfigState(db, map[string]any{"crowd_sec_mode": "disabled"})
	require.NoError(t, err)

	cfg, err := models.FindDefaultSecurityConfig(db)
	require.NoError(t, err)
	assert.Equal(t, "disabled", cfg.CrowdSecMode)
	assert.Equal(t, "block", cfg.WAFMode)
}
