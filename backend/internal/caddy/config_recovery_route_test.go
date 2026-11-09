package caddy

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// recoveryProbePaths are paths near the reset endpoint that must never be
// served by a route that skips the security pre-handlers, for any host.
var recoveryProbePaths = []string{
	"/emergency/..;/admin",
	"/emergency/x",
	"/api/v1/emergency/token/x",
	"/api/v1/emergency/security-reset/extra",
	"/emergency/security-reset/",
	"/api/v1/emergency/security-reset;x",
}

// matchesRoutePattern follows the path matcher rules: an exact match, or a
// prefix match when the pattern ends in "*".
func matchesRoutePattern(pattern, path string) bool {
	if prefix, ok := strings.CutSuffix(pattern, "*"); ok {
		return strings.HasPrefix(path, prefix)
	}
	return pattern == path
}

func routeMatchesPath(patterns []string, path string) bool {
	return slices.ContainsFunc(patterns, func(p string) bool {
		return matchesRoutePattern(p, path)
	})
}

// checkFreePathRoutes returns the path-scoped routes for domain that run no
// security pre-handler. Location routes and the main route both carry the
// pre-handler, so only a recovery route can appear here.
func checkFreePathRoutes(routes []*Route, domain string) []*Route {
	var out []*Route
	for _, r := range routes {
		if len(r.Match) == 0 || !slices.Equal(r.Match[0].Host, []string{domain}) || len(r.Match[0].Path) == 0 {
			continue
		}
		if handlerIndex(r.Handle, "subroute") >= 0 {
			continue
		}
		out = append(out, r)
	}
	return out
}

// recoveryHostCase describes one host kind for the recovery route checks.
type recoveryHostCase struct {
	name         string
	domain       string
	host         models.ProxyHost
	opts         []GenerateConfigOption
	selfUpstream bool
}

func recoveryHostCases(t *testing.T) []recoveryHostCase {
	t.Helper()
	secret := testHopSecret(t)
	return []recoveryHostCase{
		{
			name:   "plain upstream",
			domain: "plain.example.com",
			host: models.ProxyHost{
				UUID: "plain", DomainNames: "plain.example.com", ForwardHost: "10.0.0.5", ForwardPort: 3000, Enabled: true,
			},
		},
		{
			name:   "plain upstream with locations",
			domain: "locs.example.com",
			host: models.ProxyHost{
				UUID: "locs", DomainNames: "locs.example.com", ForwardHost: "10.0.0.6", ForwardPort: 3001, Enabled: true,
				Locations: []models.Location{{Path: "/api", ForwardHost: "10.0.0.7", ForwardPort: 9000}},
			},
		},
		{
			name:   "loopback port that is not the API port",
			domain: "other.example.com",
			host: models.ProxyHost{
				UUID: "other", DomainNames: "other.example.com", ForwardHost: "127.0.0.1", ForwardPort: 9090, Enabled: true,
			},
			opts: []GenerateConfigOption{WithSelfHop(secret, "8080")},
		},
		{
			name:   "self port on a remote host",
			domain: "remote.example.com",
			host: models.ProxyHost{
				UUID: "remote", DomainNames: "remote.example.com", ForwardHost: "127.0.0.1", ForwardPort: 8080, Enabled: true,
			},
			opts: []GenerateConfigOption{
				WithSelfHop(secret, "8080"),
				WithRemoteHosts(map[string]struct{}{"remote": {}}),
			},
		},
		{
			name:   "self upstream",
			domain: "self.example.com",
			host: models.ProxyHost{
				UUID: "self", DomainNames: "self.example.com", ForwardHost: "127.0.0.1", ForwardPort: 8080, Enabled: true,
			},
			opts:         []GenerateConfigOption{WithSelfHop(secret, "8080")},
			selfUpstream: true,
		},
	}
}

// selfUpstreamCase returns the self upstream host kind.
func selfUpstreamCase(t *testing.T) recoveryHostCase {
	t.Helper()
	for _, tt := range recoveryHostCases(t) {
		if tt.selfUpstream {
			return tt
		}
	}
	require.Fail(t, "no self upstream host case")
	return recoveryHostCase{}
}

// TestRecoveryRouteAbsentForNonSelfUpstream checks that a host whose upstream
// is not Charon itself gets no route that skips the security pre-handlers.
func TestRecoveryRouteAbsentForNonSelfUpstream(t *testing.T) {
	for _, tt := range recoveryHostCases(t) {
		if tt.selfUpstream {
			continue
		}
		t.Run(tt.name, func(t *testing.T) {
			routes := pipelineRoutesFor(t, []models.ProxyHost{tt.host}, tt.opts...)
			assert.Empty(t, checkFreePathRoutes(routes, tt.domain))
		})
	}
}

// TestRecoveryRouteMatchesOnlyExactResetPaths checks that a self upstream host
// gets exactly one check-free route, that it lists the two exact reset paths
// with no wildcard, and that it is terminal and proxies last.
func TestRecoveryRouteMatchesOnlyExactResetPaths(t *testing.T) {
	selfCase := selfUpstreamCase(t)
	routes := pipelineRoutesFor(t, []models.ProxyHost{selfCase.host}, selfCase.opts...)
	recovery := checkFreePathRoutes(routes, selfCase.domain)
	require.Len(t, recovery, 1, "a self upstream host gets exactly one recovery route")

	route := recovery[0]
	assert.Equal(t, []string{"/api/v1/emergency/security-reset", "/emergency/security-reset"}, route.Match[0].Path)
	assert.True(t, route.Terminal)
	require.NotEmpty(t, route.Handle)
	assert.Equal(t, "reverse_proxy", route.Handle[len(route.Handle)-1]["handler"], "the recovery route proxies last")

	assert.True(t, routeMatchesPath(route.Match[0].Path, "/emergency/security-reset"))
	assert.True(t, routeMatchesPath(route.Match[0].Path, "/api/v1/emergency/security-reset"))
	for _, p := range route.Match[0].Path {
		assert.False(t, strings.HasSuffix(p, "*"), "recovery path %q must not use a wildcard", p)
	}
}

// TestRecoveryRouteProbePathsNeverMatch checks that paths near the reset
// endpoint, token endpoints included, match no check-free route for any host
// kind. Token endpoints run through the main route with the pre-handlers.
func TestRecoveryRouteProbePathsNeverMatch(t *testing.T) {
	for _, tt := range recoveryHostCases(t) {
		t.Run(tt.name, func(t *testing.T) {
			routes := pipelineRoutesFor(t, []models.ProxyHost{tt.host}, tt.opts...)
			for _, route := range checkFreePathRoutes(routes, tt.domain) {
				for _, probe := range recoveryProbePaths {
					assert.False(t, routeMatchesPath(route.Match[0].Path, probe),
						"probe path %q must not match a check-free route", probe)
				}
			}
		})
	}
}

// TestRecoveryRouteTokenPathUsesMainRoute checks that a token endpoint on a self
// upstream host is served by the main route, which runs the pre-handlers first.
func TestRecoveryRouteTokenPathUsesMainRoute(t *testing.T) {
	selfCase := selfUpstreamCase(t)
	routes := pipelineRoutesFor(t, []models.ProxyHost{selfCase.host}, selfCase.opts...)

	var mainRoute *Route
	for _, r := range routes {
		if len(r.Match) > 0 && slices.Equal(r.Match[0].Host, []string{selfCase.domain}) && len(r.Match[0].Path) == 0 {
			mainRoute = r
		}
	}
	require.NotNil(t, mainRoute, "self upstream host keeps a main route")
	assertPipelineOrder(t, mainRoute.Handle)
}
