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

// pipelineHeaderProfile is the profile used by the ordering cases that carry a
// security header pair.
func pipelineHeaderProfile() *models.SecurityHeaderProfile {
	return &models.SecurityHeaderProfile{XFrameOptions: "DENY", ReferrerPolicy: "no-referrer"}
}

// TestGeneratedRoutesKeepPipelineOrder checks, for each host kind, that the
// routes serving the host match the host, that the security header pair (when
// the host has a profile) leads the route, and that the security pre-handlers
// run before the reverse proxy on the main route and on every location route.
func TestGeneratedRoutesKeepPipelineOrder(t *testing.T) {
	secret := testHopSecret(t)
	tests := []struct {
		name          string
		domain        string
		host          models.ProxyHost
		opts          []GenerateConfigOption
		locationPaths [][]string
		wantHeaders   bool
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
		{
			name:   "host with security header profile and locations",
			domain: "hdrs.example.com",
			host: models.ProxyHost{
				UUID: "hdrs", DomainNames: "hdrs.example.com", ForwardHost: "10.0.0.8", ForwardPort: 3002, Enabled: true,
				SecurityHeaderProfile: pipelineHeaderProfile(),
				Locations:             []models.Location{{Path: "/api", ForwardHost: "10.0.0.9", ForwardPort: 9001}},
			},
			locationPaths: [][]string{{"/api", "/api/*"}},
			wantHeaders:   true,
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
			assertPipelineOrder(t, main.Handle, tt.wantHeaders)

			require.Len(t, locationRoutes, len(tt.locationPaths))
			for _, r := range locationRoutes {
				assert.Equal(t, []string{tt.domain}, r.Match[0].Host)
				assertPipelineOrder(t, r.Handle, tt.wantHeaders)
			}
		})
	}
}

// TestRecoveryRouteCarriesHeaderPairFirst checks that the recovery route, which
// skips the security pre-handlers, still leads with the security header pair
// and proxies last.
func TestRecoveryRouteCarriesHeaderPairFirst(t *testing.T) {
	secret := testHopSecret(t)
	host := models.ProxyHost{
		UUID: "recovery", DomainNames: "recovery.example.com", ForwardHost: "127.0.0.1", ForwardPort: 8080, Enabled: true,
		SecurityHeaderProfile: pipelineHeaderProfile(),
	}
	routes := pipelineRoutesFor(t, []models.ProxyHost{host}, WithSelfHop(secret, "8080"))

	var recovery *Route
	for _, r := range routes {
		if len(r.Match) > 0 && slices.Equal(r.Match[0].Path, resetRecoveryPaths) {
			recovery = r
		}
	}
	require.NotNil(t, recovery, "self upstream host has a recovery route")
	require.GreaterOrEqual(t, len(recovery.Handle), 3)

	set := assertHeaderPair(t, recovery.Handle[:2])
	assert.Equal(t, []string{"DENY"}, set["X-Frame-Options"])
	assert.Equal(t, "reverse_proxy", recovery.Handle[len(recovery.Handle)-1]["handler"])
	assert.Equal(t, -1, handlerIndex(recovery.Handle, "subroute"), "recovery route has no security pre-handler")
}

// TestHostWithoutSecurityHeaderProfileKeepsRouteShape checks that a host with
// no security header profile gets no headers handler: its main route is the
// security pre-handler followed by the reverse proxy, as before the header pair
// was moved to the front of the route.
func TestHostWithoutSecurityHeaderProfileKeepsRouteShape(t *testing.T) {
	host := models.ProxyHost{
		UUID: "bare", DomainNames: "bare.example.com", ForwardHost: "10.0.0.20", ForwardPort: 3003, Enabled: true,
	}
	routes := pipelineRoutesFor(t, []models.ProxyHost{host})

	var main *Route
	for _, r := range routes {
		if len(r.Match) > 0 && len(r.Match[0].Path) == 0 && len(r.Match[0].Host) == 1 && r.Match[0].Host[0] == "bare.example.com" {
			main = r
		}
	}
	require.NotNil(t, main)

	names := make([]any, 0, len(main.Handle))
	for _, h := range main.Handle {
		names = append(names, h["handler"])
	}
	assert.Equal(t, []any{"subroute", "reverse_proxy"}, names)
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
// proxy, with the pre-handler first and the proxy last. When wantHeaders is
// set, the route must open with the security header pair, which runs before
// the security pre-handlers so refusals carry the headers; the deferred half
// then replaces any upstream copy on proxied responses. Without a profile the
// route must carry no headers handler at all.
func assertPipelineOrder(t *testing.T, handle []Handler, wantHeaders bool) {
	t.Helper()
	proxyAt := handlerIndex(handle, "reverse_proxy")
	require.GreaterOrEqual(t, proxyAt, 0, "route has a reverse_proxy handler")
	assert.Equal(t, len(handle)-1, proxyAt, "reverse_proxy is the final handler")

	securityAt := handlerIndex(handle, "subroute")
	require.GreaterOrEqual(t, securityAt, 0, "route has a security pre-handler")
	assert.Less(t, securityAt, proxyAt, "security pre-handler runs before the proxy")

	if !wantHeaders {
		assert.Equal(t, -1, handlerIndex(handle, "headers"), "host without a profile has no headers handler")
		assert.Equal(t, 0, securityAt, "security pre-handler is the first handler")
		return
	}
	require.GreaterOrEqual(t, len(handle), 2)
	set := assertHeaderPair(t, handle[:2])
	assert.Equal(t, []string{"DENY"}, set["X-Frame-Options"])
	assert.Less(t, 1, securityAt, "header pair runs before the security pre-handler so refusals carry it")
}
