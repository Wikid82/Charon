package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/cerberus"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

// readSettingValue returns the stored value of a setting and whether the row exists.
func readSettingValue(t *testing.T, db *gorm.DB, key string) (string, bool) {
	t.Helper()
	var s models.Setting
	err := db.Where("key = ?", key).First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false
	}
	require.NoError(t, err)
	return s.Value, true
}

// seedSwitchSetting stores a setting value for the test fixture.
func seedSwitchSetting(t *testing.T, db *gorm.DB, key, value string) {
	t.Helper()
	require.NoError(t, db.Create(&models.Setting{Key: key, Value: value, Category: "security", Type: "bool"}).Error)
}

// cerberusIsEnabled reports the Cerberus flag as the runtime sees it, without the static config defaults.
func cerberusIsEnabled(db *gorm.DB) bool {
	return cerberus.New(config.SecurityConfig{}, db).IsEnabled()
}

func postCrowdsec(t *testing.T, h *CrowdsecHandler, path string) *httptest.ResponseRecorder {
	t.Helper()
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/"+path, http.NoBody))
	return w
}

// Stop must not change the global security switch, even when it is on.
func TestCrowdsecHandler_Stop_LeavesGlobalSwitchOn(t *testing.T) {
	t.Parallel()
	db := setupCrowdDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-stop", Name: models.DefaultSecurityConfigName, Enabled: true, CrowdSecMode: "local"}).Error)
	seedSwitchSetting(t, db, "security.crowdsec.enabled", "true")

	h := newTestCrowdsecHandler(t, db, &fakeExec{started: true}, "/bin/false", t.TempDir())
	require.Equal(t, http.StatusOK, postCrowdsec(t, h, "stop").Code)

	cfg, err := models.FindDefaultSecurityConfig(db)
	require.NoError(t, err)
	require.True(t, cfg.Enabled, "Stop must leave the global switch as it was")
	require.Equal(t, "disabled", cfg.CrowdSecMode)
	value, _ := readSettingValue(t, db, "security.crowdsec.enabled")
	require.Equal(t, "false", value)
}

// A successful Start turns Cerberus on through the shared helper, even when the feature flag is "false".
func TestCrowdsecHandler_Start_TurnsCerberusOnDespiteFeatureFlagFalse(t *testing.T) {
	t.Parallel()
	db := setupCrowdDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-start", Name: models.DefaultSecurityConfigName, Enabled: false, CrowdSecMode: "disabled"}).Error)
	seedSwitchSetting(t, db, "feature.cerberus.enabled", "false")

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", t.TempDir())
	require.Equal(t, http.StatusOK, postCrowdsec(t, h, "start").Code)

	cfg, err := models.FindDefaultSecurityConfig(db)
	require.NoError(t, err)
	require.True(t, cfg.Enabled)
	require.Equal(t, "local", cfg.CrowdSecMode)
	for _, key := range []string{"feature.cerberus.enabled", "security.cerberus.enabled", "security.crowdsec.enabled"} {
		value, ok := readSettingValue(t, db, key)
		require.True(t, ok, key)
		require.Equal(t, "true", value, key)
	}
	require.True(t, cerberusIsEnabled(db), "Cerberus must be on after a successful Start")
}

// A failed Start restores the exact pre-Start values and never forces the global switch off.
func TestCrowdsecHandler_Start_FailureRestoresPreStartSnapshot(t *testing.T) {
	t.Parallel()
	db := setupCrowdDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-fail", Name: models.DefaultSecurityConfigName, Enabled: true, CrowdSecMode: "disabled"}).Error)
	seedSwitchSetting(t, db, "security.crowdsec.enabled", "false")

	h := newTestCrowdsecHandler(t, db, &fakeExec{startErr: fmt.Errorf("boom")}, "/bin/false", t.TempDir())
	require.Equal(t, http.StatusInternalServerError, postCrowdsec(t, h, "start").Code)

	cfg, err := models.FindDefaultSecurityConfig(db)
	require.NoError(t, err)
	require.True(t, cfg.Enabled, "a failed Start must not turn the global switch off")
	require.Equal(t, "disabled", cfg.CrowdSecMode)
	value, ok := readSettingValue(t, db, "security.crowdsec.enabled")
	require.True(t, ok)
	require.Equal(t, "false", value)
	_, ok = readSettingValue(t, db, "feature.cerberus.enabled")
	require.False(t, ok, "a setting absent before Start must be deleted again")
	_, ok = readSettingValue(t, db, "security.cerberus.enabled")
	require.False(t, ok, "a setting absent before Start must be deleted again")
}

// A failed Start keeps a default row that it created, with the seed values.
func TestCrowdsecHandler_Start_FailureKeepsRowCreatedByStart(t *testing.T) {
	t.Parallel()
	db := setupCrowdDB(t)

	h := newTestCrowdsecHandler(t, db, &fakeExec{startErr: fmt.Errorf("boom")}, "/bin/false", t.TempDir())
	require.Equal(t, http.StatusInternalServerError, postCrowdsec(t, h, "start").Code)

	cfg, err := models.FindDefaultSecurityConfig(db)
	require.NoError(t, err, "the row created by the failed Start must be kept")
	require.False(t, cfg.Enabled)
	require.Equal(t, "disabled", cfg.CrowdSecMode)
}

// Start must refuse to write when the settings it snapshots cannot be read.
func TestCrowdsecHandler_Start_SnapshotFailureWritesNothing(t *testing.T) {
	t.Parallel()
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	fe := &fakeExec{}
	h := newTestCrowdsecHandler(t, db, fe, "/bin/false", t.TempDir())
	require.Equal(t, http.StatusInternalServerError, postCrowdsec(t, h, "start").Code)
	require.False(t, fe.started, "the engine must not start when the snapshot fails")
}

// Stop still succeeds when the CrowdSec setting cannot be written, and it still writes the row.
func TestCrowdsecHandler_Stop_SettingWriteFailureStillStops(t *testing.T) {
	t.Parallel()
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-stop-err", Name: models.DefaultSecurityConfigName, Enabled: true, CrowdSecMode: "local"}).Error)

	h := newTestCrowdsecHandler(t, db, &fakeExec{started: true}, "/bin/false", t.TempDir())
	require.Equal(t, http.StatusOK, postCrowdsec(t, h, "stop").Code)

	cfg, err := models.FindDefaultSecurityConfig(db)
	require.NoError(t, err)
	require.Equal(t, "disabled", cfg.CrowdSecMode)
	require.True(t, cfg.Enabled)
}

// Stop still succeeds when the default row cannot be read, because the write is logged only.
func TestCrowdsecHandler_Stop_MissingSecurityTableStillStops(t *testing.T) {
	t.Parallel()
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}))

	h := newTestCrowdsecHandler(t, db, &fakeExec{started: true}, "/bin/false", t.TempDir())
	require.Equal(t, http.StatusOK, postCrowdsec(t, h, "stop").Code)
	value, ok := readSettingValue(t, db, "security.crowdsec.enabled")
	require.True(t, ok)
	require.Equal(t, "false", value)
}

// The shared enable helper reports a failed settings write instead of skipping it.
func TestEnableSecurityFramework_SettingWriteFailure(t *testing.T) {
	t.Parallel()
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	err := enableSecurityFramework(db)
	require.ErrorContains(t, err, "save setting")
}
