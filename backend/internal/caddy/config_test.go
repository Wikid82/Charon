package caddy

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
)

func TestGenerateConfig_Empty(t *testing.T) {
	config, err := GenerateConfig([]models.ProxyHost{}, "/tmp/caddy-data", "admin@example.com", "", "", false, nil, nil, nil, nil)
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

	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, nil, nil, nil, nil)
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

	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, nil, nil, nil, nil)
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

	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, nil, nil, nil, nil)
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

	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, nil, nil, nil, nil)
	require.NoError(t, err)
	// Should produce empty routes (or just catch-all if frontendDir was set, but it's empty here)
	require.Empty(t, config.Apps.HTTP.Servers["cpm_server"].Routes)
}

func TestGenerateConfig_Logging(t *testing.T) {
	hosts := []models.ProxyHost{}
	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, nil, nil, nil, nil)
	require.NoError(t, err)

	// Verify logging configuration
	require.NotNil(t, config.Logging)
	require.NotNil(t, config.Logging.Logs)
	require.NotNil(t, config.Logging.Logs["access"])
	require.Equal(t, "INFO", config.Logging.Logs["access"].Level)
	require.Contains(t, config.Logging.Logs["access"].Writer.Filename, "access.log")
	require.Equal(t, 10, config.Logging.Logs["access"].Writer.RollSize)
	require.Equal(t, 5, config.Logging.Logs["access"].Writer.RollKeep)
	require.Equal(t, 7, config.Logging.Logs["access"].Writer.RollKeepDays)
}

func TestGenerateConfig_Advanced(t *testing.T) {
	hosts := []models.ProxyHost{
		{
			UUID:           "advanced-uuid",
			Name:           "Advanced",
			DomainNames:    "advanced.example.com",
			ForwardScheme:  "http",
			ForwardHost:    "advanced",
			ForwardPort:    8080,
			SSLForced:      true,
			HSTSEnabled:    true,
			HSTSSubdomains: true,
			BlockExploits:  true,
			Enabled:        true,
			Locations: []models.Location{
				{
					Path:        "/api",
					ForwardHost: "api-service",
					ForwardPort: 9000,
				},
			},
		},
	}

	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, nil, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, config)

	server := config.Apps.HTTP.Servers["cpm_server"]
	require.NotNil(t, server)
	// Should have 2 routes: 1 for location /api, 1 for main domain
	require.Len(t, server.Routes, 2)

	// Check Location Route (should be first as it is more specific)
	locRoute := server.Routes[0]
	require.Equal(t, []string{"/api", "/api/*"}, locRoute.Match[0].Path)
	require.Equal(t, []string{"advanced.example.com"}, locRoute.Match[0].Host)

	// Check Main Route
	mainRoute := server.Routes[1]
	require.Nil(t, mainRoute.Match[0].Path) // No path means all paths
	require.Equal(t, []string{"advanced.example.com"}, mainRoute.Match[0].Host)

	// Check HSTS and BlockExploits handlers in main route
	// Handlers are: [HSTS, BlockExploits, ReverseProxy]
	// But wait, BlockExploitsHandler implementation details?
	// Let's just check count for now or inspect types if possible.
	// Based on code:
	// handlers = append(handlers, HeaderHandler(...)) // HSTS
	// handlers = append(handlers, BlockExploitsHandler()) // BlockExploits
	// mainHandlers = append(handlers, ReverseProxyHandler(...))

	require.Len(t, mainRoute.Handle, 3)

	// Check HSTS
	hstsHandler := mainRoute.Handle[0]
	require.Equal(t, "headers", hstsHandler["handler"])
}

func TestGenerateConfig_ACMEStaging(t *testing.T) {
	hosts := []models.ProxyHost{
		{
			UUID:        "test-uuid",
			DomainNames: "test.example.com",
			ForwardHost: "app",
			ForwardPort: 8080,
			Enabled:     true,
		},
	}

	// Test with staging enabled
	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "letsencrypt", true, nil, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, config.Apps.TLS)
	require.NotNil(t, config.Apps.TLS.Automation)
	require.Len(t, config.Apps.TLS.Automation.Policies, 1)

	issuers := config.Apps.TLS.Automation.Policies[0].IssuersRaw
	require.Len(t, issuers, 1)

	acmeIssuer := issuers[0].(map[string]interface{})
	require.Equal(t, "acme", acmeIssuer["module"])
	require.Equal(t, "admin@example.com", acmeIssuer["email"])
	require.Equal(t, "https://acme-staging-v02.api.letsencrypt.org/directory", acmeIssuer["ca"])

	// Test with staging disabled (production)
	config, err = GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "letsencrypt", false, nil, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, config.Apps.TLS)
	require.NotNil(t, config.Apps.TLS.Automation)
	require.Len(t, config.Apps.TLS.Automation.Policies, 1)

	issuers = config.Apps.TLS.Automation.Policies[0].IssuersRaw
	require.Len(t, issuers, 1)

	acmeIssuer = issuers[0].(map[string]interface{})
	require.Equal(t, "acme", acmeIssuer["module"])
	require.Equal(t, "admin@example.com", acmeIssuer["email"])
	_, hasCA := acmeIssuer["ca"]
	require.False(t, hasCA, "Production mode should not set ca field (uses default)")
	// We can't easily check the map content without casting, but we know it's there.
}

