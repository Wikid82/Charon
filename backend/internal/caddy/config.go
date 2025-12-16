package caddy

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/Wikid82/charon/backend/internal/logger"

	"github.com/Wikid82/charon/backend/internal/models"
)

// GenerateConfig creates a Caddy JSON configuration from proxy hosts.
// This is the core transformation layer from our database model to Caddy config.
func GenerateConfig(hosts []models.ProxyHost, storageDir, acmeEmail, frontendDir, sslProvider string, acmeStaging, crowdsecEnabled, wafEnabled, rateLimitEnabled, aclEnabled bool, adminWhitelist string, rulesets []models.SecurityRuleSet, rulesetPaths map[string]string, decisions []models.SecurityDecision, secCfg *models.SecurityConfig) (*Config, error) {
	// Define log file paths for Caddy access logs.
	// When CrowdSec is enabled, we use /var/log/caddy/access.log which is the standard
	// location that CrowdSec's acquis.yaml is configured to monitor.
	// Otherwise, we fall back to the storageDir-relative path for development/non-Docker use.
	logFile := getAccessLogPath(storageDir, crowdsecEnabled)

	config := &Config{
		Admin: &AdminConfig{
			Listen: "0.0.0.0:2019", // Bind to all interfaces for container access
		},
		Logging: &LoggingConfig{
			Logs: map[string]*LogConfig{
				"access": {
					Level: "INFO",
					Writer: &WriterConfig{
						Output:       "file",
						Filename:     logFile,
						Roll:         true,
						RollSize:     10, // 10 MB
						RollKeep:     5,  // Keep 5 files
						RollKeepDays: 7,  // Keep for 7 days
					},
					Encoder: &EncoderConfig{
						Format: "json",
					},
					Include: []string{"http.log.access.access_log"},
				},
			},
		},
		Apps: Apps{
			HTTP: &HTTPApp{
				Servers: map[string]*Server{},
			},
		},
		Storage: Storage{
			System: "file_system",
			Root:   storageDir,
		},
	}

	// Configure CrowdSec app if enabled
	if crowdsecEnabled {
		apiURL := "http://127.0.0.1:8085"
		if secCfg != nil && secCfg.CrowdSecAPIURL != "" {
			apiURL = secCfg.CrowdSecAPIURL
		}
		apiKey := getCrowdSecAPIKey()
		enableStreaming := true

		config.Apps.CrowdSec = &CrowdSecApp{
			APIUrl:          apiURL,
			APIKey:          apiKey,
			TickerInterval:  "60s",
			EnableStreaming: &enableStreaming,
		}
	}

	if acmeEmail != "" {
		var issuers []interface{}

		// Configure issuers based on provider preference
		switch sslProvider {
		case "letsencrypt":
			acmeIssuer := map[string]interface{}{
				"module": "acme",
				"email":  acmeEmail,
			}
			if acmeStaging {
				acmeIssuer["ca"] = "https://acme-staging-v02.api.letsencrypt.org/directory"
			}
			issuers = append(issuers, acmeIssuer)
		case "zerossl":
			issuers = append(issuers, map[string]interface{}{
				"module": "zerossl",
			})
		default: // "both" or empty
			acmeIssuer := map[string]interface{}{
				"module": "acme",
				"email":  acmeEmail,
			}
			if acmeStaging {
				acmeIssuer["ca"] = "https://acme-staging-v02.api.letsencrypt.org/directory"
			}
			issuers = append(issuers, acmeIssuer)
			issuers = append(issuers, map[string]interface{}{
				"module": "zerossl",
			})
		}

		config.Apps.TLS = &TLSApp{
			Automation: &AutomationConfig{
				Policies: []*AutomationPolicy{
					{
						IssuersRaw: issuers,
					},
				},
			},
		}
	}

	// Collect CUSTOM certificates only (not Let's Encrypt - those are managed by ACME)
	// Only custom/uploaded certificates should be loaded via LoadPEM
	customCerts := make(map[uint]models.SSLCertificate)
	for _, host := range hosts {
		if host.CertificateID != nil && host.Certificate != nil {
			// Only include custom certificates, not ACME-managed ones
			if host.Certificate.Provider == "custom" {
				customCerts[*host.CertificateID] = *host.Certificate
			}
		}
	}

	if len(customCerts) > 0 {
		var loadPEM []LoadPEMConfig
		for _, cert := range customCerts {
			// Validate that custom cert has both certificate and key
			if cert.Certificate == "" || cert.PrivateKey == "" {
				logger.Log().WithField("cert", cert.Name).Warn("Custom certificate missing certificate or key, skipping")
				continue
			}
			loadPEM = append(loadPEM, LoadPEMConfig{
				Certificate: cert.Certificate,
				Key:         cert.PrivateKey,
				Tags:        []string{cert.UUID},
			})
		}

		if len(loadPEM) > 0 {
			if config.Apps.TLS == nil {
				config.Apps.TLS = &TLSApp{}
			}
			config.Apps.TLS.Certificates = &CertificatesConfig{
				LoadPEM: loadPEM,
			}
		}
	}

	if len(hosts) == 0 && frontendDir == "" {
		return config, nil
	}

	// Initialize routes slice
	routes := make([]*Route, 0)
	// Track IP-only hostnames to skip AutoHTTPS/ACME
	ipSubjects := make([]string, 0)

	// Track processed domains to prevent duplicates (Ghost Host fix)
	processedDomains := make(map[string]bool)

	// Sort hosts by UpdatedAt desc to prefer newer configs in case of duplicates
	// Note: This assumes the input slice is already sorted or we don't care about order beyond duplicates
	// The caller (ApplyConfig) fetches all hosts. We should probably sort them here or there.
	// For now, we'll just process them. If we encounter a duplicate domain, we skip it.
	// To ensure we keep the *latest* one, we should iterate in reverse or sort.
	// But ApplyConfig uses db.Find(&hosts), which usually returns by ID asc.
	// So later IDs (newer) come last.
	// We want to keep the NEWER one.
	// So we should iterate backwards? Or just overwrite?
	// Caddy config structure is a list of servers/routes.
	// If we have multiple routes matching the same host, Caddy uses the first one?
	// Actually, Caddy matches routes in order.
	// If we emit two routes for "example.com", the first one will catch it.
	// So we want the NEWEST one to be FIRST in the list?
	// Or we want to only emit ONE route for "example.com".
	// If we emit only one, it should be the newest one.
	// So we should process hosts from newest to oldest, and skip duplicates.

	// Let's iterate in reverse order (assuming input is ID ASC)
	for i := len(hosts) - 1; i >= 0; i-- {
		host := hosts[i]

		if !host.Enabled {
			continue
		}

		if host.DomainNames == "" {
			// Log warning?
			continue
		}

		// Parse comma-separated domains
		rawDomains := strings.Split(host.DomainNames, ",")
		var uniqueDomains []string
		isIPOnly := true

		for _, d := range rawDomains {
			d = strings.TrimSpace(d)
			d = strings.ToLower(d) // Normalize to lowercase
			if d == "" {
				continue
			}
			if processedDomains[d] {
				logger.Log().WithField("domain", d).WithField("host", host.UUID).Warn("Skipping duplicate domain for host (Ghost Host detection)")
				continue
			}
			processedDomains[d] = true
			uniqueDomains = append(uniqueDomains, d)
			if net.ParseIP(d) == nil {
				isIPOnly = false
			}
		}

		if len(uniqueDomains) == 0 {
			continue
		}

		if isIPOnly {
			ipSubjects = append(ipSubjects, uniqueDomains...)
		}

		// Build handlers for this host
		handlers := make([]Handler, 0)

		// Build security pre-handlers for this host, in pipeline order.
		securityHandlers := make([]Handler, 0)

		// Global decisions (e.g. manual block by IP) are applied first; collect IP blocks where action == "block"
		decisionIPs := make([]string, 0)
		for _, d := range decisions {
			if d.Action == "block" && d.IP != "" {
				decisionIPs = append(decisionIPs, d.IP)
			}
		}
		if len(decisionIPs) > 0 {
			// Build a subroute to match these remote IPs and serve 403
			// Admin whitelist exclusion must be applied: exclude adminWhitelist if present
			// Build matchParts
			var matchParts []map[string]interface{}
			matchParts = append(matchParts, map[string]interface{}{"remote_ip": map[string]interface{}{"ranges": decisionIPs}})
			if adminWhitelist != "" {
				adminParts := strings.Split(adminWhitelist, ",")
				trims := make([]string, 0)
				for _, p := range adminParts {
					p = strings.TrimSpace(p)
					if p == "" {
						continue
					}
					trims = append(trims, p)
				}
				if len(trims) > 0 {
					matchParts = append(matchParts, map[string]interface{}{"not": []map[string]interface{}{{"remote_ip": map[string]interface{}{"ranges": trims}}}})
				}
			}
			decHandler := Handler{
				"handler": "subroute",
				"routes": []map[string]interface{}{
					{
						"match": matchParts,
						"handle": []map[string]interface{}{
							{
								"handler":     "static_response",
								"status_code": 403,
								"body":        "Access denied: Blocked by security decision",
							},
						},
						"terminal": true,
					},
				},
			}
			// Prepend at the start of securityHandlers so it's evaluated first
			securityHandlers = append(securityHandlers, decHandler)
		}

		// CrowdSec handler (placeholder) — first in pipeline. The handler builder
		// now consumes the runtime flag so we can rely on the computed value
		// rather than requiring a persisted SecurityConfig row to be present.
		if csH, err := buildCrowdSecHandler(&host, secCfg, crowdsecEnabled); err == nil && csH != nil {
			securityHandlers = append(securityHandlers, csH)
		}

		// WAF handler (placeholder) — add according to runtime flag
		if wafH, err := buildWAFHandler(&host, rulesets, rulesetPaths, secCfg, wafEnabled); err == nil && wafH != nil {
			securityHandlers = append(securityHandlers, wafH)
		}

		// Rate Limit handler (placeholder)
		if rateLimitEnabled {
			if rlH, err := buildRateLimitHandler(&host, secCfg); err == nil && rlH != nil {
				securityHandlers = append(securityHandlers, rlH)
			}
		}

		// Add Access Control List (ACL) handler if configured and global ACL is enabled
		if aclEnabled && host.AccessListID != nil && host.AccessList != nil && host.AccessList.Enabled {
			aclHandler, err := buildACLHandler(host.AccessList, adminWhitelist)
			if err != nil {
				logger.Log().WithField("host", host.UUID).WithError(err).Warn("Failed to build ACL handler for host")
			} else if aclHandler != nil {
				securityHandlers = append(securityHandlers, aclHandler)
			}
		}

		// Add HSTS header if enabled
		if host.HSTSEnabled {
			hstsValue := "max-age=31536000"
			if host.HSTSSubdomains {
				hstsValue += "; includeSubDomains"
			}
			handlers = append(handlers, HeaderHandler(map[string][]string{
				"Strict-Transport-Security": {hstsValue},
			}))
		}

		// Add exploit blocking if enabled
		if host.BlockExploits {
			handlers = append(handlers, BlockExploitsHandler())
		}

		// Handle custom locations first (more specific routes)
		for _, loc := range host.Locations {
			dial := fmt.Sprintf("%s:%d", loc.ForwardHost, loc.ForwardPort)
			// For each location, we want the same security pre-handlers before proxy
			locHandlers := append(append([]Handler{}, securityHandlers...), handlers...)
			locHandlers = append(locHandlers, ReverseProxyHandler(dial, host.WebsocketSupport, host.Application))
			locRoute := &Route{
				Match: []Match{
					{
						Host: uniqueDomains,
						Path: []string{loc.Path, loc.Path + "/*"},
					},
				},
				Handle:   locHandlers,
				Terminal: true,
			}
			routes = append(routes, locRoute)
		}

		// Main proxy handler
		dial := fmt.Sprintf("%s:%d", host.ForwardHost, host.ForwardPort)
		// Insert user advanced config (if present) as headers or handlers before the reverse proxy
		// so user-specified headers/handlers are applied prior to proxying.
		if host.AdvancedConfig != "" {
			var parsed interface{}
			if err := json.Unmarshal([]byte(host.AdvancedConfig), &parsed); err != nil {
				logger.Log().WithField("host", host.UUID).WithError(err).Warn("Failed to parse advanced_config for host")
			} else {
				switch v := parsed.(type) {
				case map[string]interface{}:
					// Append as a handler
					// Ensure it has a "handler" key
					if _, ok := v["handler"]; ok {
						// Capture ruleset_name if present, remove it from advanced_config,
						// and set up 'directives' with Include statement for coraza-caddy plugin.
						if rn, has := v["ruleset_name"]; has {
							if rnStr, ok := rn.(string); ok && rnStr != "" {
								// Set 'directives' with Include statement for coraza-caddy
								if rulesetPaths != nil {
									if p, ok := rulesetPaths[rnStr]; ok && p != "" {
										v["directives"] = fmt.Sprintf("Include %s", p)
									}
								}
							}
							delete(v, "ruleset_name")
						}
						normalizeHandlerHeaders(v)
						handlers = append(handlers, Handler(v))
					} else {
						logger.Log().WithField("host", host.UUID).Warn("advanced_config for host is not a handler object")
					}
				case []interface{}:
					for _, it := range v {
						if m, ok := it.(map[string]interface{}); ok {
							if rn, has := m["ruleset_name"]; has {
								if rnStr, ok := rn.(string); ok && rnStr != "" {
									if rulesetPaths != nil {
										if p, ok := rulesetPaths[rnStr]; ok && p != "" {
											m["directives"] = fmt.Sprintf("Include %s", p)
										}
									}
								}
								delete(m, "ruleset_name")
							}
							normalizeHandlerHeaders(m)
							if _, ok2 := m["handler"]; ok2 {
								handlers = append(handlers, Handler(m))
							}
						}
					}
				default:
					logger.Log().WithField("host", host.UUID).Warn("advanced_config for host has unexpected JSON structure")
				}
			}
		}
		// Build main handlers: security pre-handlers, other host-level handlers, then reverse proxy
		mainHandlers := append(append([]Handler{}, securityHandlers...), handlers...)
		mainHandlers = append(mainHandlers, ReverseProxyHandler(dial, host.WebsocketSupport, host.Application))

		route := &Route{
			Match: []Match{
				{Host: uniqueDomains},
			},
			Handle:   mainHandlers,
			Terminal: true,
		}

		routes = append(routes, route)
	}

	// Add catch-all 404 handler
	// This matches any request that wasn't handled by previous routes
	if frontendDir != "" {
		catchAllRoute := &Route{
			Handle: []Handler{
				RewriteHandler("/unknown.html"),
				FileServerHandler(frontendDir),
			},
			Terminal: true,
		}
		routes = append(routes, catchAllRoute)
	}

	autoHTTPS := &AutoHTTPSConfig{Disable: false, DisableRedir: false}
	if len(ipSubjects) > 0 {
		// Skip AutoHTTPS/ACME for IP literals to avoid ERR_SSL_PROTOCOL_ERROR
		autoHTTPS.Skip = append(autoHTTPS.Skip, ipSubjects...)
	}

	// Configure trusted proxies for proper client IP detection from X-Forwarded-For headers
	// This is required for CrowdSec bouncer to correctly identify and block real client IPs
	// when running behind Docker networks, reverse proxies, or CDNs
	// Reference: https://caddyserver.com/docs/json/apps/http/servers/#trusted_proxies
	trustedProxies := &TrustedProxies{
		Source: "static",
		Ranges: []string{
			"127.0.0.1/32",   // Localhost
			"::1/128",        // IPv6 localhost
			"172.16.0.0/12",  // Docker bridge networks (172.16-31.x.x)
			"10.0.0.0/8",     // Private network
			"192.168.0.0/16", // Private network
		},
	}

	config.Apps.HTTP.Servers["charon_server"] = &Server{
		Listen:         []string{":80", ":443"},
		Routes:         routes,
		AutoHTTPS:      autoHTTPS,
		TrustedProxies: trustedProxies,
		Logs: &ServerLogs{
			DefaultLoggerName: "access_log",
		},
	}

	// Provide internal certificates for IP subjects when present so optional TLS can succeed without ACME
	if len(ipSubjects) > 0 {
		if config.Apps.TLS == nil {
			config.Apps.TLS = &TLSApp{}
		}
		policy := &AutomationPolicy{
			Subjects:   ipSubjects,
			IssuersRaw: []interface{}{map[string]interface{}{"module": "internal"}},
		}
		if config.Apps.TLS.Automation == nil {
			config.Apps.TLS.Automation = &AutomationConfig{}
		}
		config.Apps.TLS.Automation.Policies = append(config.Apps.TLS.Automation.Policies, policy)
	}

	return config, nil
}

