// Package muzzle implements an HTTP allowlist filter for Docker socket proxy traffic.
//
// Read-only connections are restricted to a curated set of read-only Docker
// API endpoints; all other methods and paths are rejected with 403
// Forbidden. Write-enabled connections (an explicit, per-agent operator
// opt-in — see Filter's doc comment) get the full, unrestricted Docker
// Engine API instead: every request is forwarded as-is, with no
// per-endpoint or per-body restriction.
package muzzle

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"path"
	"regexp"
	"strings"
)

const forbiddenResponse = "HTTP/1.1 403 Forbidden\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"

// versionPrefixRe strips a leading Docker API version segment (e.g.
// "/v1.47") from a path. Used by normalizeDockerPath, which every allowlist
// check below is matched against — so a single normalization pass covers
// both the unversioned form (e.g. Dockhand sends GET /containers/json) and
// the versioned form (e.g. the canonical Docker CLI sends GET
// /v1.47/containers/json) with one set of unversioned-only allowlist
// entries, instead of duplicating every entry into an unversioned and a
// "/v*/..." form.
//
// Must stay byte-identical to backend/internal/orthrus/muzzle.go's
// versionPrefixRe declaration — scripts/ci/check_muzzle_allowlist_parity.go
// structurally compares the two regex source strings.
var versionPrefixRe = regexp.MustCompile(`^/v\d+\.\d+`)

// normalizeDockerPath strips a Docker API version prefix (e.g. "/v1.47")
// using versionPrefixRe, THEN runs path.Clean — matching
// backend/internal/orthrus/muzzle.go's ServeHTTP order exactly (see that
// file's identically-named helper), so both filters reach the same
// allow/deny decision on the same raw input in isolation, not only when
// backend happens to run first in the real request pipeline (GH #1160).
//
// Stripping first, before path.Clean has resolved any ".." segments, means
// versionPrefixRe is evaluated against the path exactly as received: a
// traversal-disguised version prefix (e.g. "/foo/../v1.44/images/x/json")
// is not mistaken for a real one, since the regex is anchored to the start
// of the raw string, not the cleaned one.
func normalizeDockerPath(reqPath string) string {
	stripped := versionPrefixRe.ReplaceAllString(reqPath, "")
	return path.Clean("/" + strings.TrimLeft(stripped, "/"))
}

// allowedDockerPaths is the set of Docker API paths that are safe to expose
// to agents, matched by exact string equality against the
// normalizeDockerPath-normalized path. Mirrors
// backend/internal/orthrus/muzzle.go's allowedDockerPaths declaration 1:1 —
// scripts/ci/check_muzzle_allowlist_parity.go structurally compares the two.
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
// /containers/{id}/json, /volumes/{name}, and /networks/{id}. Matching uses
// path.Match against the normalizeDockerPath-normalized path. Mirrors
// backend/internal/orthrus/muzzle.go's allowedDockerPatterns declaration
// 1:1.
var allowedDockerPatterns = []string{
	"/containers/*/json",
	"/containers/*/logs",
	"/containers/*/stats",
	"/containers/*/top",
	"/volumes/*",
	"/networks/*",
}

// allowedDockerPrefixSuffixPatterns covers per-image inspect
// (/images/{name}/json) and the registry digest check
// (/distribution/{name}/json), matched against the
// normalizeDockerPath-normalized path. These are matched by prefix/suffix
// string comparison rather than path.Match, because Go's path.Match "*"
// does not cross "/", and real-world Docker image references are
// frequently namespaced with multiple "/"-separated segments (e.g.
// "ghcr.io/org/repo", "lscr.io/linuxserver/prowlarr"). A path.Match pattern
// like "/images/*/json" only matches single-segment names (e.g. "nginx")
// and would silently reject the overwhelming majority of real-world images
// — the same bug already found and fixed once in
// backend/internal/orthrus/muzzle.go (commits 98a68b67 and b71cbd62), which
// this mirrors on the remote-agent side of the tunnel. Mirrors
// backend/internal/orthrus/muzzle.go's allowedDockerPrefixSuffixPatterns
// declaration 1:1.
//
// Both entries are GET-only like every other read-only allowlist entry: for
// a non-write-enabled connection, the method check in Allow runs before
// this check, so POST/PUT/DELETE against either path are rejected. Neither
// permits a write/mutate operation on its own — write access to any path,
// including these, is governed entirely by Filter.writeEnabled instead.
//
// The suffix check ("/json") stays narrow despite allowing multiple path
// segments in the middle: real Docker Engine API endpoints under
// /images/{name}/ that are NOT safe to expose end in a distinct final
// segment rather than literally "json" — /images/{name}/history,
// /images/{name}/get, and /images/{name}/changes all fail the "/json"
// suffix check and remain blocked.
var allowedDockerPrefixSuffixPatterns = []struct {
	prefix string
	suffix string
}{
	{prefix: "/images/", suffix: "/json"},       // per-image inspect (RepoDigests) — read-only
	{prefix: "/distribution/", suffix: "/json"}, // registry digest check — read-only
}

