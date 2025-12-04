package caddy

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// TestBuildWAFHandler_RulesetSelectionPriority verifies the priority order:
// 1. secCfg.WAFRulesSource (user's global choice)
// 2. hostRulesetName from advanced_config
// 3. host.Application
// 4. owasp-crs fallback
func TestBuildWAFHandler_RulesetSelectionPriority(t *testing.T) {
	tests := []struct {
		name            string
		host            *models.ProxyHost
		rulesets        []models.SecurityRuleSet
		rulesetPaths    map[string]string
		secCfg          *models.SecurityConfig
		wafEnabled      bool
		expectedInclude string // Expected substring in directives, empty if handler should be nil
	}{
		{
			name:     "WAFRulesSource takes priority over owasp-crs",
			host:     &models.ProxyHost{UUID: "test-host"},
			rulesets: []models.SecurityRuleSet{{Name: "owasp-crs"}, {Name: "custom-xss"}},
			rulesetPaths: map[string]string{
				"owasp-crs":  "/app/data/rulesets/owasp-crs.conf",
				"custom-xss": "/app/data/rulesets/custom-xss.conf",
			},
			secCfg:          &models.SecurityConfig{WAFMode: "block", WAFRulesSource: "custom-xss"},
			wafEnabled:      true,
			expectedInclude: "custom-xss.conf",
		},
		{
			name: "hostRulesetName takes priority over owasp-crs",
			host: &models.ProxyHost{
				UUID:           "test-host",
				AdvancedConfig: `{"ruleset_name":"per-host-rules"}`,
			},
			rulesets: []models.SecurityRuleSet{{Name: "owasp-crs"}, {Name: "per-host-rules"}},
			rulesetPaths: map[string]string{
				"owasp-crs":      "/app/data/rulesets/owasp-crs.conf",
				"per-host-rules": "/app/data/rulesets/per-host-rules.conf",
			},
			secCfg:          &models.SecurityConfig{WAFMode: "block"},
			wafEnabled:      true,
			expectedInclude: "per-host-rules.conf",
		},
		{
			name: "host.Application takes priority over owasp-crs",
			host: &models.ProxyHost{
				UUID:        "test-host",
				Application: "wordpress",
			},
			rulesets: []models.SecurityRuleSet{{Name: "owasp-crs"}, {Name: "wordpress"}},
			rulesetPaths: map[string]string{
				"owasp-crs": "/app/data/rulesets/owasp-crs.conf",
				"wordpress": "/app/data/rulesets/wordpress.conf",
			},
			secCfg:          &models.SecurityConfig{WAFMode: "block"},
			wafEnabled:      true,
			expectedInclude: "wordpress.conf",
		},
		{
			name:     "owasp-crs used as fallback when no other match",
			host:     &models.ProxyHost{UUID: "test-host"},
			rulesets: []models.SecurityRuleSet{{Name: "owasp-crs"}, {Name: "unrelated-rules"}},
			rulesetPaths: map[string]string{
				"owasp-crs":       "/app/data/rulesets/owasp-crs.conf",
				"unrelated-rules": "/app/data/rulesets/unrelated.conf",
			},
			secCfg:          &models.SecurityConfig{WAFMode: "block"},
			wafEnabled:      true,
			expectedInclude: "owasp-crs.conf",
		},
		{
			name: "WAFRulesSource takes priority over host.Application and owasp-crs",
			host: &models.ProxyHost{
				UUID:        "test-host",
				Application: "wordpress",
			},
			rulesets: []models.SecurityRuleSet{{Name: "owasp-crs"}, {Name: "wordpress"}, {Name: "global-custom"}},
			rulesetPaths: map[string]string{
				"owasp-crs":     "/app/data/rulesets/owasp-crs.conf",
				"wordpress":     "/app/data/rulesets/wordpress.conf",
				"global-custom": "/app/data/rulesets/global-custom.conf",
			},
			secCfg:          &models.SecurityConfig{WAFMode: "block", WAFRulesSource: "global-custom"},
			wafEnabled:      true,
			expectedInclude: "global-custom.conf",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handler, err := buildWAFHandler(tc.host, tc.rulesets, tc.rulesetPaths, tc.secCfg, tc.wafEnabled)
			require.NoError(t, err)

			if tc.expectedInclude == "" {
				require.Nil(t, handler)
				return
			}

			require.NotNil(t, handler)
			directives, ok := handler["directives"].(string)
			require.True(t, ok, "directives should be a string")
			require.Contains(t, directives, tc.expectedInclude)
		})
	}
}

