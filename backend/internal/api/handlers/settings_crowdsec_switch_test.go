package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// settingsCrowdsecLifecycle stands in for the CrowdSec lifecycle behind the settings API. Like the real Start and
// Stop, a successful call writes the enabled key and the deprecated mode key; a scripted error writes nothing.
type settingsCrowdsecLifecycle struct {
	db         *gorm.DB
	startCalls int
	stopCalls  int
	startErr   error
	stopErr    error
}

func (f *settingsCrowdsecLifecycle) ResetCrowdSecState(write func() error) error { return write() }

func (f *settingsCrowdsecLifecycle) StopAfterReset(error) {}

func (f *settingsCrowdsecLifecycle) StartCrowdSec(_ context.Context) (CrowdsecStartResult, error) {
	f.startCalls++
	if f.startErr != nil {
		return CrowdsecStartResult{}, f.startErr
	}
	mustWriteCrowdsecSetting(f.db, models.CrowdSecEnabledSettingKey, "true")
	mustWriteCrowdsecSetting(f.db, models.CrowdSecDeprecatedModeSettingKey, "local")
	return CrowdsecStartResult{Status: "started", LAPIReady: true, ProxyApplied: true}, nil
}

func (f *settingsCrowdsecLifecycle) StopCrowdSec(_ context.Context) (CrowdsecStopResult, error) {
	f.stopCalls++
	if f.stopErr != nil {
		return CrowdsecStopResult{}, f.stopErr
	}
	mustWriteCrowdsecSetting(f.db, models.CrowdSecEnabledSettingKey, "false")
	mustWriteCrowdsecSetting(f.db, models.CrowdSecDeprecatedModeSettingKey, "disabled")
	return CrowdsecStopResult{ProxyApplied: true}, nil
}

func mustWriteCrowdsecSetting(db *gorm.DB, key, value string) {
	if err := upsertSetting(db, models.Setting{Key: key, Value: value, Category: "security", Type: "string"}); err != nil {
		panic(err)
	}
}

// newCrowdsecSettingsRouter builds the settings routes for an admin caller. lifecycle may be nil.
func newCrowdsecSettingsRouter(t *testing.T, lifecycle CrowdsecLifecycle) (*gin.Engine, *gorm.DB) {
	t.Helper()
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}))

	h := NewSettingsHandler(db)
	if lifecycle != nil {
		h.WithCrowdsecLifecycle(lifecycle)
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", uint(1))
		c.Next()
	})
	router.POST("/settings", h.UpdateSetting)
	router.PATCH("/settings", h.UpdateSetting)
	router.PATCH("/config", h.PatchConfig)
	return router, db
}

func sendSettingsJSON(t *testing.T, router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// storedSettingValue returns the stored value of key and whether a row exists.
func storedSettingValue(t *testing.T, db *gorm.DB, key string) (string, bool) {
	t.Helper()
	var s models.Setting
	err := db.Where("key = ?", key).First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false
	}
	require.NoError(t, err)
	return s.Value, true
}

func seedCrowdsecSettings(t *testing.T, db *gorm.DB, values map[string]string) {
	t.Helper()
	for key, value := range values {
		mustWriteCrowdsecSetting(db, key, value)
	}
}

// seedDefaultCrowdsecRow stores the default SecurityConfig row with the given CrowdSec mode.
func seedDefaultCrowdsecRow(t *testing.T, db *gorm.DB, mode string) {
	t.Helper()
	require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, Enabled: true, CrowdSecMode: mode}).Error)
}

func storedCrowdsecRowMode(t *testing.T, db *gorm.DB) string {
	t.Helper()
	var row models.SecurityConfig
	require.NoError(t, db.Where("name = ?", models.DefaultSecurityConfigName).First(&row).Error)
	return row.CrowdSecMode
}

