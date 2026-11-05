package caddy

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// ownHandlerRulesetPaths resolves one per-host ruleset name. The global source is an empty set with no path.
var ownHandlerRulesetPaths = map[string]string{"custom-set": "/path/custom.conf"}

// ownHandlerGlobalEmpty is a WAF whose global source is an empty set, so no global handler is built for a host.
func ownHandlerGlobalEmpty() ([]models.SecurityRuleSet, *models.SecurityConfig) {
	rulesets := []models.SecurityRuleSet{{Name: "empty-set", Content: ""}}
	return rulesets, &models.SecurityConfig{WAFMode: "block", WAFRulesSource: "empty-set"}
}

func ownHandlerHost(advancedConfig string) []models.ProxyHost {
	return []models.ProxyHost{{
		UUID:           "own-waf",
		DomainNames:    "own-waf.example.com",
		Enabled:        true,
		ForwardHost:    "app",
		ForwardPort:    8080,
		AdvancedConfig: advancedConfig,
	}}
}

// collectWAFHandlers returns every handler with handler "waf" found anywhere in the generated config JSON.
func collectWAFHandlers(t *testing.T, cfg *Config) []map[string]any {
	t.Helper()
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	var doc any
	require.NoError(t, json.Unmarshal(raw, &doc))
	var found []map[string]any
	var walk func(v any)
	walk = func(v any) {
		switch n := v.(type) {
		case map[string]any:
			if n["handler"] == "waf" {
				found = append(found, n)
			}
			for _, child := range n {
				walk(child)
			}
		case []any:
			for _, child := range n {
				walk(child)
			}
		}
	}
	walk(doc)
	return found
}

func TestHostHasOwnWAFHandler(t *testing.T) {
	tests := []struct {
		name           string
		advancedConfig string
		want           bool
	}{
		{name: "map ruleset_name resolves", advancedConfig: `{"handler":"waf","ruleset_name":"custom-set"}`, want: true},
		{name: "array ruleset_name resolves", advancedConfig: `[{"handler":"headers"},{"handler":"waf","ruleset_name":"custom-set"}]`, want: true},
		{name: "map explicit directives", advancedConfig: `{"handler":"waf","directives":"SecRuleEngine On"}`, want: true},
		{name: "array explicit directives", advancedConfig: `[{"handler":"waf","directives":"SecRuleEngine On"}]`, want: true},
		{name: "blank directives do not count", advancedConfig: `{"handler":"waf","directives":"   "}`, want: false},
		{name: "unresolvable ruleset_name", advancedConfig: `{"handler":"waf","ruleset_name":"missing-set"}`, want: false},
		{name: "no directives and no ruleset_name", advancedConfig: `{"handler":"waf"}`, want: false},
		{name: "non waf handler with resolvable ruleset_name", advancedConfig: `{"handler":"headers","ruleset_name":"custom-set"}`, want: false},
		{name: "array non waf handler only", advancedConfig: `[{"handler":"headers","directives":"SecRuleEngine On"}]`, want: false},
		{name: "empty advanced_config", advancedConfig: ``, want: false},
		{name: "invalid JSON", advancedConfig: `{bad`, want: false},
		{name: "unexpected scalar JSON", advancedConfig: `"waf"`, want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			host := &models.ProxyHost{UUID: "h", AdvancedConfig: tc.advancedConfig}
			assert.Equal(t, tc.want, hostHasOwnWAFHandler(host, ownHandlerRulesetPaths))
		})
	}

	t.Run("nil host", func(t *testing.T) {
		assert.False(t, hostHasOwnWAFHandler(nil, ownHandlerRulesetPaths))
	})

	t.Run("empty path does not resolve", func(t *testing.T) {
		host := &models.ProxyHost{AdvancedConfig: `{"handler":"waf","ruleset_name":"custom-set"}`}
		assert.False(t, hostHasOwnWAFHandler(host, map[string]string{"custom-set": ""}))
	})

	t.Run("does not mutate advanced config", func(t *testing.T) {
		const raw = `{"handler":"waf","ruleset_name":"custom-set"}`
		host := &models.ProxyHost{AdvancedConfig: raw}
		require.True(t, hostHasOwnWAFHandler(host, ownHandlerRulesetPaths))
		assert.Equal(t, raw, host.AdvancedConfig)
	})
}