// normalizeHandlerHeaders ensures header values in handlers are arrays of strings
// Caddy's JSON schema expects header values to be an array of strings (e.g. ["websocket"]) rather than a single string.
func normalizeHandlerHeaders(h map[string]interface{}) {
	// normalize top-level headers key
	if headersRaw, ok := h["headers"].(map[string]interface{}); ok {
		normalizeHeaderOps(headersRaw)
	}
	// also normalize in nested request/response if present explicitly
	for _, side := range []string{"request", "response"} {
		if sideRaw, ok := h[side].(map[string]interface{}); ok {
			normalizeHeaderOps(sideRaw)
		}
	}
}

func normalizeHeaderOps(headerOps map[string]interface{}) {
	if setRaw, ok := headerOps["set"].(map[string]interface{}); ok {
		for k, v := range setRaw {
			switch vv := v.(type) {
			case string:
				setRaw[k] = []string{vv}
			case []interface{}:
				// convert to []string
				arr := make([]string, 0, len(vv))
				for _, it := range vv {
					arr = append(arr, fmt.Sprintf("%v", it))
				}
				setRaw[k] = arr
			case []string:
				// nothing to do
			default:
				// coerce anything else to string slice
				setRaw[k] = []string{fmt.Sprintf("%v", vv)}
			}
		}
		headerOps["set"] = setRaw
	}
}

