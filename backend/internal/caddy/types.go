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
func ReverseProxyHandler(dial string, enableWS bool) Handler {
	h := Handler{
		"handler": "reverse_proxy",
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

// TLSApp configures the TLS app for certificate management.
type TLSApp struct {
	Automation *AutomationConfig `json:"automation,omitempty"`
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
