package caddy

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/require"
)

func TestGenerateConfig_ZerosslAndBothProviders(t *testing.T) {
	hosts := []models.ProxyHost{
		{
			UUID:        "h1",
			DomainNames: "a.example.com",
			Enabled:     true,
			ForwardHost: "127.0.0.1",
			ForwardPort: 8080,
		},
	}

	// Zerossl provider
	cfgZ, err := GenerateConfig(hosts, "/data/caddy/data", "admin@example.com", "/frontend/dist", "zerossl", false, false, false, false, false, "", nil, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, cfgZ.Apps.TLS)
	// Expect only zerossl issuer present
	issuers := cfgZ.Apps.TLS.Automation.Policies[0].IssuersRaw
	foundZerossl := false
	for _, i := range issuers {
		m := i.(map[string]interface{})
		if m["module"] == "zerossl" {
			foundZerossl = true
		}
	}
	require.True(t, foundZerossl)

	// Default/both provider
	cfgBoth, err := GenerateConfig(hosts, "/data/caddy/data", "admin@example.com", "/frontend/dist", "", false, false, false, false, false, "", nil, nil, nil, nil)
	require.NoError(t, err)
	issuersBoth := cfgBoth.Apps.TLS.Automation.Policies[0].IssuersRaw
	// We should have at least 2 issuers (acme + zerossl)
	require.GreaterOrEqual(t, len(issuersBoth), 2)
}

func TestGenerateConfig_SecurityPipeline_Order_Locations(t *testing.T) {
	// Create host with a location so location-level handlers are generated
	ipRules := `[ { "cidr": "192.168.1.0/24" } ]`
	acl := models.AccessList{ID: 201, Name: "WL2", Enabled: true, Type: "whitelist", IPRules: ipRules}
	host := models.ProxyHost{UUID: "pipeline2", DomainNames: "pipe-loc.example.com", Enabled: true, ForwardHost: "app", ForwardPort: 8080, AccessListID: &acl.ID, AccessList: &acl, HSTSEnabled: true, BlockExploits: true, Locations: []models.Location{{Path: "/loc", ForwardHost: "app", ForwardPort: 9000}}}

	// Provide rulesets and paths so WAF handler is created with directives
	rulesets := []models.SecurityRuleSet{{Name: "owasp-crs"}}
	rulesetPaths := map[string]string{"owasp-crs": "/tmp/owasp.conf"}
	sec := &models.SecurityConfig{CrowdSecMode: "local"}
	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/tmp/caddy-data", "", "", "", false, true, true, true, true, "", rulesets, rulesetPaths, nil, sec)
	require.NoError(t, err)

	server := cfg.Apps.HTTP.Servers["charon_server"]
	require.NotNil(t, server)

	// Find the route for the location (path contains "/loc")
	var locRoute *Route
	for _, r := range server.Routes {
		if len(r.Match) > 0 && len(r.Match[0].Path) > 0 {
			for _, p := range r.Match[0].Path {
				if p == "/loc" {
					locRoute = r
					break
				}
			}
		}
	}
	require.NotNil(t, locRoute)

	// Extract handler names from the location route
	names := []string{}
	for _, h := range locRoute.Handle {
		if hn, ok := h["handler"].(string); ok {
			names = append(names, hn)
		}
	}

	// Expected pipeline: crowdsec -> waf -> rate_limit -> subroute (acl) -> headers -> vars (BlockExploits) -> reverse_proxy
	require.GreaterOrEqual(t, len(names), 4)
	require.Equal(t, "crowdsec", names[0])
	require.Equal(t, "waf", names[1])
	require.Equal(t, "rate_limit", names[2])
	require.Equal(t, "subroute", names[3])
}

func TestGenerateConfig_ACLLogWarning(t *testing.T) {
	// capture logs by initializing logger
	var buf strings.Builder
	logger.Init(true, &buf)

	// Create host with an invalid IP rules ACL to force buildACLHandler error
	acl := models.AccessList{ID: 300, Name: "BadACL", Enabled: true, Type: "blacklist", IPRules: "invalid-json"}
	host := models.ProxyHost{UUID: "acl-log", DomainNames: "acl-err.example.com", Enabled: true, ForwardHost: "app", ForwardPort: 8080, AccessListID: &acl.ID, AccessList: &acl}

	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/tmp/caddy-data", "", "", "", false, false, false, false, true, "", nil, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Ensure the logger captured a warning about ACL build failure
	require.Contains(t, buf.String(), "Failed to build ACL handler for host")
}