func TestUpdateSetting_CrowdSecEnabledTrueStartsEngine(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			lc := &settingsCrowdsecLifecycle{}
			router, db := newCrowdsecSettingsRouter(t, lc)
			lc.db = db
			seedDefaultCrowdsecRow(t, db, "disabled")

			w := sendSettingsJSON(t, router, method, "/settings", `{"key":"security.crowdsec.enabled","value":"true"}`)

			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			assert.Equal(t, 1, lc.startCalls)
			assert.Equal(t, 0, lc.stopCalls)
			var body map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			assert.Equal(t, "true", body["value"])
		})
	}
}

func TestUpdateSetting_CrowdSecEnabledFalseStopsEngine(t *testing.T) {
	lc := &settingsCrowdsecLifecycle{}
	router, db := newCrowdsecSettingsRouter(t, lc)
	lc.db = db
	seedCrowdsecSettings(t, db, map[string]string{models.CrowdSecEnabledSettingKey: "true"})
	seedDefaultCrowdsecRow(t, db, "local")

	w := sendSettingsJSON(t, router, http.MethodPost, "/settings", `{"key":"security.crowdsec.enabled","value":" FALSE "}`)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, 0, lc.startCalls)
	assert.Equal(t, 1, lc.stopCalls)
	value, _ := storedSettingValue(t, db, models.CrowdSecEnabledSettingKey)
	assert.Equal(t, "false", value)
}

func TestUpdateSetting_CrowdSecEnabledOtherValueRefusedWithoutWriting(t *testing.T) {
	for _, value := range []string{"yes", "1", "on", "enabled"} {
		t.Run(value, func(t *testing.T) {
			lc := &settingsCrowdsecLifecycle{}
			router, db := newCrowdsecSettingsRouter(t, lc)
			lc.db = db

			body := fmt.Sprintf(`{"key":"security.crowdsec.enabled","value":%q}`, value)
			w := sendSettingsJSON(t, router, http.MethodPost, "/settings", body)

			require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
			assert.Contains(t, w.Body.String(), "invalid_crowdsec_setting")
			assert.Equal(t, 0, lc.startCalls+lc.stopCalls)
			_, found := storedSettingValue(t, db, models.CrowdSecEnabledSettingKey)
			assert.False(t, found)
		})
	}
}

func TestUpdateSetting_CrowdSecModeKeyRefusedOnBothMethods(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			lc := &settingsCrowdsecLifecycle{}
			router, db := newCrowdsecSettingsRouter(t, lc)
			lc.db = db

			w := sendSettingsJSON(t, router, method, "/settings", `{"key":"security.crowdsec.mode","value":"local"}`)

			require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
			assert.Contains(t, w.Body.String(), "crowdsec_setting_use_switch")
			assert.Equal(t, 0, lc.startCalls+lc.stopCalls)
			_, found := storedSettingValue(t, db, models.CrowdSecDeprecatedModeSettingKey)
			assert.False(t, found)
		})
	}
}

func TestUpdateSetting_CrowdSecEnabledEmptyIsNoOpWhenDecisionsMatch(t *testing.T) {
	tests := []struct {
		name     string
		settings map[string]string
		rowMode  string
		seedRow  bool
	}{
		{name: "no setting, no mode key, row disabled", rowMode: "disabled", seedRow: true},
		{name: "no setting, no mode key, no row"},
		{
			name: "running: enabled true, mode local, row local",
			settings: map[string]string{
				models.CrowdSecEnabledSettingKey:        "true",
				models.CrowdSecDeprecatedModeSettingKey: "local",
			},
			rowMode: "local",
			seedRow: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lc := &settingsCrowdsecLifecycle{}
			router, db := newCrowdsecSettingsRouter(t, lc)
			lc.db = db
			seedCrowdsecSettings(t, db, tt.settings)
			if tt.seedRow {
				seedDefaultCrowdsecRow(t, db, tt.rowMode)
			}

			w := sendSettingsJSON(t, router, http.MethodPost, "/settings", `{"key":"security.crowdsec.enabled","value":""}`)

			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			assert.Equal(t, 0, lc.startCalls+lc.stopCalls, "no lifecycle call when the decision already matches")
			assert.Equal(t, tt.rowMode, storedCrowdsecRowModeOrEmpty(t, db, tt.seedRow))
			var body map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			assert.Equal(t, models.CrowdSecEnabledSettingKey, body["key"])
		})
	}
}

