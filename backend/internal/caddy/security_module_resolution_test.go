package caddy

import (
	"context"
	"encoding/json"
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
	twoHosts := []models.ProxyHost{
		enforcementTestHost("first", "first.example.com", false),
		enforcementTestHost("second", "second.example.com", false),
	}
	owasp := []models.SecurityRuleSet{{Name: "owasp-crs"}}
	owaspPath := map[string]string{"owasp-crs": "/data/coraza/owasp-crs.conf"}
	blockRow := &models.SecurityConfig{WAFMode: "block"}
	thresholds := &models.SecurityConfig{WAFMode: "block", RateLimitRequests: 100, RateLimitWindowSec: 60, RateLimitBurst: 20}
	disabledHost := enforcementTestHost("off", "off.example.com", false)
	disabledHost.Enabled = false

	cases := []struct {
		name     string
		hosts    []models.ProxyHost
		wafOn    bool
		rlOn     bool
		rulesets []models.SecurityRuleSet
		paths    map[string]string
		secCfg   *models.SecurityConfig
		wantWAF  ModuleEnforcement
		wantRL   ModuleEnforcement
	}{
		{
			name: "no ruleset", hosts: twoHosts, wafOn: true, secCfg: blockRow,
			wantWAF: ModuleEnforcement{Eligible: 2, Reason: ReasonNoRuleset},
		},
		{
			name: "ruleset not selected", hosts: twoHosts, wafOn: true, secCfg: blockRow,
			rulesets: []models.SecurityRuleSet{{Name: "alpha"}, {Name: "beta"}},
			paths:    map[string]string{"alpha": "/data/a.conf", "beta": "/data/b.conf"},
			wantWAF:  ModuleEnforcement{Eligible: 2, Reason: ReasonRulesetNotSelected},
		},
		{
			name: "selected ruleset has no file", hosts: twoHosts, wafOn: true, secCfg: blockRow,
			rulesets: owasp, paths: map[string]string{},
			wantWAF: ModuleEnforcement{Eligible: 2, Reason: ReasonRulesetUnavailable},
		},
		{
			name: "every host protected", hosts: twoHosts, wafOn: true, rlOn: true, secCfg: thresholds,
			rulesets: owasp, paths: owaspPath,
			wantWAF: ModuleEnforcement{Eligible: 2, Protected: 2},
			wantRL:  ModuleEnforcement{Eligible: 2, Protected: 2},
		},
		{
			name: "rate limit without thresholds", hosts: twoHosts, rlOn: true, secCfg: &models.SecurityConfig{RateLimitMode: "enabled"},
			wantRL: ModuleEnforcement{Eligible: 2, Reason: ReasonThresholdsUnset},
		},
		{
			name: "disabled row mode with the WAF key on", hosts: twoHosts, wafOn: true,
			rulesets: owasp, paths: owaspPath, secCfg: &models.SecurityConfig{WAFMode: "disabled"},
			wantWAF: ModuleEnforcement{Eligible: 2, Protected: 2},
		},
		{
			name: "no enabled host", hosts: []models.ProxyHost{disabledHost},
			wafOn: true, rlOn: true, rulesets: owasp, paths: owaspPath, secCfg: thresholds,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report := generateEnforcementReport(t, tc.hosts, tc.wafOn, tc.rlOn, tc.rulesets, tc.paths, tc.secCfg)
			require.Equal(t, tc.wafOn, report.WAFOn)
			require.Equal(t, tc.rlOn, report.RateLimitOn)
			require.Equal(t, tc.wantWAF, report.WAF)
			require.Equal(t, tc.wantRL, report.RateLimit)
		})
	}
}

