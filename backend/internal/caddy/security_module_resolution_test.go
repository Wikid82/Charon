package caddy

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

// openModuleResolutionDB opens an in-memory database with the settings and security config tables.
func openModuleResolutionDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}))
	return db
}

// The proxy resolves each module as: recognised settings key, then the default security config row,
// then the static configuration. Only the "true" and "false" key values are recognised.

// TestSecurityModuleResolution_KeyTrueEnablesWAFOverDisabledRow pins that a "true" key turns the WAF on
// for the proxy when the default row has the WAF disabled.
func TestSecurityModuleResolution_KeyTrueEnablesWAFOverDisabledRow(t *testing.T) {
	db := openModuleResolutionDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, Enabled: true, WAFMode: "disabled"}).Error)
	require.NoError(t, db.Create(&models.Setting{Key: "security.waf.enabled", Value: "true", Category: "security", Type: "bool"}).Error)

	manager := NewManager(nil, db, "", "", false, config.SecurityConfig{CerberusEnabled: true})
	_, _, wafEnabled, _, _ := manager.computeEffectiveFlags(context.Background())
	require.True(t, wafEnabled)
}

// TestSecurityModuleResolution_KeyFalseDisablesWAFOverBlockRow pins that a "false" key turns the WAF off
// for the proxy when the default row has the WAF in blocking mode.
func TestSecurityModuleResolution_KeyFalseDisablesWAFOverBlockRow(t *testing.T) {
	db := openModuleResolutionDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, Enabled: true, WAFMode: "block"}).Error)
	require.NoError(t, db.Create(&models.Setting{Key: "security.waf.enabled", Value: "false", Category: "security", Type: "bool"}).Error)

	manager := NewManager(nil, db, "", "", false, config.SecurityConfig{CerberusEnabled: true})
	_, _, wafEnabled, _, _ := manager.computeEffectiveFlags(context.Background())
	require.False(t, wafEnabled)
}

// TestSecurityModuleResolution_UnrecognisedKeyValueFallsThroughToRow pins that a key value other than
// "true" or "false" is not used, so the default row decides.
func TestSecurityModuleResolution_UnrecognisedKeyValueFallsThroughToRow(t *testing.T) {
	db := openModuleResolutionDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, Enabled: true, WAFMode: "block"}).Error)
	require.NoError(t, db.Create(&models.Setting{Key: "security.waf.enabled", Value: "maybe", Category: "security", Type: "bool"}).Error)

	manager := NewManager(nil, db, "", "", false, config.SecurityConfig{CerberusEnabled: true})
	_, _, wafEnabled, _, _ := manager.computeEffectiveFlags(context.Background())
	require.True(t, wafEnabled)
}

// TestSecurityModuleResolution_RateLimitKeyFalseOverridesEnabledRow pins that a "false" rate limit key
// turns the rate limit off for the proxy when the default row has it enabled.
func TestSecurityModuleResolution_RateLimitKeyFalseOverridesEnabledRow(t *testing.T) {
	db := openModuleResolutionDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, Enabled: true, RateLimitMode: "enabled", RateLimitEnable: true}).Error)
	require.NoError(t, db.Create(&models.Setting{Key: "security.rate_limit.enabled", Value: "false", Category: "security", Type: "bool"}).Error)

	manager := NewManager(nil, db, "", "", false, config.SecurityConfig{CerberusEnabled: true})
	_, _, _, rateLimitEnabled, _ := manager.computeEffectiveFlags(context.Background())
	require.False(t, rateLimitEnabled)
}

// TestSecurityModuleResolution_AbsentRowUsesStaticWAFMode pins that without a default row the static
// WAF mode decides, and that a "true" key still turns the WAF on over a disabled static mode.
func TestSecurityModuleResolution_AbsentRowUsesStaticWAFMode(t *testing.T) {
	db := openModuleResolutionDB(t)

	manager := NewManager(nil, db, "", "", false, config.SecurityConfig{CerberusEnabled: true, WAFMode: "block"})
	_, _, wafEnabled, _, _ := manager.computeEffectiveFlags(context.Background())
	require.True(t, wafEnabled, "static block mode with no row")

	disabledStatic := NewManager(nil, db, "", "", false, config.SecurityConfig{CerberusEnabled: true, WAFMode: "disabled"})
	require.NoError(t, db.Create(&models.Setting{Key: "security.waf.enabled", Value: "true", Category: "security", Type: "bool"}).Error)
	_, _, wafEnabled, _, _ = disabledStatic.computeEffectiveFlags(context.Background())
	require.True(t, wafEnabled, "true key over disabled static mode with no row")
}

