package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// TestApplyUpdate_ClearedRateLimitModeLeavesRateLimitOff describes a mode-only save that clears the mode: the enable
// flag follows the mode, so the row no longer resolves rate limiting as on. The static decision (off here) applies.
func TestApplyUpdate_ClearedRateLimitModeLeavesRateLimitOff(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)
	seeded := fullUpdateTestRow()
	seeded.RateLimitEnable = true
	seeded.RateLimitMode = "enabled"
	seedUpdateTestRow(t, db, seeded)

	got, _, err := svc.ApplyUpdate("test-actor", models.DefaultSecurityConfigName, decodeUpdateTestBody(t, `{"rate_limit_mode":""}`))
	require.NoError(t, err)

	assert.False(t, got.RateLimitEnable)
	res, err := models.ResolveRateLimit(db, *got, false)
	require.NoError(t, err)
	assert.False(t, res.Enabled, "a cleared mode must not switch rate limiting on")
}

// TestApplyUpdate_ModeOnlyMirrorsTheEnableFlag describes a mode-only save: enabled turns the enable flag on, and any
// other mode turns it off, so the two columns always agree.
func TestApplyUpdate_ModeOnlyMirrorsTheEnableFlag(t *testing.T) {
	tests := []struct {
		name       string
		seedEnable bool
		body       string
		wantEnable bool
	}{
		{name: "enabled turns the flag on", seedEnable: false, body: `{"rate_limit_mode":"enabled"}`, wantEnable: true},
		{name: "disabled turns the flag off", seedEnable: true, body: `{"rate_limit_mode":"disabled"}`, wantEnable: false},
		{name: "empty turns the flag off", seedEnable: true, body: `{"rate_limit_mode":""}`, wantEnable: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupSecurityTestDB(t)
			svc := newTestSecurityService(t, db)
			seeded := fullUpdateTestRow()
			seeded.RateLimitEnable = tt.seedEnable
			seedUpdateTestRow(t, db, seeded)

			got, _, err := svc.ApplyUpdate("test-actor", models.DefaultSecurityConfigName, decodeUpdateTestBody(t, tt.body))
			require.NoError(t, err)

			assert.Equal(t, tt.wantEnable, got.RateLimitEnable)
			assert.Equal(t, tt.wantEnable, loadUpdateTestRow(t, db).RateLimitEnable)
		})
	}
}
