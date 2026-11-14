package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// crowdsecStartState is the state a CrowdSec Start may change, as the lifecycle tests compare it: the value and category
// of each stored switch key, and the default row's enabled flag and CrowdSec mode. Update times are not compared, so a
// revert that writes the same values compares equal.
type crowdsecStartState struct {
	switches   map[string]string // switch key -> "value|category", for each key that is stored
	rowExists  bool
	rowEnabled bool
	rowMode    string
}

// crowdsecStartStateOf reads the state a Start may change.
func crowdsecStartStateOf(t *testing.T, db *gorm.DB) crowdsecStartState {
	t.Helper()
	state := crowdsecStartState{switches: map[string]string{}}
	for _, key := range []string{models.CrowdSecEnabledSettingKey, models.CerberusFeatureSettingKey, models.CerberusLegacySettingKey} {
		var setting models.Setting
		err := db.Where("key = ?", key).First(&setting).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		require.NoError(t, err)
		state.switches[key] = setting.Value + "|" + setting.Category
	}

	var cfg models.SecurityConfig
	err := db.Where("name = ?", models.DefaultSecurityConfigName).First(&cfg).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return state
	}
	require.NoError(t, err)
	state.rowExists = true
	state.rowEnabled = cfg.Enabled
	state.rowMode = cfg.CrowdSecMode
	return state
}

// TestStartCrowdSec_EngineExitOnAbsentRowRevertsToAbsentRow: a Start on an instance with no default row creates it, and
// a failed Start deletes it again, so the state is the one before Start.
func TestStartCrowdSec_EngineExitOnAbsentRowRevertsToAbsentRow(t *testing.T) {
	eng := &exitScriptEngine{running: []bool{false}}
	h := newResetLifecycleHandler(t, eng, &lifecycleLAPIFake{readyAt: 0}, &recordingApplier{})
	require.NoError(t, h.DB.Where("name = ?", models.DefaultSecurityConfigName).Delete(&models.SecurityConfig{}).Error)
	before := crowdsecStartStateOf(t, h.DB)
	require.False(t, before.rowExists)

	_, err := h.StartCrowdSec(context.Background())

	require.ErrorIs(t, err, errCrowdsecEngineExited)
	after := crowdsecStartStateOf(t, h.DB)
	require.False(t, after.rowExists, "a row Start created is removed again")
	require.Equal(t, before, after)
}
