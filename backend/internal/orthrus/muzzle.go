package orthrus

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/Wikid82/charon/backend/internal/logger"
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
	"/containers/json": {},
	"/images/json":     {},
	"/info":            {},
	"/version":         {},
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
// are forwarded; all others receive 403 Forbidden.
func (m *Muzzle) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		logger.Log().WithField("method", r.Method).WithField("path", sanitizePath(r.URL.Path)).
			Warn("orthrus: muzzle blocked non-GET Docker request")
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	stripped := versionPrefixRe.ReplaceAllString(r.URL.Path, "")
	if _, ok := allowedDockerPaths[stripped]; ok {
		m.next.ServeHTTP(w, r)
		return
	}

	logger.Log().WithField("method", r.Method).WithField("path", sanitizePath(r.URL.Path)).
		Warn("orthrus: muzzle blocked disallowed Docker path")
	http.Error(w, "Forbidden", http.StatusForbidden)
}
