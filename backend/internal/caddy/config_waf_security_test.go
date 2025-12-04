package caddy

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// TestBuildWAFHandler_PathTraversalAttack tests path traversal attempts in ruleset names
func TestBuildWAFHandler_PathTraversalAttack(t *testing.T) {
	tests := []struct {
		name        string
		rulesetName string
		shouldMatch bool // Whether the ruleset should be found
		description string
	}{
		{
			name:        "Path traversal in ruleset name",
			rulesetName: "../../../etc/passwd",
			shouldMatch: false,
			description: "Ruleset with path traversal should not match any legitimate path",
		},
		{
			name:        "Null byte injection",
			rulesetName: "rules\x00.conf",
			shouldMatch: false,
			description: "Ruleset with null bytes should not match",
		},
		{
			name:        "URL encoded traversal",
			rulesetName: "..%2F..%2Fetc%2Fpasswd",
			shouldMatch: false,
			description: "URL encoded path traversal should not match",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			host := &models.ProxyHost{UUID: "test-host"}
			rulesets := []models.SecurityRuleSet{{Name: tc.rulesetName}}
			// Only provide paths for legitimate rulesets
			rulesetPaths := map[string]string{
				"owasp-crs": "/app/data/caddy/coraza/rulesets/owasp-crs.conf",
			}
			secCfg := &models.SecurityConfig{WAFMode: "block", WAFRulesSource: tc.rulesetName}

			handler, err := buildWAFHandler(host, rulesets, rulesetPaths, secCfg, true)
			require.NoError(t, err)

			if tc.shouldMatch {
				require.NotNil(t, handler)
			} else {
				// Handler should be nil since no matching path exists
				require.Nil(t, handler, tc.description)
			}
		})
	}
}

// TestBuildWAFHandler_SQLInjectionInRulesetName tests SQL injection patterns in ruleset names
func TestBuildWAFHandler_SQLInjectionInRulesetName(t *testing.T) {
	sqlInjectionPatterns := []string{
		"'; DROP TABLE rulesets; --",
		"1' OR '1'='1",
		"UNION SELECT * FROM users--",
		"admin'/*",
	}

	for _, pattern := range sqlInjectionPatterns {
		t.Run(pattern, func(t *testing.T) {
			host := &models.ProxyHost{UUID: "test-host"}
			// Create ruleset with malicious name but only provide path for safe ruleset
			rulesets := []models.SecurityRuleSet{{Name: pattern}, {Name: "owasp-crs"}}
			rulesetPaths := map[string]string{
				"owasp-crs": "/app/data/caddy/coraza/rulesets/owasp-crs.conf",
			}
			secCfg := &models.SecurityConfig{WAFMode: "block", WAFRulesSource: pattern}

			handler, err := buildWAFHandler(host, rulesets, rulesetPaths, secCfg, true)
			require.NoError(t, err)
			// Should return nil since the malicious name has no corresponding path
			require.Nil(t, handler, "SQL injection pattern should not produce valid handler")
		})
	}
}

// TestBuildWAFHandler_XSSInAdvancedConfig tests XSS patterns in advanced_config JSON
func TestBuildWAFHandler_XSSInAdvancedConfig(t *testing.T) {
	xssPatterns := []string{
		`{"ruleset_name":"<script>alert(1)</script>"}`,
		`{"ruleset_name":"<img src=x onerror=alert(1)>"}`,
		`{"ruleset_name":"javascript:alert(1)"}`,
		`{"ruleset_name":"<svg/onload=alert(1)>"}`,
	}

	for _, pattern := range xssPatterns {
		t.Run(pattern, func(t *testing.T) {
			host := &models.ProxyHost{
				UUID:           "test-host",
				AdvancedConfig: pattern,
			}
			rulesets := []models.SecurityRuleSet{{Name: "owasp-crs"}}
			rulesetPaths := map[string]string{
				"owasp-crs": "/app/data/caddy/coraza/rulesets/owasp-crs.conf",
			}
			secCfg := &models.SecurityConfig{WAFMode: "block"}

			handler, err := buildWAFHandler(host, rulesets, rulesetPaths, secCfg, true)
			require.NoError(t, err)
			// Should fall back to owasp-crs since XSS pattern won't match any ruleset
			require.NotNil(t, handler)
			directives := handler["directives"].(string)
			require.Contains(t, directives, "owasp-crs")
			// Ensure XSS content is NOT in the output
			require.NotContains(t, directives, "<script>")
			require.NotContains(t, directives, "javascript:")
		})
	}
}

// TestBuildWAFHandler_HugePayload tests handling of very large inputs
func TestBuildWAFHandler_HugePayload(t *testing.T) {
	// Create a very large ruleset name (1MB)
	hugeName := strings.Repeat("A", 1024*1024)

	host := &models.ProxyHost{UUID: "test-host"}
	rulesets := []models.SecurityRuleSet{{Name: hugeName}, {Name: "owasp-crs"}}
	rulesetPaths := map[string]string{
		"owasp-crs": "/app/data/caddy/coraza/rulesets/owasp-crs.conf",
	}
	secCfg := &models.SecurityConfig{WAFMode: "block"}

	// Should not panic or crash
	handler, err := buildWAFHandler(host, rulesets, rulesetPaths, secCfg, true)
	require.NoError(t, err)
	// Falls back to owasp-crs since huge name has no path
	require.NotNil(t, handler)
	directives := handler["directives"].(string)
	require.Contains(t, directives, "owasp-crs")
}

