package caddy

// Config represents Caddy's top-level JSON configuration structure.
// Reference: https://caddyserver.com/docs/json/
type Config struct {
	Apps    Apps           `json:"apps"`
	Logging *LoggingConfig `json:"logging,omitempty"`
	Storage Storage        `json:"storage,omitempty"`
}

// LoggingConfig configures Caddy's logging facility.
type LoggingConfig struct {
	Logs  map[string]*LogConfig `json:"logs,omitempty"`
	Sinks *SinkConfig           `json:"sinks,omitempty"`
}

// LogConfig configures a specific logger.
type LogConfig struct {
	Writer  *WriterConfig  `json:"writer,omitempty"`
	Encoder *EncoderConfig `json:"encoder,omitempty"`
	Level   string         `json:"level,omitempty"`
	Include []string       `json:"include,omitempty"`
	Exclude []string       `json:"exclude,omitempty"`
}

// WriterConfig configures the log writer (output).
type WriterConfig struct {
	Output       string `json:"output"`
	Filename     string `json:"filename,omitempty"`
	Roll         bool   `json:"roll,omitempty"`
	RollSize     int    `json:"roll_size_mb,omitempty"`
	RollKeep     int    `json:"roll_keep,omitempty"`
	RollKeepDays int    `json:"roll_keep_days,omitempty"`
}

// EncoderConfig configures the log format.
type EncoderConfig struct {
	Format string `json:"format"` // "json", "console", etc.
}

// SinkConfig configures log sinks (e.g. stderr).
type SinkConfig struct {
	Writer *WriterConfig `json:"writer,omitempty"`
}

// Storage configures the storage module.
type Storage struct {
	System string `json:"module"`
	Root   string `json:"root,omitempty"`
}

// Apps contains all Caddy app modules.
type Apps struct {
	HTTP     *HTTPApp     `json:"http,omitempty"`
	TLS      *TLSApp      `json:"tls,omitempty"`
	Security *SecurityApp `json:"security,omitempty"`
}

// HTTPApp configures the HTTP app.
type HTTPApp struct {
	Servers map[string]*Server `json:"servers"`
}

// Server represents an HTTP server instance.
type Server struct {
	Listen    []string         `json:"listen"`
	Routes    []*Route         `json:"routes"`
	AutoHTTPS *AutoHTTPSConfig `json:"automatic_https,omitempty"`
	Logs      *ServerLogs      `json:"logs,omitempty"`
}

// AutoHTTPSConfig controls automatic HTTPS behavior.
type AutoHTTPSConfig struct {
	Disable      bool     `json:"disable,omitempty"`
	DisableRedir bool     `json:"disable_redirects,omitempty"`
	Skip         []string `json:"skip,omitempty"`
}

// ServerLogs configures access logging.
type ServerLogs struct {
	DefaultLoggerName string `json:"default_logger_name,omitempty"`
}

// Route represents an HTTP route (matcher + handlers).
type Route struct {
	Match    []Match   `json:"match,omitempty"`
	Handle   []Handler `json:"handle"`
	Terminal bool      `json:"terminal,omitempty"`
}

// Match represents a request matcher.
type Match struct {
	Host []string `json:"host,omitempty"`
	Path []string `json:"path,omitempty"`
}

// Handler is the interface for all handler types.
// Actual types will implement handler-specific fields.
type Handler map[string]interface{}

// ReverseProxyHandler creates a reverse_proxy handler.
func ReverseProxyHandler(dial string, enableWS bool) Handler {
	h := Handler{
		"handler":        "reverse_proxy",
		"flush_interval": -1, // Disable buffering for better streaming performance (Plex, etc.)
		"upstreams": []map[string]interface{}{
			{"dial": dial},
		},
	}

	if enableWS {
		// Enable WebSocket support by preserving upgrade headers
		h["headers"] = map[string]interface{}{
			"request": map[string]interface{}{
				"set": map[string][]string{
					"Upgrade":    {"{http.request.header.Upgrade}"},
					"Connection": {"{http.request.header.Connection}"},
				},
			},
		}
	}

	return h
}

// HeaderHandler creates a handler that sets HTTP response headers.
func HeaderHandler(headers map[string][]string) Handler {
	return Handler{
		"handler": "headers",
		"response": map[string]interface{}{
			"set": headers,
		},
	}
}

// BlockExploitsHandler creates a handler that blocks common exploits.
// This uses Caddy's request matchers to block malicious patterns.
func BlockExploitsHandler() Handler {
	return Handler{
		"handler": "vars",
		// Placeholder for future exploit blocking logic
		// Can be extended with specific matchers for SQL injection, XSS, etc.
	}
}

// RewriteHandler creates a rewrite handler.
func RewriteHandler(uri string) Handler {
	return Handler{
		"handler": "rewrite",
		"uri":     uri,
	}
}

// FileServerHandler creates a file_server handler.
func FileServerHandler(root string) Handler {
	return Handler{
		"handler": "file_server",
		"root":    root,
	}
}

