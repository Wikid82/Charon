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

// Filter is an HTTP allowlist filter for Docker socket proxy streams.
type Filter struct{}

// New returns a new Muzzle filter.
func New() *Filter {
	return &Filter{}
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

	// Forward the full request (headers + body) to the Docker socket.
	if err := req.Write(conn); err != nil {
		return fmt.Errorf("muzzle: forward request to docker: %w", err)
	}

	// Stream the response back to the caller.
	_, err = io.Copy(w, conn)
	return err
}
