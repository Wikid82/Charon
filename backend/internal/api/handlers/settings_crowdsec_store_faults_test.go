package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/testutil/gormfault"
)

// inertCrowdsecLifecycle answers Start and Stop without touching the database, so a test can fail a read that
// happens after the engine call.
type inertCrowdsecLifecycle struct{}

func (inertCrowdsecLifecycle) ResetCrowdSecState(write func() error) error { return write() }

func (inertCrowdsecLifecycle) StopAfterReset(error) {}

func (inertCrowdsecLifecycle) StartCrowdSec(context.Context) (CrowdsecStartResult, error) {
	return CrowdsecStartResult{Status: "started"}, nil
}

func (inertCrowdsecLifecycle) StopCrowdSec(context.Context) (CrowdsecStopResult, error) {
	return CrowdsecStopResult{}, nil
}

// recordingCacheInvalidator counts the cache invalidations the settings handler requests.
type recordingCacheInvalidator struct{ calls int }

func (r *recordingCacheInvalidator) InvalidateCache() { r.calls++ }

// postCrowdsecEnabled sends the enabled key with value through the settings write route.
func postCrowdsecEnabled(t *testing.T, router *gin.Engine, value string) *httptest.ResponseRecorder {
	t.Helper()
	body := `{"key":"` + models.CrowdSecEnabledSettingKey + `","value":"` + value + `"}`
	return sendSettingsJSON(t, router, http.MethodPost, "/settings", body)
}

// An empty enabled value reads the default row first. A failed read there is answered as a server error and
// nothing is stored.
func TestUpdateSetting_CrowdSecEmptyAnswersErrorWhenDefaultRowReadFails(t *testing.T) {
	h, db := newCrowdsecSettingsHandler(t, nil)
	gormfault.Inject(t, db, gormfault.Query, "security_configs", nil)

	w := postCrowdsecEnabled(t, settingsRouterFor(h), "")

	require.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to read CrowdSec setting")
	_, found := storedSettingValue(t, db, models.CrowdSecEnabledSettingKey)
	assert.False(t, found, "a failed read must not store the empty value")
}

// The deprecated mode key decides the absent-key case, so a failed read of it is answered as a server error.
func TestUpdateSetting_CrowdSecEmptyAnswersErrorWhenModeReadFails(t *testing.T) {
	h, db := newCrowdsecSettingsHandler(t, nil)
	gormfault.Inject(t, db, gormfault.Query, "settings", gormfault.VarIs(models.CrowdSecDeprecatedModeSettingKey))

	w := postCrowdsecEnabled(t, settingsRouterFor(h), "")

	require.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to read CrowdSec setting")
}

// The stored enabled value is read after the mode key, so a failed read of it is answered as a server error.
func TestUpdateSetting_CrowdSecEmptyAnswersErrorWhenEnabledReadFails(t *testing.T) {
	h, db := newCrowdsecSettingsHandler(t, nil)
	gormfault.Inject(t, db, gormfault.Query, "settings", gormfault.VarIs(models.CrowdSecEnabledSettingKey))

	w := postCrowdsecEnabled(t, settingsRouterFor(h), "")

	require.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to read CrowdSec setting")
}

// A successful start drops the cached security decision, so the next request sees the new state.
func TestUpdateSetting_CrowdSecTrueInvalidatesCacheAfterStart(t *testing.T) {
	h, _ := newCrowdsecSettingsHandler(t, inertCrowdsecLifecycle{})
	invalidator := &recordingCacheInvalidator{}
	h.Cerberus = invalidator

	w := postCrowdsecEnabled(t, settingsRouterFor(h), "true")

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, invalidator.calls)
}

// After a successful start the stored setting is read back to answer the request. A failed read is a server error.
func TestUpdateSetting_CrowdSecTrueAnswersErrorWhenStoredSettingReadFails(t *testing.T) {
	h, db := newCrowdsecSettingsHandler(t, inertCrowdsecLifecycle{})
	gormfault.Inject(t, db, gormfault.Query, "settings", gormfault.VarIs(models.CrowdSecEnabledSettingKey))

	w := postCrowdsecEnabled(t, settingsRouterFor(h), "true")

	require.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to read CrowdSec setting")
}
