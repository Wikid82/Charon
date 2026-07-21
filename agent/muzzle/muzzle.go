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
var allowedDockerPrefixSuffixPatterns = []struct {
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
// when write mode is on, matched against the normalizeDockerPath-normalized
// path — the version prefix is stripped once, up front, so no separate
// "/v*/..." entry is needed here. Mirrors
// backend/internal/orthrus/muzzle.go's allowedWritePatterns declaration
// 1:1.
//
// /containers/*/rename takes the new container name via a "?name=" query
// parameter, not a request body, so — like start/stop/restart — it needs no
// body-validation function; Allow only ever sees normalizedPath, never the
// query string.
var allowedWritePatterns = []struct {
	method  string
	pattern string
}{
	{method: http.MethodPost, pattern: "/containers/*/start"},
	{method: http.MethodPost, pattern: "/containers/*/stop"},
	{method: http.MethodPost, pattern: "/containers/*/restart"},
	{method: http.MethodPost, pattern: "/containers/*/rename"},
	{method: http.MethodDelete, pattern: "/containers/*"},
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
// write endpoint) is on the allowlist, after normalizeDockerPath has
// stripped any version prefix and cleaned the path. Only GET is permitted
// for read traffic, except HEAD which is allowed on /_ping (Docker SDK
// connectivity check, matched against the normalized path so a versioned
// request like /v1.47/_ping is also accepted); POST/DELETE are additionally
// permitted for the fixed write-endpoint set when f.writeEnabled is true.
//
// body is only consulted for the one body-validated case (POST to
// /containers/create, versioned or not); every other allowlist branch
// ignores it entirely, so passing nil for read-only calls is safe.
func (f *Filter) Allow(method, reqPath string, body []byte) bool {
	normalizedPath := normalizeDockerPath(reqPath)

	// HEAD is permitted only for /_ping (Docker SDK connectivity check).
	if strings.EqualFold(method, http.MethodHead) {
		return normalizedPath == "/_ping"
	}

	if f.writeEnabled && (strings.EqualFold(method, http.MethodPost) || strings.EqualFold(method, http.MethodDelete)) {
		if f.allowWrite(method, normalizedPath, body) {
			return true
		}
		// Not on the write allowlist even with write mode on — fall through
		// to the same false every other disallowed request gets below.
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

// allowWrite checks method+normalizedPath against allowedWriteExactPaths
// and allowedWritePatterns, applying validateContainerCreateBody
// specifically for /containers/create. Only called when f.writeEnabled is
// true.
//
// normalizedPath must already be the result of normalizeDockerPath, applied
// once by the caller (Allow). Previously this method separately re-derived
// an "unversioned" value for its exact-path branch only, while its pattern
// branch matched the raw (non-version-stripped) cleanPath directly — a
// second, independent instance of the GH #1160 normalization-order
// divergence, reaching a write endpoint. Both branches now share the same
// pre-normalized path.
func (f *Filter) allowWrite(method, normalizedPath string, body []byte) bool {
	if strings.EqualFold(method, http.MethodPost) {
		if _, ok := allowedWriteExactPaths[normalizedPath]; ok {
			if normalizedPath == "/containers/create" {
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
		if matched, err := path.Match(p.pattern, normalizedPath); err == nil && matched {
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