// NormalizeAdvancedConfig traverses a parsed JSON advanced config (map or array)
// and normalizes any headers blocks so that header values are arrays of strings.
// It returns the modified config object which can be JSON marshaled again.
func NormalizeAdvancedConfig(parsed interface{}) interface{} {
	switch v := parsed.(type) {
	case map[string]interface{}:
		// This might be a handler object
		normalizeHandlerHeaders(v)
		// Also inspect nested 'handle' or 'routes' arrays for nested handlers
		if handles, ok := v["handle"].([]interface{}); ok {
			for _, it := range handles {
				if m, ok := it.(map[string]interface{}); ok {
					NormalizeAdvancedConfig(m)
				}
			}
		}
		if routes, ok := v["routes"].([]interface{}); ok {
			for _, rit := range routes {
				if rm, ok := rit.(map[string]interface{}); ok {
					if handles, ok := rm["handle"].([]interface{}); ok {
						for _, it := range handles {
							if m, ok := it.(map[string]interface{}); ok {
								NormalizeAdvancedConfig(m)
							}
						}
					}
				}
			}
		}
		return v
	case []interface{}:
		for _, it := range v {
			if m, ok := it.(map[string]interface{}); ok {
				NormalizeAdvancedConfig(m)
			}
		}
		return v
	default:
		return parsed
	}
}

