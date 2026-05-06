package cloudflare

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

// CloudflaredConfig is the top-level structure for a cloudflared YAML configuration file.
type CloudflaredConfig struct {
	Tunnel          string        `yaml:"tunnel"`
	CredentialsFile string        `yaml:"credentials-file"`
	Ingress         []IngressRule `yaml:"ingress"`
}

// IngressRule maps a hostname to a backend service in the cloudflared ingress configuration.
type IngressRule struct {
	Hostname string `yaml:"hostname,omitempty"`
	Service  string `yaml:"service"`
}

// GenerateCloudflaredConfig produces a cloudflared YAML configuration string.
// A catch-all rule (http_status:404) is always appended as the final ingress entry.
func GenerateCloudflaredConfig(tunnelID, credentialsFile string, rules []IngressRule) (string, error) {
	if tunnelID == "" {
		return "", fmt.Errorf("cloudflare: tunnelID is required")
	}

	combined := make([]IngressRule, len(rules)+1)
	copy(combined, rules)
	combined[len(rules)] = IngressRule{Service: "http_status:404"}

	cfg := CloudflaredConfig{
		Tunnel:          tunnelID,
		CredentialsFile: credentialsFile,
		Ingress:         combined,
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(cfg); err != nil {
		return "", fmt.Errorf("cloudflare: marshal config: %w", err)
	}
	return buf.String(), nil
}
