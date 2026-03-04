package caddy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
)

// Executor defines an interface for executing shell commands.
type Executor interface {
	Execute(name string, args ...string) ([]byte, error)
}

// DefaultExecutor implements Executor using os/exec.
type DefaultExecutor struct{}

func (e *DefaultExecutor) Execute(name string, args ...string) ([]byte, error) {
	// Set a reasonable timeout for Caddy commands (5 seconds should be plenty for fmt/adapt)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()

	// If context timed out, return a clear error message
	if ctx.Err() == context.DeadlineExceeded {
		return output, fmt.Errorf("command timed out after 5 seconds (caddy binary may be unavailable or misconfigured): %s %v", name, args)
	}

	return output, err
}

// CaddyConfig represents the root structure of Caddy's JSON config.
type CaddyConfig struct {
	Apps *CaddyApps `json:"apps,omitempty"`
}

// CaddyApps contains application-specific configurations.
type CaddyApps struct {
	HTTP *CaddyHTTP `json:"http,omitempty"`
}

// CaddyHTTP represents the HTTP app configuration.
type CaddyHTTP struct {
	Servers map[string]*CaddyServer `json:"servers,omitempty"`
}

// CaddyServer represents a single server configuration.
type CaddyServer struct {
	Listen                []string      `json:"listen,omitempty"`
	Routes                []*CaddyRoute `json:"routes,omitempty"`
	TLSConnectionPolicies any           `json:"tls_connection_policies,omitempty"`
}

// CaddyRoute represents a single route with matchers and handlers.
type CaddyRoute struct {
	Match  []*CaddyMatcher `json:"match,omitempty"`
	Handle []*CaddyHandler `json:"handle,omitempty"`
}

// CaddyMatcher represents route matching criteria.
type CaddyMatcher struct {
	Host []string `json:"host,omitempty"`
}

// CaddyHandler represents a handler in the route.
type CaddyHandler struct {
	Handler   string `json:"handler"`
	Upstreams any    `json:"upstreams,omitempty"`
	Headers   any    `json:"headers,omitempty"`
	Routes    any    `json:"routes,omitempty"` // For subroute handlers
}

// ParsedHost represents a single host detected during Caddyfile import.
type ParsedHost struct {
	DomainNames      string   `json:"domain_names"`
	ForwardScheme    string   `json:"forward_scheme"`
	ForwardHost      string   `json:"forward_host"`
	ForwardPort      int      `json:"forward_port"`
	SSLForced        bool     `json:"ssl_forced"`
	WebsocketSupport bool     `json:"websocket_support"`
	RawJSON          string   `json:"raw_json"` // Original Caddy JSON for this route
	Warnings         []string `json:"warnings"` // Unsupported features
}

// ImportResult contains parsed hosts and detected conflicts.
type ImportResult struct {
	Hosts     []ParsedHost `json:"hosts"`
	Conflicts []string     `json:"conflicts"`
	Errors    []string     `json:"errors"`
}

// Importer handles Caddyfile parsing and conversion to Charon models.
type Importer struct {
	caddyBinaryPath string
	executor        Executor
}

// NewImporter creates a new Caddyfile importer.
func NewImporter(binaryPath string) *Importer {
	if binaryPath == "" {
		binaryPath = "caddy" // Default to PATH
	}
	return &Importer{
		caddyBinaryPath: binaryPath,
		executor:        &DefaultExecutor{},
	}
}

// forceSplitFallback used in tests to exercise the fallback branch
var forceSplitFallback bool

// NormalizeCaddyfile formats single-line Caddyfiles using Caddy's native formatter.
// This ensures compatibility with Caddy's parser by delegating to `caddy fmt`.
//
// Why caddy fmt instead of regex:
// - Handles nested blocks, comments, imports correctly
// - Future-proof: automatically supports new Caddy syntax
// - Battle-tested by Caddy project itself
func (i *Importer) NormalizeCaddyfile(content string) (string, error) {
	// Write content to temp file (caddy fmt reads from file)
	tmpFile, err := os.CreateTemp("", "caddyfile-*.tmp")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	// Note: These OS-level temp file error paths (WriteString/Close failures)
	// require disk fault injection to test and are impractical to cover in unit tests.
	// They are defensive error handling for rare I/O failures.
	if _, writeErr := tmpFile.WriteString(content); writeErr != nil {
		return "", fmt.Errorf("failed to write temp file: %w", writeErr)
	}
	if closeErr := tmpFile.Close(); closeErr != nil {
		return "", fmt.Errorf("failed to close temp file: %w", closeErr)
	}

	// Run: caddy fmt --overwrite <tempfile>
	// Note: --overwrite modifies the file in-place
	output, err := i.executor.Execute(i.caddyBinaryPath, "fmt", "--overwrite", tmpFile.Name())
	if err != nil {
		return "", fmt.Errorf("caddy fmt failed: %w (output: %s)", err, string(output))
	}

	// Read formatted output
	formatted, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return "", fmt.Errorf("failed to read formatted file: %w", err)
	}

	return string(formatted), nil
}