// buildACLHandler creates access control handlers based on the AccessList configuration
func buildACLHandler(acl *models.AccessList, adminWhitelist string) (Handler, error) {
	// For geo-blocking, we use CEL (Common Expression Language) matcher with caddy-geoip2 placeholders
	// For IP-based ACLs, we use Caddy's native remote_ip matcher

	if strings.HasPrefix(acl.Type, "geo_") {
		// Geo-blocking using caddy-geoip2
		countryCodes := strings.Split(acl.CountryCodes, ",")
		var trimmedCodes []string
		for _, code := range countryCodes {
			trimmedCodes = append(trimmedCodes, `"`+strings.TrimSpace(code)+`"`)
		}

		var expression string
		if acl.Type == "geo_whitelist" {
			// Allow only these countries, so block when not in the whitelist
			expression = fmt.Sprintf("{geoip2.country_code} in [%s]", strings.Join(trimmedCodes, ", "))
			// For whitelist, block when NOT in the list
			return Handler{
				"handler": "subroute",
				"routes": []map[string]interface{}{
					{
						"match": []map[string]interface{}{
							{
								"not": []map[string]interface{}{
									{
										"expression": expression,
									},
								},
							},
						},
						"handle": []map[string]interface{}{
							{
								"handler":     "static_response",
								"status_code": 403,
								"body":        "Access denied: Geographic restriction",
							},
						},
						"terminal": true,
					},
				},
			}, nil
		}
		// geo_blacklist: Block these countries directly
		expression = fmt.Sprintf("{geoip2.country_code} in [%s]", strings.Join(trimmedCodes, ", "))
		return Handler{
			"handler": "subroute",
			"routes": []map[string]interface{}{
				{
					"match": []map[string]interface{}{
						{
							"expression": expression,
						},
					},
					"handle": []map[string]interface{}{
						{
							"handler":     "static_response",
							"status_code": 403,
							"body":        "Access denied: Geographic restriction",
						},
					},
					"terminal": true,
				},
			},
		}, nil
	}

	// IP/CIDR-based ACLs using Caddy's native remote_ip matcher
	if acl.LocalNetworkOnly {
		// Allow only RFC1918 private networks
		return Handler{
			"handler": "subroute",
			"routes": []map[string]interface{}{
				{
					"match": []map[string]interface{}{
						{
							"not": []map[string]interface{}{
								{
									"remote_ip": map[string]interface{}{
										"ranges": []string{
											"10.0.0.0/8",
											"172.16.0.0/12",
											"192.168.0.0/16",
											"127.0.0.0/8",
											"169.254.0.0/16",
											"fc00::/7",
											"fe80::/10",
											"::1/128",
										},
									},
								},
							},
						},
					},
					"handle": []map[string]interface{}{
						{
							"handler":     "static_response",
							"status_code": 403,
							"body":        "Access denied: Not a local network IP",
						},
					},
					"terminal": true,
				},
			},
		}, nil
	}

	// Parse IP rules
	if acl.IPRules == "" {
		return nil, nil
	}

	var rules []models.AccessListRule
	if err := json.Unmarshal([]byte(acl.IPRules), &rules); err != nil {
		return nil, fmt.Errorf("invalid IP rules JSON: %w", err)
	}

	if len(rules) == 0 {
		return nil, nil
	}

	// Extract CIDR ranges
	var cidrs []string
	for _, rule := range rules {
		cidrs = append(cidrs, rule.CIDR)
	}

	if acl.Type == "whitelist" {
		// Allow only these IPs (block everything else)
		// Merge adminWhitelist into allowed cidrs so that admins always bypass whitelist checks
		if adminWhitelist != "" {
			adminParts := strings.Split(adminWhitelist, ",")
			for _, p := range adminParts {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				cidrs = append(cidrs, p)
			}
		}
		return Handler{
			"handler": "subroute",
			"routes": []map[string]interface{}{
				{
					"match": []map[string]interface{}{
						{
							"not": []map[string]interface{}{
								{
									"remote_ip": map[string]interface{}{
										"ranges": cidrs,
									},
								},
							},
						},
					},
					"handle": []map[string]interface{}{
						{
							"handler":     "static_response",
							"status_code": 403,
							"body":        "Access denied: IP not in whitelist",
						},
					},
					"terminal": true,
				},
			},
		}, nil
	}

	if acl.Type == "blacklist" {
		// Block these IPs (allow everything else)
		// For blacklist, add an explicit 'not' clause excluding adminWhitelist ranges from the match
		var adminExclusion interface{}
		if adminWhitelist != "" {
			adminParts := strings.Split(adminWhitelist, ",")
			trims := make([]string, 0)
			for _, p := range adminParts {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				trims = append(trims, p)
			}
			if len(trims) > 0 {
				adminExclusion = map[string]interface{}{"not": []map[string]interface{}{{"remote_ip": map[string]interface{}{"ranges": trims}}}}
			}
		}
		// Build matcher parts
		matchParts := []map[string]interface{}{}
		matchParts = append(matchParts, map[string]interface{}{"remote_ip": map[string]interface{}{"ranges": cidrs}})
		if adminExclusion != nil {
			matchParts = append(matchParts, adminExclusion.(map[string]interface{}))
		}
		return Handler{
			"handler": "subroute",
			"routes": []map[string]interface{}{
				{
					"match": matchParts,
					"handle": []map[string]interface{}{
						{
							"handler":     "static_response",
							"status_code": 403,
							"body":        "Access denied: IP blacklisted",
						},
					},
					"terminal": true,
				},
			},
		}, nil
	}

	return nil, nil
}