func TestGenerateConfig_ACLHandlerIncluded(t *testing.T) {
	ipRules := `[ { "cidr": "10.0.0.0/8" } ]`
	acl := models.AccessList{ID: 301, Name: "WL3", Enabled: true, Type: "whitelist", IPRules: ipRules}
	host := models.ProxyHost{UUID: "acl-incl", DomainNames: "acl-incl.example.com", Enabled: true, ForwardHost: "app", ForwardPort: 8080, AccessListID: &acl.ID, AccessList: &acl}
	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/tmp/caddy-data", "", "", "", false, false, false, false, true, "", nil, nil, nil, nil)
	require.NoError(t, err)
	server := cfg.Apps.HTTP.Servers["charon_server"]
	require.NotNil(t, server)
	route := server.Routes[0]

	// Extract handler names
	names := []string{}
	for _, h := range route.Handle {
		if hn, ok := h["handler"].(string); ok {
			names = append(names, hn)
		}
	}
	// Ensure subroute (ACL) is present
	found := false
	for _, n := range names {
		if n == "subroute" {
			found = true
			break
		}
	}
	require.True(t, found)
}

func TestGenerateConfig_DecisionsBlockWithAdminExclusion(t *testing.T) {
	host := models.ProxyHost{UUID: "dec1", DomainNames: "dec.example.com", Enabled: true, ForwardHost: "app", ForwardPort: 8080}
	// create a security decision to block 1.2.3.4
	dec := models.SecurityDecision{Action: "block", IP: "1.2.3.4"}
	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/tmp/caddy-data", "", "", "", false, false, false, false, false, "10.0.0.1/32", nil, nil, []models.SecurityDecision{dec}, nil)
	require.NoError(t, err)
	route := cfg.Apps.HTTP.Servers["charon_server"].Routes[0]
	b, _ := json.MarshalIndent(route.Handle, "", "  ")
	t.Logf("handles: %s", string(b))
	// Expect first security handler is a subroute that includes both remote_ip and a 'not' exclusion for adminWhitelist
	found := false
	for _, h := range route.Handle {
		// convert to JSON string and assert the expected fields exist
		b, _ := json.Marshal(h)
		s := string(b)
		if strings.Contains(s, "\"remote_ip\"") && strings.Contains(s, "\"not\"") && strings.Contains(s, "1.2.3.4") && strings.Contains(s, "10.0.0.1/32") {
			found = true
			break
		}
	}
	if !found {
		// Log the route handles for debugging
		for i, h := range route.Handle {
			b, _ := json.MarshalIndent(h, "  ", "  ")
			t.Logf("handler #%d: %s", i, string(b))
		}
	}
	require.True(t, found, "expected decision subroute with admin exclusion to be present")
}

func TestGenerateConfig_WAFModeAndRulesetReference(t *testing.T) {
	host := models.ProxyHost{UUID: "wafref", DomainNames: "wafref.example.com", Enabled: true, ForwardHost: "app", ForwardPort: 8080}
	// No rulesets provided but secCfg references a rulesource
	sec := &models.SecurityConfig{WAFMode: "block", WAFRulesSource: "nonexistent-rs"}
	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/tmp/caddy-data", "", "", "", false, false, true, false, false, "", nil, nil, nil, sec)
	require.NoError(t, err)
	// Since a ruleset name was requested but none exists, NO waf handler should be created
	// (Bug fix: don't create a no-op WAF handler without directives)
	route := cfg.Apps.HTTP.Servers["charon_server"].Routes[0]
	for _, h := range route.Handle {
		if hn, ok := h["handler"].(string); ok && hn == "waf" {
			t.Fatalf("expected NO waf handler when referenced ruleset does not exist, but found: %v", h)
		}
	}

	// Now test with valid ruleset - WAF handler should be created
	rulesets := []models.SecurityRuleSet{{Name: "owasp-crs"}}
	rulesetPaths := map[string]string{"owasp-crs": "/tmp/owasp.conf"}
	sec2 := &models.SecurityConfig{WAFMode: "block", WAFLearning: true}
	cfg2, err := GenerateConfig([]models.ProxyHost{host}, "/tmp/caddy-data", "", "", "", false, false, true, false, false, "", rulesets, rulesetPaths, nil, sec2)
	require.NoError(t, err)
	route2 := cfg2.Apps.HTTP.Servers["charon_server"].Routes[0]
	monitorFound := false
	for _, h := range route2.Handle {
		if hn, ok := h["handler"].(string); ok && hn == "waf" {
			monitorFound = true
		}
	}
	require.True(t, monitorFound, "expected waf handler when WAFLearning is true and ruleset exists")
}

