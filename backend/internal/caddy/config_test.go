package caddy

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
)

func TestGenerateConfig_Empty(t *testing.T) {
	config, err := GenerateConfig([]models.ProxyHost{}, "/tmp/caddy-data", "admin@example.com")
	require.NoError(t, err)
	require.NotNil(t, config)
	require.NotNil(t, config.Apps.HTTP)
	require.Empty(t, config.Apps.HTTP.Servers)
}

func TestGenerateConfig_SingleHost(t *testing.T) {
	hosts := []models.ProxyHost{
		{
			UUID:             "test-uuid",
			Name:             "Media",
			DomainNames:      "media.example.com",
			ForwardScheme:    "http",
			ForwardHost:      "media",
			ForwardPort:      32400,
			SSLForced:        true,
			WebsocketSupport: false,
			Enabled:          true,
		},
	}

	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com")
	require.NoError(t, err)
	require.NotNil(t, config)
	require.NotNil(t, config.Apps.HTTP)
	require.Len(t, config.Apps.HTTP.Servers, 1)

	server := config.Apps.HTTP.Servers["cpm_server"]
	require.NotNil(t, server)
	require.Contains(t, server.Listen, ":80")
	require.Contains(t, server.Listen, ":443")
	require.Len(t, server.Routes, 1)

	route := server.Routes[0]
	require.Len(t, route.Match, 1)
	require.Equal(t, []string{"media.example.com"}, route.Match[0].Host)
	require.Len(t, route.Handle, 1)
	require.True(t, route.Terminal)

	handler := route.Handle[0]
	require.Equal(t, "reverse_proxy", handler["handler"])
}

func TestGenerateConfig_MultipleHosts(t *testing.T) {
	hosts := []models.ProxyHost{
		{
			UUID:        "uuid-1",
			DomainNames: "site1.example.com",
			ForwardHost: "app1",
			ForwardPort: 8080,
			Enabled:     true,
		},
		{
			UUID:        "uuid-2",
			DomainNames: "site2.example.com",
			ForwardHost: "app2",
			ForwardPort: 8081,
			Enabled:     true,
		},
	}

	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com")
	require.NoError(t, err)
	require.Len(t, config.Apps.HTTP.Servers["cpm_server"].Routes, 2)
}

func TestGenerateConfig_WebSocketEnabled(t *testing.T) {
	hosts := []models.ProxyHost{
		{
			UUID:             "uuid-ws",
			DomainNames:      "ws.example.com",
			ForwardHost:      "wsapp",
			ForwardPort:      3000,
			WebsocketSupport: true,
			Enabled:          true,
		},
	}

	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com")
	require.NoError(t, err)

	route := config.Apps.HTTP.Servers["cpm_server"].Routes[0]
	handler := route.Handle[0]

	// Check WebSocket headers are present
	require.NotNil(t, handler["headers"])
}

func TestGenerateConfig_EmptyDomain(t *testing.T) {
	hosts := []models.ProxyHost{
		{
			UUID:        "bad-uuid",
			DomainNames: "",
			ForwardHost: "app",
			ForwardPort: 8080,
			Enabled:     true,
		},
	}

	_, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com")
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty domain")
}