// buildCrowdSecHandler returns a minimal CrowdSec handler for the caddy-crowdsec-bouncer plugin.
// The app-level configuration (apps.crowdsec) is populated in GenerateConfig(),
// so the handler only needs to reference the module name.
// Reference: https://github.com/hslatman/caddy-crowdsec-bouncer
func buildCrowdSecHandler(_ *models.ProxyHost, _ *models.SecurityConfig, crowdsecEnabled bool) (Handler, error) {
	// Only add a handler when the computed runtime flag indicates CrowdSec is enabled.
	if !crowdsecEnabled {
		return nil, nil
	}

	// Return minimal handler - all config is at app-level
	return Handler{"handler": "crowdsec"}, nil
}

// getCrowdSecAPIKey retrieves the CrowdSec bouncer API key from environment variables.
func getCrowdSecAPIKey() string {
	envVars := []string{
		"CROWDSEC_API_KEY",
		"CROWDSEC_BOUNCER_API_KEY",
		"CERBERUS_SECURITY_CROWDSEC_API_KEY",
		"CHARON_SECURITY_CROWDSEC_API_KEY",
		"CPM_SECURITY_CROWDSEC_API_KEY",
	}

	for _, key := range envVars {
		if val := os.Getenv(key); val != "" {
			return val
		}
	}
	return ""
}

