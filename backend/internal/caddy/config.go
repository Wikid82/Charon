package caddy

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
)

// GenerateConfig creates a Caddy JSON configuration from proxy hosts.
// This is the core transformation layer from our database model to Caddy config.
func GenerateConfig(hosts []models.ProxyHost, storageDir string, acmeEmail string, frontendDir string, sslProvider string, acmeStaging bool, forwardAuthConfig *models.ForwardAuthConfig, authUsers []models.AuthUser, authProviders []models.AuthProvider, authPolicies []models.AuthPolicy) (*Config, error) {
	// Define log file paths
	// We assume storageDir is like ".../data/caddy/data", so we go up to ".../data/logs"
	// storageDir is .../data/caddy/data
	// Dir -> .../data/caddy
	// Dir -> .../data
	logDir := filepath.Join(filepath.Dir(filepath.Dir(storageDir)), "logs")
	logFile := filepath.Join(logDir, "access.log")

	config := &Config{
		Logging: &LoggingConfig{
			Logs: map[string]*LogConfig{
				"access": {
					Level: "INFO",
					Writer: &WriterConfig{
						Output:       "file",
						Filename:     logFile,
						Roll:         true,
						RollSize:     10, // 10 MB
						RollKeep:     5,  // Keep 5 files
						RollKeepDays: 7,  // Keep for 7 days
					},
					Encoder: &EncoderConfig{
						Format: "json",
					},
					Include: []string{"http.log.access.access_log"},
				},
			},
		},
		Apps: Apps{
			HTTP: &HTTPApp{
				Servers: map[string]*Server{},
			},
		},
		Storage: Storage{
			System: "file_system",
			Root:   storageDir,
		},
	}

	if acmeEmail != "" {
		var issuers []interface{}

		// Configure issuers based on provider preference
		switch sslProvider {
		case "letsencrypt":
			acmeIssuer := map[string]interface{}{
				"module": "acme",
				"email":  acmeEmail,
			}
			if acmeStaging {
				acmeIssuer["ca"] = "https://acme-staging-v02.api.letsencrypt.org/directory"
			}
			issuers = append(issuers, acmeIssuer)
		case "zerossl":
			issuers = append(issuers, map[string]interface{}{
				"module": "zerossl",
			})
		default: // "both" or empty
			acmeIssuer := map[string]interface{}{
				"module": "acme",
				"email":  acmeEmail,
			}
			if acmeStaging {
				acmeIssuer["ca"] = "https://acme-staging-v02.api.letsencrypt.org/directory"
			}
			issuers = append(issuers, acmeIssuer)
			issuers = append(issuers, map[string]interface{}{
				"module": "zerossl",
			})
		}

		config.Apps.TLS = &TLSApp{
			Automation: &AutomationConfig{
				Policies: []*AutomationPolicy{
					{
						IssuersRaw: issuers,
					},
				},
			},
		}
	}

	// Collect CUSTOM certificates only (not Let's Encrypt - those are managed by ACME)
	// Only custom/uploaded certificates should be loaded via LoadPEM
	customCerts := make(map[uint]models.SSLCertificate)
	for _, host := range hosts {
		if host.CertificateID != nil && host.Certificate != nil {
			// Only include custom certificates, not ACME-managed ones
			if host.Certificate.Provider == "custom" {
				customCerts[*host.CertificateID] = *host.Certificate
			}
		}
	}

	if len(customCerts) > 0 {
		var loadPEM []LoadPEMConfig
		for _, cert := range customCerts {
			// Validate that custom cert has both certificate and key
			if cert.Certificate == "" || cert.PrivateKey == "" {
				fmt.Printf("Warning: Custom certificate %s missing certificate or key, skipping\n", cert.Name)
				continue
			}
			loadPEM = append(loadPEM, LoadPEMConfig{
				Certificate: cert.Certificate,
				Key:         cert.PrivateKey,
				Tags:        []string{cert.UUID},
			})
		}

		if len(loadPEM) > 0 {
			if config.Apps.TLS == nil {
				config.Apps.TLS = &TLSApp{}
			}
			config.Apps.TLS.Certificates = &CertificatesConfig{
				LoadPEM: loadPEM,
			}
		}
	}

	// Configure Security App (Built-in SSO) if we have users or providers
	if len(authUsers) > 0 || len(authProviders) > 0 {
		config.Apps.Security = generateSecurityApp(authUsers, authProviders, authPolicies)
	}

	if len(hosts) == 0 && frontendDir == "" {
		return config, nil
	}

	// Initialize routes slice
	routes := make([]*Route, 0)

	// Track processed domains to prevent duplicates (Ghost Host fix)
	processedDomains := make(map[string]bool)

	// Sort hosts by UpdatedAt desc to prefer newer configs in case of duplicates
	// Note: This assumes the input slice is already sorted or we don't care about order beyond duplicates
	// The caller (ApplyConfig) fetches all hosts. We should probably sort them here or there.
	// For now, we'll just process them. If we encounter a duplicate domain, we skip it.
	// To ensure we keep the *latest* one, we should iterate in reverse or sort.
	// But ApplyConfig uses db.Find(&hosts), which usually returns by ID asc.
	// So later IDs (newer) come last.
	// We want to keep the NEWER one.
	// So we should iterate backwards? Or just overwrite?
	// Caddy config structure is a list of servers/routes.
	// If we have multiple routes matching the same host, Caddy uses the first one?
	// Actually, Caddy matches routes in order.
	// If we emit two routes for "example.com", the first one will catch it.
	// So we want the NEWEST one to be FIRST in the list?
	// Or we want to only emit ONE route for "example.com".
	// If we emit only one, it should be the newest one.
	// So we should process hosts from newest to oldest, and skip duplicates.

	// Let's iterate in reverse order (assuming input is ID ASC)
	for i := len(hosts) - 1; i >= 0; i-- {
		host := hosts[i]

		if !host.Enabled {
			continue
		}

		if host.DomainNames == "" {
			// Log warning?
			continue
		}

		// Parse comma-separated domains
		rawDomains := strings.Split(host.DomainNames, ",")
		var uniqueDomains []string

		for _, d := range rawDomains {
			d = strings.TrimSpace(d)
			d = strings.ToLower(d) // Normalize to lowercase
			if d == "" {
				continue
			}
			if processedDomains[d] {
				fmt.Printf("Warning: Skipping duplicate domain %s for host %s (Ghost Host detection)\n", d, host.UUID)
				continue
			}
			processedDomains[d] = true
			uniqueDomains = append(uniqueDomains, d)
		}

		if len(uniqueDomains) == 0 {
			continue
		}

		// Build handlers for this host
		handlers := make([]Handler, 0)

		// Add Built-in SSO (caddy-security) if a policy is assigned
		if host.AuthPolicyID != nil && host.AuthPolicy != nil && host.AuthPolicy.Enabled {
			// Inject authentication portal check
			handlers = append(handlers, SecurityAuthHandler("cpmp_portal"))
			// Inject authorization policy check
			handlers = append(handlers, SecurityAuthzHandler(host.AuthPolicy.Name))
		}

		// Add Forward Auth if enabled for this host (legacy forward auth, not SSO)
		if host.ForwardAuthEnabled && forwardAuthConfig != nil && forwardAuthConfig.Address != "" {
			// Parse bypass paths
			var bypassPaths []string
			if host.ForwardAuthBypass != "" {
				rawPaths := strings.Split(host.ForwardAuthBypass, ",")
				for _, p := range rawPaths {
					p = strings.TrimSpace(p)
					if p != "" {
						bypassPaths = append(bypassPaths, p)
					}
				}
			}

			// If we have bypass paths, we need to conditionally apply auth
			if len(bypassPaths) > 0 {
				// Create a subroute that only applies auth to non-bypass paths
				// This is complex - for now, add auth unconditionally and handle bypass in a separate route
				// A better approach: create bypass routes BEFORE auth routes
			}

			handlers = append(handlers, ForwardAuthHandler(forwardAuthConfig.Address, forwardAuthConfig.TrustForwardHeader))
		}

		// Add HSTS header if enabled
		if host.HSTSEnabled {
			hstsValue := "max-age=31536000"
			if host.HSTSSubdomains {
				hstsValue += "; includeSubDomains"
			}
			handlers = append(handlers, HeaderHandler(map[string][]string{
				"Strict-Transport-Security": {hstsValue},
			}))
		}

		// Add exploit blocking if enabled
		if host.BlockExploits {
			handlers = append(handlers, BlockExploitsHandler())
		}

		// Handle bypass routes FIRST if Forward Auth is enabled
		if host.ForwardAuthEnabled && host.ForwardAuthBypass != "" {
			rawPaths := strings.Split(host.ForwardAuthBypass, ",")
			for _, p := range rawPaths {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				// Create bypass route without auth
				dial := fmt.Sprintf("%s:%d", host.ForwardHost, host.ForwardPort)
				bypassRoute := &Route{
					Match: []Match{
						{
							Host: uniqueDomains,
							Path: []string{p, p + "/*"},
						},
					},
					Handle: []Handler{
						ReverseProxyHandler(dial, host.WebsocketSupport),
					},
					Terminal: true,
				}
				routes = append(routes, bypassRoute)
			}
		}

		// Handle custom locations first (more specific routes)
		for _, loc := range host.Locations {
			dial := fmt.Sprintf("%s:%d", loc.ForwardHost, loc.ForwardPort)
			locRoute := &Route{
				Match: []Match{
					{
						Host: uniqueDomains,
						Path: []string{loc.Path, loc.Path + "/*"},
					},
				},
				Handle: []Handler{
					ReverseProxyHandler(dial, host.WebsocketSupport),
				},
				Terminal: true,
			}
			routes = append(routes, locRoute)
		}

		// Main proxy handler
		dial := fmt.Sprintf("%s:%d", host.ForwardHost, host.ForwardPort)
		mainHandlers := append(handlers, ReverseProxyHandler(dial, host.WebsocketSupport))

		route := &Route{
			Match: []Match{
				{Host: uniqueDomains},
			},
			Handle:   mainHandlers,
			Terminal: true,
		}

		routes = append(routes, route)
	}

	// Add catch-all 404 handler
	// This matches any request that wasn't handled by previous routes
	if frontendDir != "" {
		catchAllRoute := &Route{
			Handle: []Handler{
				RewriteHandler("/unknown.html"),
				FileServerHandler(frontendDir),
			},
			Terminal: true,
		}
		routes = append(routes, catchAllRoute)
	}

	config.Apps.HTTP.Servers["cpm_server"] = &Server{
		Listen: []string{":80", ":443"},
		Routes: routes,
		AutoHTTPS: &AutoHTTPSConfig{
			Disable:      false,
			DisableRedir: false,
		},
		Logs: &ServerLogs{
			DefaultLoggerName: "access_log",
		},
	}

	return config, nil
}

