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
	"github.com/Wikid82/charon/backend/pkg/dnsprovider"
)

// GenerateConfig creates a Caddy JSON configuration from proxy hosts.
// This is the core transformation layer from our database model to Caddy config.
func GenerateConfig(hosts []models.ProxyHost, storageDir, acmeEmail, frontendDir, sslProvider string, acmeStaging, crowdsecEnabled, wafEnabled, rateLimitEnabled, aclEnabled bool, adminWhitelist string, rulesets []models.SecurityRuleSet, rulesetPaths map[string]string, decisions []models.SecurityDecision, secCfg *models.SecurityConfig, dnsProviderConfigs []DNSProviderConfig) (*Config, error) {
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

	// Group hosts by DNS provider for TLS automation policies
	// We need separate policies for:
	// 1. Wildcard domains with DNS challenge (per DNS provider)
	// 2. Regular domains with HTTP challenge (default policy)
	var tlsPolicies []*AutomationPolicy

	// Build a map of DNS provider ID to DNS provider config for quick lookup
	dnsProviderMap := make(map[uint]DNSProviderConfig)
	for _, cfg := range dnsProviderConfigs {
		dnsProviderMap[cfg.ID] = cfg
	}

	// Build a map of DNS provider ID to domains that need DNS challenge
	dnsProviderDomains := make(map[uint][]string)
	var httpChallengeDomains []string

	isE2E := os.Getenv("CHARON_ENV") == "e2e"

	if acmeEmail != "" || isE2E {
		for _, host := range hosts {
			if !host.Enabled || host.DomainNames == "" {
				continue
			}

			rawDomains := strings.Split(host.DomainNames, ",")
			var cleanDomains []string
			var nonIPDomains []string
			for _, d := range rawDomains {
				d = strings.TrimSpace(d)
				d = strings.ToLower(d)
				if d != "" {
					cleanDomains = append(cleanDomains, d)
					// Skip IP addresses for ACME issuers (they'll get internal issuer later)
					if net.ParseIP(d) == nil {
						nonIPDomains = append(nonIPDomains, d)
					}
				}
			}

			// Check if this host has wildcard domains and DNS provider
			if hasWildcard(cleanDomains) && host.DNSProviderID != nil && host.DNSProvider != nil {
				// Use DNS challenge for this host (include all domains including IPs for routing)
				dnsProviderDomains[*host.DNSProviderID] = append(dnsProviderDomains[*host.DNSProviderID], cleanDomains...)
			} else if len(nonIPDomains) > 0 {
				// Use HTTP challenge for non-IP domains only
				httpChallengeDomains = append(httpChallengeDomains, nonIPDomains...)
			}
		}

		// Create DNS challenge policies for each DNS provider
		for providerID, domains := range dnsProviderDomains {
			if isE2E {
				tlsPolicies = append(tlsPolicies, &AutomationPolicy{
					Subjects:   dedupeDomains(domains),
					IssuersRaw: []any{map[string]any{"module": "internal"}},
				})
				continue
			}

			// Find the DNS provider config
			dnsConfig, ok := dnsProviderMap[providerID]
			if !ok {
				logger.Log().WithField("provider_id", providerID).Warn("DNS provider not found in decrypted configs")
				continue
			}

			// **CHANGED: Multi-credential support**
			// If provider uses multi-credentials, create separate policies per domain
			if dnsConfig.UseMultiCredentials && len(dnsConfig.ZoneCredentials) > 0 {
				// Get provider plugin from registry
				provider, providerOK := dnsprovider.Global().Get(dnsConfig.ProviderType)
				if !providerOK {
					logger.Log().WithField("provider_type", dnsConfig.ProviderType).Warn("DNS provider type not found in registry")
					continue
				}

				// Create a separate TLS automation policy for each domain with its own credentials
				for baseDomain, credentials := range dnsConfig.ZoneCredentials {
					// Find all domains that match this base domain
					var matchingDomains []string
					for _, domain := range domains {
						if extractBaseDomain(domain) == baseDomain {
							matchingDomains = append(matchingDomains, domain)
						}
					}

					if len(matchingDomains) == 0 {
						continue // No domains for this credential
					}

					// Build provider config using registry plugin
					var providerConfig map[string]any
					if provider.SupportsMultiCredential() {
						providerConfig = provider.BuildCaddyConfigForZone(baseDomain, credentials)
					} else {
						providerConfig = provider.BuildCaddyConfig(credentials)
					}

					// Get propagation timeout from provider
					propagationTimeout := int64(provider.PropagationTimeout().Seconds())

					// Build issuer config with these credentials
					var issuers []any
					switch sslProvider {
					case "letsencrypt":
						acmeIssuer := map[string]any{
							"module": "acme",
							"email":  acmeEmail,
							"challenges": map[string]any{
								"dns": map[string]any{
									"provider":            providerConfig,
									"propagation_timeout": propagationTimeout * 1_000_000_000,
								},
							},
						}
						if acmeStaging {
							acmeIssuer["ca"] = "https://acme-staging-v02.api.letsencrypt.org/directory"
						}
						issuers = append(issuers, acmeIssuer)
					case "zerossl":
						issuers = append(issuers, map[string]any{
							"module": "zerossl",
							"challenges": map[string]any{
								"dns": map[string]any{
									"provider":            providerConfig,
									"propagation_timeout": propagationTimeout * 1_000_000_000,
								},
							},
						})
					default: // "both" or empty
						acmeIssuer := map[string]any{
							"module": "acme",
							"email":  acmeEmail,
							"challenges": map[string]any{
								"dns": map[string]any{
									"provider":            providerConfig,
									"propagation_timeout": propagationTimeout * 1_000_000_000,
								},
							},
						}
						if acmeStaging {
							acmeIssuer["ca"] = "https://acme-staging-v02.api.letsencrypt.org/directory"
						}
						issuers = append(issuers, acmeIssuer)
						issuers = append(issuers, map[string]any{
							"module": "zerossl",
							"challenges": map[string]any{
								"dns": map[string]any{
									"provider":            providerConfig,
									"propagation_timeout": propagationTimeout * 1_000_000_000,
								},
							},
						})
					}

					// Create TLS automation policy for this domain with zone-specific credentials
					tlsPolicies = append(tlsPolicies, &AutomationPolicy{
						Subjects:   dedupeDomains(matchingDomains),
						IssuersRaw: issuers,
					})

					logger.Log().WithFields(map[string]any{
						"provider_id":     providerID,
						"base_domain":     baseDomain,
						"domain_count":    len(matchingDomains),
						"credential_used": true,
					}).Debug("created DNS challenge policy with zone-specific credential")
				}

				// Skip the original single-credential logic below
				continue
			}

			// **ORIGINAL: Single-credential mode (backward compatible)**
			// Get provider plugin from registry
			provider, ok := dnsprovider.Global().Get(dnsConfig.ProviderType)
			if !ok {
				logger.Log().WithField("provider_type", dnsConfig.ProviderType).Warn("DNS provider type not found in registry")
				continue
			}

			// Build provider config using registry plugin
			providerConfig := provider.BuildCaddyConfig(dnsConfig.Credentials)

			// Get propagation timeout from provider
			propagationTimeout := int64(provider.PropagationTimeout().Seconds())

			// Create DNS challenge issuer
			var issuers []any
			switch sslProvider {
			case "letsencrypt":
				acmeIssuer := map[string]any{
					"module": "acme",
					"email":  acmeEmail,
					"challenges": map[string]any{
						"dns": map[string]any{
							"provider":            providerConfig,
							"propagation_timeout": propagationTimeout * 1_000_000_000, // convert seconds to nanoseconds
						},
					},
				}
				if acmeStaging {
					acmeIssuer["ca"] = "https://acme-staging-v02.api.letsencrypt.org/directory"
				}
				issuers = append(issuers, acmeIssuer)
			case "zerossl":
				// ZeroSSL with DNS challenge
				issuers = append(issuers, map[string]any{
					"module": "zerossl",
					"challenges": map[string]any{
						"dns": map[string]any{
							"provider":            providerConfig,
							"propagation_timeout": propagationTimeout * 1_000_000_000,
						},
					},
				})
			default: // "both" or empty
				acmeIssuer := map[string]any{
					"module": "acme",
					"email":  acmeEmail,
					"challenges": map[string]any{
						"dns": map[string]any{
							"provider":            providerConfig,
							"propagation_timeout": propagationTimeout * 1_000_000_000,
						},
					},
				}
				if acmeStaging {
					acmeIssuer["ca"] = "https://acme-staging-v02.api.letsencrypt.org/directory"
				}
				issuers = append(issuers, acmeIssuer)
				issuers = append(issuers, map[string]any{
					"module": "zerossl",
					"challenges": map[string]any{
						"dns": map[string]any{
							"provider":            providerConfig,
							"propagation_timeout": propagationTimeout * 1_000_000_000,
						},
					},
				})
			}

			tlsPolicies = append(tlsPolicies, &AutomationPolicy{
				Subjects:   dedupeDomains(domains),
				IssuersRaw: issuers,
			})
		}

		// Create default HTTP challenge policy for non-wildcard domains
		if len(httpChallengeDomains) > 0 {
			if isE2E {
				tlsPolicies = append(tlsPolicies, &AutomationPolicy{
					Subjects:   dedupeDomains(httpChallengeDomains),
					IssuersRaw: []any{map[string]any{"module": "internal"}},
				})
			} else {
				var issuers []any
				switch sslProvider {
				case "letsencrypt":
					acmeIssuer := map[string]any{
						"module": "acme",
						"email":  acmeEmail,
					}
					if acmeStaging {
						acmeIssuer["ca"] = "https://acme-staging-v02.api.letsencrypt.org/directory"
					}
					issuers = append(issuers, acmeIssuer)
				case "zerossl":
					issuers = append(issuers, map[string]any{
						"module": "zerossl",
					})
				default: // "both" or empty
					acmeIssuer := map[string]any{
						"module": "acme",
						"email":  acmeEmail,
					}
					if acmeStaging {
						acmeIssuer["ca"] = "https://acme-staging-v02.api.letsencrypt.org/directory"
					}
					issuers = append(issuers, acmeIssuer)
					issuers = append(issuers, map[string]any{
						"module": "zerossl",
					})
				}

				tlsPolicies = append(tlsPolicies, &AutomationPolicy{
					Subjects:   dedupeDomains(httpChallengeDomains),
					IssuersRaw: issuers,
				})
			}
		}

		// Create default policy if no specific domains were configured
		if len(tlsPolicies) == 0 {
			if isE2E {
				tlsPolicies = append(tlsPolicies, &AutomationPolicy{
					IssuersRaw: []any{map[string]any{"module": "internal"}},
				})
			} else {
				var issuers []any
				switch sslProvider {
				case "letsencrypt":
					acmeIssuer := map[string]any{
						"module": "acme",
						"email":  acmeEmail,
					}
					if acmeStaging {
						acmeIssuer["ca"] = "https://acme-staging-v02.api.letsencrypt.org/directory"
					}
					issuers = append(issuers, acmeIssuer)
				case "zerossl":
					issuers = append(issuers, map[string]any{
						"module": "zerossl",
					})
				default: // "both" or empty
					acmeIssuer := map[string]any{
						"module": "acme",
						"email":  acmeEmail,
					}
					if acmeStaging {
						acmeIssuer["ca"] = "https://acme-staging-v02.api.letsencrypt.org/directory"
					}
					issuers = append(issuers, acmeIssuer)
					issuers = append(issuers, map[string]any{
						"module": "zerossl",
					})
				}

				tlsPolicies = append(tlsPolicies, &AutomationPolicy{
					IssuersRaw: issuers,
				})
			}
		}

		config.Apps.TLS = &TLSApp{
			Automation: &AutomationConfig{
				Policies: tlsPolicies,
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
	// The loop condition (i >= 0) prevents out-of-bounds access even if hosts is empty
	for i := len(hosts) - 1; i >= 0; i-- {
		host := hosts[i] // #nosec G602 -- bounds checked by loop condition

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
			var matchParts []map[string]any
			matchParts = append(matchParts, map[string]any{"remote_ip": map[string]any{"ranges": decisionIPs}})
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
					matchParts = append(matchParts, map[string]any{"not": []map[string]any{{"remote_ip": map[string]any{"ranges": trims}}}})
				}
			}
			decHandler := Handler{
				"handler": "subroute",
				"routes": []map[string]any{
					{
						"match": matchParts,
						"handle": []map[string]any{
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

		// Add Security Headers handler
		if secHeadersHandler, err := buildSecurityHeadersHandler(&host); err == nil && secHeadersHandler != nil {
			handlers = append(handlers, secHeadersHandler)
		}

		// Add HSTS header if enabled (legacy - deprecated in favor of SecurityHeaderProfile)
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
			// Determine if standard headers should be enabled (default true if nil)
			enableStdHeaders := host.EnableStandardHeaders == nil || *host.EnableStandardHeaders
			locHandlers = append(locHandlers, ReverseProxyHandler(dial, host.WebsocketSupport, host.Application, enableStdHeaders))
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
			var parsed any
			if err := json.Unmarshal([]byte(host.AdvancedConfig), &parsed); err != nil {
				logger.Log().WithField("host", host.UUID).WithError(err).Warn("Failed to parse advanced_config for host")
			} else {
				switch v := parsed.(type) {
				case map[string]any:
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
				case []any:
					for _, it := range v {
						if m, ok := it.(map[string]any); ok {
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
		// Determine if standard headers should be enabled (default true if nil)
		enableStdHeaders := host.EnableStandardHeaders == nil || *host.EnableStandardHeaders
		emergencyPaths := []string{
			"/api/v1/emergency/security-reset",
			"/api/v1/emergency/*",
			"/emergency/security-reset",
			"/emergency/*",
		}
		emergencyHandlers := append(append([]Handler{}, handlers...), ReverseProxyHandler(dial, host.WebsocketSupport, host.Application, enableStdHeaders))
		emergencyRoute := &Route{
			Match: []Match{
				{
					Host: uniqueDomains,
					Path: emergencyPaths,
				},
			},
			Handle:   emergencyHandlers,
			Terminal: true,
		}
		logger.Log().WithFields(map[string]any{
			"host_id":        host.ID,
			"host_uuid":      host.UUID,
			"unique_domains": uniqueDomains,
			"has_paths":      true,
			"path_count":     len(emergencyPaths),
		}).Debug("[CONFIG DEBUG] Creating EMERGENCY route")
		routes = append(routes, emergencyRoute)

		mainHandlers := append(append([]Handler{}, securityHandlers...), handlers...)
		mainHandlers = append(mainHandlers, ReverseProxyHandler(dial, host.WebsocketSupport, host.Application, enableStdHeaders))

		route := &Route{
			Match: []Match{
				{Host: uniqueDomains},
			},
			Handle:   mainHandlers,
			Terminal: true,
		}

		logger.Log().WithFields(map[string]any{
			"host_id":        host.ID,
			"host_uuid":      host.UUID,
			"unique_domains": uniqueDomains,
			"has_paths":      false,
			"route_type":     "main",
		}).Debug("[CONFIG DEBUG] Creating MAIN route (no path matchers)")

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
			IssuersRaw: []any{map[string]any{"module": "internal"}},
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
func normalizeHandlerHeaders(h map[string]any) {
	// normalize top-level headers key
	if headersRaw, ok := h["headers"].(map[string]any); ok {
		normalizeHeaderOps(headersRaw)
	}
	// also normalize in nested request/response if present explicitly
	for _, side := range []string{"request", "response"} {
		if sideRaw, ok := h[side].(map[string]any); ok {
			normalizeHeaderOps(sideRaw)
		}
	}
}

func normalizeHeaderOps(headerOps map[string]any) {
	if setRaw, ok := headerOps["set"].(map[string]any); ok {
		for k, v := range setRaw {
			switch vv := v.(type) {
			case string:
				setRaw[k] = []string{vv}
			case []any:
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
func NormalizeAdvancedConfig(parsed any) any {
	switch v := parsed.(type) {
	case map[string]any:
		// This might be a handler object
		normalizeHandlerHeaders(v)
		// Also inspect nested 'handle' or 'routes' arrays for nested handlers
		if handles, ok := v["handle"].([]any); ok {
			for _, it := range handles {
				if m, ok := it.(map[string]any); ok {
					NormalizeAdvancedConfig(m)
				}
			}
		}
		if routes, ok := v["routes"].([]any); ok {
			for _, rit := range routes {
				if rm, ok := rit.(map[string]any); ok {
					if handles, ok := rm["handle"].([]any); ok {
						for _, it := range handles {
							if m, ok := it.(map[string]any); ok {
								NormalizeAdvancedConfig(m)
							}
						}
					}
				}
			}
		}
		return v
	case []any:
		for _, it := range v {
			if m, ok := it.(map[string]any); ok {
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
				"routes": []map[string]any{
					{
						"match": []map[string]any{
							{
								"not": []map[string]any{
									{
										"expression": expression,
									},
								},
							},
						},
						"handle": []map[string]any{
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
			"routes": []map[string]any{
				{
					"match": []map[string]any{
						{
							"expression": expression,
						},
					},
					"handle": []map[string]any{
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
			"routes": []map[string]any{
				{
					"match": []map[string]any{
						{
							"not": []map[string]any{
								{
									"remote_ip": map[string]any{
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
					"handle": []map[string]any{
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
			"routes": []map[string]any{
				{
					"match": []map[string]any{
						{
							"not": []map[string]any{
								{
									"remote_ip": map[string]any{
										"ranges": cidrs,
									},
								},
							},
						},
					},
					"handle": []map[string]any{
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
		var adminExclusion any
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
				adminExclusion = map[string]any{"not": []map[string]any{{"remote_ip": map[string]any{"ranges": trims}}}}
			}
		}
		// Build matcher parts
		matchParts := []map[string]any{}
		matchParts = append(matchParts, map[string]any{"remote_ip": map[string]any{"ranges": cidrs}})
		if adminExclusion != nil {
			matchParts = append(matchParts, adminExclusion.(map[string]any))
		}
		return Handler{
			"handler": "subroute",
			"routes": []map[string]any{
				{
					"match": matchParts,
					"handle": []map[string]any{
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

// getCrowdSecAPIKey retrieves the CrowdSec bouncer API key.
// Priority order (per Bug 1 fix in lapi_translation_bugs.md):
//  1. Persistent key file (/app/data/crowdsec/bouncer_key) - auto-generated valid keys
//  2. Environment variables - user-configured keys (may be invalid)
//
// This order ensures that after auto-registration, the validated key is used
// even if an invalid env var key is still set in docker-compose.yml.
func getCrowdSecAPIKey() string {
	const bouncerKeyFile = "/app/data/crowdsec/bouncer_key"

	// Priority 1: Check persistent key file first
	// This takes precedence because it contains a validated, auto-generated key
	if data, err := os.ReadFile(bouncerKeyFile); err == nil {
		key := strings.TrimSpace(string(data))
		if key != "" {
			logger.Log().WithField("source", "file").WithField("file", bouncerKeyFile).Debug("CrowdSec API key loaded from file")
			return key
		}
	}

	// Priority 2: Fall back to environment variables
	envVars := []string{
		"CHARON_SECURITY_CROWDSEC_API_KEY",
		"CROWDSEC_API_KEY",
		"CROWDSEC_BOUNCER_API_KEY",
		"CERBERUS_SECURITY_CROWDSEC_API_KEY",
		"CPM_SECURITY_CROWDSEC_API_KEY",
	}

	for _, envVar := range envVars {
		if val := os.Getenv(envVar); val != "" {
			logger.Log().WithField("source", "env_var").WithField("env_var", envVar).Debug("CrowdSec API key loaded from environment variable")
			return val
		}
	}

	logger.Log().Debug("No CrowdSec API key found in file or environment variables")
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
		var ac map[string]any
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
		switch {
		case hostRulesetMatch != nil:
			selected = hostRulesetMatch
		case appMatch != nil:
			selected = appMatch
		case owaspFallback != nil:
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
	rateLimitHandler["rate_limits"] = map[string]any{
		"static": map[string]any{
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
		"routes": []map[string]any{
			{
				// Route 1: Match bypass IPs - terminal with no handlers (skip rate limiting)
				"match": []map[string]any{
					{
						"remote_ip": map[string]any{
							"ranges": bypassCIDRs,
						},
					},
				},
				// No handlers - just pass through without rate limiting
				"handle": []map[string]any{},
			},
			{
				// Route 2: Default - apply rate limiting to everyone else
				"handle": []map[string]any{
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

// buildSecurityHeadersHandler creates a headers handler for security headers
// based on the profile configuration or host-level settings
func buildSecurityHeadersHandler(host *models.ProxyHost) (Handler, error) {
	if host == nil {
		return nil, nil
	}

	// Use profile if configured
	var cfg *models.SecurityHeaderProfile
	switch {
	case host.SecurityHeaderProfile != nil:
		cfg = host.SecurityHeaderProfile
	case !host.SecurityHeadersEnabled:
		// No profile and headers disabled - skip
		return nil, nil
	default:
		// Use default secure headers
		cfg = getDefaultSecurityHeaderProfile()
	}

	responseHeaders := make(map[string][]string)

	// HSTS
	if cfg.HSTSEnabled {
		hstsValue := fmt.Sprintf("max-age=%d", cfg.HSTSMaxAge)
		if cfg.HSTSIncludeSubdomains {
			hstsValue += "; includeSubDomains"
		}
		if cfg.HSTSPreload {
			hstsValue += "; preload"
		}
		responseHeaders["Strict-Transport-Security"] = []string{hstsValue}
	}

	// CSP
	if cfg.CSPEnabled && cfg.CSPDirectives != "" {
		cspHeader := "Content-Security-Policy"
		if cfg.CSPReportOnly {
			cspHeader = "Content-Security-Policy-Report-Only"
		}
		cspString, err := buildCSPString(cfg.CSPDirectives)
		if err == nil && cspString != "" {
			responseHeaders[cspHeader] = []string{cspString}
		}
	}

	// X-Frame-Options
	if cfg.XFrameOptions != "" {
		responseHeaders["X-Frame-Options"] = []string{cfg.XFrameOptions}
	}

	// X-Content-Type-Options
	if cfg.XContentTypeOptions {
		responseHeaders["X-Content-Type-Options"] = []string{"nosniff"}
	}

	// Referrer-Policy
	if cfg.ReferrerPolicy != "" {
		responseHeaders["Referrer-Policy"] = []string{cfg.ReferrerPolicy}
	}

	// Permissions-Policy
	if cfg.PermissionsPolicy != "" {
		ppString, err := buildPermissionsPolicyString(cfg.PermissionsPolicy)
		if err == nil && ppString != "" {
			responseHeaders["Permissions-Policy"] = []string{ppString}
		}
	}

	// Cross-Origin headers
	if cfg.CrossOriginOpenerPolicy != "" {
		responseHeaders["Cross-Origin-Opener-Policy"] = []string{cfg.CrossOriginOpenerPolicy}
	}
	if cfg.CrossOriginResourcePolicy != "" {
		responseHeaders["Cross-Origin-Resource-Policy"] = []string{cfg.CrossOriginResourcePolicy}
	}
	if cfg.CrossOriginEmbedderPolicy != "" {
		responseHeaders["Cross-Origin-Embedder-Policy"] = []string{cfg.CrossOriginEmbedderPolicy}
	}

	// X-XSS-Protection
	if cfg.XSSProtection {
		responseHeaders["X-XSS-Protection"] = []string{"1; mode=block"}
	}

	// Cache-Control
	if cfg.CacheControlNoStore {
		responseHeaders["Cache-Control"] = []string{"no-store"}
	}

	if len(responseHeaders) == 0 {
		return nil, nil
	}

	return Handler{
		"handler": "headers",
		"response": map[string]any{
			"set": responseHeaders,
		},
	}, nil
}

// buildCSPString converts JSON CSP directives to a CSP string
func buildCSPString(directivesJSON string) (string, error) {
	if directivesJSON == "" {
		return "", nil
	}

	var directivesMap map[string][]string
	if err := json.Unmarshal([]byte(directivesJSON), &directivesMap); err != nil {
		return "", fmt.Errorf("invalid CSP JSON: %w", err)
	}

	var parts []string
	for directive, values := range directivesMap {
		if len(values) > 0 {
			part := fmt.Sprintf("%s %s", directive, strings.Join(values, " "))
			parts = append(parts, part)
		}
	}

	return strings.Join(parts, "; "), nil
}

// buildPermissionsPolicyString converts JSON permissions to policy string
func buildPermissionsPolicyString(permissionsJSON string) (string, error) {
	if permissionsJSON == "" {
		return "", nil
	}

	var permissions []models.PermissionsPolicyItem
	if err := json.Unmarshal([]byte(permissionsJSON), &permissions); err != nil {
		return "", fmt.Errorf("invalid permissions JSON: %w", err)
	}

	var parts []string
	for _, perm := range permissions {
		var allowlist string
		if len(perm.Allowlist) == 0 {
			allowlist = "()"
		} else {
			// Convert allowlist items to policy format
			items := make([]string, len(perm.Allowlist))
			for i, item := range perm.Allowlist {
				switch item {
				case "self":
					items[i] = "self"
				case "*":
					items[i] = "*"
				default:
					items[i] = fmt.Sprintf("\"%s\"", item)
				}
			}
			allowlist = fmt.Sprintf("(%s)", strings.Join(items, " "))
		}
		parts = append(parts, fmt.Sprintf("%s=%s", perm.Feature, allowlist))
	}

	return strings.Join(parts, ", "), nil
}

// getDefaultSecurityHeaderProfile returns secure defaults
func getDefaultSecurityHeaderProfile() *models.SecurityHeaderProfile {
	return &models.SecurityHeaderProfile{
		HSTSEnabled:               true,
		HSTSMaxAge:                31536000,
		HSTSIncludeSubdomains:     false,
		HSTSPreload:               false,
		CSPEnabled:                false, // Off by default to avoid breaking sites
		XFrameOptions:             "SAMEORIGIN",
		XContentTypeOptions:       true,
		ReferrerPolicy:            "strict-origin-when-cross-origin",
		XSSProtection:             true,
		CrossOriginOpenerPolicy:   "same-origin",
		CrossOriginResourcePolicy: "same-origin",
	}
}

// hasWildcard checks if any domain in the list is a wildcard domain
func hasWildcard(domains []string) bool {
	for _, domain := range domains {
		if strings.HasPrefix(domain, "*.") {
			return true
		}
	}
	return false
}

// dedupeDomains removes duplicate domains from a list while preserving order
func dedupeDomains(domains []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(domains))
	for _, domain := range domains {
		if !seen[domain] {
			seen[domain] = true
			result = append(result, domain)
		}
	}
	return result
}
