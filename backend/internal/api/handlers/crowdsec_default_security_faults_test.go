package handlers

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/testutil/gormfault"
)

// captureHandlerLogs routes the logger into a buffer for the rest of the test.
func captureHandlerLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	logger.Init(false, buf)
	t.Cleanup(func() { logger.Init(false, io.Discard) })
	return buf
}

// TestStopFault_DefaultRowWriteFailureIsLoggedAndStillApplies: a failed row write after a successful stop
// is logged, the response stays a success, and the proxy is still applied once.
func TestStopFault_DefaultRowWriteFailureIsLoggedAndStillApplies(t *testing.T) {
	lapiURL, _ := newKeyLAPI(t)
	clearKeyEnv(t, "")
	applier := &recordingApplier{}
	h := newStartApplyHandler(t, lapiURL, t.TempDir(), new(MockCommandExecutor), &fakeExec{started: true}, applier)
	seedRunningCrowdSecState(t, h)
	gormfault.Inject(t, h.DB, gormfault.Update, "security_configs", gormfault.UpdateSets("crowd_sec_mode"))
	logs := captureHandlerLogs(t)

	w := stopRequest(t, h, context.Background())

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, logs.String(), "Failed to update SecurityConfig after stopping CrowdSec")
	assert.Equal(t, 1, applier.count())
	_, setting := readStoppedCrowdSecState(t, h)
	assert.Equal(t, "false", setting, "the setting write is independent of the row write")
}

// TestStartFault_SnapshotFailureStopsBeforeAnyWrite: when the state cannot be read first, Start refuses
// without writing, starting the engine or applying the proxy.
func TestStartFault_SnapshotFailureStopsBeforeAnyWrite(t *testing.T) {
	lapiURL, _ := newKeyLAPI(t)
	clearKeyEnv(t, "")
	applier := &recordingApplier{}
	exec := &fakeExec{}
	h := newStartApplyHandler(t, lapiURL, t.TempDir(), new(MockCommandExecutor), exec, applier)
	gormfault.Inject(t, h.DB, gormfault.Query, "security_configs", nil)

	w := startRequest(t, h, context.Background())

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "Failed to persist configuration")
	assert.False(t, exec.started, "the engine must not start when the state cannot be read")
	assert.Zero(t, applier.count())
}

// TestStartFault_ModeWriteFailureRevertsAndDoesNotStart: a failed crowd_sec_mode write during Start
// reports a persist error, starts nothing and applies nothing.
func TestStartFault_ModeWriteFailureRevertsAndDoesNotStart(t *testing.T) {
	lapiURL, _ := newKeyLAPI(t)
	clearKeyEnv(t, "")
	applier := &recordingApplier{}
	exec := &fakeExec{}
	h := newStartApplyHandler(t, lapiURL, t.TempDir(), new(MockCommandExecutor), exec, applier)
	// Known pre-Start state: the global switch off, CrowdSec disabled, Cerberus settings "false".
	require.NoError(t, h.DB.Model(&models.SecurityConfig{}).Where("name = ?", models.DefaultSecurityConfigName).
		Updates(map[string]any{"enabled": false, "crowd_sec_mode": "disabled"}).Error)
	require.NoError(t, h.DB.Create(&models.Setting{Key: "feature.cerberus.enabled", Value: "false", Category: "feature", Type: "bool"}).Error)
	require.NoError(t, h.DB.Create(&models.Setting{Key: "security.cerberus.enabled", Value: "false", Category: "security", Type: "bool"}).Error)
	gormfault.Inject(t, h.DB, gormfault.Update, "security_configs", gormfault.UpdateSets("crowd_sec_mode"))
	logs := captureHandlerLogs(t)

	w := startRequest(t, h, context.Background())

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "Failed to persist configuration")
	assert.Contains(t, logs.String(), "Failed to persist SecurityConfig")
	assert.False(t, exec.started, "a failed persist must not start the engine")
	assert.Zero(t, applier.count())

	// The revert puts back the pre-Start state: the row is off again, and the settings Start wrote are restored.
	var row models.SecurityConfig
	require.NoError(t, h.DB.Where("name = ?", models.DefaultSecurityConfigName).First(&row).Error)
	assert.False(t, row.Enabled, "the global switch Start turned on must be reverted")
	assert.Equal(t, "disabled", row.CrowdSecMode)
	assertSettingValue(t, h.DB, "feature.cerberus.enabled", "false")
	assertSettingValue(t, h.DB, "security.cerberus.enabled", "false")
	assertSettingAbsent(t, h.DB, "security.crowdsec.enabled")
}

// assertSettingValue fails unless the settings table holds exactly one row for key with the given value.
func assertSettingValue(t *testing.T, db *gorm.DB, key, want string) {
	t.Helper()
	var setting models.Setting
	require.NoError(t, db.Where("key = ?", key).First(&setting).Error, "setting %s", key)
	assert.Equal(t, want, setting.Value, "setting %s", key)
}

// assertSettingAbsent fails when the settings table holds a row for key.
func assertSettingAbsent(t *testing.T, db *gorm.DB, key string) {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(&models.Setting{}).Where("key = ?", key).Count(&count).Error)
	assert.Zero(t, count, "setting %s must be absent", key)
}

// TestStatusFault_UnreadableCrowdSecSettingIsLoggedAndStatusStillAnswers: the status endpoint falls back to
// the row and still answers when the enabled setting cannot be read.
func TestStatusFault_UnreadableCrowdSecSettingIsLoggedAndStatusStillAnswers(t *testing.T) {
	db := setupCrowdDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "sc-default", Name: models.DefaultSecurityConfigName, Enabled: true, CrowdSecMode: "local"}).Error)
	require.NoError(t, db.Create(&models.Setting{Key: "security.crowdsec.enabled", Value: "false", Category: "security", Type: "bool"}).Error)
	gormfault.Inject(t, db, gormfault.Query, "settings", gormfault.VarIs("security.crowdsec.enabled"))
	logs := captureHandlerLogs(t)

	// The setting is unreadable, so the status must report the row's mode ("local") and not the disabled setting.
	mode, enabled := crowdSecStatusFromGetStatus(t, config.SecurityConfig{}, db)

	assert.Equal(t, "local", mode)
	assert.True(t, enabled)
	assert.Contains(t, logs.String(), "Failed to read CrowdSec setting for status, using the default SecurityConfig row")
}

// TestToggleFault_EnableFrameworkFailureIsA500: when the global security switch cannot be enabled, the
// module toggle answers 500 and the module is not left on.
func TestToggleFault_EnableFrameworkFailureIsA500(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}))
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "sc-default", Name: models.DefaultSecurityConfigName, Enabled: false}).Error)
	require.NoError(t, db.Create(&models.Setting{Key: "security.waf.enabled", Value: "false", Category: "security", Type: "bool"}).Error)
	gormfault.Inject(t, db, gormfault.Update, "security_configs", gormfault.UpdateSets("enabled"))
	manager := caddy.NewManager(noopCaddyClient{}, db, t.TempDir(), "", false, config.SecurityConfig{})
	h := NewSecurityHandler(config.SecurityConfig{}, db, manager)
	t.Cleanup(h.Close)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/security/waf", strings.NewReader(`{"enabled":true}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("role", "admin")
	h.PatchWAF(c)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to update security module", "the framework enable failure is a 500 with no partial write")
	var setting models.Setting
	require.NoError(t, db.Where("key = ?", "security.waf.enabled").First(&setting).Error)
	assert.Equal(t, "false", setting.Value)
}
