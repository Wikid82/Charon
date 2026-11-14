package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// maxAuthKeyFileBytes caps the key file size. A real key is far smaller; the
// cap stops a wrong path (a large file) from being read into memory.
const maxAuthKeyFileBytes = 4 << 10

// authKeyFileEnv names the environment variable that points at the key file.
const authKeyFileEnv = "ORTHRUS_AUTH_KEY_FILE"

// readAuthKeyFile returns the agent key stored at path, trimmed of surrounding
// whitespace. The path must be absolute and must name a regular file no larger
// than maxAuthKeyFileBytes. Error messages name the path but never its content.
func readAuthKeyFile(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("orthrus: %s %s must be an absolute path", authKeyFileEnv, path)
	}
	clean := filepath.Clean(path)

	// The path is operator-supplied startup configuration, validated above, not request input.
	f, err := os.Open(clean) //nolint:gosec // G304: path is operator-supplied startup configuration (ORTHRUS_AUTH_KEY_FILE), validated absolute and cleaned above, not request input
	if err != nil {
		return "", fmt.Errorf("orthrus: open auth key file %s: %w", clean, err)
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("orthrus: stat auth key file %s: %w", clean, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("orthrus: auth key file %s is not a regular file", clean)
	}

	// Read at most one byte past the limit so an oversize file is detected even
	// if it grew after Stat.
	data, err := io.ReadAll(io.LimitReader(f, maxAuthKeyFileBytes+1))
	if err != nil {
		return "", fmt.Errorf("orthrus: read auth key file %s: %w", clean, err)
	}
	if len(data) > maxAuthKeyFileBytes {
		return "", fmt.Errorf("orthrus: auth key file %s is larger than %d bytes", clean, maxAuthKeyFileBytes)
	}

	key := strings.TrimSpace(string(data))
	if key == "" {
		return "", fmt.Errorf("orthrus: the auth key file %s is empty: the agent key is not set yet; paste the key shown in Charon into this file", clean)
	}
	return key, nil
}
