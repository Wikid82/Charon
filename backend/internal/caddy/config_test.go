package caddy

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
)

func TestGenerateConfig_Empty(t *testing.T) {
	config, err := GenerateConfig([]models.ProxyHost{})
	require.NoError(t, err)
	require.NotNil(t, config)
	require.NotNil(t, config.Apps.HTTP)
	require.Empty(t, config.Apps.HTTP.Servers)
}

func TestGenerateConfig_SingleHost(t *testing.T) {
	hosts := []models.ProxyHost{
		{
			UUID:         "test-uuid",
			Name:         "Media",
			Domain:       "media.example.com",
			TargetScheme: "http",
			TargetHost:   "media",
			TargetPort:   32400,
			EnableTLS:    true,
			EnableWS:     false,
		},
	}

	config, err := GenerateConfig(hosts)
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
			UUID:       "uuid-1",
			Domain:     "site1.example.com",
			TargetHost: "app1",
			TargetPort: 8080,
		},
		{
			UUID:       "uuid-2",
			Domain:     "site2.example.com",
			TargetHost: "app2",
			TargetPort: 8081,
		},
	}

	config, err := GenerateConfig(hosts)
	require.NoError(t, err)
	require.Len(t, config.Apps.HTTP.Servers["cpm_server"].Routes, 2)
}

func TestGenerateConfig_WebSocketEnabled(t *testing.T) {
	hosts := []models.ProxyHost{
		{
			UUID:       "uuid-ws",
			Domain:     "ws.example.com",
			TargetHost: "wsapp",
			TargetPort: 3000,
			EnableWS:   true,
		},
	}

	config, err := GenerateConfig(hosts)
	require.NoError(t, err)

	route := config.Apps.HTTP.Servers["cpm_server"].Routes[0]
	handler := route.Handle[0]
	
	// Check WebSocket headers are present
	require.NotNil(t, handler["headers"])
}

func TestGenerateConfig_EmptyDomain(t *testing.T) {
	hosts := []models.ProxyHost{
		{
			UUID:       "bad-uuid",
			Domain:     "",
			TargetHost: "app",
			TargetPort: 8080,
		},
	}

	_, err := GenerateConfig(hosts)
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty domain")
}