func TestGenerateSecurityApp(t *testing.T) {
	t.Run("empty inputs", func(t *testing.T) {
		app := generateSecurityApp(nil, nil, nil)
		require.NotNil(t, app)
		require.NotNil(t, app.Config)
		require.NotNil(t, app.Config.AuthenticationPortals)
		require.NotNil(t, app.Config.AuthorizationPolicies)
		require.NotNil(t, app.Config.IdentityProviders)
	})

	t.Run("with local users", func(t *testing.T) {
		users := []models.AuthUser{
			{Username: "admin", Email: "admin@example.com", PasswordHash: "hash123", Enabled: true},
			{Username: "user", Email: "user@example.com", PasswordHash: "hash456", Enabled: true},
		}
		app := generateSecurityApp(users, nil, nil)
		require.NotNil(t, app)
		require.NotNil(t, app.Config)

		// Check Identity Providers
		require.Len(t, app.Config.IdentityProviders, 1)
		localProvider := app.Config.IdentityProviders[0]
		require.Equal(t, "local", localProvider.Name)
		require.Equal(t, "local", localProvider.Kind)

		// Check Portal
		require.Len(t, app.Config.AuthenticationPortals, 1)
		portal := app.Config.AuthenticationPortals[0]
		require.Equal(t, "cpmp_portal", portal.Name)
		require.Contains(t, portal.IdentityProviders, "local")
	})

	t.Run("with disabled users", func(t *testing.T) {
		users := []models.AuthUser{
			{Username: "active", Email: "active@example.com", PasswordHash: "hash", Enabled: true},
			{Username: "inactive", Email: "inactive@example.com", PasswordHash: "hash", Enabled: false},
		}
		app := generateSecurityApp(users, nil, nil)

		require.Len(t, app.Config.IdentityProviders, 1)
		localProvider := app.Config.IdentityProviders[0]

		usersConfig := localProvider.Params["users"].([]map[string]interface{})
		require.Len(t, usersConfig, 1)
		require.Equal(t, "active", usersConfig[0]["username"])
	})

	t.Run("with oauth providers", func(t *testing.T) {
		providers := []models.AuthProvider{
			{
				Name:         "Google",
				Type:         "google",
				Enabled:      true,
				ClientID:     "google-client-id",
				ClientSecret: "google-secret",
				Scopes:       "openid,profile,email",
			},
			{
				Name:         "GitHub",
				Type:         "github",
				Enabled:      true,
				ClientID:     "github-client-id",
				ClientSecret: "github-secret",
			},
		}
		app := generateSecurityApp(nil, providers, nil)

		require.Len(t, app.Config.IdentityProviders, 2)

		// Find Google provider
		var googleProvider *IdentityProvider
		for _, p := range app.Config.IdentityProviders {
			if p.Name == "Google" {
				googleProvider = p
				break
			}
		}
		require.NotNil(t, googleProvider)
		require.Equal(t, "oauth", googleProvider.Kind)
		require.Equal(t, "google", googleProvider.Params["realm"])
		require.Equal(t, "google-client-id", googleProvider.Params["client_id"])

		// Check Portal references
		require.Len(t, app.Config.AuthenticationPortals, 1)
		portal := app.Config.AuthenticationPortals[0]
		require.Contains(t, portal.IdentityProviders, "Google")
		require.Contains(t, portal.IdentityProviders, "GitHub")
	})

	t.Run("with disabled providers", func(t *testing.T) {
		providers := []models.AuthProvider{
			{Name: "Active", Type: "oidc", Enabled: true, ClientID: "id", ClientSecret: "secret"},
			{Name: "Inactive", Type: "oidc", Enabled: false, ClientID: "id2", ClientSecret: "secret2"},
		}
		app := generateSecurityApp(nil, providers, nil)

		require.Len(t, app.Config.IdentityProviders, 1)
		require.Equal(t, "Active", app.Config.IdentityProviders[0].Name)
	})

	t.Run("with authorization policies", func(t *testing.T) {
		policies := []models.AuthPolicy{
			{
				Name:         "admin_policy",
				Enabled:      true,
				AllowedRoles: "admin,super",
				AllowedUsers: "user1,user2",
				RequireMFA:   true,
			},
			{
				Name:         "user_policy",
				Enabled:      true,
				AllowedRoles: "user",
			},
		}
		app := generateSecurityApp(nil, nil, policies)

		require.Len(t, app.Config.AuthorizationPolicies, 2)

		// Find admin policy
		var adminPolicy *AuthzPolicy
		for _, p := range app.Config.AuthorizationPolicies {
			if p.Name == "admin_policy" {
				adminPolicy = p
				break
			}
		}
		require.NotNil(t, adminPolicy)

		// Check rules
		// Note: The implementation converts roles/users to conditions in AccessListRules
		require.NotEmpty(t, adminPolicy.AccessListRules)
		rule := adminPolicy.AccessListRules[0]
		require.Contains(t, rule.Conditions, "match roles admin")
		require.Contains(t, rule.Conditions, "match email user1")
	})

	t.Run("with disabled policies", func(t *testing.T) {
		policies := []models.AuthPolicy{
			{Name: "active", Enabled: true},
			{Name: "inactive", Enabled: false},
		}
		app := generateSecurityApp(nil, nil, policies)

		require.Len(t, app.Config.AuthorizationPolicies, 1)
		require.Equal(t, "active", app.Config.AuthorizationPolicies[0].Name)
	})

	t.Run("provider with custom URLs", func(t *testing.T) {
		providers := []models.AuthProvider{
			{
				Name:         "Custom OIDC",
				Type:         "oidc",
				Enabled:      true,
				ClientID:     "client-id",
				ClientSecret: "secret",
				IssuerURL:    "https://issuer.example.com",
				AuthURL:      "https://auth.example.com/authorize",
				TokenURL:     "https://auth.example.com/token",
			},
		}
		app := generateSecurityApp(nil, providers, nil)

		require.Len(t, app.Config.IdentityProviders, 1)
		provider := app.Config.IdentityProviders[0]

		require.Equal(t, "https://issuer.example.com", provider.Params["base_auth_url"])
		require.Equal(t, "https://auth.example.com/authorize", provider.Params["authorization_url"])
		require.Equal(t, "https://auth.example.com/token", provider.Params["token_url"])
	})
}

