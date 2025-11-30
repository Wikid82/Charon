package caddy

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Wikid82/charon/backend/internal/logger"

	"github.com/Wikid82/charon/backend/internal/models"
)

// GenerateConfig creates a Caddy JSON configuration from proxy hosts.
// This is the core transformation layer from our database model to Caddy config.
func GenerateConfig(hosts []models.ProxyHost, storageDir string, acmeEmail string, frontendDir string, sslProvider string, acmeStaging bool) (*Config, error) {
	// Define log file paths
	// We assume storageDir is like ".../data/caddy/data", so we go up to ".../data/logs"
	// storageDir is .../data/caddy/data
	// Dir -> .../data/caddy
	// Dir -> .../data
	logDir := filepath.Join(filepath.Dir(filepath.Dir(storageDir)), "logs")
	logFile := filepath.Join(logDir, "access.log")

	config := &Config{
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
		}

		if len(uniqueDomains) == 0 {
			continue
		}

		// Build handlers for this host
		handlers := make([]Handler, 0)

		// Add Access Control List (ACL) handler if configured
		if host.AccessListID != nil && host.AccessList != nil && host.AccessList.Enabled {
			aclHandler, err := buildACLHandler(host.AccessList)
			if err != nil {
				logger.Log().WithField("host", host.UUID).WithError(err).Warn("Failed to build ACL handler for host")
			} else if aclHandler != nil {
				handlers = append(handlers, aclHandler)
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
			locRoute := &Route{
				Match: []Match{
					{
						Host: uniqueDomains,
						Path: []string{loc.Path, loc.Path + "/*"},
					},
				},
				Handle: []Handler{
					ReverseProxyHandler(dial, host.WebsocketSupport, host.Application),
				},
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
						normalizeHandlerHeaders(v)
						handlers = append(handlers, Handler(v))
					} else {
						logger.Log().WithField("host", host.UUID).Warn("advanced_config for host is not a handler object")
					}
				case []interface{}:
					for _, it := range v {
						if m, ok := it.(map[string]interface{}); ok {
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
		mainHandlers := append(handlers, ReverseProxyHandler(dial, host.WebsocketSupport, host.Application))

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

	config.Apps.HTTP.Servers["charon_server"] = &Server{
		Listen: []string{":80", ":443"},
		Routes: routes,
		AutoHTTPS: &AutoHTTPSConfig{
			Disable:      false,
			DisableRedir: false,
		},
		Logs: &ServerLogs{
			DefaultLoggerName: "access_log",
		},
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
func buildACLHandler(acl *models.AccessList) (Handler, error) {
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
			// Allow only these countries
			expression = fmt.Sprintf("{geoip2.country_code} in [%s]", strings.Join(trimmedCodes, ", "))
		} else {
			// geo_blacklist: Block these countries
			expression = fmt.Sprintf("{geoip2.country_code} not_in [%s]", strings.Join(trimmedCodes, ", "))
		}

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
		return Handler{
			"handler": "subroute",
			"routes": []map[string]interface{}{
				{
					"match": []map[string]interface{}{
						{
							"remote_ip": map[string]interface{}{
								"ranges": cidrs,
							},
						},
					},
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