// getAccessLogPath determines the appropriate path for Caddy access logs.
// When CrowdSec is enabled or running in Docker (detected via /.dockerenv),
// we use /var/log/caddy/access.log which is the standard location that
// CrowdSec's acquis.yaml is configured to monitor.
// Otherwise, we fall back to the storageDir-relative path for development use.
//
// The access logs written to this path include:
//   - Standard HTTP fields (method, uri, status, duration, size)
//   - Client IP for CrowdSec and security analysis
//   - User-Agent for attack detection
//   - Security-relevant response headers (X-Coraza-Id, X-RateLimit-Remaining)
func getAccessLogPath(storageDir string, crowdsecEnabled bool) string {
	// Standard CrowdSec-compatible path used in production Docker containers
	const crowdsecLogPath = "/var/log/caddy/access.log"

	// Use standard path when CrowdSec is enabled (explicit request)
	if crowdsecEnabled {
		return crowdsecLogPath
	}

	// Detect Docker environment via /.dockerenv file
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return crowdsecLogPath
	}

	// Check for CHARON_ENV=production or container-like environment
	if env := os.Getenv("CHARON_ENV"); env == "production" {
		return crowdsecLogPath
	}

	// Development fallback: use storageDir-relative path
	// storageDir is .../data/caddy/data
	// Dir -> .../data/caddy
	// Dir -> .../data
	logDir := filepath.Join(filepath.Dir(filepath.Dir(storageDir)), "logs")
	return filepath.Join(logDir, "access.log")
}

