package handlers

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/testutil/gormfault"

	"github.com/Wikid82/charon/backend/internal/models"
)

// newSwitchWriteDB opens a database with the settings and default row tables.
func newSwitchWriteDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}))
	return db
}

// requireCerberusKeysAbsent checks that neither Cerberus switch key is stored.
func requireCerberusKeysAbsent(t *testing.T, db *gorm.DB) {
	t.Helper()
	_, found := storedSettingValue(t, db, models.CerberusFeatureSettingKey)
	assert.False(t, found, "the feature key must not be stored")
	_, found = storedSettingValue(t, db, models.CerberusLegacySettingKey)
	assert.False(t, found, "the legacy key must not be stored")
}

// requireCerberusKeysValue checks that both Cerberus switch keys hold value.
func requireCerberusKeysValue(t *testing.T, db *gorm.DB, value string) {
	t.Helper()
	feature, found := storedSettingValue(t, db, models.CerberusFeatureSettingKey)
	require.True(t, found)
	assert.Equal(t, value, feature)
	legacy, found := storedSettingValue(t, db, models.CerberusLegacySettingKey)
	require.True(t, found)
	assert.Equal(t, value, legacy)
}

// TestSwitchWrites_CerberusValueMustBeTrueOrFalse describes both settings write paths: a value other than true or false
// is refused with 400 before anything is stored.
func TestSwitchWrites_CerberusValueMustBeTrueOrFalse(t *testing.T) {
	tests := []struct {
		name string
		path string
		body string
	}{
		{name: "settings key", path: "/settings", body: `{"key":"feature.cerberus.enabled","value":"maybe"}`},
		{name: "legacy key through settings", path: "/settings", body: `{"key":"security.cerberus.enabled","value":"1"}`},
		{name: "bulk config", path: "/config", body: `{"feature":{"cerberus":{"enabled":"maybe"}}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newSwitchWriteDB(t)
			applier := &recordingApplier{}
			h := NewSettingsHandlerWithDeps(db, applier, &countingInvalidator{}, nil, "")
			c, w := proxyApplyAdminContext(http.MethodPost, tt.path, []byte(tt.body), context.Background())

			if tt.path == "/config" {
				h.PatchConfig(c)
			} else {
				h.UpdateSetting(c)
			}

			require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
			assert.Contains(t, w.Body.String(), "must be true or false")
			requireCerberusKeysAbsent(t, db)
			assert.Equal(t, 0, applier.count())
		})
	}
}

// TestSwitchWrites_PaddedTrueStoresExactTrueOnBothKeys describes a padded, upper-case value: it is accepted, both keys
// store exactly "true", the cache is invalidated and the proxy is applied once.
func TestSwitchWrites_PaddedTrueStoresExactTrueOnBothKeys(t *testing.T) {
	db := newSwitchWriteDB(t)
	applier := &recordingApplier{}
	cache := &countingInvalidator{}
	h := NewSettingsHandlerWithDeps(db, applier, cache, nil, "")
	c, w := proxyApplyAdminContext(http.MethodPost, "/settings", []byte(`{"key":"feature.cerberus.enabled","value":" TRUE "}`), context.Background())

	h.UpdateSetting(c)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	requireCerberusKeysValue(t, db, "true")
	assert.Equal(t, 1, applier.count())
	assert.Equal(t, 1, cache.calls)
}

// TestSwitchWrites_RefusedApplyRestoresBothKeys describes a refused apply on either write path: the answer is the
// generic 500 and both Cerberus keys are back to the values they held before the write.
func TestSwitchWrites_RefusedApplyRestoresBothKeys(t *testing.T) {
	tests := []struct {
		name string
		path string
		body string
	}{
		{name: "settings", path: "/settings", body: `{"key":"feature.cerberus.enabled","value":"true"}`},
		{name: "bulk config", path: "/config", body: `{"security":{"cerberus":{"enabled":"true"}}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newSwitchWriteDB(t)
			require.NoError(t, db.Create(&models.Setting{Key: models.CerberusLegacySettingKey, Value: "false", Category: "security", Type: "bool"}).Error)
			applier := &recordingApplier{err: errors.New("apply refused")}
			h := NewSettingsHandlerWithDeps(db, applier, &countingInvalidator{}, nil, "")
			c, w := proxyApplyAdminContext(http.MethodPost, tt.path, []byte(tt.body), context.Background())

			if tt.path == "/config" {
				h.PatchConfig(c)
			} else {
				h.UpdateSetting(c)
			}

			require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
			assert.Contains(t, w.Body.String(), msgModuleApplyFailed)
			assert.Equal(t, 1, applier.count())
			_, found := storedSettingValue(t, db, models.CerberusFeatureSettingKey)
			assert.False(t, found, "the feature key that was absent is removed again")
			legacy, found := storedSettingValue(t, db, models.CerberusLegacySettingKey)
			require.True(t, found)
			assert.Equal(t, "false", legacy, "the legacy key is restored to its value before the write")
		})
	}
}

// TestSwitchWrites_StorageFailureAnswers500 describes a write that cannot store the switch: the answer is the generic
// save failure and the proxy is not applied.
func TestSwitchWrites_StorageFailureAnswers500(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))
	applier := &recordingApplier{}
	h := NewSettingsHandlerWithDeps(db, applier, nil, nil, "")
	c, w := proxyApplyAdminContext(http.MethodPost, "/settings", []byte(`{"key":"feature.cerberus.enabled","value":"true"}`), context.Background())

	h.UpdateSetting(c)

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "Failed to save setting")
	assert.Equal(t, 0, applier.count())
}