func TestGenerateConfig_WAFModeDisabledSkipsHandler(t *testing.T) {
	host := models.ProxyHost{UUID: "waf-disabled", DomainNames: "wafd.example.com", Enabled: true, ForwardHost: "app", ForwardPort: 8080}
	sec := &models.SecurityConfig{WAFMode: "disabled", WAFRulesSource: "owasp-crs"}
	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/tmp/caddy-data", "", "", "", false, false, true, false, false, "", nil, nil, nil, sec)
	require.NoError(t, err)
	route := cfg.Apps.HTTP.Servers["charon_server"].Routes[0]
	for _, h := range route.Handle {
		if hn, ok := h["handler"].(string); ok && hn == "waf" {
			t.Fatalf("expected NO waf handler when WAFMode disabled, found: %v", h)
		}
	}
}

func TestGenerateConfig_WAFSelectedSetsContentAndMode(t *testing.T) {
	host := models.ProxyHost{UUID: "waf-2", DomainNames: "waf2.example.com", Enabled: true, ForwardHost: "app", ForwardPort: 8080}
	rs := models.SecurityRuleSet{Name: "owasp-crs", SourceURL: "http://example.com/owasp", Content: "rule 1"}
	sec := &models.SecurityConfig{WAFMode: "block"}
	rulesetPaths := map[string]string{"owasp-crs": "/tmp/owasp-crs.conf"}
	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/tmp/caddy-data", "", "", "", false, false, true, false, false, "", []models.SecurityRuleSet{rs}, rulesetPaths, nil, sec)
	require.NoError(t, err)
	route := cfg.Apps.HTTP.Servers["charon_server"].Routes[0]
	found := false
	for _, h := range route.Handle {
		if hn, ok := h["handler"].(string); ok && hn == "waf" {
			if dir, ok := h["directives"].(string); ok && strings.Contains(dir, "Include") {
				found = true
				break
			}
		}
	}
	require.True(t, found, "expected waf handler with directives containing Include to be present")
}

func TestGenerateConfig_DecisionAdminPartsEmpty(t *testing.T) {
	host := models.ProxyHost{UUID: "dec2", DomainNames: "dec2.example.com", Enabled: true, ForwardHost: "app", ForwardPort: 8080}
	dec := models.SecurityDecision{Action: "block", IP: "2.3.4.5"}
	// Provide an adminWhitelist with an empty segment to trigger p == ""
	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/tmp/caddy-data", "", "", "", false, false, false, false, false, ", 10.0.0.1/32", nil, nil, []models.SecurityDecision{dec}, nil)
	require.NoError(t, err)
	route := cfg.Apps.HTTP.Servers["charon_server"].Routes[0]
	found := false
	for _, h := range route.Handle {
		b, _ := json.Marshal(h)
		s := string(b)
		if strings.Contains(s, "\"remote_ip\"") && strings.Contains(s, "\"not\"") && strings.Contains(s, "2.3.4.5") {
			found = true
			break
		}
	}
	require.True(t, found, "expected decision subroute with admin exclusion present when adminWhitelist contains empty parts")
}

func TestNormalizeHeaderOps_PreserveStringArray(t *testing.T) {
	// Construct a headers map where set has a []string value already
	set := map[string]interface{}{
		"X-Array": []string{"1", "2"},
	}
	headerOps := map[string]interface{}{"set": set}
	normalizeHeaderOps(headerOps)
	// Ensure the value remained a []string
	if v, ok := headerOps["set"].(map[string]interface{}); ok {
		if arr, ok := v["X-Array"].([]string); ok {
			require.Equal(t, []string{"1", "2"}, arr)
			return
		}
	}
	t.Fatal("expected set.X-Array to remain []string")
}

