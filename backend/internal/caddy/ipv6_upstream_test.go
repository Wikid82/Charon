package caddy

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Wikid82/charon/backend/internal/models"
)

func TestForwardDial_BracketsIPv6Literals(t *testing.T) {
	t.Parallel()
	cases := []struct {
		host string
		port int
		want string
	}{
		{"10.0.0.5", 8080, "10.0.0.5:8080"},
		{"app.internal", 80, "app.internal:80"},
		{"fd12::5", 8080, "[fd12::5]:8080"},
		{"[fd12::5]", 8080, "[fd12::5]:8080"},
		{" 2001:db8::7 ", 443, "[2001:db8::7]:443"},
		{"::1", 9000, "[::1]:9000"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, forwardDial(tc.host, tc.port), tc.host)
	}
}

func TestGenerateConfig_IPv6ForwardTargetsGetValidDials(t *testing.T) {
	hosts := []models.ProxyHost{
		{
			UUID: "ula", DomainNames: "ula.example.com", ForwardScheme: "http",
			ForwardHost: "fd12::5", ForwardPort: 8080, Enabled: true,
			Locations: []models.Location{{Path: "/api", ForwardHost: "2001:db8::7", ForwardPort: 9090}},
		},
		{
			UUID: "bracketed", DomainNames: "bracketed.example.com", ForwardScheme: "http",
			ForwardHost: "[fd00:1::2]", ForwardPort: 80, Enabled: true,
		},
		{
			UUID: "v4", DomainNames: "v4.example.com", ForwardScheme: "http",
			ForwardHost: "10.0.0.5", ForwardPort: 8080, Enabled: true,
		},
	}
	cfg := generateWithOpts(t, hosts)
	dials := map[string]bool{}
	for _, use := range collectProxies(t, cfg) {
		if use.dial != "" {
			dials[use.dial] = true
		}
	}
	assert.True(t, dials["[fd12::5]:8080"], "ULA main target: %v", dials)
	assert.True(t, dials["[2001:db8::7]:9090"], "IPv6 location target: %v", dials)
	assert.True(t, dials["[fd00:1::2]:80"], "bracketed input is not bracketed twice: %v", dials)
	assert.True(t, dials["10.0.0.5:8080"], "IPv4 target is unchanged: %v", dials)
}