// TestFeatureFlagsCerberus_WritesBothKeysAndAppliesOnce describes the System Settings toggle: the Cerberus flag goes
// through the switch writer, so both keys change, the cache is invalidated and the proxy is applied once.
func TestFeatureFlagsCerberus_WritesBothKeysAndAppliesOnce(t *testing.T) {
	db := newSwitchWriteDB(t)
	applier := &recordingApplier{}
	cache := &countingInvalidator{}
	h := NewFeatureFlagsHandler(db, applier, cache)
	c, w := proxyApplyAdminContext(http.MethodPut, "/feature-flags", []byte(`{"feature.cerberus.enabled":true}`), context.Background())

	h.UpdateFlags(c)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	requireCerberusKeysValue(t, db, "true")
	assert.Equal(t, 1, applier.count())
	assert.Equal(t, 1, cache.calls)
}

// TestFeatureFlagsCerberus_BatchKeepsOtherFlags describes a batch with the Cerberus flag and another flag: the other
// flag is still stored in the batch's transaction.
func TestFeatureFlagsCerberus_BatchKeepsOtherFlags(t *testing.T) {
	db := newSwitchWriteDB(t)
	h := NewFeatureFlagsHandler(db, nil, nil)
	c, w := proxyApplyAdminContext(http.MethodPut, "/feature-flags", []byte(`{"feature.cerberus.enabled":false,"feature.uptime.enabled":false}`), context.Background())

	h.UpdateFlags(c)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	requireCerberusKeysValue(t, db, "false")
	uptime, found := storedSettingValue(t, db, "feature.uptime.enabled")
	require.True(t, found)
	assert.Equal(t, "false", uptime)
}

// TestFeatureFlagsCerberus_RefusedApplyAnswers500AndRestores describes a refused proxy apply from the toggle: the answer
// is the generic 500 and the Cerberus keys are restored.
func TestFeatureFlagsCerberus_RefusedApplyAnswers500AndRestores(t *testing.T) {
	db := newSwitchWriteDB(t)
	applier := &recordingApplier{err: errors.New("apply refused")}
	h := NewFeatureFlagsHandler(db, applier, nil)
	c, w := proxyApplyAdminContext(http.MethodPut, "/feature-flags", []byte(`{"feature.cerberus.enabled":true}`), context.Background())

	h.UpdateFlags(c)

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), msgModuleApplyFailed)
	requireCerberusKeysAbsent(t, db)
}

// TestFeatureFlagsCerberus_StorageFailureAnswers500 describes a flag write that cannot store: the answer is the generic
// flag save failure.
func TestFeatureFlagsCerberus_StorageFailureAnswers500(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))
	h := NewFeatureFlagsHandler(db, nil, nil)
	c, w := proxyApplyAdminContext(http.MethodPut, "/feature-flags", []byte(`{"feature.cerberus.enabled":true}`), context.Background())

	h.UpdateFlags(c)

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "Failed to update feature flags")
}

// TestSwitchWriter_ApplyErrorIsTheSentinel describes the writer's error contract: a refused apply is errSwitchApplyFailed.
func TestSwitchWriter_ApplyErrorIsTheSentinel(t *testing.T) {
	db := newSwitchWriteDB(t)
	w := switchWriter{db: db, caddy: &recordingApplier{err: errors.New("apply refused")}}

	_, err := w.write(context.Background(), models.CerberusFeatureSettingKey, true, "test")

	require.ErrorIs(t, err, errSwitchApplyFailed)
}

// TestFeatureFlagsCerberus_RefusedApplyRestoresTheWholeBatch describes a batch with the Cerberus flag and another flag
// when the proxy refuses the apply: the other flag is put back with the switch, and the answer is the generic apply
// failure.
func TestFeatureFlagsCerberus_RefusedApplyRestoresTheWholeBatch(t *testing.T) {
	db := newSwitchWriteDB(t)
	require.NoError(t, db.Create(&models.Setting{Key: "feature.uptime.enabled", Value: "true", Category: "feature", Type: "bool"}).Error)
	applier := &recordingApplier{err: errors.New("apply refused")}
	h := NewFeatureFlagsHandler(db, applier, nil)
	c, w := proxyApplyAdminContext(http.MethodPut, "/feature-flags",
		[]byte(`{"feature.cerberus.enabled":true,"feature.uptime.enabled":false}`), context.Background())

	h.UpdateFlags(c)

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), msgModuleApplyFailed)
	requireCerberusKeysAbsent(t, db)
	uptime, found := storedSettingValue(t, db, "feature.uptime.enabled")
	require.True(t, found)
	assert.Equal(t, "true", uptime, "the other flag is restored with the switch")
}

// TestFeatureFlagsCerberus_FailedFlagWriteStoresNothing describes a batch with the Cerberus flag when a flag write fails
// after the switch was written: the transaction rolls back, so the switch is not stored, and the proxy is not applied.
func TestFeatureFlagsCerberus_FailedFlagWriteStoresNothing(t *testing.T) {
	db := newSwitchWriteDB(t)
	gormfault.Inject(t, db, gormfault.Query, "settings", whereEqualsKey("feature.uptime.enabled"))
	applier := &recordingApplier{}
	h := NewFeatureFlagsHandler(db, applier, nil)
	c, w := proxyApplyAdminContext(http.MethodPut, "/feature-flags",
		[]byte(`{"feature.cerberus.enabled":true,"feature.uptime.enabled":false}`), context.Background())

	h.UpdateFlags(c)

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "Failed to update feature flags")
	requireCerberusKeysAbsent(t, db)
	assert.Equal(t, 0, applier.count())
}