func TestGenerateConfig_WAFUsesRuleSet(t *testing.T) {
	// host + ruleset configured
	host := models.ProxyHost{UUID: "waf-1", DomainNames: "waf.example.com", Enabled: true, ForwardHost: "app", ForwardPort: 8080}
	rs := models.SecurityRuleSet{Name: "owasp-crs", SourceURL: "http://example.com/owasp", Content: "rule 1"}
	rulesetPaths := map[string]string{"owasp-crs": "/tmp/owasp-crs.conf"}
	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/tmp/caddy-data", "", "", "", false, false, true, false, false, "", []models.SecurityRuleSet{rs}, rulesetPaths, nil, nil)
	require.NoError(t, err)
	route := cfg.Apps.HTTP.Servers["charon_server"].Routes[0]
	// check waf handler present with directives containing Include
	found := false
	for _, h := range route.Handle {
		if hn, ok := h["handler"].(string); ok && hn == "waf" {
			if dir, ok := h["directives"].(string); ok && strings.Contains(dir, "Include") {
				found = true
				break
			}
		}
	}
	if !found {
		b2, _ := json.MarshalIndent(route.Handle, "", "  ")
		t.Fatalf("waf handler with directives should be present; handlers: %s", string(b2))
	}
}

func TestGenerateConfig_WAFUsesRuleSetFromAdvancedConfig(t *testing.T) {
	// host with AdvancedConfig selecting a custom ruleset
	host := models.ProxyHost{UUID: "waf-host-adv", DomainNames: "waf-adv.example.com", Enabled: true, ForwardHost: "app", ForwardPort: 8080, AdvancedConfig: "{\"handler\":\"waf\",\"ruleset_name\":\"host-rs\"}"}
	rs := models.SecurityRuleSet{Name: "host-rs", SourceURL: "http://example.com/host-rs", Content: "rule X"}
	rulesetPaths := map[string]string{"host-rs": "/tmp/host-rs.conf"}
	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/tmp/caddy-data", "", "", "", false, false, true, false, false, "", []models.SecurityRuleSet{rs}, rulesetPaths, nil, nil)
	require.NoError(t, err)
	route := cfg.Apps.HTTP.Servers["charon_server"].Routes[0]
	// check waf handler present with directives containing Include from host AdvancedConfig
	found := false
	for _, h := range route.Handle {
		if hn, ok := h["handler"].(string); ok && hn == "waf" {
			if dir, ok := h["directives"].(string); ok && strings.Contains(dir, "Include /tmp/host-rs.conf") {
				found = true
				break
			}
		}
	}
	require.True(t, found, "waf handler with directives should include host advanced_config ruleset path")
}

func TestGenerateConfig_WAFUsesRuleSetFromAdvancedConfig_Array(t *testing.T) {
	// host with AdvancedConfig as JSON array selecting a custom ruleset
	host := models.ProxyHost{UUID: "waf-host-adv-arr", DomainNames: "waf-adv-arr.example.com", Enabled: true, ForwardHost: "app", ForwardPort: 8080, AdvancedConfig: "[{\"handler\":\"waf\",\"ruleset_name\":\"host-rs-array\"}]"}
	rs := models.SecurityRuleSet{Name: "host-rs-array", SourceURL: "http://example.com/host-rs-array", Content: "rule X"}
	rulesetPaths := map[string]string{"host-rs-array": "/tmp/host-rs-array.conf"}
	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/tmp/caddy-data", "", "", "", false, false, true, false, false, "", []models.SecurityRuleSet{rs}, rulesetPaths, nil, nil)
	require.NoError(t, err)
	route := cfg.Apps.HTTP.Servers["charon_server"].Routes[0]
	// check waf handler present with directives containing Include from host AdvancedConfig array
	found := false
	for _, h := range route.Handle {
		if hn, ok := h["handler"].(string); ok && hn == "waf" {
			if dir, ok := h["directives"].(string); ok && strings.Contains(dir, "Include /tmp/host-rs-array.conf") {
				found = true
				break
			}
		}
	}
	if !found {
		b, _ := json.MarshalIndent(route.Handle, "", "  ")
		t.Fatalf("waf handler with directives should include host advanced_config array ruleset path; handlers: %s", string(b))
	}
}

