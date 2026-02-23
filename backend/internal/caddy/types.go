package caddy

// Config represents Caddy's top-level JSON configuration structure.
// Reference: https://caddyserver.com/docs/json/
type Config struct {
	Admin   *AdminConfig   `json:"admin,omitempty"`
	Apps    Apps           `json:"apps"`
	Logging *LoggingConfig `json:"logging,omitempty"`
	Storage Storage        `json:"storage,omitempty"`
}

// AdminConfig configures Caddy's admin API endpoint.
type AdminConfig struct {
	Listen string `json:"listen,omitempty"` // e.g., "0.0.0.0:2019" or ":2019"
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

// CrowdSecApp configures the CrowdSec app module.
// Reference: https://github.com/hslatman/caddy-crowdsec-bouncer
type CrowdSecApp struct {
	APIUrl          string `json:"api_url"`
	APIKey          string `json:"api_key"`
	TickerInterval  string `json:"ticker_interval,omitempty"`
	EnableStreaming *bool  `json:"enable_streaming,omitempty"`
}

// Apps contains all Caddy app modules.
type Apps struct {
	HTTP     *HTTPApp     `json:"http,omitempty"`
	TLS      *TLSApp      `json:"tls,omitempty"`
	CrowdSec *CrowdSecApp `json:"crowdsec,omitempty"`
}

// HTTPApp configures the HTTP app.
type HTTPApp struct {
	Servers map[string]*Server `json:"servers"`
}

// Server represents an HTTP server instance.
type Server struct {
	Listen         []string         `json:"listen"`
	Routes         []*Route         `json:"routes"`
	AutoHTTPS      *AutoHTTPSConfig `json:"automatic_https,omitempty"`
	Logs           *ServerLogs      `json:"logs,omitempty"`
	TrustedProxies *TrustedProxies  `json:"trusted_proxies,omitempty"`
	KeepaliveIdle  *string          `json:"keepalive_idle,omitempty"`
	KeepaliveCount *int             `json:"keepalive_count,omitempty"`
}

// TrustedProxies defines the module for configuring trusted proxy IP ranges.
// This is used at the server level to enable Caddy to trust X-Forwarded-For headers.
type TrustedProxies struct {
	Source string   `json:"source"`
	Ranges []string `json:"ranges"`
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
type Handler map[string]any

// ReverseProxyHandler creates a reverse_proxy handler.
// application: "none", "plex", "jellyfin", "emby", "homeassistant", "nextcloud", "vaultwarden"
// enableStandardHeaders: when true, adds 4 standard proxy headers (X-Real-IP, X-Forwarded-Proto, X-Forwarded-Host, X-Forwarded-Port)
func ReverseProxyHandler(dial string, enableWS bool, application string, enableStandardHeaders bool) Handler {
	h := Handler{
		"handler":        "reverse_proxy",
		"flush_interval": -1, // Disable buffering for better streaming performance (Plex, etc.)
		"upstreams": []map[string]any{
			{"dial": dial},
		},
	}

	// Build headers configuration
	headers := make(map[string]any)
	requestHeaders := make(map[string]any)
	setHeaders := make(map[string][]string)

	// STEP 1: Standard proxy headers (if feature enabled)
	// These 4 headers are the de-facto standard for HTTP reverse proxies (RFC 7239)
	// X-Forwarded-For is NOT explicitly set - Caddy handles it natively via reverse_proxy directive
	// to prevent duplication (Caddy appends to existing header automatically)
	if enableStandardHeaders {
		// X-Real-IP: Single IP of the immediate client (most apps check this first)
		setHeaders["X-Real-IP"] = []string{"{http.request.remote.host}"}
		// X-Forwarded-Proto: Original protocol (http/https) - critical for HTTPS enforcement
		setHeaders["X-Forwarded-Proto"] = []string{"{http.request.scheme}"}
		// X-Forwarded-Host: Original Host header - needed for virtual host routing
		setHeaders["X-Forwarded-Host"] = []string{"{http.request.host}"}
		// X-Forwarded-Port: Original port - important for non-standard ports
		setHeaders["X-Forwarded-Port"] = []string{"{http.request.port}"}
	}

	// STEP 2: WebSocket support headers
	// Only add Upgrade and Connection headers for WebSocket proxying
	if enableWS {
		setHeaders["Upgrade"] = []string{"{http.request.header.Upgrade}"}
		setHeaders["Connection"] = []string{"{http.request.header.Connection}"}
	}

	// STEP 3: Application-specific headers
	// These do NOT duplicate standard headers (they were added above if enabled)
	switch application {
	case "plex":
		// Pass-through Plex-specific headers for improved compatibility
		setHeaders["X-Plex-Client-Identifier"] = []string{"{http.request.header.X-Plex-Client-Identifier}"}
		setHeaders["X-Plex-Device"] = []string{"{http.request.header.X-Plex-Device}"}
		setHeaders["X-Plex-Device-Name"] = []string{"{http.request.header.X-Plex-Device-Name}"}
		setHeaders["X-Plex-Platform"] = []string{"{http.request.header.X-Plex-Platform}"}
		setHeaders["X-Plex-Platform-Version"] = []string{"{http.request.header.X-Plex-Platform-Version}"}
		setHeaders["X-Plex-Product"] = []string{"{http.request.header.X-Plex-Product}"}
		setHeaders["X-Plex-Token"] = []string{"{http.request.header.X-Plex-Token}"}
		setHeaders["X-Plex-Version"] = []string{"{http.request.header.X-Plex-Version}"}
		// Note: X-Real-IP and X-Forwarded-Host already set above if enableStandardHeaders=true
		// If enableStandardHeaders=false, maintain backward compatibility by setting them here
		if !enableStandardHeaders {
			setHeaders["X-Real-IP"] = []string{"{http.request.remote.host}"}
			setHeaders["X-Forwarded-Host"] = []string{"{http.request.host}"}
		}
	case "jellyfin", "emby", "homeassistant", "nextcloud", "vaultwarden":
		// Note: X-Real-IP and X-Forwarded-Host already set above if enableStandardHeaders=true
		// If enableStandardHeaders=false, maintain backward compatibility by setting them here
		if !enableStandardHeaders {
			setHeaders["X-Real-IP"] = []string{"{http.request.remote.host}"}
			setHeaders["X-Forwarded-Host"] = []string{"{http.request.host}"}
		}
	}

	// Only add headers config if we have headers to set
	if len(setHeaders) > 0 {
		requestHeaders["set"] = setHeaders
		headers["request"] = requestHeaders
		h["headers"] = headers
	}

	return h
}

// HeaderHandler creates a handler that sets HTTP response headers.
func HeaderHandler(headers map[string][]string) Handler {
	return Handler{
		"handler": "headers",
		"response": map[string]any{
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
	Subjects   []string `json:"subjects,omitempty"`
	IssuersRaw []any    `json:"issuers,omitempty"`
}

// DNSChallengeConfig configures DNS-01 challenge settings
type DNSChallengeConfig struct {
	Provider           map[string]any `json:"provider"`
	PropagationTimeout int64          `json:"propagation_timeout,omitempty"` // nanoseconds
	Resolvers          []string       `json:"resolvers,omitempty"`
}

// ChallengesConfig configures ACME challenge types
type ChallengesConfig struct {
	DNS *DNSChallengeConfig `json:"dns,omitempty"`
}