func storedCrowdsecRowModeOrEmpty(t *testing.T, db *gorm.DB, seeded bool) string {
	t.Helper()
	if !seeded {
		return ""
	}
	return storedCrowdsecRowMode(t, db)
}

func TestUpdateSetting_CrowdSecEnabledEmptyStartsWhenStoredSplit(t *testing.T) {
	lc := &settingsCrowdsecLifecycle{}
	router, db := newCrowdsecSettingsRouter(t, lc)
	lc.db = db
	// An older version left the setting off while the mode key says local: the decisions differ.
	seedCrowdsecSettings(t, db, map[string]string{
		models.CrowdSecEnabledSettingKey:        "false",
		models.CrowdSecDeprecatedModeSettingKey: "local",
	})
	seedDefaultCrowdsecRow(t, db, "disabled")

	w := sendSettingsJSON(t, router, http.MethodPost, "/settings", `{"key":"security.crowdsec.enabled","value":""}`)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, 1, lc.startCalls)
	assert.Equal(t, 0, lc.stopCalls)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "true", body["value"], "the stored value becomes explicit")
}

func TestUpdateSetting_CrowdSecEnabledEmptyStopsWhenStalePair(t *testing.T) {
	lc := &settingsCrowdsecLifecycle{}
	router, db := newCrowdsecSettingsRouter(t, lc)
	lc.db = db
	// The upgrade case: enabled true with the mode key disabled, from an older version.
	seedCrowdsecSettings(t, db, map[string]string{
		models.CrowdSecEnabledSettingKey:        "true",
		models.CrowdSecDeprecatedModeSettingKey: "disabled",
	})
	seedDefaultCrowdsecRow(t, db, "local")

	w := sendSettingsJSON(t, router, http.MethodPost, "/settings", `{"key":"security.crowdsec.enabled","value":""}`)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, 0, lc.startCalls)
	assert.Equal(t, 1, lc.stopCalls)
}

func TestUpdateSetting_CrowdSecEnabledEmptyBusyWritesNothing(t *testing.T) {
	lc := &settingsCrowdsecLifecycle{}
	router, db := newCrowdsecSettingsRouter(t, nil)
	lc.db = db
	busy := &busyCrowdsecLifecycle{settingsCrowdsecLifecycle: lc}
	h := NewSettingsHandler(db).WithCrowdsecLifecycle(busy)
	router.POST("/busy-settings", h.UpdateSetting)
	seedCrowdsecSettings(t, db, map[string]string{
		models.CrowdSecEnabledSettingKey:        "false",
		models.CrowdSecDeprecatedModeSettingKey: "local",
	})
	seedDefaultCrowdsecRow(t, db, "local")

	w := sendSettingsJSON(t, router, http.MethodPost, "/busy-settings", `{"key":"security.crowdsec.enabled","value":""}`)

	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Equal(t, 1, busy.startAttempts)
	value, _ := storedSettingValue(t, db, models.CrowdSecEnabledSettingKey)
	assert.Equal(t, "false", value)
	mode, _ := storedSettingValue(t, db, models.CrowdSecDeprecatedModeSettingKey)
	assert.Equal(t, "local", mode)
	assert.Equal(t, "local", storedCrowdsecRowMode(t, db))
}

// busyCrowdsecLifecycle refuses every Start as if another change held the engine.
type busyCrowdsecLifecycle struct {
	*settingsCrowdsecLifecycle
	startAttempts int
}