// TestBuildWAFHandler_NoDirectivesReturnsNil verifies that the handler returns nil
// when no directives can be set (Bug fix #2 from the plan)
func TestBuildWAFHandler_NoDirectivesReturnsNil(t *testing.T) {
	tests := []struct {
		name         string
		host         *models.ProxyHost
		rulesets     []models.SecurityRuleSet
		rulesetPaths map[string]string
		secCfg       *models.SecurityConfig
		wafEnabled   bool
	}{
		{
			name:         "Empty rulesets returns nil",
			host:         &models.ProxyHost{UUID: "test-host"},
			rulesets:     []models.SecurityRuleSet{},
			rulesetPaths: map[string]string{},
			secCfg:       &models.SecurityConfig{WAFMode: "block"},
			wafEnabled:   true,
		},
		{
			name:     "Ruleset exists but no path mapping returns nil",
			host:     &models.ProxyHost{UUID: "test-host"},
			rulesets: []models.SecurityRuleSet{{Name: "my-rules"}},
			rulesetPaths: map[string]string{
				"other-rules": "/path/to/other.conf", // Path for different ruleset
			},
			secCfg:     &models.SecurityConfig{WAFMode: "block"},
			wafEnabled: true,
		},
		{
			name:         "WAFRulesSource specified but not in rulesets or paths returns nil",
			host:         &models.ProxyHost{UUID: "test-host"},
			rulesets:     []models.SecurityRuleSet{{Name: "other-rules"}},
			rulesetPaths: map[string]string{},
			secCfg:       &models.SecurityConfig{WAFMode: "block", WAFRulesSource: "nonexistent"},
			wafEnabled:   true,
		},
		{
			name:     "Empty path in rulesetPaths returns nil",
			host:     &models.ProxyHost{UUID: "test-host"},
			rulesets: []models.SecurityRuleSet{{Name: "owasp-crs"}},
			rulesetPaths: map[string]string{
				"owasp-crs": "", // Empty path
			},
			secCfg:     &models.SecurityConfig{WAFMode: "block"},
			wafEnabled: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handler, err := buildWAFHandler(tc.host, tc.rulesets, tc.rulesetPaths, tc.secCfg, tc.wafEnabled)
			require.NoError(t, err)
			require.Nil(t, handler, "Handler should be nil when no directives can be set")
		})
	}
}

// TestBuildWAFHandler_DisabledModes verifies WAF is disabled correctly
func TestBuildWAFHandler_DisabledModes(t *testing.T) {
	rulesets := []models.SecurityRuleSet{{Name: "owasp-crs"}}
	rulesetPaths := map[string]string{"owasp-crs": "/path/to/rules.conf"}
	host := &models.ProxyHost{UUID: "test-host"}

	tests := []struct {
		name       string
		secCfg     *models.SecurityConfig
		wafEnabled bool
	}{
		{
			name:       "wafEnabled false returns nil",
			secCfg:     &models.SecurityConfig{WAFMode: "block"},
			wafEnabled: false,
		},
		{
			name:       "WAFMode disabled returns nil",
			secCfg:     &models.SecurityConfig{WAFMode: "disabled"},
			wafEnabled: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handler, err := buildWAFHandler(host, rulesets, rulesetPaths, tc.secCfg, tc.wafEnabled)
			require.NoError(t, err)
			require.Nil(t, handler)
		})
	}
}

// TestBuildWAFHandler_HandlerStructure verifies the JSON structure matches the Handoff Contract
func TestBuildWAFHandler_HandlerStructure(t *testing.T) {
	host := &models.ProxyHost{UUID: "test-host"}
	rulesets := []models.SecurityRuleSet{{Name: "integration-xss"}}
	rulesetPaths := map[string]string{
		"integration-xss": "/app/data/caddy/coraza/rulesets/integration-xss-a1b2c3d4.conf",
	}
	secCfg := &models.SecurityConfig{WAFMode: "block", WAFRulesSource: "integration-xss"}

	handler, err := buildWAFHandler(host, rulesets, rulesetPaths, secCfg, true)
	require.NoError(t, err)
	require.NotNil(t, handler)

	// Verify handler type
	require.Equal(t, "waf", handler["handler"])

	// Verify directives contain Include statement
	directives, ok := handler["directives"].(string)
	require.True(t, ok)
	require.Contains(t, directives, "Include /app/data/caddy/coraza/rulesets/integration-xss-a1b2c3d4.conf")

	// Verify JSON marshaling produces expected structure
	jsonBytes, err := json.Marshal(handler)
	require.NoError(t, err)
	require.Contains(t, string(jsonBytes), `"handler":"waf"`)
	require.Contains(t, string(jsonBytes), `"directives":"Include`)
}

// TestBuildWAFHandler_AdvancedConfigParsing verifies advanced_config JSON parsing
func TestBuildWAFHandler_AdvancedConfigParsing(t *testing.T) {
	rulesets := []models.SecurityRuleSet{
		{Name: "owasp-crs"},
		{Name: "custom-ruleset"},
	}
	rulesetPaths := map[string]string{
		"owasp-crs":      "/path/owasp.conf",
		"custom-ruleset": "/path/custom.conf",
	}
	secCfg := &models.SecurityConfig{WAFMode: "block"}

	tests := []struct {
		name            string
		advancedConfig  string
		expectedInclude string
	}{
		{
			name:            "Valid ruleset_name in advanced_config",
			advancedConfig:  `{"ruleset_name":"custom-ruleset"}`,
			expectedInclude: "custom.conf",
		},
		{
			name:            "Invalid JSON falls back to owasp-crs",
			advancedConfig:  `{invalid json`,
			expectedInclude: "owasp.conf",
		},
		{
			name:            "Empty advanced_config falls back to owasp-crs",
			advancedConfig:  "",
			expectedInclude: "owasp.conf",
		},
		{
			name:            "Empty ruleset_name string falls back to owasp-crs",
			advancedConfig:  `{"ruleset_name":""}`,
			expectedInclude: "owasp.conf",
		},
		{
			name:            "Non-string ruleset_name falls back to owasp-crs",
			advancedConfig:  `{"ruleset_name":123}`,
			expectedInclude: "owasp.conf",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			host := &models.ProxyHost{
				UUID:           "test-host",
				AdvancedConfig: tc.advancedConfig,
			}
			handler, err := buildWAFHandler(host, rulesets, rulesetPaths, secCfg, true)
			require.NoError(t, err)
			require.NotNil(t, handler)
			directives := handler["directives"].(string)
			require.Contains(t, directives, tc.expectedInclude)
		})
	}
}
