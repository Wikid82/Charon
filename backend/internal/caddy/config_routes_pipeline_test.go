package caddy

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// handlerIndex returns the position of the first handler named name, or -1.
func handlerIndex(handle []Handler, name string) int {
	for i, h := range handle {
		if h["handler"] == name {
			return i
		}
	}
	return -1
}

// pipelineRoutesFor generates routes with one block decision in place so every
// host carries a security pre-handler, which lets the ordering check see it.
func pipelineRoutesFor(t *testing.T, hosts []models.ProxyHost, opts ...GenerateConfigOption) []*Route {
	t.Helper()
	decisions := []models.SecurityDecision{{Action: "block", IP: "203.0.113.7"}}
	cfg, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, false, false, false, true, "", nil, nil, decisions, nil, nil, opts...)
	require.NoError(t, err)
	require.NotNil(t, cfg.Apps.HTTP)
	server := cfg.Apps.HTTP.Servers["charon_server"]
	require.NotNil(t, server)
	return server.Routes
}

// TestGeneratedRoutesKeepPipelineOrder checks, for each host kind, that the
// routes serving the host match the host and that the security pre-handlers
// run before the reverse proxy on the main route and on every location route.
func TestGeneratedRoutesKeepPipelineOrder(t *testing.T) {
	secret := testHopSecret(t)
	tests := []struct {
		name          string
		domain        string
		host          models.ProxyHost
		opts          []GenerateConfigOption
		locationPaths [][]string
	}{
		{
			name:   "plain upstream",
			domain: "plain.example.com",
			host: models.ProxyHost{
				UUID: "plain", DomainNames: "plain.example.com", ForwardHost: "10.0.0.5", ForwardPort: 3000, Enabled: true,
			},
		},
		{
			name:   "self upstream",
			domain: "self.example.com",
			host: models.ProxyHost{
				UUID: "self", DomainNames: "self.example.com", ForwardHost: "127.0.0.1", ForwardPort: 8080, Enabled: true,
			},
			opts: []GenerateConfigOption{WithSelfHop(secret, "8080")},
		},
		{
			name:   "host with locations",
			domain: "locs.example.com",
			host: models.ProxyHost{
				UUID: "locs", DomainNames: "locs.example.com", ForwardHost: "10.0.0.6", ForwardPort: 3001, Enabled: true,
				Locations: []models.Location{{Path: "/api", ForwardHost: "10.0.0.7", ForwardPort: 9000}},
			},
			locationPaths: [][]string{{"/api", "/api/*"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			routes := pipelineRoutesFor(t, []models.ProxyHost{tt.host}, tt.opts...)

			// Routes with no path matcher are the main route. Location routes are
			// matched by their configured paths; any other path-scoped route for
			// the host is a recovery route and is outside this check.
			var mainRoutes, locationRoutes []*Route
			for _, r := range routes {
				if len(r.Match) == 0 || len(r.Match[0].Host) != 1 || r.Match[0].Host[0] != tt.domain {
					continue
				}
				if len(r.Match[0].Path) == 0 {
					mainRoutes = append(mainRoutes, r)
				} else if isLocationPath(tt.locationPaths, r.Match[0].Path) {
					locationRoutes = append(locationRoutes, r)
				}
			}

			require.Len(t, mainRoutes, 1, "exactly one main route for the host")
			main := mainRoutes[0]
			assert.Equal(t, []string{tt.domain}, main.Match[0].Host)
			assertPipelineOrder(t, main.Handle)

			require.Len(t, locationRoutes, len(tt.locationPaths))
			for _, r := range locationRoutes {
				assert.Equal(t, []string{tt.domain}, r.Match[0].Host)
				assertPipelineOrder(t, r.Handle)
			}
		})
	}
}

// isLocationPath reports whether path is the path matcher of one of the
// configured locations.
func isLocationPath(locationPaths [][]string, path []string) bool {
	for _, want := range locationPaths {
		if slices.Equal(want, path) {
			return true
		}
	}
	return false
}

// assertPipelineOrder requires a security pre-handler and exactly one reverse
// proxy, with the pre-handler first and the proxy last.
func assertPipelineOrder(t *testing.T, handle []Handler) {
	t.Helper()
	proxyAt := handlerIndex(handle, "reverse_proxy")
	require.GreaterOrEqual(t, proxyAt, 0, "route has a reverse_proxy handler")
	assert.Equal(t, len(handle)-1, proxyAt, "reverse_proxy is the final handler")

	securityAt := handlerIndex(handle, "subroute")
	require.GreaterOrEqual(t, securityAt, 0, "route has a security pre-handler")
	assert.Less(t, securityAt, proxyAt, "security pre-handler runs before the proxy")
}