// ForwardAuthHandler creates a forward authentication handler using reverse_proxy.
// This sends the request to an auth provider and uses handle_response to process the result.
func ForwardAuthHandler(authAddress string, trustForwardHeader bool) Handler {
	h := Handler{
		"handler": "reverse_proxy",
		"upstreams": []map[string]interface{}{
			{"dial": authAddress},
		},
		"handle_response": []map[string]interface{}{
			{
				"match": map[string]interface{}{
					"status_code": []int{200},
				},
				"routes": []map[string]interface{}{
					{
						"handle": []map[string]interface{}{
							{
								"handler": "headers",
								"request": map[string]interface{}{
									"set": map[string][]string{
										"Remote-User":   {"{http.reverse_proxy.header.Remote-User}"},
										"Remote-Email":  {"{http.reverse_proxy.header.Remote-Email}"},
										"Remote-Name":   {"{http.reverse_proxy.header.Remote-Name}"},
										"Remote-Groups": {"{http.reverse_proxy.header.Remote-Groups}"},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if trustForwardHeader {
		h["headers"] = map[string]interface{}{
			"request": map[string]interface{}{
				"set": map[string][]string{
					"X-Forwarded-Method": {"{http.request.method}"},
					"X-Forwarded-Uri":    {"{http.request.uri}"},
				},
			},
		}
	}

	return h
}

// TLSApp configures the TLS app for certificate management.
type TLSApp struct {
	Automation   *AutomationConfig   `json:"automation,omitempty"`
	Certificates *CertificatesConfig `json:"certificates,omitempty"`
}

// CertificatesConfig configures manual certificate loading.
type CertificatesConfig struct {
	LoadPEM []LoadPEMConfig `json:"load_pem,omitempty"`
}

// LoadPEMConfig defines a PEM-loaded certificate.
type LoadPEMConfig struct {
	Certificate string   `json:"certificate"`
	Key         string   `json:"key"`
	Tags        []string `json:"tags,omitempty"`
}

// AutomationConfig controls certificate automation.
type AutomationConfig struct {
	Policies []*AutomationPolicy `json:"policies,omitempty"`
}

// AutomationPolicy defines certificate management for specific domains.
type AutomationPolicy struct {
	Subjects   []string      `json:"subjects,omitempty"`
	IssuersRaw []interface{} `json:"issuers,omitempty"`
}

// SecurityApp configures the caddy-security plugin for SSO/authentication.
type SecurityApp struct {
	Config *SecurityConfig `json:"config,omitempty"`
}

// SecurityConfig holds the configuration for caddy-security.
type SecurityConfig struct {
	AuthenticationPortals []*AuthPortal       `json:"authentication_portals,omitempty"`
	AuthorizationPolicies []*AuthzPolicy      `json:"authorization_policies,omitempty"`
	IdentityProviders     []*IdentityProvider `json:"identity_providers,omitempty"`
	IdentityStores        []*IdentityStore    `json:"identity_stores,omitempty"`
}

// AuthPortal represents an authentication portal configuration.
type AuthPortal struct {
	Name                  string                 `json:"name,omitempty"`
	UISettings            map[string]interface{} `json:"ui,omitempty"`
	CookieDomain          string                 `json:"cookie_domain,omitempty"`
	CookieConfig          map[string]interface{} `json:"cookie_config,omitempty"`
	IdentityProviders     []string               `json:"identity_providers,omitempty"`
	IdentityStores        []string               `json:"identity_stores,omitempty"`
	TokenValidatorOptions map[string]interface{} `json:"token_validator_options,omitempty"`
	CryptoKeyStoreConfig  map[string]interface{} `json:"crypto_key_store_config,omitempty"`
	TokenGrantorOptions   map[string]interface{} `json:"token_grantor_options,omitempty"`
	PortalAdminRoles      map[string]bool        `json:"portal_admin_roles,omitempty"`
	PortalUserRoles       map[string]bool        `json:"portal_user_roles,omitempty"`
	PortalGuestRoles      map[string]bool        `json:"portal_guest_roles,omitempty"`
	API                   map[string]interface{} `json:"api,omitempty"`
}

// IdentityProvider represents an identity provider configuration.
type IdentityProvider struct {
	Name   string                 `json:"name"`
	Kind   string                 `json:"kind"` // "oauth", "saml"
	Params map[string]interface{} `json:"params,omitempty"`
}

// IdentityStore represents an identity store configuration.
type IdentityStore struct {
	Name   string                 `json:"name"`
	Kind   string                 `json:"kind"` // "local", "ldap"
	Params map[string]interface{} `json:"params,omitempty"`
}

// AuthzPolicy represents an authorization policy.
type AuthzPolicy struct {
	Name                   string            `json:"name,omitempty"`
	AuthURLPath            string            `json:"auth_url_path,omitempty"`
	AuthRedirectQueryParam string            `json:"auth_redirect_query_param,omitempty"`
	AuthRedirectStatusCode int               `json:"auth_redirect_status_code,omitempty"`
	AccessListRules        []*AccessListRule `json:"access_list_rules,omitempty"`
}

// AccessListRule represents a rule in an authorization policy.
type AccessListRule struct {
	Conditions []string `json:"conditions,omitempty"`
	Action     string   `json:"action,omitempty"`
}

// SecurityAuthHandler creates a caddy-security authentication handler.
func SecurityAuthHandler(portalName string) Handler {
	return Handler{
		"handler": "authentication",
		"portal":  portalName,
	}
}

// SecurityAuthzHandler creates a caddy-security authorization handler.
func SecurityAuthzHandler(policyName string) Handler {
	return Handler{
		"handler": "authorization",
		"policy":  policyName,
	}
}
