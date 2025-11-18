package caddy

import (
	"fmt"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
)

// GenerateConfig creates a Caddy JSON configuration from proxy hosts.
// This is the core transformation layer from our database model to Caddy config.
func GenerateConfig(hosts []models.ProxyHost) (*Config, error) {
	if len(hosts) == 0 {
		return &Config{
			Apps: Apps{
				HTTP: &HTTPApp{
					Servers: map[string]*Server{},
				},
			},
		}, nil
	}

	routes := make([]*Route, 0, len(hosts))

	for _, host := range hosts {
		if host.Domain == "" {
			return nil, fmt.Errorf("proxy host %s has empty domain", host.UUID)
		}

		dial := fmt.Sprintf("%s:%d", host.TargetHost, host.TargetPort)

		route := &Route{
			Match: []Match{
				{Host: []string{host.Domain}},
			},
			Handle: []Handler{
				ReverseProxyHandler(dial, host.EnableWS),
			},
			Terminal: true,
		}

		routes = append(routes, route)
	}

	config := &Config{
		Apps: Apps{
			HTTP: &HTTPApp{
				Servers: map[string]*Server{
					"cpm_server": {
						Listen: []string{":80", ":443"},
						Routes: routes,
						AutoHTTPS: &AutoHTTPSConfig{
							// Enable automatic HTTPS by default
							Disable: false,
						},
					},
				},
			},
		},
	}

	return config, nil
}
