package caddy

import (
	"encoding/json"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// placeholderPattern matches anything Caddy would treat as a placeholder.
var placeholderPattern = regexp.MustCompile(`\{[^{}]*\}`)

// locationOf returns the Location header value of a redirect route's
// static_response handler.
func locationOf(t *testing.T, route *Route) string {
	t.Helper()
	require.NotEmpty(t, route.Handle)
	headers, ok := route.Handle[len(route.Handle)-1]["headers"].(map[string]any)
	require.True(t, ok, "redirect handler must carry a flat headers map")
	locations, ok := headers["Location"].([]string)
	require.True(t, ok)
	require.Len(t, locations, 1)
	return locations[0]
}

func TestRedirectTargetIsLiteral(t *testing.T) {
	hostile := "https://new.example.com/{env.ENCRYPTION_KEY}/{http.request.header.Cookie}"

	t.Run("build routes without preserve_path", func(t *testing.T) {
		routes, _ := BuildRedirectRoutes([]models.RedirectionHost{{
			UUID: "rh-1", DomainNames: "old.example.com", TargetURL: hostile, StatusCode: 301, Enabled: true,
		}}, map[string]bool{})
		require.Len(t, routes, 1)
		loc := locationOf(t, routes[0])
		assert.Equal(t, "https://new.example.com/%7Benv.ENCRYPTION_KEY%7D/%7Bhttp.request.header.Cookie%7D", loc)
		assert.Empty(t, placeholderPattern.FindAllString(loc, -1))
	})

	t.Run("build routes with preserve_path keeps only the generator placeholder", func(t *testing.T) {
		routes, _ := BuildRedirectRoutes([]models.RedirectionHost{{
			UUID: "rh-2", DomainNames: "old.example.com", TargetURL: hostile, StatusCode: 308, Enabled: true, PreservePath: true,
		}}, map[string]bool{})
		require.Len(t, routes, 1)
		loc := locationOf(t, routes[0])
		assert.Equal(t, "https://new.example.com/%7Benv.ENCRYPTION_KEY%7D/%7Bhttp.request.header.Cookie%7D{http.request.uri}", loc)
		assert.Equal(t, []string{"{http.request.uri}"}, placeholderPattern.FindAllString(loc, -1))
	})

	t.Run("generated config carries encoded braces and no raw user braces", func(t *testing.T) {
		cfg, err := GenerateConfig([]models.ProxyHost{}, "/tmp/caddy-data", "admin@example.com", "", "", false, false, false, false, true, "", nil, nil, nil, nil, nil,
			WithRedirectionHosts([]models.RedirectionHost{{
				UUID: "rh-3", DomainNames: "old.example.com", TargetURL: hostile, StatusCode: 301, Enabled: true,
			}}))
		require.NoError(t, err)
		raw, err := json.Marshal(cfg.Apps.HTTP.Servers["charon_server"].Routes)
		require.NoError(t, err)
		out := string(raw)
		assert.Contains(t, out, "%7Benv.ENCRYPTION_KEY%7D")
		assert.Contains(t, out, "%7Bhttp.request.header.Cookie%7D")
		assert.NotContains(t, out, "{env.ENCRYPTION_KEY}")
		assert.NotContains(t, out, "{http.request.header.Cookie}")
	})

	t.Run("domain matchers are literal", func(t *testing.T) {
		routes, _ := BuildRedirectRoutes([]models.RedirectionHost{{
			UUID: "rh-4", DomainNames: "{env.HOST}.example.com", TargetURL: "https://new.example.com", StatusCode: 301, Enabled: true,
		}}, map[string]bool{})
		require.Len(t, routes, 1)
		assert.Equal(t, []string{"%7Benv.host%7D.example.com"}, routes[0].Match[0].Host)
	})
}
