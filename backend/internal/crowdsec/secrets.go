package crowdsec

import (
	"os"
	"path/filepath"
	"strings"
)

// secretNames are the lowercase file names of stored account and connection details. They belong to
// the live server: the editor hides them, exports omit them and imports never replace them.
var secretNames = map[string]struct{}{
	"bouncer_key":                 {},
	"local_api_credentials.yaml":  {},
	"online_api_credentials.yaml": {},
}

// IsSecretPath reports whether rel (a slash- or OS-separated path relative to DataDir) names a stored
// secret. Matching is case-insensitive and exact (near-miss names such as "bouncer_key.bak" are not
// secrets). Any path component may match, so entries beneath a directory named like a secret are
// covered as well.
func IsSecretPath(rel string) bool {
	slash := strings.ToLower(filepath.ToSlash(filepath.Clean(rel)))
	for _, part := range strings.Split(slash, "/") {
		if _, ok := secretNames[part]; ok {
			return true
		}
	}
	return false
}

// IsPreservedPath reports whether rel is server-local state that imports must neither remove, replace
// nor take from an upload: engine-owned state and stored secrets.
func IsPreservedPath(rel string) bool {
	return IsEngineOwnedPath(rel) || IsSecretPath(rel)
}

// FindConfigFile returns the CrowdSec config file below dataDir: config/config.yaml when it exists,
// else config.yaml at the root, else "".
func FindConfigFile(dataDir string) string {
	for _, p := range []string{
		filepath.Join(dataDir, "config", "config.yaml"),
		filepath.Join(dataDir, "config.yaml"),
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
