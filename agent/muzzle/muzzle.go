// Package muzzle implements an HTTP allowlist filter for Docker socket proxy traffic.
//
// Only a curated set of read-only Docker API endpoints may be proxied. All other
// methods and paths are rejected with 403 Forbidden. The filter fails closed: any
// request that does not explicitly match the allowlist is blocked.
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

// allowedPatterns enumerates the Docker API paths that agents may access.
// Matching uses path.Match after stripping query parameters; each pattern
// uses `*` to match any single path segment (never crosses a slash).
//
// Both versioned (/v*/...) and unversioned (/...) forms are listed because
// Docker clients such as Dockhand send unversioned requests (e.g. GET /containers/json)
// while the canonical Docker CLI sends versioned requests (e.g. GET /v1.47/containers/json).
var allowedPatterns = []string{
	"/_ping",    // no version prefix (Docker < 1.24 / direct health check)
	"/v*/_ping", // versioned ping for Docker client health checks

	// Container listing and inspection — unversioned (RC8 fix) + versioned
	"/containers/json",
	"/v*/containers/json",
	"/containers/*/json",
	"/v*/containers/*/json",
	"/containers/*/logs",
	"/v*/containers/*/logs",
	"/containers/*/stats",
	"/v*/containers/*/stats",
	"/containers/*/top",
	"/v*/containers/*/top",

	// Daemon info — unversioned + versioned
	"/info",
	"/v*/info",
	"/images/json",
	"/v*/images/json",
	"/version",
	"/v*/version",
	"/events",
	"/v*/events",

	// Volumes — unversioned + versioned
	"/volumes",
	"/v*/volumes",
	"/volumes/*",
	"/v*/volumes/*",

	// Networks — unversioned + versioned
	"/networks",
	"/v*/networks",
	"/networks/*",
	"/v*/networks/*",

	// System disk usage — unversioned + versioned
	"/system/df",
	"/v*/system/df",
}

// versionPrefixRe strips a leading Docker API version segment (e.g.
// "/v1.47") from a path before the imageDistributionPatterns prefix/suffix
// check below, so a single comparison covers both the unversioned and
// versioned forms of those two entries. It is used only for that check;
// every other entry in allowedPatterns keeps its literal "/v*/..." form
// matched via path.Match.
var versionPrefixRe = regexp.MustCompile(`^/v\d+\.\d+`)

// imageDistributionPatterns covers per-image inspect (/images/{name}/json)
// and the registry digest check (/distribution/{name}/json). These are
// matched by prefix/suffix string comparison rather than path.Match,
// because Go's path.Match "*" does not cross "/", and real-world Docker
// image references are frequently namespaced with multiple "/"-separated
// segments (e.g. "ghcr.io/org/repo", "lscr.io/linuxserver/prowlarr"). A
// path.Match pattern like "/images/*/json" only matches single-segment
// names (e.g. "nginx") and would silently reject the overwhelming majority
// of real-world images — the same bug already found and fixed once in
// backend/internal/orthrus/muzzle.go (commits 98a68b67 and b71cbd62), which
// this mirrors on the remote-agent side of the tunnel.
//
// Both entries are GET-only like every other allowlist entry: the method
// check in Allow runs unconditionally before this check, so POST/PUT/DELETE
// against either path are rejected regardless of this list. Neither permits
// a write/mutate operation — /images/create (image pull) is deliberately
// not added.
//
// The suffix check ("/json") stays narrow despite allowing multiple path
// segments in the middle: real Docker Engine API endpoints under
// /images/{name}/ that are NOT safe to expose end in a distinct final
// segment rather than literally "json" — /images/{name}/history,
// /images/{name}/get, and /images/{name}/changes all fail the "/json"
// suffix check and remain blocked.
var imageDistributionPatterns = []struct {
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
type Filter struct {
	writeEnabled bool
}

// New returns a new Muzzle filter. writeEnabled governs whether the optional
// write-endpoint allowlist is consulted for this connection; false (the
// default for any caller not yet passing the negotiated handshake value)
// preserves today's unconditional read-only behavior.
func New(writeEnabled bool) *Filter {
	return &Filter{writeEnabled: writeEnabled}
}

// Allow returns true if method+reqPath is on the allowlist.
// Only GET is permitted, except HEAD which is allowed on /_ping and /v*/_ping
// (Docker SDK connectivity check).
func (f *Filter) Allow(method, reqPath string) bool {
	// HEAD is permitted only for /_ping (Docker SDK connectivity check).
	if strings.EqualFold(method, http.MethodHead) {
		cleanPath := path.Clean(reqPath)
		for _, p := range []string{"/_ping", "/v*/_ping"} {
			if matched, _ := path.Match(p, cleanPath); matched {
				return true
			}
		}
		return false
	}

	if !strings.EqualFold(method, http.MethodGet) {
		return false
	}

	// path.Clean normalises redundant separators and removes trailing slashes.
	cleanPath := path.Clean(reqPath)

	for _, pattern := range allowedPatterns {
		matched, err := path.Match(pattern, cleanPath)
		if err == nil && matched {
			return true
		}
	}

	// Check image/distribution inspect paths, which may contain namespaced
	// references with multiple "/"-separated segments (see doc comment on
	// imageDistributionPatterns for why path.Match is not used here).
	unversioned := versionPrefixRe.ReplaceAllString(cleanPath, "")
	for _, idp := range imageDistributionPatterns {
		if strings.HasPrefix(unversioned, idp.prefix) && strings.HasSuffix(unversioned, idp.suffix) {
			return true
		}
	}

	return false
}

// ServeProxy reads the HTTP request from r, checks Allow(), then dials the Docker
// unix socket at dst and proxies the full HTTP transaction. If the request is
// blocked, a 403 response is written to w and an error is returned.
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
	defer conn.Close()

	// Ensure Docker closes the socket after the response so ServeProxy can
	// terminate cleanly instead of waiting on an idle keep-alive connection.
	req.Close = true

	// Forward the full request (headers + body) to the Docker socket.
	if err := req.Write(conn); err != nil {
		return fmt.Errorf("muzzle: forward request to docker: %w", err)
	}

	// Stream the response back to the caller.
	_, err = io.Copy(w, conn)
	return err
}