// TestEnforcementReport_WAFDisabledHostsNotEligible describes hosts with the WAF switched off for their own
// entry: they are excluded from the WAF eligible count.
func TestEnforcementReport_WAFDisabledHostsNotEligible(t *testing.T) {
	owasp := []models.SecurityRuleSet{{Name: "owasp-crs"}}
	owaspPath := map[string]string{"owasp-crs": "/data/coraza/owasp-crs.conf"}
	block := &models.SecurityConfig{WAFMode: "block"}

	mixed := []models.ProxyHost{
		enforcementTestHost("opted-out", "opted-out.example.com", true),
		enforcementTestHost("covered", "covered.example.com", false),
	}
	report := generateEnforcementReport(t, mixed, true, false, owasp, owaspPath, block)
	require.Equal(t, ModuleEnforcement{Eligible: 1, Protected: 1}, report.WAF, "the host with the WAF off for itself is not counted")
	require.True(t, report.WAF.Enforced())

	onlyOptedOut := []models.ProxyHost{enforcementTestHost("opted-out", "opted-out.example.com", true)}
	report = generateEnforcementReport(t, onlyOptedOut, true, false, owasp, owaspPath, block)
	require.Equal(t, ModuleEnforcement{}, report.WAF, "no eligible host leaves enforced omitted")
	require.False(t, report.WAF.Enforced())
}

// TestBuildWAFHandler_KeyOnRowDisabledBuildsHandler describes the WAF handler being generated when the
// WAF key is on and the default row holds the disabled mode, with no row-mode gate.
func TestBuildWAFHandler_KeyOnRowDisabledBuildsHandler(t *testing.T) {
	host := &models.ProxyHost{UUID: "keyon-host"}
	owasp := []models.SecurityRuleSet{{Name: "owasp-crs"}}
	paths := map[string]string{"owasp-crs": "/data/owasp-crs.conf"}

	handler, reason := buildWAFHandlerWithReason(host, owasp, paths, &models.SecurityConfig{WAFMode: "disabled"})
	require.NotNil(t, handler, "the WAF key decides; a disabled row mode does not stop the handler being built")
	require.Empty(t, reason)
}

// TestApplyConfig_StartupReconcileMonitorMode describes the start-up step that moves a WAF key that is on
// with a disabled or empty row to monitor mode, before the first apply.
func TestApplyConfig_StartupReconcileMonitorMode(t *testing.T) {
	db := openEnforcementApplyDB(t)
	// The WAF key is on and the row holds the disabled mode.
	require.NoError(t, db.Model(&models.SecurityConfig{}).Where("name = ?", models.DefaultSecurityConfigName).Update("waf_mode", "disabled").Error)
	require.NoError(t, db.Create(&models.Setting{Key: models.SecurityWAFEnabledSettingKey, Value: "true", Category: "security", Type: "bool"}).Error)

	changed, err := models.ReconcileWAFDetectionMode(db)
	require.NoError(t, err)
	require.Equal(t, int64(1), changed)

	client := &recordingCaddyClient{}
	manager := NewManager(client, db, t.TempDir(), "", false, config.SecurityConfig{CerberusEnabled: true})
	require.NoError(t, manager.ApplyConfig(context.Background()))
	body, err := json.Marshal(client.last)
	require.NoError(t, err)
	require.Contains(t, string(body), "SecRuleEngine DetectionOnly", "the reconciled row runs the WAF in detection mode")

	report, ok := manager.EnforcementReport()
	require.True(t, ok)
	require.True(t, report.WAF.Enforced())

	var stored models.SecurityConfig
	require.NoError(t, db.Where("name = ?", models.DefaultSecurityConfigName).First(&stored).Error)
	require.Equal(t, "monitor", stored.WAFMode)

	again, err := models.ReconcileWAFDetectionMode(db)
	require.NoError(t, err)
	require.Zero(t, again, "a second start-up changes nothing")
}

// recordingCaddyClient keeps the last configuration it was asked to load.
type recordingCaddyClient struct {
	last *Config
}

func (c *recordingCaddyClient) Load(_ context.Context, cfg *Config) error {
	c.last = cfg
	return nil
}

func (c *recordingCaddyClient) Ping(context.Context) error { return nil }

func (c *recordingCaddyClient) GetConfig(context.Context) (*Config, error) {
	return &Config{}, nil
}

