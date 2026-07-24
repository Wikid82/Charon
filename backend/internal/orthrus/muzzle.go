package orthrus

import (
	"encoding/json"
	"net/http"
	"path"
	"regexp"
	"strings"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/util"
	"golang.org/x/time/rate"
)

// AuditLogger is the narrow interface Muzzle depends on for write-path audit
// logging. Satisfied by *services.SecurityService at the call site in
// session.go, where the concrete type is available. Muzzle cannot import
// services directly: services/orthrus_service.go already imports
// "github.com/Wikid82/charon/backend/internal/orthrus", so an orthrus →
// services import would create a cycle.
type AuditLogger interface {
	LogAudit(a *models.SecurityAudit) error
}

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
// POST/PUT/DELETE to either path is rejected regardless of this list for a
// read-only (non-write-enabled) session. Neither permits a write/mutate
// operation on its own — /images/create (image pull) is deliberately not
// added here, since it's a write operation gated by write mode instead.
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

// writeAuditDetails marshals a small flat field set into the JSON string
// stored in a SecurityAudit's Details column. Marshal failure (unreachable
// for the string-only maps this is called with) degrades to an empty JSON
// object rather than losing the audit entry entirely.
func writeAuditDetails(fields map[string]string) string {
	b, err := json.Marshal(fields)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// logAudit is a nil-safe wrapper around m.auditLogger.LogAudit. auditLogger
// is nil for read-only sessions and in most tests — a no-op in that case,
// not an error, since read-only traffic was never audited before this
// feature and isn't in scope for it now (see Section 3.3.7 of the
// write-mode spec).
func (m *Muzzle) logAudit(action, details string) {
	if m.auditLogger == nil {
		return
	}
	// Actor identifies the agent, not a Charon user — there is no
	// authenticated operator in this request path, only a third-party tool
	// on the other end of the tunnel. agentUUID (not agent name) is used
	// here because it's what NewMuzzle actually receives; ResourceUUID
	// carries the same value for querying, and a UUID is a more stable
	// identifier for an audit trail than a renamable display name anyway.
	_ = m.auditLogger.LogAudit(&models.SecurityAudit{
		Actor:         "orthrus-agent:" + m.agentUUID,
		Action:        action,
		EventCategory: "orthrus_write",
		ResourceUUID:  m.agentUUID,
		Details:       details,
	})
}

func (m *Muzzle) auditAllowed(r *http.Request) {
	m.logAudit("orthrus_write_allowed", writeAuditDetails(map[string]string{
		"method": r.Method,
		"path":   sanitizePath(r.URL.Path),
	}))
}

func (m *Muzzle) auditRateLimited(r *http.Request) {
	m.logAudit("orthrus_write_rate_limited", writeAuditDetails(map[string]string{
		"method": r.Method,
		"path":   sanitizePath(r.URL.Path),
	}))
}

// Muzzle is an http.Handler wrapper around the Docker socket proxy.
// Read-only sessions (writeEnabled false) are restricted to a curated
// allowlist of read-only, non-destructive GET endpoints. Write-enabled
// sessions get the full, unrestricted Docker Engine API: every request is
// forwarded as-is (rate-limited and audited), with no per-endpoint or
// per-field restriction.
//
// This is a deliberate operator trust model, not an oversight: write mode
// is opt-in per agent (OrthrusAgent.WriteEnabled), gated behind an explicit
// typed-confirmation UI step that documents this is equivalent to giving
// the connected tool full control of the Docker host — see
// AgentWriteModeDialog on the frontend. Every write-mode request is still
// audited (auditAllowed/auditRateLimited), so an operator who enables write
// mode retains a full trace of what was done, even though nothing about
// the request content itself is inspected or restricted.
type Muzzle struct {
	next http.Handler
	// writeEnabled is fixed at construction time (one Muzzle per AgentSession,
	// per external-proxy start) — never re-read from the DB per-request. See
	// NewMuzzle doc comment for why.
	writeEnabled bool
	// writeLimiter bounds write-request throughput; nil unless writeEnabled.
	writeLimiter *rate.Limiter
	// auditLogger records every write attempt; nil is tolerated (no-op) so
	// tests and read-only sessions don't need one.
	auditLogger AuditLogger
	// agentUUID identifies the session this Muzzle guards, for audit entries.
	agentUUID string
}

// NewMuzzle wraps handler with the Docker socket proxy filter.
//
// writeEnabled, writeLimiter, auditLogger, and agentUUID govern write-mode
// behavior (see the Muzzle doc comment). writeEnabled is captured once, at
// construction time, and never re-checked against the database on a
// per-request basis — StartExternalProxy constructs a new Muzzle per
// AgentSession using the value negotiated at connect time, so toggling the
// DB flag only takes effect on the agent's next reconnect. Re-reading it
// per-request would both reintroduce a TOCTOU-like inconsistency with that
// reconnect-to-apply guarantee and add a DB round-trip to the hot proxy
// path.
func NewMuzzle(next http.Handler, writeEnabled bool, writeLimiter *rate.Limiter, auditLogger AuditLogger, agentUUID string) *Muzzle {
	return &Muzzle{
		next:         next,
		writeEnabled: writeEnabled,
		writeLimiter: writeLimiter,
		auditLogger:  auditLogger,
		agentUUID:    agentUUID,
	}
}

// normalizeDockerPath strips a Docker API version prefix (e.g. "/v1.47")
// using versionPrefixRe, THEN runs path.Clean — in that order. Stripping
// first means the version-prefix match runs against the raw, uncleaned
// path, so a traversal-disguised prefix (e.g. "/foo/../v1.44/...") is not
// mistaken for a real one: versionPrefixRe is anchored to the start of the
// string as given, before ".." segments have been resolved away. Normalize
// away any "." or ".." segments only after that check so that traversal-style
// paths such as /containers/../json cannot match patterns like
// /containers/*/json. path.Clean always returns a rooted result when given a
// rooted input; the explicit "/" prefix guards against an empty stripped
// value.
//
// agent/muzzle/muzzle.go has an identically-named helper that must apply
// versionPrefixRe (same regex source) and path.Clean in this exact order —
// see that file's doc comment and scripts/ci/check_muzzle_allowlist_parity.go,
// which structurally compares the two files' versionPrefixRe declarations.
func normalizeDockerPath(rawPath string) string {
	stripped := versionPrefixRe.ReplaceAllString(rawPath, "")
	return path.Clean("/" + strings.TrimLeft(stripped, "/"))
}

// ServeHTTP implements http.Handler.
//
// Read-only sessions (m.writeEnabled false) only forward GET requests to
// allowlisted paths, with HEAD permitted for /_ping (Docker client health
// checks) — every other request receives 403 Forbidden. This half is
// unchanged from before write mode existed.
//
// Write-enabled sessions forward every request unconditionally, through
// the rate limiter and audit log (see forwardWrite) — no endpoint or
// request-body restriction. See the Muzzle doc comment for why.
func (m *Muzzle) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	stripped := normalizeDockerPath(r.URL.Path)

	// HEAD /_ping is permitted alongside GET for Docker client health checks.
	if r.Method == http.MethodHead && stripped == "/_ping" {
		m.next.ServeHTTP(w, r)
		return
	}

	if m.writeEnabled {
		m.forwardWrite(w, r)
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

// forwardWrite applies the write-path rate limiter, audits the outcome, and
// forwards the request. Only called for write-enabled sessions, for every
// request regardless of method, path, or body.
func (m *Muzzle) forwardWrite(w http.ResponseWriter, r *http.Request) {
	if m.writeLimiter != nil && !m.writeLimiter.Allow() {
		m.auditRateLimited(r)
		http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
		return
	}
	m.auditAllowed(r)
	m.next.ServeHTTP(w, r)
}
