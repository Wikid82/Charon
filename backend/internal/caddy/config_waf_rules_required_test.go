package caddy

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

const rulesRequiredValidRule = `SecRule ARGS "@contains attack" "id:1001,phase:1,deny"`

func TestBuildWAFHandler_SkipsLegacyEmptyRulesetInFallbackTiers(t *testing.T) {
	// The legacy empty row is an OWASP-named duplicate. It must not hide the valid set with the same fallback name.
	valid := models.SecurityRuleSet{Name: "owasp-crs", Content: rulesRequiredValidRule}
	cases := map[string]struct {
		host      *models.ProxyHost
		secCfg    *models.SecurityConfig
		empty     models.SecurityRuleSet
		wantValid bool
	}{
		"owasp fallback tier": {
			empty: models.SecurityRuleSet{Name: "OWASP Core Rule Set", Content: ""},
		},
		"application tier": {
			host:  &models.ProxyHost{Application: "legacy"},
			empty: models.SecurityRuleSet{Name: "legacy", Content: "# only a comment"},
		},
		"host ruleset_name tier": {
			host:  &models.ProxyHost{AdvancedConfig: `{"ruleset_name":"legacy"}`},
			empty: models.SecurityRuleSet{Name: "legacy", Content: ""},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rulesets := []models.SecurityRuleSet{tc.empty, valid}
			paths := map[string]string{"owasp-crs": "/cfg/coraza/rulesets/owasp-crs-abc.conf"}

			handler, reason := buildWAFHandlerWithReason(tc.host, rulesets, paths, tc.secCfg)
			require.NotNil(t, handler, "the valid rule set must be selected; reason %q", reason)
			assert.Contains(t, handler["directives"], "Include /cfg/coraza/rulesets/owasp-crs-abc.conf")
		})
	}
}

func TestBuildWAFHandler_LoneValidRulesetAmongEmptyRowIsSelected(t *testing.T) {
	rulesets := []models.SecurityRuleSet{
		{Name: "legacy-empty", Content: ""},
		{Name: "custom", Content: rulesRequiredValidRule},
	}
	paths := map[string]string{"custom": "/cfg/coraza/rulesets/custom-abc.conf"}

	handler, _ := buildWAFHandlerWithReason(nil, rulesets, paths, nil)
	require.NotNil(t, handler)
	assert.Contains(t, handler["directives"], "Include /cfg/coraza/rulesets/custom-abc.conf")
}

func TestBuildWAFHandler_ExplicitEmptyRulesSourceBuildsNoHandler(t *testing.T) {
	// Tier 1 stays strict: the user picked the empty set, so no handler is built even though a valid set exists.
	rulesets := []models.SecurityRuleSet{
		{Name: "chosen-empty", Content: "SecRuleEngine On"},
		{Name: "owasp-crs", Content: rulesRequiredValidRule},
	}
	paths := map[string]string{"owasp-crs": "/cfg/coraza/rulesets/owasp-crs-abc.conf"}
	secCfg := &models.SecurityConfig{WAFRulesSource: "chosen-empty"}

	handler, _ := buildWAFHandlerWithReason(nil, rulesets, paths, secCfg)
	assert.Nil(t, handler)
}

func TestGenerateConfig_AdvancedRulesetNameSkippedLeavesHandlerUntouched(t *testing.T) {
	// The named set was skipped by the manager, so it has no path. The handler must match the one written without a
	// ruleset_name key: no Include directive is added.
	rulesets := []models.SecurityRuleSet{{Name: "legacy-empty", Content: ""}}
	withName := models.ProxyHost{UUID: "skip-named", DomainNames: "skip-named.example.com", Enabled: true, ForwardHost: "app", ForwardPort: 8080, AdvancedConfig: `{"handler":"waf","ruleset_name":"legacy-empty"}`}
	withoutName := models.ProxyHost{UUID: "skip-plain", DomainNames: "skip-plain.example.com", Enabled: true, ForwardHost: "app", ForwardPort: 8080, AdvancedConfig: `{"handler":"waf"}`}

	cfgNamed, err := GenerateConfig([]models.ProxyHost{withName}, "/tmp/caddy-data", "", "", "", false, false, true, false, false, "", rulesets, map[string]string{}, nil, nil, nil)
	require.NoError(t, err)
	cfgPlain, err := GenerateConfig([]models.ProxyHost{withoutName}, "/tmp/caddy-data", "", "", "", false, false, true, false, false, "", rulesets, map[string]string{}, nil, nil, nil)
	require.NoError(t, err)

	namedHandlers, err := json.Marshal(cfgNamed.Apps.HTTP.Servers["charon_server"].Routes[0].Handle)
	require.NoError(t, err)
	plainHandlers, err := json.Marshal(cfgPlain.Apps.HTTP.Servers["charon_server"].Routes[0].Handle)
	require.NoError(t, err)
	assert.Equal(t, string(plainHandlers), string(namedHandlers))
	assert.False(t, strings.Contains(string(namedHandlers), "Include"), "no Include directive may point at the skipped set")
}
