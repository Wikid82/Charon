package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// TestSeedDefaultSecurityConfig_KeepsExistingModeColumns pins that seeding over an existing default row
// returns and keeps the stored module mode columns.
func TestSeedDefaultSecurityConfig_KeepsExistingModeColumns(t *testing.T) {
	db := newSeedTestDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{
		Name:            models.DefaultSecurityConfigName,
		Enabled:         true,
		WAFMode:         "monitor",
		RateLimitMode:   "enabled",
		RateLimitEnable: true,
	}).Error)

	rec, err := models.SeedDefaultSecurityConfig(db)
	require.NoError(t, err)
	assert.Equal(t, "monitor", rec.WAFMode)
	assert.Equal(t, "enabled", rec.RateLimitMode)
	assert.True(t, rec.RateLimitEnable)

	var stored models.SecurityConfig
	require.NoError(t, db.Where("name = ?", models.DefaultSecurityConfigName).First(&stored).Error)
	assert.Equal(t, "monitor", stored.WAFMode)
	assert.Equal(t, "enabled", stored.RateLimitMode)
}

// TestModuleEnabled_TruthTable describes the shared module resolver: a recognised settings key wins, then the
// default row, then the static configuration, and Cerberus off forces every module off. Planned matrix:
//
//	| key        | row (WAF mode / rate-limit enable) | static | Cerberus | expected |
//	|------------|------------------------------------|--------|----------|----------|
//	| "true"     | any                                | any    | on       | on       |
//	| "false"    | any                                | any    | on       | off      |
//	| absent     | block / enabled                    | any    | on       | on       |
//	| absent     | disabled / disabled                | any    | on       | off      |
//	| absent     | empty                              | enabled| on       | on       |
//	| absent     | empty                              | disabled| on      | off      |
//	| "maybe"    | block / enabled                    | any    | on       | on       |
//	| " TRUE "   | any                                | any    | on       | recognised after trimming, case-insensitive |
//	| any        | any                                | any    | off      | off      |
//	| DB error   | any                                | enabled| on       | static, error logged |
func TestModuleEnabled_TruthTable(t *testing.T) {
	t.Skip("enabled in commit 2 of this PR: shared module resolver (models.ModuleEnabled)")
}

// TestModuleEnabled_ThresholdsAndCerberusDoNotChangeKeyResult is reserved for the rate-limit and WAF wrappers
// that call the shared resolver with the row callbacks (commit 2 of this PR).
func TestModuleEnabled_WrapperCallbacksUseSharedRule(t *testing.T) {
	t.Skip("enabled in commit 2 of this PR: shared module resolver (models.ModuleEnabled)")
}