// TestBuildWAFHandler_EmptyAndWhitespaceInputs tests boundary conditions
func TestBuildWAFHandler_EmptyAndWhitespaceInputs(t *testing.T) {
	tests := []struct {
		name           string
		rulesetName    string
		wafRulesSource string
		expectNil      bool
	}{
		{
			name:           "Empty string WAFRulesSource",
			rulesetName:    "owasp-crs",
			wafRulesSource: "",
			expectNil:      false, // Falls back to owasp-crs
		},
		{
			name:           "Whitespace-only WAFRulesSource",
			rulesetName:    "owasp-crs",
			wafRulesSource: "   ",
			expectNil:      false, // Falls back to owasp-crs (whitespace doesn't match, but fallback exists)
		},
		{
			name:           "Tab and newline in WAFRulesSource",
			rulesetName:    "owasp-crs",
			wafRulesSource: "\t\n",
			expectNil:      false, // Falls back to owasp-crs (special chars don't match, but fallback exists)
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			host := &models.ProxyHost{UUID: "test-host"}
			rulesets := []models.SecurityRuleSet{{Name: tc.rulesetName}}
			rulesetPaths := map[string]string{
				tc.rulesetName: "/app/data/caddy/coraza/rulesets/" + tc.rulesetName + ".conf",
			}
			secCfg := &models.SecurityConfig{WAFMode: "block", WAFRulesSource: tc.wafRulesSource}

			handler, err := buildWAFHandler(host, rulesets, rulesetPaths, secCfg, true)
			require.NoError(t, err)

			if tc.expectNil {
				require.Nil(t, handler)
			} else {
				require.NotNil(t, handler)
			}
		})
	}
}

// TestBuildWAFHandler_ConcurrentRulesetSelection tests that selection is deterministic
func TestBuildWAFHandler_ConcurrentRulesetSelection(t *testing.T) {
	host := &models.ProxyHost{UUID: "test-host"}
	rulesets := []models.SecurityRuleSet{
		{Name: "ruleset-a"},
		{Name: "ruleset-b"},
		{Name: "ruleset-c"},
		{Name: "owasp-crs"},
	}
	rulesetPaths := map[string]string{
		"ruleset-a": "/path/ruleset-a.conf",
		"ruleset-b": "/path/ruleset-b.conf",
		"ruleset-c": "/path/ruleset-c.conf",
		"owasp-crs": "/path/owasp.conf",
	}
	secCfg := &models.SecurityConfig{WAFMode: "block", WAFRulesSource: "ruleset-b"}

	// Run 100 times to verify determinism
	for i := 0; i < 100; i++ {
		handler, err := buildWAFHandler(host, rulesets, rulesetPaths, secCfg, true)
		require.NoError(t, err)
		require.NotNil(t, handler)
		directives := handler["directives"].(string)
		require.Contains(t, directives, "ruleset-b", "Selection should always pick WAFRulesSource")
	}
}

// TestBuildWAFHandler_NilSecCfg tests handling when secCfg is nil
func TestBuildWAFHandler_NilSecCfg(t *testing.T) {
	host := &models.ProxyHost{UUID: "test-host"}
	rulesets := []models.SecurityRuleSet{{Name: "owasp-crs"}}
	rulesetPaths := map[string]string{
		"owasp-crs": "/app/data/caddy/coraza/rulesets/owasp-crs.conf",
	}

	// nil secCfg should not panic, should fall back to owasp-crs
	handler, err := buildWAFHandler(host, rulesets, rulesetPaths, nil, true)
	require.NoError(t, err)
	require.NotNil(t, handler)
	directives := handler["directives"].(string)
	require.Contains(t, directives, "owasp-crs")
}

// TestBuildWAFHandler_NilHost tests handling when host is nil
func TestBuildWAFHandler_NilHost(t *testing.T) {
	rulesets := []models.SecurityRuleSet{{Name: "owasp-crs"}}
	rulesetPaths := map[string]string{
		"owasp-crs": "/app/data/caddy/coraza/rulesets/owasp-crs.conf",
	}
	secCfg := &models.SecurityConfig{WAFMode: "block"}

	// nil host should not panic
	handler, err := buildWAFHandler(nil, rulesets, rulesetPaths, secCfg, true)
	require.NoError(t, err)
	require.NotNil(t, handler)
	directives := handler["directives"].(string)
	require.Contains(t, directives, "owasp-crs")
}

// TestBuildWAFHandler_SpecialCharactersInRulesetName tests handling of special chars
func TestBuildWAFHandler_SpecialCharactersInRulesetName(t *testing.T) {
	specialNames := []struct {
		name     string
		safeName string
	}{
		{"ruleset with spaces", "ruleset-with-spaces"},
		{"ruleset/with/slashes", "ruleset-with-slashes"},
		{"UPPERCASE-RULESET", "uppercase-ruleset"},
		{"ruleset_with_underscores", "ruleset_with_underscores"},
		{"ruleset.with.dots", "ruleset.with.dots"},
	}

	for _, tc := range specialNames {
		t.Run(tc.name, func(t *testing.T) {
			host := &models.ProxyHost{UUID: "test-host"}
			rulesets := []models.SecurityRuleSet{{Name: tc.name}}
			// Simulate path that would be generated by manager.go
			rulesetPaths := map[string]string{
				tc.name: "/app/data/caddy/coraza/rulesets/" + tc.safeName + "-abc123.conf",
			}
			secCfg := &models.SecurityConfig{WAFMode: "block", WAFRulesSource: tc.name}

			handler, err := buildWAFHandler(host, rulesets, rulesetPaths, secCfg, true)
			require.NoError(t, err)
			require.NotNil(t, handler)
			directives := handler["directives"].(string)
			require.Contains(t, directives, tc.safeName)
		})
	}
}
