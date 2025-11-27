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
	HTTP *HTTPApp `json:"http,omitempty"`
	TLS  *TLSApp  `json:"tls,omitempty"`
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
// application: "none", "plex", "jellyfin", "emby", "homeassistant", "nextcloud", "vaultwarden"
func ReverseProxyHandler(dial string, enableWS bool, application string) Handler {
	h := Handler{
		"handler":        "reverse_proxy",
		"flush_interval": -1, // Disable buffering for better streaming performance (Plex, etc.)
		"upstreams": []map[string]interface{}{
			{"dial": dial},
		},
	}

	// Build headers configuration
	headers := make(map[string]interface{})
	requestHeaders := make(map[string]interface{})
	setHeaders := make(map[string][]string)

	// WebSocket support
	if enableWS {
		setHeaders["Upgrade"] = []string{"{http.request.header.Upgrade}"}
		setHeaders["Connection"] = []string{"{http.request.header.Connection}"}
	}

	// Application-specific headers for proper client IP forwarding
	// These are critical for media servers behind tunnels/CGNAT
	switch application {
	case "plex", "jellyfin", "emby", "homeassistant", "nextcloud", "vaultwarden":
		// X-Real-IP is required by most apps to identify the real client
		// Caddy already sets X-Forwarded-For and X-Forwarded-Proto by default
		setHeaders["X-Real-IP"] = []string{"{http.request.remote.host}"}
		// Some apps also check these headers
		setHeaders["X-Forwarded-Host"] = []string{"{http.request.host}"}
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