// ParseCaddyfile reads a Caddyfile and converts it to Caddy JSON.
func (i *Importer) ParseCaddyfile(caddyfilePath string) ([]byte, error) {
	// Sanitize the incoming path to detect forbidden traversal sequences.
	clean := filepath.Clean(caddyfilePath)
	if clean == "" || clean == "." {
		return nil, fmt.Errorf("invalid caddyfile path")
	}
	if strings.Contains(clean, ".."+string(os.PathSeparator)) || strings.HasPrefix(clean, "..") {
		return nil, fmt.Errorf("invalid caddyfile path")
	}
	if _, err := os.Stat(clean); os.IsNotExist(err) {
		return nil, fmt.Errorf("caddyfile not found: %s", clean)
	}

	output, err := i.executor.Execute(i.caddyBinaryPath, "adapt", "--config", clean, "--adapter", "caddyfile")
	if err != nil {
		return nil, fmt.Errorf("caddy adapt failed: %w (output: %s)", err, string(output))
	}

	return output, nil
}

// extractHandlers recursively extracts handlers from a list, flattening subroutes.
func (i *Importer) extractHandlers(handles []*CaddyHandler) []*CaddyHandler {
	var result []*CaddyHandler

	for _, handler := range handles {
		// Regular handler, add it directly
		if handler.Handler != "subroute" {
			result = append(result, handler)
			continue
		}

		// It's a subroute; extract handlers from its first route
		routes, ok := handler.Routes.([]any)
		if !ok || len(routes) == 0 {
			continue
		}
		subroute, ok := routes[0].(map[string]any)
		if !ok {
			continue
		}
		subhandles, ok := subroute["handle"].([]any)
		if !ok {
			continue
		}
		// Convert the subhandles to CaddyHandler objects
		for _, sh := range subhandles {
			shMap, ok := sh.(map[string]any)
			if !ok {
				continue
			}
			subHandler := &CaddyHandler{}
			if handlerType, ok := shMap["handler"].(string); ok {
				subHandler.Handler = handlerType
			}
			if upstreams, ok := shMap["upstreams"]; ok {
				subHandler.Upstreams = upstreams
			}
			if headers, ok := shMap["headers"]; ok {
				subHandler.Headers = headers
			}
			result = append(result, subHandler)
		}
	}

	return result
}

// ExtractHosts parses Caddy JSON and extracts proxy host information.
func (i *Importer) ExtractHosts(caddyJSON []byte) (*ImportResult, error) {
	var config CaddyConfig
	if err := json.Unmarshal(caddyJSON, &config); err != nil {
		return nil, fmt.Errorf("parsing caddy json: %w", err)
	}

	result := &ImportResult{
		Hosts:     []ParsedHost{},
		Conflicts: []string{},
		Errors:    []string{},
	}

	if config.Apps == nil || config.Apps.HTTP == nil || config.Apps.HTTP.Servers == nil {
		return result, nil // Empty config
	}

	seenDomains := make(map[string]bool)

	for serverName, server := range config.Apps.HTTP.Servers {
		// Detect if this server uses SSL based on listen address or TLS policies
		serverUsesSSL := server.TLSConnectionPolicies != nil
		for _, listenAddr := range server.Listen {
			// Check if listening on :443 or any HTTPS port indicator
			if strings.Contains(listenAddr, ":443") || strings.HasSuffix(listenAddr, "443") {
				serverUsesSSL = true
				break
			}
		}

		for routeIdx, route := range server.Routes {
			for _, match := range route.Match {
				for _, hostMatcher := range match.Host {
					domain := hostMatcher

					// Check for duplicate domains (report domain names only)
					if seenDomains[domain] {
						result.Conflicts = append(result.Conflicts, domain)
						continue
					}
					seenDomains[domain] = true

					// Extract reverse proxy handler
					host := ParsedHost{
						DomainNames: domain,
						SSLForced:   strings.HasPrefix(domain, "https") || serverUsesSSL,
					}

					// Find reverse_proxy handler (may be nested in subroute)
					handlers := i.extractHandlers(route.Handle)

					for _, handler := range handlers {
						if handler.Handler == "reverse_proxy" {
							upstreams, _ := handler.Upstreams.([]any)
							if len(upstreams) > 0 {
								if upstream, ok := upstreams[0].(map[string]any); ok {
									dial, _ := upstream["dial"].(string)
									if dial != "" {
										hostStr, portStr, err := net.SplitHostPort(dial)
										if err == nil && !forceSplitFallback {
											host.ForwardHost = hostStr
											if _, err := fmt.Sscanf(portStr, "%d", &host.ForwardPort); err != nil {
												host.ForwardPort = 80
											}
										} else {
											// Fallback: assume dial is just the host or has some other format
											// Try to handle simple "host:port" manually if net.SplitHostPort failed for some reason
											// or assume it's just a host
											parts := strings.Split(dial, ":")
											if len(parts) == 2 {
												host.ForwardHost = parts[0]
												if _, err := fmt.Sscanf(parts[1], "%d", &host.ForwardPort); err != nil {
													host.ForwardPort = 80
												}
											} else {
												host.ForwardHost = dial
												host.ForwardPort = 80
											}
										}
									}
								}
							}

							// Check for websocket support
							if headers, ok := handler.Headers.(map[string]any); ok {
								if upgrade, ok := headers["Upgrade"].([]any); ok {
									for _, v := range upgrade {
										if v == "websocket" {
											host.WebsocketSupport = true
											break
										}
									}
								}
							}

							// Default scheme
							host.ForwardScheme = "http"
							if host.SSLForced {
								host.ForwardScheme = "https"
							}
						}

						// Detect unsupported features
						if handler.Handler == "rewrite" {
							host.Warnings = append(host.Warnings, "Rewrite rules not supported - manual configuration required")
						}
						if handler.Handler == "file_server" {
							host.Warnings = append(host.Warnings, "File server directives not supported")
						}
					}

					// Store raw JSON for this route
					routeJSON, _ := json.Marshal(map[string]any{
						"server": serverName,
						"route":  routeIdx,
						"data":   route,
					})
					host.RawJSON = string(routeJSON)

					result.Hosts = append(result.Hosts, host)
				}
			}
		}
	}

	return result, nil
}