func TestGenerateConfig_OwnWAFHandlerCountsHostProtected(t *testing.T) {
	tests := []struct {
		name           string
		advancedConfig string
	}{
		{name: "map ruleset_name resolves", advancedConfig: `{"handler":"waf","ruleset_name":"custom-set"}`},
		{name: "array ruleset_name resolves", advancedConfig: `[{"handler":"waf","ruleset_name":"custom-set"}]`},
		{name: "explicit directives map", advancedConfig: `{"handler":"waf","directives":"SecRuleEngine On"}`},
		{name: "explicit directives array", advancedConfig: `[{"handler":"waf","directives":"SecRuleEngine On"}]`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rulesets, secCfg := ownHandlerGlobalEmpty()
			report := generateEnforcementReport(t, ownHandlerHost(tc.advancedConfig), true, false, rulesets, ownHandlerRulesetPaths, secCfg)
			assert.Equal(t, 1, report.WAF.Eligible)
			assert.Equal(t, 1, report.WAF.Protected, "a host with its own resolvable WAF handler is protected")
			assert.Empty(t, report.WAF.Reason)
			assert.True(t, report.WAF.Enforced())
		})
	}
}

func TestGenerateConfig_OwnWAFHandlerGeneratedConfigUnchanged(t *testing.T) {
	t.Run("ruleset_name map form adds only the Include handler", func(t *testing.T) {
		rulesets, secCfg := ownHandlerGlobalEmpty()
		cfg, err := GenerateConfig(ownHandlerHost(`{"handler":"waf","ruleset_name":"custom-set"}`), t.TempDir(), "", "", "", false, false, true, false, false, "", rulesets, ownHandlerRulesetPaths, nil, secCfg, nil)
		require.NoError(t, err)
		// No global handler is added. Every WAF handler is the host's own, with the Include and no ruleset_name key.
		assertOnlyWAFHandlers(t, cfg, map[string]any{"handler": "waf", "directives": "Include /path/custom.conf"})
	})

	t.Run("explicit directives are passed through untouched", func(t *testing.T) {
		rulesets, secCfg := ownHandlerGlobalEmpty()
		cfg, err := GenerateConfig(ownHandlerHost(`{"handler":"waf","directives":"SecRuleEngine On"}`), t.TempDir(), "", "", "", false, false, true, false, false, "", rulesets, ownHandlerRulesetPaths, nil, secCfg, nil)
		require.NoError(t, err)
		assertOnlyWAFHandlers(t, cfg, map[string]any{"handler": "waf", "directives": "SecRuleEngine On"})
	})

	t.Run("unresolvable ruleset_name adds no WAF handler", func(t *testing.T) {
		rulesets, secCfg := ownHandlerGlobalEmpty()
		cfg, err := GenerateConfig(ownHandlerHost(`{"handler":"waf","ruleset_name":"missing-set"}`), t.TempDir(), "", "", "", false, false, true, false, false, "", rulesets, ownHandlerRulesetPaths, nil, secCfg, nil)
		require.NoError(t, err)
		// The host's raw handler is still emitted as before, but no directives are added to it.
		assertOnlyWAFHandlers(t, cfg, map[string]any{"handler": "waf"})
	})
}

func TestGenerateConfig_OwnWAFHandlerNotProtectedKeepsReason(t *testing.T) {
	tests := []struct {
		name           string
		advancedConfig string
	}{
		{name: "unresolvable ruleset_name", advancedConfig: `{"handler":"waf","ruleset_name":"missing-set"}`},
		{name: "waf handler with no directives", advancedConfig: `{"handler":"waf"}`},
		{name: "array unresolvable ruleset_name", advancedConfig: `[{"handler":"waf","ruleset_name":"missing-set"}]`},
		{name: "non waf handler with resolvable ruleset_name", advancedConfig: `{"handler":"static_response","ruleset_name":"custom-set"}`},
		{name: "array non waf handler only", advancedConfig: `[{"handler":"headers","directives":"SecRuleEngine On"}]`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rulesets, secCfg := ownHandlerGlobalEmpty()
			report := generateEnforcementReport(t, ownHandlerHost(tc.advancedConfig), true, false, rulesets, ownHandlerRulesetPaths, secCfg)
			assert.Equal(t, 1, report.WAF.Eligible)
			assert.Equal(t, 0, report.WAF.Protected)
			assert.Equal(t, ReasonRulesetEmpty, report.WAF.Reason, "the global reason is kept")
		})
	}
}

// assertOnlyWAFHandlers asserts that the config holds at least one WAF handler and that each one equals want.
func assertOnlyWAFHandlers(t *testing.T, cfg *Config, want map[string]any) {
	t.Helper()
	handlers := collectWAFHandlers(t, cfg)
	require.NotEmpty(t, handlers)
	for _, h := range handlers {
		assert.Equal(t, want, h)
	}
}
