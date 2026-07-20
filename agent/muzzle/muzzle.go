// Package muzzle implements an HTTP allowlist filter for Docker socket proxy traffic.
//
// Only a curated set of read-only Docker API endpoints may be proxied. All other
// methods and paths are rejected with 403 Forbidden. The filter fails closed: any
// request that does not explicitly match the allowlist is blocked.
package muzzle

import (
	"bufio"
	"bytes"
	"encoding/json"
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

// allowedWriteExactPaths lists the fixed-path write endpoints permitted
// when a connection has negotiated write mode (Filter.writeEnabled), keyed
// on the version-stripped path. /containers/create is handled as its own
// special case in allowWrite (body-validated); /images/create takes its
// parameters via query string and has no body to validate. This MUST stay
// in exact policy agreement with allowedWriteExactPaths/allowedWritePatterns
// in backend/internal/orthrus/muzzle.go — the shared test corpus
// (backend/internal/orthrus/testdata/muzzle_corpus.json) exists specifically
// to catch drift between the two independently-maintained copies.
var allowedWriteExactPaths = map[string]struct{}{
	"/containers/create": {},
	"/images/create":     {},
}

// allowedWritePatterns lists the dynamic-segment write endpoints permitted
// when write mode is on, both unversioned and versioned forms, matching
// this file's existing convention for the read-only allowlist above.
var allowedWritePatterns = []struct {
	method  string
	pattern string
}{
	{http.MethodPost, "/containers/*/start"},
	{http.MethodPost, "/v*/containers/*/start"},
	{http.MethodPost, "/containers/*/stop"},
	{http.MethodPost, "/v*/containers/*/stop"},
	{http.MethodPost, "/containers/*/restart"},
	{http.MethodPost, "/v*/containers/*/restart"},
	{http.MethodDelete, "/containers/*"},
	{http.MethodDelete, "/v*/containers/*"},
}

// maxContainerCreateBodyBytes bounds the size of a /containers/create body
// accepted for validation. Must stay numerically identical to the backend's
// copy (backend/internal/orthrus/muzzle.go) — the shared test corpus
// includes a boundary-size case to catch drift.
const maxContainerCreateBodyBytes = 64 * 1024

// hostConfigAllowedKeys, mountEntry, and the two value-level validators
// below are an exact policy mirror of backend/internal/orthrus/muzzle.go's
// copies. See that file's doc comments for the full rationale (allowlist
// over denylist for fail-closed behavior against unknown future fields; the
// VolumeOptions.DriverConfig check closing the local-driver
// bind-mount-via-volume bypass). Duplicated here, not shared via import,
// because agent/ is a separate Go module built as a minimal standalone
// binary and does not import backend/ packages.
var hostConfigAllowedKeys = map[string]struct{}{
	"PortBindings":   {},
	"RestartPolicy":  {},
	"Memory":         {},
	"MemorySwap":     {},
	"NanoCpus":       {},
	"CpuShares":      {},
	"Mounts":         {},
	"Dns":            {},
	"DnsSearch":      {},
	"ExtraHosts":     {},
	"LogConfig":      {},
	"AutoRemove":     {},
	"ReadonlyRootfs": {},
	"Init":           {},
	"NetworkMode":    {},
}

type mountEntry struct {
	Type          string          `json:"Type"`
	VolumeOptions json.RawMessage `json:"VolumeOptions"`
}

func validateNetworkModeValue(raw json.RawMessage) bool {
	var mode string
	if err := json.Unmarshal(raw, &mode); err != nil {
		return false
	}
	if mode == "host" {
		return false
	}
	if strings.HasPrefix(mode, "container:") {
		return false
	}
	return true
}

func validateMountsValue(raw json.RawMessage) bool {
	var mounts []mountEntry
	if err := json.Unmarshal(raw, &mounts); err != nil {
		return false
	}
	for _, mnt := range mounts {
		if mnt.Type != "volume" && mnt.Type != "tmpfs" {
			return false
		}
		if len(mnt.VolumeOptions) == 0 {
			continue
		}
		var volumeOptions map[string]json.RawMessage
		if err := json.Unmarshal(mnt.VolumeOptions, &volumeOptions); err != nil {
			return false
		}
		if _, hasDriverConfig := volumeOptions["DriverConfig"]; hasDriverConfig {
			return false
		}
	}
	return true
}

// validateContainerCreateBody validates a /containers/create request body
// against hostConfigAllowedKeys plus the NetworkMode/Mounts value-level
// checks. Unlike the backend's copy, this function does not re-buffer
// req.Body itself — ServeProxy already reads the full body into bodyBytes
// once, up front, for every request (see the doc comment there), so there
// is nothing left to re-buffer by the time this function runs.
func validateContainerCreateBody(bodyBytes []byte) (ok bool, reason string) {
	if len(bodyBytes) == 0 {
		return true, ""
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(bodyBytes, &top); err != nil {
		return false, "malformed request body"
	}

	hostConfigRaw, hasHostConfig := top["HostConfig"]
	if !hasHostConfig {
		return true, ""
	}

	var hostConfig map[string]json.RawMessage
	if err := json.Unmarshal(hostConfigRaw, &hostConfig); err != nil {
		return false, "malformed request body"
	}

	for key, rawValue := range hostConfig {
		if _, allowed := hostConfigAllowedKeys[key]; !allowed {
			return false, "disallowed HostConfig field: " + key
		}
		switch key {
		case "NetworkMode":
			if !validateNetworkModeValue(rawValue) {
				return false, "disallowed HostConfig field: NetworkMode"
			}
		case "Mounts":
			if !validateMountsValue(rawValue) {
				return false, "disallowed HostConfig field: Mounts"
			}
		}
	}

	return true, ""
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

// Allow returns true if method+reqPath (+body, for the one body-validated
// write endpoint) is on the allowlist. Only GET is permitted for read
// traffic, except HEAD which is allowed on /_ping and /v*/_ping (Docker SDK
// connectivity check); POST/DELETE are additionally permitted for the fixed
// write-endpoint set when f.writeEnabled is true.
//
// body is only consulted for the one body-validated case (POST to
// /containers/create or /v*/containers/create); every other allowlist
// branch ignores it entirely, so passing nil for read-only calls is safe.
func (f *Filter) Allow(method, reqPath string, body []byte) bool {
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

	cleanPath := path.Clean(reqPath)

	if f.writeEnabled && (strings.EqualFold(method, http.MethodPost) || strings.EqualFold(method, http.MethodDelete)) {
		if f.allowWrite(method, cleanPath, body) {
			return true
		}
		// Not on the write allowlist even with write mode on — fall through
		// to the same false every other disallowed request gets below.
	}

	if !strings.EqualFold(method, http.MethodGet) {
		return false
	}

	// path.Clean normalises redundant separators and removes trailing slashes.
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

// allowWrite checks method+cleanPath against allowedWriteExactPaths and
// allowedWritePatterns, applying validateContainerCreateBody specifically
// for /containers/create. Only called when f.writeEnabled is true.
func (f *Filter) allowWrite(method, cleanPath string, body []byte) bool {
	if strings.EqualFold(method, http.MethodPost) {
		unversioned := versionPrefixRe.ReplaceAllString(cleanPath, "")
		if _, ok := allowedWriteExactPaths[unversioned]; ok {
			if unversioned == "/containers/create" {
				valid, _ := validateContainerCreateBody(body)
				return valid
			}
			return true
		}
	}

	for _, p := range allowedWritePatterns {
		if !strings.EqualFold(method, p.method) {
			continue
		}
		if matched, err := path.Match(p.pattern, cleanPath); err == nil && matched {
			return true
		}
	}
	return false
}

// ServeProxy reads the HTTP request from r, checks Allow(), then dials the Docker
// unix socket at dst and proxies the full HTTP transaction. If the request is
// blocked, a 403 response is written to w and an error is returned.
//
// Every request's body is read and re-buffered unconditionally, not only
// for /containers/create: reading req.Body to obtain bytes for Allow
// consumes the underlying io.ReadCloser exactly once, and req.Write(conn)
// below would forward an empty/already-drained body unless the bytes are
// wrapped back into a fresh reader first. GET requests present req.Body as
// nil or an already-empty, immediately-io.EOF reader (standard net/http
// behavior for bodyless requests), so this is a cheap no-op for the
// overwhelming majority of traffic — keeping this step unconditional avoids
// duplicating the "is this /containers/create" check that already has to
// live inside Allow/allowWrite.
func (f *Filter) ServeProxy(dst string, r io.Reader, w io.Writer) error {
	bufr := bufio.NewReader(r)

	req, err := http.ReadRequest(bufr)
	if err != nil {
		return fmt.Errorf("muzzle: read request: %w", err)
	}

	var bodyBytes []byte
	if req.Body != nil {
		limited := io.LimitReader(req.Body, maxContainerCreateBodyBytes+1)
		bodyBytes, err = io.ReadAll(limited)
		_ = req.Body.Close()
		if err != nil {
			return fmt.Errorf("muzzle: read body: %w", err)
		}
		if len(bodyBytes) > maxContainerCreateBodyBytes {
			_, _ = io.WriteString(w, forbiddenResponse)
			return fmt.Errorf("muzzle: blocked %s %s: body too large", req.Method, req.URL.Path)
		}
		// Re-buffer: req.Write below must still forward the original body intact.
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		req.ContentLength = int64(len(bodyBytes))
	}

	if !f.Allow(req.Method, req.URL.Path, bodyBytes) {
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