// buildWAFHandler returns a WAF handler (Coraza) configuration.
// The coraza-caddy plugin registers as http.handlers.waf and expects:
// - handler: "waf"
// - directives: ModSecurity directive string including Include statements
//
// This function builds a complete Coraza configuration with:
// - SecRuleEngine (On/DetectionOnly based on mode)
// - Paranoia level via SecAction
// - Rule exclusions via SecRuleRemoveById
// - Include statements for ruleset files
func buildWAFHandler(host *models.ProxyHost, rulesets []models.SecurityRuleSet, rulesetPaths map[string]string, secCfg *models.SecurityConfig, wafEnabled bool) (Handler, error) {
	// Early exit if WAF is disabled globally
	if !wafEnabled {
		return nil, nil
	}
	if secCfg != nil && secCfg.WAFMode == "disabled" {
		return nil, nil
	}

	// Check per-host WAF toggle - if host has WAF disabled, skip
	if host != nil && host.WAFDisabled {
		return nil, nil
	}

	// If the host provided an advanced_config containing a 'ruleset_name', prefer that value
	var hostRulesetName string
	if host != nil && host.AdvancedConfig != "" {
		var ac map[string]interface{}
		if err := json.Unmarshal([]byte(host.AdvancedConfig), &ac); err == nil {
			if rn, ok := ac["ruleset_name"]; ok {
				if rnStr, ok2 := rn.(string); ok2 && rnStr != "" {
					hostRulesetName = rnStr
				}
			}
		}
	}

	// Find a ruleset to associate with WAF
	// Priority order:
	// 1. Exact match to secCfg.WAFRulesSource (user's global choice)
	// 2. Exact match to hostRulesetName (per-host advanced_config)
	// 3. Match to host.Application (app-specific defaults)
	// 4. Fallback to owasp-crs
	var selected *models.SecurityRuleSet
	var hostRulesetMatch, appMatch, owaspFallback *models.SecurityRuleSet

	// First pass: find all potential matches
	for i, r := range rulesets {
		// Priority 1: Global WAF rules source - highest priority, select immediately
		if secCfg != nil && secCfg.WAFRulesSource != "" && r.Name == secCfg.WAFRulesSource {
			selected = &rulesets[i]
			break
		}
		// Priority 2: Per-host ruleset name from advanced_config
		if hostRulesetName != "" && r.Name == hostRulesetName && hostRulesetMatch == nil {
			hostRulesetMatch = &rulesets[i]
		}
		// Priority 3: Match by host application
		if host != nil && r.Name == host.Application && appMatch == nil {
			appMatch = &rulesets[i]
		}
		// Priority 4: Track owasp-crs as fallback
		if r.Name == "owasp-crs" && owaspFallback == nil {
			owaspFallback = &rulesets[i]
		}
	}

	// Second pass: select by priority if not already selected
	if selected == nil {
		if hostRulesetMatch != nil {
			selected = hostRulesetMatch
		} else if appMatch != nil {
			selected = appMatch
		} else if owaspFallback != nil {
			selected = owaspFallback
		}
	}

	// Build the directives string for Coraza
	directives := buildWAFDirectives(secCfg, selected, rulesetPaths)

	// Bug fix: Don't return a WAF handler without directives - it creates a no-op WAF
	if directives == "" {
		return nil, nil
	}

	h := Handler{
		"handler":    "waf",
		"directives": directives,
	}

	return h, nil
}

// buildWAFDirectives constructs the ModSecurity directive string for Coraza.
// It includes:
// - SecRuleEngine directive (On or DetectionOnly)
// - SecRequestBodyAccess and SecResponseBodyAccess
// - Paranoia level via SecAction
// - Rule exclusions via SecRuleRemoveById
// - Include statements for ruleset files
//
// Returns empty string if no ruleset Include can be generated, since a WAF
// without loaded rules is essentially a no-op.
func buildWAFDirectives(secCfg *models.SecurityConfig, ruleset *models.SecurityRuleSet, rulesetPaths map[string]string) string {
	var directives strings.Builder

	// Track if we found a ruleset to include
	hasRuleset := false
	var rulesetPath string

	// Include ruleset file if available
	if ruleset != nil && rulesetPaths != nil {
		if p, ok := rulesetPaths[ruleset.Name]; ok && p != "" {
			hasRuleset = true
			rulesetPath = p
		}
	} else if secCfg != nil && secCfg.WAFRulesSource != "" && rulesetPaths != nil {
		// Fallback: include path if known from WAFRulesSource
		if p, ok := rulesetPaths[secCfg.WAFRulesSource]; ok && p != "" {
			hasRuleset = true
			rulesetPath = p
		}
	}

	// If no ruleset to include, return empty - WAF without rules is a no-op
	if !hasRuleset {
		return ""
	}

	// Determine SecRuleEngine mode
	engine := "On"
	if secCfg != nil && secCfg.WAFMode == "monitor" {
		engine = "DetectionOnly"
	}
	directives.WriteString(fmt.Sprintf("SecRuleEngine %s\n", engine))

	// Enable request body inspection, disable response body for performance
	directives.WriteString("SecRequestBodyAccess On\n")
	directives.WriteString("SecResponseBodyAccess Off\n")

	// Set paranoia level (default to 1 if not configured)
	paranoiaLevel := 1
	if secCfg != nil && secCfg.WAFParanoiaLevel >= 1 && secCfg.WAFParanoiaLevel <= 4 {
		paranoiaLevel = secCfg.WAFParanoiaLevel
	}
	directives.WriteString(fmt.Sprintf("SecAction \"id:900000,phase:1,nolog,pass,t:none,setvar:tx.paranoia_level=%d\"\n", paranoiaLevel))

	// Include the ruleset file
	directives.WriteString(fmt.Sprintf("Include %s\n", rulesetPath))

	// Process exclusions from SecurityConfig
	if secCfg != nil && secCfg.WAFExclusions != "" {
		exclusions := parseWAFExclusions(secCfg.WAFExclusions)
		for _, excl := range exclusions {
			if excl.Target != "" {
				// Use SecRuleUpdateTargetById to exclude specific targets
				directives.WriteString(fmt.Sprintf("SecRuleUpdateTargetById %d \"!%s\"\n", excl.RuleID, excl.Target))
			} else {
				// Remove the rule entirely
				directives.WriteString(fmt.Sprintf("SecRuleRemoveById %d\n", excl.RuleID))
			}
		}
	}

	return directives.String()
}