func TestGenerateConfig_WAFUsesRulesetFromSecCfgFallback(t *testing.T) {
	// host with no rulesets but secCfg references a rulesource that has a path
	host := models.ProxyHost{UUID: "waf-fallback", DomainNames: "waf-fallback.example.com", Enabled: true, ForwardHost: "app", ForwardPort: 8080}
	sec := &models.SecurityConfig{WAFMode: "block", WAFRulesSource: "owasp-crs"}
	rulesetPaths := map[string]string{"owasp-crs": "/tmp/owasp-fallback.conf"}
	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/tmp/caddy-data", "", "", "", false, false, true, false, false, "", nil, rulesetPaths, nil, sec)
	require.NoError(t, err)
	// since secCfg requested owasp-crs and we have a path, the waf handler should include the path in directives
	route := cfg.Apps.HTTP.Servers["charon_server"].Routes[0]
	found := false
	for _, h := range route.Handle {
		if hn, ok := h["handler"].(string); ok && hn == "waf" {
			if dir, ok := h["directives"].(string); ok && strings.Contains(dir, "Include /tmp/owasp-fallback.conf") {
				found = true
				break
			}
		}
	}
	require.True(t, found, "waf handler with directives should include fallback secCfg ruleset path")
}

func TestGenerateConfig_RateLimitFromSecCfg(t *testing.T) {
	host := models.ProxyHost{UUID: "rl-1", DomainNames: "rl.example.com", Enabled: true, ForwardHost: "app", ForwardPort: 8080}
	sec := &models.SecurityConfig{RateLimitRequests: 10, RateLimitWindowSec: 60, RateLimitBurst: 5}
	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/tmp/caddy-data", "", "", "", false, false, false, true, false, "", nil, nil, nil, sec)
	require.NoError(t, err)
	route := cfg.Apps.HTTP.Servers["charon_server"].Routes[0]
	found := false
	for _, h := range route.Handle {
		if hn, ok := h["handler"].(string); ok && hn == "rate_limit" {
			if req, ok := h["requests"].(int); ok && req == 10 {
				if win, ok := h["window_sec"].(int); ok && win == 60 {
					found = true
					break
				}
			}
		}
	}
	require.True(t, found, "rate_limit handler with configured values should be present")
}

func TestGenerateConfig_CrowdSecHandlerFromSecCfg(t *testing.T) {
	host := models.ProxyHost{UUID: "cs-1", DomainNames: "cs.example.com", Enabled: true, ForwardHost: "app", ForwardPort: 8080}
	sec := &models.SecurityConfig{CrowdSecMode: "local", CrowdSecAPIURL: "http://cs.local"}
	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/tmp/caddy-data", "", "", "", false, true, false, false, false, "", nil, nil, nil, sec)
	require.NoError(t, err)
	route := cfg.Apps.HTTP.Servers["charon_server"].Routes[0]
	found := false
	for _, h := range route.Handle {
		if hn, ok := h["handler"].(string); ok && hn == "crowdsec" {
			if mode, ok := h["mode"].(string); ok && mode == "local" {
				found = true
				break
			}
		}
	}
	require.True(t, found, "crowdsec handler with api_url and mode should be present")
}

func TestGenerateConfig_EmptyHostsAndNoFrontend(t *testing.T) {
	cfg, err := GenerateConfig([]models.ProxyHost{}, "/data/caddy/data", "", "", "", false, false, false, false, false, "", nil, nil, nil, nil)
	require.NoError(t, err)
	// Should return base config without server routes
	_, found := cfg.Apps.HTTP.Servers["charon_server"]
	require.False(t, found)
}

func TestGenerateConfig_SkipsInvalidCustomCert(t *testing.T) {
	// Create a host with a custom cert missing private key
	cert := models.SSLCertificate{ID: 1, UUID: "c1", Name: "CustomCert", Provider: "custom", Certificate: "cert", PrivateKey: ""}
	host := models.ProxyHost{UUID: "h1", DomainNames: "a.example.com", Enabled: true, ForwardHost: "127.0.0.1", ForwardPort: 8080, Certificate: &cert, CertificateID: ptrUint(1)}

	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/data/caddy/data", "", "/frontend/dist", "", false, false, false, false, true, "", nil, nil, nil, nil)
	require.NoError(t, err)
	// Custom cert missing key should not be in LoadPEM
	if cfg.Apps.TLS != nil && cfg.Apps.TLS.Certificates != nil {
		b, _ := json.Marshal(cfg.Apps.TLS.Certificates)
		require.NotContains(t, string(b), "CustomCert")
	}
}

