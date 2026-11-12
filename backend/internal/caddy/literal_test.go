package caddy

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

func TestProxyLiteral(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "plain host", in: "example.com", want: "example.com"},
		{name: "plain url", in: "https://a.example.com/x?y=1&z=2", want: "https://a.example.com/x?y=1&z=2"},
		{name: "ipv6 dial", in: "[::1]:8080", want: "[::1]:8080"},
		{name: "existing percent escapes untouched", in: "https://a.example.com/%7Bx%7D", want: "https://a.example.com/%7Bx%7D"},
		{name: "literal percent untouched", in: "100%", want: "100%"},
		{name: "unicode untouched", in: "https://exämple.test/ü", want: "https://exämple.test/ü"},
		{name: "open brace", in: "{", want: "%7B"},
		{name: "close brace", in: "}", want: "%7D"},
		{name: "placeholder", in: "https://a.example.com{http.request.uri}", want: "https://a.example.com%7Bhttp.request.uri%7D"},
		{name: "env placeholder", in: "{env.ENCRYPTION_KEY}", want: "%7Benv.ENCRYPTION_KEY%7D"},
		{name: "mixed braces", in: "a}b{c}", want: "a%7Db%7Bc%7D"},
		{name: "nested braces", in: "{{x}}", want: "%7B%7Bx%7D%7D"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, proxyLiteral(tt.in))
		})
	}
}

func TestProxyLiteral_BraceFreeInputUnchanged(t *testing.T) {
	inputs := []string{
		"https://new.example.com/path/to?q=a%20b",
		"127.0.0.1:8080",
		"Strict-Transport-Security",
		"max-age=31536000; includeSubDomains",
	}
	for _, in := range inputs {
		assert.Equal(t, in, proxyLiteral(in))
	}
}

func TestProxyLiterals(t *testing.T) {
	assert.Nil(t, proxyLiterals(nil))
	assert.Equal(t, []string{}, proxyLiterals([]string{}))
	assert.Equal(t, []string{"a.example.com", "%7Bb%7D.example.com"}, proxyLiterals([]string{"a.example.com", "{b}.example.com"}))
}

func TestReverseProxyHandler_DialIsLiteral(t *testing.T) {
	h := ReverseProxyHandler("{env.UPSTREAM}:80", "none", false)
	upstreams, ok := h["upstreams"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, upstreams, 1)
	assert.Equal(t, "%7Benv.UPSTREAM%7D:80", upstreams[0]["dial"])
}

func TestBuildSecurityHeadersHandler_ProfileValuesAreLiteral(t *testing.T) {
	host := &models.ProxyHost{
		SecurityHeaderProfile: &models.SecurityHeaderProfile{
			XFrameOptions: "ALLOW-FROM {http.request.host}",
		},
	}
	handlers := buildSecurityHeaderHandlers(host)
	require.NotEmpty(t, handlers)
	raw, err := json.Marshal(handlers)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "ALLOW-FROM %7Bhttp.request.host%7D")
	assert.NotContains(t, string(raw), "{http.request.host}")
}

func TestGenerateConfig_UserValuesAreWrittenAsLiterals(t *testing.T) {
	hosts := []models.ProxyHost{
		{
			UUID:        "h-literal",
			DomainNames: "{env.DOMAIN}.example.com",
			ForwardHost: "127.0.0.1",
			ForwardPort: 8080,
			Enabled:     true,
			Locations: []models.Location{
				{Path: "/{env.SECTION}", ForwardHost: "{env.LOC_HOST}", ForwardPort: 8081},
			},
		},
	}
	cfg, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, false, false, false, false, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)

	raw, err := json.Marshal(cfg.Apps.HTTP.Servers["charon_server"].Routes)
	require.NoError(t, err)
	out := string(raw)
	assert.Contains(t, out, "%7Benv.domain%7D.example.com")
	assert.Contains(t, out, "/%7Benv.SECTION%7D")
	assert.Contains(t, out, "%7Benv.LOC_HOST%7D:8081")
	for _, placeholder := range []string{"{env.domain}", "{env.SECTION}", "{env.LOC_HOST}"} {
		assert.False(t, strings.Contains(out, placeholder), "raw placeholder %s must not reach the config", placeholder)
	}
}