func TestConvertAuthUsersToConfig(t *testing.T) {
	t.Run("empty users", func(t *testing.T) {
		result := convertAuthUsersToConfig(nil)
		require.Empty(t, result)
	})

	t.Run("filters disabled users", func(t *testing.T) {
		users := []models.AuthUser{
			{Username: "active", Email: "active@example.com", PasswordHash: "hash1", Enabled: true},
			{Username: "disabled", Email: "disabled@example.com", PasswordHash: "hash2", Enabled: false},
		}
		result := convertAuthUsersToConfig(users)
		require.Len(t, result, 1)
		require.Equal(t, "active", result[0]["username"])
	})

	t.Run("includes user details", func(t *testing.T) {
		users := []models.AuthUser{
			{
				Username:     "testuser",
				Email:        "test@example.com",
				PasswordHash: "bcrypt-hash",
				Name:         "Test User",
				Roles:        "admin,editor",
				Enabled:      true,
			},
		}
		result := convertAuthUsersToConfig(users)
		require.Len(t, result, 1)

		userConfig := result[0]
		require.Equal(t, "testuser", userConfig["username"])
		require.Equal(t, "test@example.com", userConfig["email"])
		require.Equal(t, "bcrypt-hash", userConfig["password"])
		require.Equal(t, "Test User", userConfig["name"])
		require.Equal(t, []string{"admin", "editor"}, userConfig["roles"])
	})

	t.Run("omits empty name", func(t *testing.T) {
		users := []models.AuthUser{
			{Username: "noname", Email: "noname@example.com", PasswordHash: "hash", Enabled: true},
		}
		result := convertAuthUsersToConfig(users)
		_, hasName := result[0]["name"]
		require.False(t, hasName)
	})

	t.Run("omits empty roles", func(t *testing.T) {
		users := []models.AuthUser{
			{Username: "noroles", Email: "noroles@example.com", PasswordHash: "hash", Enabled: true},
		}
		result := convertAuthUsersToConfig(users)
		_, hasRoles := result[0]["roles"]
		require.False(t, hasRoles)
	})
}