func (b *busyCrowdsecLifecycle) StartCrowdSec(_ context.Context) (CrowdsecStartResult, error) {
	b.startAttempts++
	return CrowdsecStartResult{}, &CrowdsecBusyError{}
}

func (b *busyCrowdsecLifecycle) StopCrowdSec(_ context.Context) (CrowdsecStopResult, error) {
	return CrowdsecStopResult{}, &CrowdsecBusyError{}
}

func (b *busyCrowdsecLifecycle) ResetCrowdSecState(write func() error) error { return write() }

func (b *busyCrowdsecLifecycle) StopAfterReset(error) {}

func TestUpdateSetting_CrowdSecEnabledEmptyFailedStartWritesNothing(t *testing.T) {
	lc := &settingsCrowdsecLifecycle{startErr: fmt.Errorf("%w: boom", errCrowdsecStart)}
	router, db := newCrowdsecSettingsRouter(t, lc)
	lc.db = db
	seedCrowdsecSettings(t, db, map[string]string{
		models.CrowdSecEnabledSettingKey:        "false",
		models.CrowdSecDeprecatedModeSettingKey: "local",
	})
	seedDefaultCrowdsecRow(t, db, "disabled")

	w := sendSettingsJSON(t, router, http.MethodPost, "/settings", `{"key":"security.crowdsec.enabled","value":""}`)

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "failed to start CrowdSec")
	value, _ := storedSettingValue(t, db, models.CrowdSecEnabledSettingKey)
	assert.Equal(t, "false", value)
	assert.Equal(t, "disabled", storedCrowdsecRowMode(t, db))
}

func TestUpdateSetting_CrowdSecWithoutLifecycleAnswersUnavailable(t *testing.T) {
	t.Run("explicit true", func(t *testing.T) {
		router, db := newCrowdsecSettingsRouter(t, nil)
		seedDefaultCrowdsecRow(t, db, "disabled")

		w := sendSettingsJSON(t, router, http.MethodPost, "/settings", `{"key":"security.crowdsec.enabled","value":"true"}`)

		require.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), "crowdsec_unavailable")
		_, found := storedSettingValue(t, db, models.CrowdSecEnabledSettingKey)
		assert.False(t, found)
	})

	t.Run("empty value with a differing decision", func(t *testing.T) {
		router, db := newCrowdsecSettingsRouter(t, nil)
		seedCrowdsecSettings(t, db, map[string]string{
			models.CrowdSecEnabledSettingKey:        "false",
			models.CrowdSecDeprecatedModeSettingKey: "local",
		})
		seedDefaultCrowdsecRow(t, db, "disabled")

		w := sendSettingsJSON(t, router, http.MethodPost, "/settings", `{"key":"security.crowdsec.enabled","value":""}`)

		require.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
		value, _ := storedSettingValue(t, db, models.CrowdSecEnabledSettingKey)
		assert.Equal(t, "false", value)
	})

	t.Run("empty value with a matching decision is a no-op", func(t *testing.T) {
		router, db := newCrowdsecSettingsRouter(t, nil)
		seedDefaultCrowdsecRow(t, db, "disabled")

		w := sendSettingsJSON(t, router, http.MethodPost, "/settings", `{"key":"security.crowdsec.enabled","value":""}`)

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	})
}