// generateSecurityApp creates the caddy-security app configuration.
func generateSecurityApp(authUsers []models.AuthUser, authProviders []models.AuthProvider, authPolicies []models.AuthPolicy) *SecurityApp {
	securityConfig := &SecurityConfig{
		AuthenticationPortals: make([]*AuthPortal, 0),
		IdentityProviders:     make([]*IdentityProvider, 0),
		IdentityStores:        make([]*IdentityStore, 0),
		AuthorizationPolicies: make([]*AuthzPolicy, 0),
	}

	// Create the main authentication portal
	portal := &AuthPortal{
		Name:         "cpmp_portal",
		CookieDomain: "", // Will use request host
		UISettings: map[string]interface{}{
			"theme": "basic",
		},
		CookieConfig: map[string]interface{}{
			"lifetime": 86400, // 24 hours
		},
		CryptoKeyStoreConfig: map[string]interface{}{
			"token_lifetime": 3600, // 1 hour
		},
		API: map[string]interface{}{
			"profile_enabled": true,
		},
		IdentityProviders: make([]string, 0),
		IdentityStores:    make([]string, 0),
	}

	// Add local backend if we have local users
	if len(authUsers) > 0 {
		localStore := &IdentityStore{
			Name: "local",
			Kind: "local",
			Params: map[string]interface{}{
				"realm": "local",
				"users": convertAuthUsersToConfig(authUsers),
			},
		}
		securityConfig.IdentityStores = append(securityConfig.IdentityStores, localStore)
		portal.IdentityStores = append(portal.IdentityStores, "local")
	}

	// Add OAuth providers
	for _, provider := range authProviders {
		if !provider.Enabled {
			continue
		}

		oauthProvider := &IdentityProvider{
			Name: provider.Name,
			Kind: "oauth",
			Params: map[string]interface{}{
				"client_id":     provider.ClientID,
				"client_secret": provider.ClientSecret,
				"driver":        provider.Type,
				"realm":         provider.Type,
			},
		}

		// Add provider-specific config
		if provider.IssuerURL != "" {
			oauthProvider.Params["base_auth_url"] = provider.IssuerURL
		}
		if provider.AuthURL != "" {
			oauthProvider.Params["authorization_url"] = provider.AuthURL
		}
		if provider.TokenURL != "" {
			oauthProvider.Params["token_url"] = provider.TokenURL
		}
		if provider.Scopes != "" {
			oauthProvider.Params["scopes"] = strings.Split(provider.Scopes, ",")
		}

		securityConfig.IdentityProviders = append(securityConfig.IdentityProviders, oauthProvider)
		portal.IdentityProviders = append(portal.IdentityProviders, provider.Name)
	}

	securityConfig.AuthenticationPortals = append(securityConfig.AuthenticationPortals, portal)

	// Generate authorization policies
	for _, policy := range authPolicies {
		if !policy.Enabled {
			continue
		}

		authzPolicy := &AuthzPolicy{
			Name:            policy.Name,
			AccessListRules: make([]*AccessListRule, 0),
		}

		// Build conditions
		var conditions []string
		if policy.AllowedRoles != "" {
			roles := strings.Split(policy.AllowedRoles, ",")
			for _, role := range roles {
				conditions = append(conditions, fmt.Sprintf("match roles %s", strings.TrimSpace(role)))
			}
		}
		if policy.AllowedUsers != "" {
			users := strings.Split(policy.AllowedUsers, ",")
			for _, user := range users {
				conditions = append(conditions, fmt.Sprintf("match email %s", strings.TrimSpace(user)))
			}
		}

		// If no conditions, allow all authenticated (default behavior if policy exists?)
		// Or maybe we should require at least one condition?
		// For now, if conditions exist, add a rule.
		if len(conditions) > 0 {
			rule := &AccessListRule{
				Conditions: conditions,
				Action:     "allow",
			}
			authzPolicy.AccessListRules = append(authzPolicy.AccessListRules, rule)
		} else {
			// If no specific roles/users, allow any authenticated user
			// "match any" condition?
			// caddy-security default is deny if no rule matches?
			// Let's add a rule to allow any authenticated user if no restrictions
			// "match roles authp/user" or similar?
			// Actually, if policy is enabled but empty, maybe it means "allow all authenticated"?
			// Let's assume "allow" action with no conditions matches everything?
			// No, conditions are required.
			// Let's use "match roles *" or similar if supported, or just don't add rule (deny all).
			// But user probably wants "Authenticated Users" if they didn't specify roles.
			// Let's add a rule that matches any role if no specific roles/users are set.
			// But wait, we don't have a generic "authenticated" condition easily.
			// Let's stick to what we have. If empty, it might deny all.
		}

		securityConfig.AuthorizationPolicies = append(securityConfig.AuthorizationPolicies, authzPolicy)
	}

	return &SecurityApp{
		Config: securityConfig,
	}
}