// WAFExclusion represents a rule exclusion for false positive handling
type WAFExclusion struct {
	RuleID      int    `json:"rule_id"`
	Target      string `json:"target,omitempty"`      // e.g., "ARGS:password"
	Description string `json:"description,omitempty"` // Human-readable reason
}

// parseWAFExclusions parses the JSON array of WAF exclusions from SecurityConfig
func parseWAFExclusions(exclusionsJSON string) []WAFExclusion {
	if exclusionsJSON == "" {
		return nil
	}

	var exclusions []WAFExclusion
	if err := json.Unmarshal([]byte(exclusionsJSON), &exclusions); err != nil {
		logger.Log().WithError(err).Warn("Failed to parse WAF exclusions JSON")
		return nil
	}

	return exclusions
}

// buildRateLimitHandler returns a rate-limit handler using the caddy-ratelimit module.
// The module is registered as http.handlers.rate_limit and expects:
// - handler: "rate_limit"
// - rate_limits: map of named rate limit zones with key, window, and max_events
// See: https://github.com/mholt/caddy-ratelimit
//
// Note: The rateLimitEnabled flag is already checked by the caller (GenerateConfig).
// This function only validates that the config has positive request/window values.
//
// If RateLimitBypassList is configured, the rate limiter is wrapped in a subroute
// that skips rate limiting for IPs matching the bypass CIDRs.
func buildRateLimitHandler(_ *models.ProxyHost, secCfg *models.SecurityConfig) (Handler, error) {
	if secCfg == nil {
		return nil, nil
	}
	if secCfg.RateLimitRequests <= 0 || secCfg.RateLimitWindowSec <= 0 {
		return nil, nil
	}

	// Build the base rate_limit handler using caddy-ratelimit format
	// Note: The caddy-ratelimit module uses a sliding window algorithm
	// and does not have a separate burst parameter
	rateLimitHandler := Handler{"handler": "rate_limit"}
	rateLimitHandler["rate_limits"] = map[string]interface{}{
		"static": map[string]interface{}{
			"key":        "{http.request.remote.host}",
			"window":     fmt.Sprintf("%ds", secCfg.RateLimitWindowSec),
			"max_events": secCfg.RateLimitRequests,
		},
	}

	// Parse bypass list CIDRs if configured
	bypassCIDRs := parseBypassCIDRs(secCfg.RateLimitBypassList)

	// If no bypass list, return the plain rate_limit handler
	if len(bypassCIDRs) == 0 {
		return rateLimitHandler, nil
	}

	// Wrap in a subroute that skips rate limiting for bypass IPs
	// Structure:
	// 1. Match bypass IPs -> do nothing (skip rate limiting)
	// 2. Everything else -> apply rate limiting
	return Handler{
		"handler": "subroute",
		"routes": []map[string]interface{}{
			{
				// Route 1: Match bypass IPs - terminal with no handlers (skip rate limiting)
				"match": []map[string]interface{}{
					{
						"remote_ip": map[string]interface{}{
							"ranges": bypassCIDRs,
						},
					},
				},
				// No handlers - just pass through without rate limiting
				"handle": []map[string]interface{}{},
			},
			{
				// Route 2: Default - apply rate limiting to everyone else
				"handle": []map[string]interface{}{
					rateLimitHandler,
				},
			},
		},
	}, nil
}

// parseBypassCIDRs parses a comma-separated list of CIDRs and returns valid ones.
// Invalid entries are silently ignored.
func parseBypassCIDRs(bypassList string) []string {
	if bypassList == "" {
		return nil
	}

	var validCIDRs []string
	parts := strings.Split(bypassList, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		// Validate CIDR format
		_, _, err := net.ParseCIDR(p)
		if err != nil {
			// Try as plain IP - convert to CIDR
			ip := net.ParseIP(p)
			if ip != nil {
				if ip.To4() != nil {
					p += "/32"
				} else {
					p += "/128"
				}
				validCIDRs = append(validCIDRs, p)
			}
			// Skip invalid entries
			continue
		}
		validCIDRs = append(validCIDRs, p)
	}
	return validCIDRs
}
