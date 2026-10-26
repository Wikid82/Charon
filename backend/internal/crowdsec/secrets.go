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
// secret, including the bouncers/ directory. Matching is case-insensitive. A secret name matches exactly, or
// with a temporary-file suffix: "<name>.tmp" and "<name>.tmp-*", which the atomic key and file writers
// leave behind after a crash (see isSecretName). Near-miss names such as "bouncer_key.bak" or
// "bouncer_key.tmpx" are not secrets. Any path component may match, so entries beneath a directory named
// like a secret are covered as well.
func IsSecretPath(rel string) bool {
	slash := strings.ToLower(filepath.ToSlash(filepath.Clean(rel)))
	if isBouncersDir(slash) {
		return true
	}
	for _, part := range strings.Split(slash, "/") {
		if isSecretName(part) {
			return true
		}
	}
	return false
}

// isSecretName reports whether a lowercase path component is a secret name or one of its temporary files.
// A temporary file is "<name>.tmp" or "<name>.tmp-<suffix>" (the os.CreateTemp pattern of the writers).
func isSecretName(part string) bool {
	if _, ok := secretNames[part]; ok {
		return true
	}
	for name := range secretNames {
		rest, ok := strings.CutPrefix(part, name+".tmp")
		if ok && (rest == "" || strings.HasPrefix(rest, "-")) {
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

// isBouncersDir reports whether the lowercase slash path lies in the bouncers directory directly under
// the data dir or under config/. It holds key files tied to the live LAPI database.
func isBouncersDir(slash string) bool {
	first, rest, _ := strings.Cut(slash, "/")
	if first == "bouncers" {
		return true
	}
	second, _, _ := strings.Cut(rest, "/")
	return first == "config" && second == "bouncers"
}