// Filter is an HTTP allowlist filter for Docker socket proxy streams.
//
// Filter is connection-scoped, not process-scoped: a fresh Filter is
// constructed for each successful agent connection (see leash.go's connect
// function), carrying the writeEnabled value negotiated for that specific
// session via the X-Orthrus-Write-Enabled handshake header. This mirrors the
// backend's per-AgentSession Muzzle scoping exactly, so a mid-connection DB
// toggle on the operator's side cannot retroactively change an
// already-negotiated session — the change only takes effect on the agent's
// next reconnect.
//
// A write-enabled Filter is a deliberate operator trust decision, not
// something this package polices further: write mode is opt-in per agent,
// gated behind an explicit typed-confirmation UI step that documents this
// is equivalent to giving the connected tool full control of the Docker
// host. See backend/internal/orthrus/muzzle.go's Muzzle doc comment for the
// full rationale.
type Filter struct {
	writeEnabled bool
}

// New returns a new Muzzle filter. writeEnabled governs whether this
// connection gets full, unrestricted Docker Engine API access; false (the
// default for any caller not yet passing the negotiated handshake value)
// preserves the unconditional read-only allowlist behavior.
func New(writeEnabled bool) *Filter {
	return &Filter{writeEnabled: writeEnabled}
}

// Allow returns true if method+reqPath is permitted, after
// normalizeDockerPath has stripped any version prefix and cleaned the path.
//
// HEAD is permitted only for /_ping (Docker SDK connectivity check),
// unconditionally — mirrors backend/internal/orthrus/muzzle.go's ServeHTTP,
// which checks this before its write-mode branch too. Every other request
// is permitted unconditionally when f.writeEnabled is true (full Docker
// Engine API access, see Filter's doc comment); otherwise only GET requests
// to allowlisted read-only paths are permitted.
func (f *Filter) Allow(method, reqPath string) bool {
	normalizedPath := normalizeDockerPath(reqPath)

	if strings.EqualFold(method, http.MethodHead) && normalizedPath == "/_ping" {
		return true
	}

	if f.writeEnabled {
		return true
	}

	if !strings.EqualFold(method, http.MethodGet) {
		return false
	}

	if _, ok := allowedDockerPaths[normalizedPath]; ok {
		return true
	}

	for _, pattern := range allowedDockerPatterns {
		matched, err := path.Match(pattern, normalizedPath)
		if err == nil && matched {
			return true
		}
	}

	// Check image/distribution inspect paths, which may contain namespaced
	// references with multiple "/"-separated segments (see doc comment on
	// allowedDockerPrefixSuffixPatterns for why path.Match is not used
	// here).
	for _, ps := range allowedDockerPrefixSuffixPatterns {
		if strings.HasPrefix(normalizedPath, ps.prefix) && strings.HasSuffix(normalizedPath, ps.suffix) {
			return true
		}
	}

	return false
}

// ServeProxy reads the HTTP request from r, checks Allow(), then dials the Docker
// unix socket at dst and proxies the full HTTP transaction. If the request is
// blocked, a 403 response is written to w and an error is returned.
//
// The request body is never buffered into memory: http.ReadRequest only
// consumes the request line and headers from bufr, leaving req.Body as a
// lazy reader over the same underlying stream, which req.Write(conn) below
// forwards directly. This works for every request, not only
// /containers/create, since nothing in this package inspects body content
// anymore (see Filter's doc comment) — Allow only ever needs method+path.
func (f *Filter) ServeProxy(dst string, r io.Reader, w io.Writer) error {
	bufr := bufio.NewReader(r)

	req, err := http.ReadRequest(bufr)
	if err != nil {
		return fmt.Errorf("muzzle: read request: %w", err)
	}

	if !f.Allow(req.Method, req.URL.Path) {
		// Fail closed: write 403 and abort the stream.
		_, _ = io.WriteString(w, forbiddenResponse)
		return fmt.Errorf("muzzle: blocked %s %s", req.Method, req.URL.Path)
	}

	conn, err := net.Dial("unix", dst)
	if err != nil {
		return fmt.Errorf("muzzle: dial docker socket: %w", err)
	}
	defer func() { _ = conn.Close() }()

	// Ensure Docker closes the socket after the response so ServeProxy can
	// terminate cleanly instead of waiting on an idle keep-alive connection.
	req.Close = true

	// Forward the full request (headers + body) to the Docker socket.
	if writeErr := req.Write(conn); writeErr != nil {
		return fmt.Errorf("muzzle: forward request to docker: %w", writeErr)
	}

	// Stream the response back to the caller.
	_, err = io.Copy(w, conn)
	return err
}
