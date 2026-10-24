// Package bouncerkey holds the lookup rules for the CrowdSec bouncer API key
// shared by the proxy config, the HTTP handlers and the startup reconciliation.
//
// It is a leaf package (standard library only) so that every caller can import
// it without creating an import cycle.
package bouncerkey

import (
	"os"
	"strings"
)

// DefaultFile is the saved bouncer key location inside the container.
const DefaultFile = "/app/data/crowdsec/bouncer_key"

// envNames lists the environment variables that may carry the bouncer key,
// in lookup order (the first non-empty value wins).
var envNames = [...]string{
	"CHARON_SECURITY_CROWDSEC_API_KEY",
	"CROWDSEC_API_KEY",
	"CROWDSEC_BOUNCER_API_KEY",
	"CERBERUS_SECURITY_CROWDSEC_API_KEY",
	"CPM_SECURITY_CROWDSEC_API_KEY",
}

// EnvNames returns the key environment variable names in lookup order.
// The returned slice is a copy and may be modified by the caller.
func EnvNames() []string {
	return append([]string(nil), envNames[:]...)
}

// FromEnv returns the first non-empty key environment variable and its name.
// It returns empty strings when none is set.
func FromEnv() (value, name string) {
	for _, n := range envNames {
		if v := os.Getenv(n); v != "" {
			return v, n
		}
	}
	return "", ""
}

// ReadFile returns the trimmed content of the key file at path. Any read error
// (missing file, directory, permission) yields an empty string.
func ReadFile(path string) string {
	// #nosec G304 -- path is DefaultFile or a caller-supplied location chosen by the server, never user input
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
