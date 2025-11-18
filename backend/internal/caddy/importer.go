package caddy

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
)

// ParsedHost represents a single host detected during Caddyfile import.
type ParsedHost struct {
	Domain       string   `json:"domain"`
	TargetScheme string   `json:"target_scheme"`
	TargetHost   string   `json:"target_host"`
	TargetPort   int      `json:"target_port"`
	EnableTLS    bool     `json:"enable_tls"`
	EnableWS     bool     `json:"enable_websockets"`
	RawJSON      string   `json:"raw_json"` // Original Caddy JSON for this route
	Warnings     []string `json:"warnings"` // Unsupported features
}

// ImportResult contains parsed hosts and detected conflicts.
type ImportResult struct {
	Hosts     []ParsedHost `json:"hosts"`
	Conflicts []string     `json:"conflicts"`
	Errors    []string     `json:"errors"`
}

// Importer handles Caddyfile parsing and conversion to CPM+ models.
type Importer struct {
	caddyBinaryPath string
}

// NewImporter creates a new Caddyfile importer.
func NewImporter(binaryPath string) *Importer {
	if binaryPath == "" {
		binaryPath = "caddy" // Default to PATH
	}
	return &Importer{caddyBinaryPath: binaryPath}
}

// ParseCaddyfile reads a Caddyfile and converts it to Caddy JSON.
func (i *Importer) ParseCaddyfile(caddyfilePath string) ([]byte, error) {
	if _, err := os.Stat(caddyfilePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("caddyfile not found: %s", caddyfilePath)
	}

	cmd := exec.Command(i.caddyBinaryPath, "adapt", "--config", caddyfilePath, "--adapter", "caddyfile")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("caddy adapt failed: %w (output: %s)", err, string(output))
	}

	return output, nil
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
		for routeIdx, route := range server.Routes {
			for _, match := range route.Match {
				for _, hostMatcher := range match.Host {
					domain := hostMatcher

					// Check for duplicate domains
					if seenDomains[domain] {
						result.Conflicts = append(result.Conflicts,
							fmt.Sprintf("Duplicate domain detected: %s", domain))
						continue
					}
					seenDomains[domain] = true

					// Extract reverse proxy handler
					host := ParsedHost{
						Domain:    domain,
						EnableTLS: strings.HasPrefix(domain, "https") || server.TLSConnectionPolicies != nil,
					}

					// Find reverse_proxy handler
					for _, handler := range route.Handle {
						if handler.Handler == "reverse_proxy" {
							upstreams, _ := handler.Upstreams.([]interface{})
							if len(upstreams) > 0 {
								if upstream, ok := upstreams[0].(map[string]interface{}); ok {
									dial, _ := upstream["dial"].(string)
									if dial != "" {
										parts := strings.Split(dial, ":")
										if len(parts) == 2 {
											host.TargetHost = parts[0]
											fmt.Sscanf(parts[1], "%d", &host.TargetPort)
										}
									}
								}
							}

							// Check for websocket support
							if headers, ok := handler.Headers.(map[string]interface{}); ok {
								if upgrade, ok := headers["Upgrade"].([]interface{}); ok {
									for _, v := range upgrade {
										if v == "websocket" {
											host.EnableWS = true
											break
										}
									}
								}
							}

							// Default scheme
							host.TargetScheme = "http"
							if host.EnableTLS {
								host.TargetScheme = "https"
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
					routeJSON, _ := json.Marshal(map[string]interface{}{
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
		if parsed.TargetHost == "" || parsed.TargetPort == 0 {
			continue // Skip invalid entries
		}

		hosts = append(hosts, models.ProxyHost{
			Name:         parsed.Domain, // Can be customized by user during review
			Domain:       parsed.Domain,
			TargetScheme: parsed.TargetScheme,
			TargetHost:   parsed.TargetHost,
			TargetPort:   parsed.TargetPort,
			EnableTLS:    parsed.EnableTLS,
			EnableWS:     parsed.EnableWS,
		})
	}

	return hosts
}

// ValidateCaddyBinary checks if the Caddy binary is available.
func (i *Importer) ValidateCaddyBinary() error {
	cmd := exec.Command(i.caddyBinaryPath, "version")
	if err := cmd.Run(); err != nil {
		return errors.New("caddy binary not found or not executable")
	}
	return nil
}

// BackupCaddyfile creates a timestamped backup of the original Caddyfile.
func BackupCaddyfile(originalPath, backupDir string) (string, error) {
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("creating backup directory: %w", err)
	}

	timestamp := fmt.Sprintf("%d", os.Getpid()) // Simple timestamp placeholder
	backupPath := filepath.Join(backupDir, fmt.Sprintf("Caddyfile.%s.backup", timestamp))

	input, err := os.ReadFile(originalPath)
	if err != nil {
		return "", fmt.Errorf("reading original file: %w", err)
	}

	if err := os.WriteFile(backupPath, input, 0644); err != nil {
		return "", fmt.Errorf("writing backup: %w", err)
	}

	return backupPath, nil
}
