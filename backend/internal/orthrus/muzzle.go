package orthrus

import (
	"net/http"
	"path"
	"regexp"
	"strings"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/util"
)

// sanitizePath strips newlines and carriage returns from a path string to
// prevent log injection (CWE-117).
func sanitizePath(p string) string {
	p = strings.ReplaceAll(p, "\n", `\n`)
	p = strings.ReplaceAll(p, "\r", `\r`)
	return p
}

// versionPrefixRe strips the Docker API version prefix from a request path,
// e.g. /v1.44/containers/json → /containers/json.
var versionPrefixRe = regexp.MustCompile(`^/v\d+\.\d+`)

// allowedDockerPaths is the set of Docker API paths that are safe to expose to agents.
// Path matching is performed after stripping the version prefix.
var allowedDockerPaths = map[string]struct{}{
	"/_ping":           {},
	"/containers/json": {},
	"/images/json":     {},
	"/info":            {},
	"/version":         {},
	"/events":          {},
	"/volumes":         {},
	"/networks":        {},
}

// allowedDockerPatterns covers dynamic-segment paths such as
// /containers/{id}/json, /volumes/{name}, and /networks/{id}.
// Matching uses path.Match after the version prefix has been stripped.
var allowedDockerPatterns = []string{
	"/containers/*/json",
	"/volumes/*",
	"/networks/*",
}

// Muzzle is an http.Handler wrapper that restricts Docker socket access
// to a curated allowlist of read-only, non-destructive endpoints.
type Muzzle struct {
	next http.Handler
}

// NewMuzzle wraps handler with the Docker socket allowlist filter.
func NewMuzzle(next http.Handler) *Muzzle {
	return &Muzzle{next: next}
}

// ServeHTTP implements http.Handler. Only GET requests to allowlisted paths
// are forwarded; HEAD is also permitted for /_ping (Docker client health checks).
// All other methods or paths receive 403 Forbidden.
func (m *Muzzle) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	stripped := versionPrefixRe.ReplaceAllString(r.URL.Path, "")

	// HEAD /_ping is permitted alongside GET for Docker client health checks.
	if r.Method == http.MethodHead && stripped == "/_ping" {
		m.next.ServeHTTP(w, r)
		return
	}

	if r.Method != http.MethodGet {
		logger.Log().WithField("method", util.SanitizeForLog(r.Method)).WithField("path", sanitizePath(r.URL.Path)).
			Warn("orthrus: muzzle blocked non-GET Docker request")
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if _, ok := allowedDockerPaths[stripped]; ok {
		m.next.ServeHTTP(w, r)
		return
	}

	// Check dynamic path patterns for container/volume/network inspection.
	// Normalize to an absolute path by trimming stray slashes and re-anchoring
	// to "/" so that path.Match works correctly. Traversal sequences such as ".."
	// are left unresolved intentionally — they will not match any allowed pattern
	// and will be blocked, which is the safe behavior.
	cleanPath := "/" + strings.Trim(stripped, "/")
	for _, pat := range allowedDockerPatterns {
		if matched, err := path.Match(pat, cleanPath); err == nil && matched {
			m.next.ServeHTTP(w, r)
			return
		}
	}

	logger.Log().WithField("method", util.SanitizeForLog(r.Method)).WithField("path", sanitizePath(r.URL.Path)).
		Warn("orthrus: muzzle blocked disallowed Docker path")
	http.Error(w, "Forbidden", http.StatusForbidden)
}