func TestPatchConfig_CrowdSecSwitchKeysRefusedWithoutWriting(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "flat enabled key", body: `{"security.crowdsec.enabled":"true","app.marker":"x"}`},
		{name: "flat mode key", body: `{"security.crowdsec.mode":"local","app.marker":"x"}`},
		{name: "nested enabled key", body: `{"security":{"crowdsec":{"enabled":"true"}},"app":{"marker":"x"}}`},
		{name: "nested mode key", body: `{"security":{"crowdsec":{"mode":"local"}},"app":{"marker":"x"}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lc := &settingsCrowdsecLifecycle{}
			router, db := newCrowdsecSettingsRouter(t, lc)
			lc.db = db

			w := sendSettingsJSON(t, router, http.MethodPatch, "/config", tt.body)

			require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
			assert.Contains(t, w.Body.String(), "crowdsec_setting_use_switch")
			assert.Equal(t, 0, lc.startCalls+lc.stopCalls)
			_, found := storedSettingValue(t, db, "app.marker")
			assert.False(t, found, "the batch stays atomic: no other key is written")
		})
	}
}

func TestCrowdsecLifecycle_StartAndStopWriteModeKey(t *testing.T) {
	db := setupCrowdDB(t)
	seedDefaultCrowdsecRow(t, db, "disabled")
	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", t.TempDir())

	_, err := h.StartCrowdSec(context.Background())
	require.NoError(t, err)
	mode, _ := storedSettingValue(t, db, models.CrowdSecDeprecatedModeSettingKey)
	assert.Equal(t, "local", mode)

	_, err = h.StopCrowdSec(context.Background())
	require.NoError(t, err)
	mode, _ = storedSettingValue(t, db, models.CrowdSecDeprecatedModeSettingKey)
	assert.Equal(t, "disabled", mode)
	enabled, _ := storedSettingValue(t, db, models.CrowdSecEnabledSettingKey)
	assert.Equal(t, "false", enabled)
}

func TestCrowdsecLifecycle_FailedStartRestoresModeKeyExactly(t *testing.T) {
	tests := []struct {
		name     string
		prior    *string
		wantMode string
		wantRow  bool
	}{
		{name: "absent before start stays absent", prior: nil, wantRow: false},
		{name: "disabled before start stays disabled", prior: strPtr("disabled"), wantMode: "disabled", wantRow: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupCrowdDB(t)
			seedDefaultCrowdsecRow(t, db, "disabled")
			if tt.prior != nil {
				mustWriteCrowdsecSetting(db, models.CrowdSecDeprecatedModeSettingKey, *tt.prior)
			}
			h := newTestCrowdsecHandler(t, db, &fakeExec{startErr: errors.New("boom")}, "/bin/false", t.TempDir())

			_, err := h.StartCrowdSec(context.Background())
			require.Error(t, err)

			mode, found := storedSettingValue(t, db, models.CrowdSecDeprecatedModeSettingKey)
			assert.Equal(t, tt.wantRow, found)
			assert.Equal(t, tt.wantMode, mode)
		})
	}
}

func strPtr(s string) *string { return &s }

// TestCrowdsecLifecycle_ResetThenStartThenEmptyIsNoOp runs the real lifecycle behind the settings API: the state
// an emergency reset leaves, then a Start, then an empty enabled value, which must not stop the engine.
func TestCrowdsecLifecycle_ResetThenStartThenEmptyIsNoOp(t *testing.T) {
	db := setupCrowdDB(t)
	seedDefaultCrowdsecRow(t, db, "disabled")
	seedCrowdsecSettings(t, db, map[string]string{
		models.CrowdSecEnabledSettingKey:        "false",
		models.CrowdSecDeprecatedModeSettingKey: "disabled",
	})
	fe := &fakeExec{}
	lifecycle := newTestCrowdsecHandler(t, db, fe, "/bin/false", t.TempDir())

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", uint(1))
		c.Next()
	})
	settings := NewSettingsHandler(db).WithCrowdsecLifecycle(lifecycle)
	router.POST("/settings", settings.UpdateSetting)

	w := sendSettingsJSON(t, router, http.MethodPost, "/settings", `{"key":"security.crowdsec.enabled","value":""}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.False(t, fe.started, "the reset state has nothing to stop or start")

	w = sendSettingsJSON(t, router, http.MethodPost, "/settings", `{"key":"security.crowdsec.enabled","value":"true"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.True(t, fe.started)

	w = sendSettingsJSON(t, router, http.MethodPost, "/settings", `{"key":"security.crowdsec.enabled","value":""}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.True(t, fe.started, "an empty value after a Start must not stop the running engine")
}
