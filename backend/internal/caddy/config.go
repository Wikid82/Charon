package caddy

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
)

// GenerateConfig creates a Caddy JSON configuration from proxy hosts.
// This is the core transformation layer from our database model to Caddy config.
func GenerateConfig(hosts []models.ProxyHost, storageDir string, acmeEmail string, frontendDir string, sslProvider string) (*Config, error) {
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
					Level: "DEBUG",
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
			issuers = append(issuers, map[string]interface{}{
				"module": "acme",
				"email":  acmeEmail,
			})
		case "zerossl":
			issuers = append(issuers, map[string]interface{}{
				"module": "zerossl",
			})
		default: // "both" or empty
			issuers = append(issuers, map[string]interface{}{
				"module": "acme",
				"email":  acmeEmail,
			})
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
				fmt.Printf("Warning: Skipping duplicate domain %s for host %s (Ghost Host detection)\n", d, host.UUID)
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
					ReverseProxyHandler(dial, host.WebsocketSupport),
				},
				Terminal: true,
			}
			routes = append(routes, locRoute)
		}

		// Main proxy handler
		dial := fmt.Sprintf("%s:%d", host.ForwardHost, host.ForwardPort)
		mainHandlers := append(handlers, ReverseProxyHandler(dial, host.WebsocketSupport))

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

	config.Apps.HTTP.Servers["cpm_server"] = &Server{
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