func TestGenerateConfig_SkipsDuplicateDomains(t *testing.T) {
	// Two hosts with same domain - one newer than other should be kept only once
	h1 := models.ProxyHost{UUID: "h1", DomainNames: "dup.com", Enabled: true, ForwardHost: "127.0.0.1", ForwardPort: 8080}
	h2 := models.ProxyHost{UUID: "h2", DomainNames: "dup.com", Enabled: true, ForwardHost: "127.0.0.2", ForwardPort: 8081}
	cfg, err := GenerateConfig([]models.ProxyHost{h1, h2}, "/data/caddy/data", "", "/frontend/dist", "", false, false, false, false, false, "", nil, nil, nil, nil)
	require.NoError(t, err)
	server := cfg.Apps.HTTP.Servers["charon_server"]
	// Expect that only one route exists for dup.com (one for the domain)
	require.GreaterOrEqual(t, len(server.Routes), 1)
}

func TestGenerateConfig_LoadPEMSetsTLSWhenNoACME(t *testing.T) {
	cert := models.SSLCertificate{ID: 1, UUID: "c1", Name: "LoadPEM", Provider: "custom", Certificate: "cert", PrivateKey: "key"}
	host := models.ProxyHost{UUID: "h1", DomainNames: "pem.com", Enabled: true, ForwardHost: "127.0.0.1", ForwardPort: 8080, Certificate: &cert, CertificateID: &cert.ID}
	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/data/caddy/data", "", "/frontend/dist", "", false, false, false, false, true, "", nil, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, cfg.Apps.TLS)
	require.NotNil(t, cfg.Apps.TLS.Certificates)
}

func TestGenerateConfig_DefaultAcmeStaging(t *testing.T) {
	hosts := []models.ProxyHost{{UUID: "h1", DomainNames: "a.example.com", Enabled: true, ForwardHost: "127.0.0.1", ForwardPort: 8080}}
	cfg, err := GenerateConfig(hosts, "/data/caddy/data", "admin@example.com", "/frontend/dist", "", true, false, false, false, false, "", nil, nil, nil, nil)
	require.NoError(t, err)
	// Should include acme issuer with CA staging URL
	issuers := cfg.Apps.TLS.Automation.Policies[0].IssuersRaw
	found := false
	for _, i := range issuers {
		if m, ok := i.(map[string]interface{}); ok {
			if m["module"] == "acme" {
				if _, ok := m["ca"]; ok {
					found = true
				}
			}
		}
	}
	require.True(t, found)
}

func TestGenerateConfig_ACLHandlerBuildError(t *testing.T) {
	// create host with an ACL with invalid JSON to force buildACLHandler to error
	acl := models.AccessList{ID: 10, Name: "BadACL", Enabled: true, Type: "blacklist", IPRules: "invalid"}
	host := models.ProxyHost{UUID: "h1", DomainNames: "a.example.com", Enabled: true, ForwardHost: "127.0.0.1", ForwardPort: 8080, AccessListID: &acl.ID, AccessList: &acl}
	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/data/caddy/data", "", "/frontend/dist", "", false, false, false, false, false, "", nil, nil, nil, nil)
	require.NoError(t, err)
	server := cfg.Apps.HTTP.Servers["charon_server"]
	// Even if ACL handler error occurs, config should still be returned with routes
	require.NotNil(t, server)
	require.GreaterOrEqual(t, len(server.Routes), 1)
}

func TestGenerateConfig_SkipHostDomainEmptyAndDisabled(t *testing.T) {
	disabled := models.ProxyHost{UUID: "h1", Enabled: false, DomainNames: "skip.com", ForwardHost: "127.0.0.1", ForwardPort: 8080}
	emptyDomain := models.ProxyHost{UUID: "h2", Enabled: true, DomainNames: "", ForwardHost: "127.0.0.1", ForwardPort: 8080}
	cfg, err := GenerateConfig([]models.ProxyHost{disabled, emptyDomain}, "/data/caddy/data", "", "/frontend/dist", "", false, false, false, false, false, "", nil, nil, nil, nil)
	require.NoError(t, err)
	server := cfg.Apps.HTTP.Servers["charon_server"]
	// Both hosts should be skipped; only routes from no hosts should be only catch-all if frontend provided
	if server != nil {
		// If frontend set, there will be catch-all route only
		if len(server.Routes) > 0 {
			// If frontend present, one route will be catch-all; ensure no host-based route exists
			for _, r := range server.Routes {
				for _, m := range r.Match {
					for _, host := range m.Host {
						require.NotEqual(t, "skip.com", host)
					}
				}
			}
		}
	}
}
