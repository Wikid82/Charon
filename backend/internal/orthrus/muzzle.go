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
	"/system/df":       {},
}

// allowedDockerPatterns covers dynamic-segment paths such as
// /containers/{id}/json, /volumes/{name}, and /networks/{id}.
// Matching uses path.Match after the version prefix has been stripped.
var allowedDockerPatterns = []string{
	"/containers/*/json",
	"/containers/*/logs",
	"/containers/*/stats",
	"/containers/*/top",
	"/volumes/*",
	"/networks/*",
}

// allowedDockerPrefixSuffixPatterns covers /images/*/json and
// /distribution/*/json specifically. These are matched by prefix/suffix
// string comparison rather than path.Match because Go's path.Match "*"
// does not cross "/", and real-world Docker image references are
// frequently namespaced with multiple "/"-separated segments (e.g.
// "ghcr.io/org/repo", "lscr.io/linuxserver/prowlarr"). A path.Match-based
// pattern like "/images/*/json" only matches single-segment names (e.g.
// "nginx") and rejects the overwhelming majority of real images, so it is
// not used here.
//
// Both entries are GET-only like every other allowlist entry: the
// unconditional method check in ServeHTTP runs before any path match, so
// POST/PUT/DELETE to either path is rejected regardless of this list.
// Neither permits a write/mutate operation — /images/create (image pull)
// is deliberately not added.
//
// Traversal segments ("..") cannot be smuggled through the prefix/suffix
// check: ServeHTTP runs path.Clean on stripped before any allowlist check
// (see the comment there), so a path like "/images/../etc/json" is
// normalized to "/etc/json" — which fails the "/images/" prefix — before
// this matching ever runs.
//
// The suffix check ("/json") stays narrow despite allowing multiple path
// segments in the middle: real Docker Engine API endpoints under
// /images/{name}/ that are NOT safe to expose end in a distinct final
// segment rather than literally "json" — /images/{name}/history,
// /images/{name}/get, and /images/{name}/changes all fail the "/json"
// suffix check and remain 403. Only paths whose final segment is exactly
// "json" match, which corresponds to image/distribution inspect — the
// intended read-only operation.
//
// /distribution/*/json causes the remote Docker daemon to make its own
// outbound request to the registry host encoded in the image name —
// read-only here means "no local Docker mutation," not "no outbound
// network activity."
var allowedDockerPrefixSuffixPatterns = []struct {
	prefix string
	suffix string
}{
	{prefix: "/images/", suffix: "/json"},       // image inspect (RepoDigests) — read-only, used by update-checker tools
	{prefix: "/distribution/", suffix: "/json"}, // registry digest check — read-only, used by update-checker tools
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
	rawPath := versionPrefixRe.ReplaceAllString(r.URL.Path, "")
	// Normalize away any "." or ".." segments before any allowlist check so that
	// traversal-style paths such as /containers/../json cannot match patterns like
	// /containers/*/json. path.Clean always returns a rooted result when given a
	// rooted input; the explicit "/" prefix guards against an empty rawPath value.
	stripped := path.Clean("/" + strings.TrimLeft(rawPath, "/"))

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
	// stripped is already rooted and cleaned; use it directly.
	for _, pat := range allowedDockerPatterns {
		if matched, err := path.Match(pat, stripped); err == nil && matched {
			m.next.ServeHTTP(w, r)
			return
		}
	}

	// Check image/distribution inspect paths, which may contain namespaced
	// references with multiple "/"-separated segments (see doc comment on
	// allowedDockerPrefixSuffixPatterns for why path.Match is not used here).
	for _, ps := range allowedDockerPrefixSuffixPatterns {
		if strings.HasPrefix(stripped, ps.prefix) && strings.HasSuffix(stripped, ps.suffix) {
			m.next.ServeHTTP(w, r)
			return
		}
	}

	logger.Log().WithField("method", util.SanitizeForLog(r.Method)).WithField("path", sanitizePath(r.URL.Path)).
		Warn("orthrus: muzzle blocked disallowed Docker path")
	http.Error(w, "Forbidden", http.StatusForbidden)
}
