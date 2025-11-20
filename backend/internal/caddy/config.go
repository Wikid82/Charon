package caddy

import (
	"fmt"
	"strings"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
)

// GenerateConfig creates a Caddy JSON configuration from proxy hosts.
// This is the core transformation layer from our database model to Caddy config.
func GenerateConfig(hosts []models.ProxyHost, storageDir string, acmeEmail string) (*Config, error) {
	config := &Config{
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
		config.Apps.TLS = &TLSApp{
			Automation: &AutomationConfig{
				Policies: []*AutomationPolicy{
					{
						IssuersRaw: []interface{}{
							map[string]interface{}{
								"module": "acme",
								"email":  acmeEmail,
							},
							map[string]interface{}{
								"module": "zerossl",
								"email":  acmeEmail,
							},
						},
					},
				},
			},
		}
	}

	if len(hosts) == 0 {
		return config, nil
	}

	routes := make([]*Route, 0)

	for _, host := range hosts {
		if !host.Enabled {
			continue
		}

		if host.DomainNames == "" {
			return nil, fmt.Errorf("proxy host %s has empty domain names", host.UUID)
		}

		// Parse comma-separated domains
		domains := strings.Split(host.DomainNames, ",")
		for i := range domains {
			domains[i] = strings.TrimSpace(domains[i])
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
						Host: domains,
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
				{Host: domains},
			},
			Handle:   mainHandlers,
			Terminal: true,
		}

		routes = append(routes, route)
	}

	config.Apps.HTTP.Servers["cpm_server"] = &Server{
		Listen: []string{":80", ":443"},
		Routes: routes,
		AutoHTTPS: &AutoHTTPSConfig{
			Disable:      false,
			DisableRedir: false,
		},
	}

	return config, nil
}