// ImportFile performs complete import: parse Caddyfile and extract hosts.
func (i *Importer) ImportFile(caddyfilePath string) (*ImportResult, error) {
	caddyJSON, err := i.ParseCaddyfile(caddyfilePath)
	if err != nil {
		return nil, err
	}

	return i.ExtractHosts(caddyJSON)
}

// ConvertToProxyHosts converts parsed hosts to ProxyHost models.
func ConvertToProxyHosts(parsedHosts []ParsedHost) []models.ProxyHost {
	hosts := make([]models.ProxyHost, 0, len(parsedHosts))

	for _, parsed := range parsedHosts {
		if parsed.ForwardHost == "" || parsed.ForwardPort == 0 {
			continue // Skip invalid entries
		}

		hosts = append(hosts, models.ProxyHost{
			Name:             parsed.DomainNames, // Can be customized by user during review
			DomainNames:      parsed.DomainNames,
			ForwardScheme:    parsed.ForwardScheme,
			ForwardHost:      parsed.ForwardHost,
			ForwardPort:      parsed.ForwardPort,
			SSLForced:        parsed.SSLForced,
			WebsocketSupport: parsed.WebsocketSupport,
		})
	}

	return hosts
}

// ValidateCaddyBinary checks if the Caddy binary is available.
func (i *Importer) ValidateCaddyBinary() error {
	_, err := i.executor.Execute(i.caddyBinaryPath, "version")
	if err != nil {
		return errors.New("caddy binary not found or not executable")
	}
	return nil
}

// BackupCaddyfile creates a timestamped backup of the original Caddyfile.
func BackupCaddyfile(originalPath, backupDir string) (string, error) {
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		return "", fmt.Errorf("creating backup directory: %w", err)
	}

	timestamp := fmt.Sprintf("%d", os.Getpid()) // Simple timestamp placeholder
	// Ensure the backup path is contained within backupDir to prevent path traversal
	backupFile := fmt.Sprintf("Caddyfile.%s.backup", timestamp)
	// Create a safe join with backupDir
	backupPath := filepath.Join(backupDir, backupFile)

	// Validate the original path: avoid traversal elements pointing outside backupDir
	clean := filepath.Clean(originalPath)
	if clean == "" || clean == "." {
		return "", fmt.Errorf("invalid original path")
	}
	if strings.Contains(clean, ".."+string(os.PathSeparator)) || strings.HasPrefix(clean, "..") {
		return "", fmt.Errorf("invalid original path")
	}
	input, err := os.ReadFile(clean)
	if err != nil {
		return "", fmt.Errorf("reading original file: %w", err)
	}

	if err := os.WriteFile(backupPath, input, 0o600); err != nil {
		return "", fmt.Errorf("writing backup: %w", err)
	}

	return backupPath, nil
}