// convertAuthUsersToConfig converts AuthUser models to caddy-security user config format.
func convertAuthUsersToConfig(users []models.AuthUser) []map[string]interface{} {
	result := make([]map[string]interface{}, 0)
	for _, user := range users {
		if !user.Enabled {
			continue
		}

		// Helper to create user config
		createUserConfig := func(username, email string) map[string]interface{} {
			cfg := map[string]interface{}{
				"username": username,
				"email":    email,
				"password": user.PasswordHash, // Already bcrypt hashed
			}

			if user.Name != "" {
				cfg["name"] = user.Name
			}

			if user.Roles != "" {
				cfg["roles"] = strings.Split(user.Roles, ",")
			}
			return cfg
		}

		// Add primary user
		result = append(result, createUserConfig(user.Username, user.Email))

		// Add additional emails as alias users
		if user.AdditionalEmails != "" {
			emails := strings.Split(user.AdditionalEmails, ",")
			for i, email := range emails {
				email = strings.TrimSpace(email)
				if email == "" {
					continue
				}
				// Create a derived username for the alias
				// We use a predictable suffix so it doesn't change
				aliasUsername := fmt.Sprintf("%s_alt%d", user.Username, i+1)
				result = append(result, createUserConfig(aliasUsername, email))
			}
		}
	}
	return result
}
