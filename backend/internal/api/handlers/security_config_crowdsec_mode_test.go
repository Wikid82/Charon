package handlers

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

// newSecurityConfigRouter builds POST /security/config for an admin caller over a fresh database that holds the
// default row. The row is seeded with the given CrowdSec mode.
func newSecurityConfigRouter(t *testing.T, mode string) (*gin.Engine, *gorm.DB, models.SecurityConfig) {
	t.Helper()
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}))
	seedDefaultCrowdsecRow(t, db, mode)
	var row models.SecurityConfig
	require.NoError(t, db.Where("name = ?", models.DefaultSecurityConfigName).First(&row).Error)

	h := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	t.Cleanup(h.Close)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", uint(1))
		c.Next()
	})
	router.POST("/security/config", h.UpdateConfig)
	return router, db, row
}

func TestUpdateConfig_CrowdSecModeStaysWithStoredRow(t *testing.T) {
	router, db, row := newSecurityConfigRouter(t, "disabled")

	w := sendSettingsJSON(t, router, http.MethodPost, "/security/config", `{"name":"default","crowdsec_mode":"local","waf_mode":"block"}`)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"crowdsec_mode":"disabled"`, "the 200 body shows the stored mode")
	assert.Contains(t, w.Body.String(), row.UUID, "the 200 body shows the stored row, not the echoed payload")
	assert.Equal(t, "disabled", storedCrowdsecRowMode(t, db), "the payload does not change the stored mode")
	var stored models.SecurityConfig
	require.NoError(t, db.Where("name = ?", models.DefaultSecurityConfigName).First(&stored).Error)
	assert.Equal(t, "block", stored.WAFMode, "the other columns still save")
}

func TestUpdateConfig_CrowdSecModeAbsentFromPayloadKeepsStoredValue(t *testing.T) {
	router, db, _ := newSecurityConfigRouter(t, "local")

	w := sendSettingsJSON(t, router, http.MethodPost, "/security/config", `{"name":"default","waf_mode":"block"}`)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "local", storedCrowdsecRowMode(t, db))
}

func TestUpdateConfig_ExternalCrowdSecModeRefusedBeforeAnySave(t *testing.T) {
	router, db, _ := newSecurityConfigRouter(t, "local")
	require.NoError(t, db.Model(&models.SecurityConfig{}).Where("name = ?", models.DefaultSecurityConfigName).Update("waf_mode", "disabled").Error)

	w := sendSettingsJSON(t, router, http.MethodPost, "/security/config", `{"name":"default","crowdsec_mode":"external","waf_mode":"block"}`)

	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "invalid crowdsec mode: external")
	var stored models.SecurityConfig
	require.NoError(t, db.Where("name = ?", models.DefaultSecurityConfigName).First(&stored).Error)
	assert.Equal(t, "local", stored.CrowdSecMode)
	assert.Equal(t, "disabled", stored.WAFMode, "nothing is saved when the mode is refused")
}