// TestSecurityModuleResolution_CerberusOffForcesModulesOff pins that Cerberus off keeps every module off, even when
// the settings keys say on and the default row enables them.
func TestSecurityModuleResolution_CerberusOffForcesModulesOff(t *testing.T) {
	db := openModuleResolutionDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, Enabled: false, WAFMode: "block", RateLimitMode: "enabled"}).Error)
	for _, key := range []string{models.SecurityWAFEnabledSettingKey, models.SecurityRateLimitEnabledSettingKey, models.SecurityACLEnabledSettingKey} {
		require.NoError(t, db.Create(&models.Setting{Key: key, Value: "true", Category: "security", Type: "bool"}).Error)
	}

	manager := NewManager(nil, db, "", "", false, config.SecurityConfig{CerberusEnabled: false, ACLMode: "enabled"})
	cerbEnabled, aclEnabled, wafEnabled, rateLimitEnabled, _ := manager.computeEffectiveFlags(context.Background())
	require.False(t, cerbEnabled)
	require.False(t, aclEnabled)
	require.False(t, wafEnabled)
	require.False(t, rateLimitEnabled)
}

// TestSecurityModuleResolution_ACLKeyEnablesOverStatic pins that a "true" ACL key turns ACL on over a static
// disabled mode, while Cerberus stays on.
func TestSecurityModuleResolution_ACLKeyEnablesOverStatic(t *testing.T) {
	db := openModuleResolutionDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, Enabled: true}).Error)
	require.NoError(t, db.Create(&models.Setting{Key: models.SecurityACLEnabledSettingKey, Value: " True ", Category: "security", Type: "bool"}).Error)

	manager := NewManager(nil, db, "", "", false, config.SecurityConfig{CerberusEnabled: true, ACLMode: "disabled"})
	cerbEnabled, aclEnabled, _, _, _ := manager.computeEffectiveFlags(context.Background())
	require.True(t, cerbEnabled)
	require.True(t, aclEnabled)
}

// TestSecurityModuleResolution_SettingsReadFailureUsesRowAndStatic pins that a failed settings read does not change
// the other sources: the WAF follows the default row and the rate limit follows the static configuration.
func TestSecurityModuleResolution_SettingsReadFailureUsesRowAndStatic(t *testing.T) {
	db := openModuleResolutionDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, Enabled: true, WAFMode: "block"}).Error)
	failSettingsQueries(db)

	manager := NewManager(nil, db, "", "", false, config.SecurityConfig{CerberusEnabled: true, RateLimitMode: "enabled"})
	_, aclEnabled, wafEnabled, rateLimitEnabled, _ := manager.computeEffectiveFlags(context.Background())
	require.True(t, wafEnabled, "row block mode decides when the settings read fails")
	require.True(t, rateLimitEnabled, "static rate limit decides when the settings read fails")
	require.False(t, aclEnabled)
}

// failSettingsQueries makes every query against the settings table fail.
func failSettingsQueries(db *gorm.DB) {
	_ = db.Callback().Query().Before("gorm:query").Register("test:fail_settings", func(tx *gorm.DB) {
		if tx.Statement.Table == "settings" {
			_ = tx.AddError(fmt.Errorf("forced settings failure"))
		}
	})
}

// TestEnforcementReport_ReasonsAndCounts describes the enforcement report the proxy stores after an apply:
// reasons no_ruleset, ruleset_not_selected, ruleset_unavailable and thresholds_unset, counts for eligible
// and protected hosts, and zero eligible hosts (enforced omitted).
func TestEnforcementReport_ReasonsAndCounts(t *testing.T) {
	t.Skip("enabled in commit 3 of this PR: enforcement report with reasons and counts")
}

// TestEnforcementReport_WAFDisabledHostsNotEligible describes hosts with the WAF switched off for their own
// entry: they are excluded from the WAF eligible count.
func TestEnforcementReport_WAFDisabledHostsNotEligible(t *testing.T) {
	t.Skip("enabled in commit 3 of this PR: enforcement report with reasons and counts")
}

// TestBuildWAFHandler_KeyOnWithDisabledRowBuildsHandler describes the WAF handler being generated when the
// WAF key is on and the default row still holds the disabled mode, with the legacy veto removed.
func TestBuildWAFHandler_KeyOnWithDisabledRowBuildsHandler(t *testing.T) {
	t.Skip("enabled in commit 4 of this PR: removal of the row veto in buildWAFHandler")
}

// TestApplyConfig_StartupReconcileMonitorMode describes the start-up step that moves a WAF key that is on
// with a disabled or empty row to monitor mode, before the first apply.
func TestApplyConfig_StartupReconcileMonitorMode(t *testing.T) {
	t.Skip("enabled in commit 4 of this PR: start-up reconciliation of the WAF mode")
}

// TestBuildRateLimitHandler_UnsetThresholdsReportsReason describes an enabled rate limit with zero thresholds:
// no handler is built and the enforcement report carries reason thresholds_unset.
func TestBuildRateLimitHandler_UnsetThresholdsReportsReason(t *testing.T) {
	t.Skip("enabled in commit 5 of this PR: unset rate limit thresholds are reported")
}

// TestBuildWAFDirectives_UnresolvedRulesetReportsReason describes the WAF handler being dropped when no
// ruleset resolves, with the reason recorded in the enforcement report.
func TestBuildWAFDirectives_UnresolvedRulesetReportsReason(t *testing.T) {
	t.Skip("enabled in commit 3 of this PR: enforcement report with reasons and counts")
}