// TestBuildRateLimitHandler_UnsetThresholdsReportsReason describes an enabled rate limit with zero thresholds:
// no handler is built and the enforcement report carries reason thresholds_unset.
func TestBuildRateLimitHandler_UnsetThresholdsReportsReason(t *testing.T) {
	host := models.ProxyHost{UUID: "unset-host", DomainNames: "unset.example.com", ForwardScheme: "http", ForwardHost: "upstream.internal", ForwardPort: 8080, Enabled: true}

	for _, row := range []*models.SecurityConfig{
		{RateLimitRequests: 0, RateLimitWindowSec: 0},
		{RateLimitRequests: 50, RateLimitWindowSec: 0},
		{RateLimitRequests: 0, RateLimitWindowSec: 60},
	} {
		require.Nil(t, buildRateLimitHandler(&host, row), "a zero request count or window builds no handler")
	}

	report := generateEnforcementReport(t, []models.ProxyHost{host}, false, true, nil, nil, &models.SecurityConfig{RateLimitMode: "enabled"})
	require.Equal(t, ModuleEnforcement{Eligible: 1, Reason: ReasonThresholdsUnset}, report.RateLimit)
	require.False(t, report.RateLimit.Enforced())
}

// TestBuildWAFDirectives_UnresolvedRulesetReportsReason describes the WAF handler being dropped when no
// ruleset resolves, with the reason recorded in the enforcement report.
func TestBuildWAFDirectives_UnresolvedRulesetReportsReason(t *testing.T) {
	host := &models.ProxyHost{UUID: "reason-host"}
	block := &models.SecurityConfig{WAFMode: "block"}
	owasp := []models.SecurityRuleSet{{Name: "owasp-crs"}}

	handler, reason := buildWAFHandlerWithReason(host, nil, nil, block)
	require.Nil(t, handler)
	require.Equal(t, ReasonNoRuleset, reason)

	two := []models.SecurityRuleSet{{Name: "alpha"}, {Name: "beta"}}
	handler, reason = buildWAFHandlerWithReason(host, two, map[string]string{"alpha": "/data/a.conf", "beta": "/data/b.conf"}, block)
	require.Nil(t, handler)
	require.Equal(t, ReasonRulesetNotSelected, reason)

	handler, reason = buildWAFHandlerWithReason(host, owasp, map[string]string{}, block)
	require.Nil(t, handler)
	require.Equal(t, ReasonRulesetUnavailable, reason)

	handler, reason = buildWAFHandlerWithReason(host, owasp, map[string]string{"owasp-crs": "/data/owasp-crs.conf"}, block)
	require.NotNil(t, handler)
	require.Empty(t, reason)
}

// enforcementTestHost returns an enabled proxy host with the given UUID and domain.
func enforcementTestHost(uuid, domain string, wafDisabled bool) models.ProxyHost {
	return models.ProxyHost{
		UUID:          uuid,
		DomainNames:   domain,
		ForwardScheme: "http",
		ForwardHost:   "upstream.internal",
		ForwardPort:   8080,
		Enabled:       true,
		WAFDisabled:   wafDisabled,
	}
}

// generateEnforcementReport runs GenerateConfig with the given module state and returns the single report it hands to
// the sink.
func generateEnforcementReport(t *testing.T, hosts []models.ProxyHost, wafOn, rateLimitOn bool, rulesets []models.SecurityRuleSet, paths map[string]string, secCfg *models.SecurityConfig) EnforcementReport {
	t.Helper()
	var reports []EnforcementReport
	_, err := GenerateConfig(hosts, t.TempDir(), "", "", "", false, false, wafOn, rateLimitOn, false, "", rulesets, paths, nil, secCfg, nil,
		WithEnforcementSink(func(r EnforcementReport) { reports = append(reports, r) }))
	require.NoError(t, err)
	require.Len(t, reports, 1, "the sink is called exactly once per generation")
	return reports[0]
}
