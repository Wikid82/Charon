package muzzle_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/agent/muzzle"
)

var errWriterClosed = errors.New("writer closed")

type closableWriter struct {
	mu         sync.Mutex
	buf        bytes.Buffer
	closed     bool
	firstWrite chan struct{}
	once       sync.Once
}

func newClosableWriter() *closableWriter {
	return &closableWriter{firstWrite: make(chan struct{})}
}

func (w *closableWriter) Write(p []byte) (int, error) {
	w.once.Do(func() { close(w.firstWrite) })

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return 0, errWriterClosed
	}
	return w.buf.Write(p)
}

func (w *closableWriter) Close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
}

func (w *closableWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

func startUnixHTTPServer(t *testing.T, handler func(net.Conn)) (string, func()) {
	t.Helper()

	sockPath := filepath.Join(t.TempDir(), "docker.sock")
	ln, err := net.Listen("unix", sockPath)
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			return
		}
		handler(conn)
	}()

	cleanup := func() {
		_ = ln.Close()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("unix server goroutine did not exit")
		}
	}

	return sockPath, cleanup
}

func TestFilter_Allow(t *testing.T) {
	f := muzzle.New(false)

	tests := []struct {
		method  string
		reqPath string
		allowed bool
	}{
		// --- Allowed: health check ---
		{"GET", "/_ping", true},
		{"GET", "/v1.44/_ping", true},
		{"HEAD", "/_ping", true},
		{"HEAD", "/v1.47/_ping", true},
		{"HEAD", "/containers/json", false},       // HEAD blocked for non-ping paths
		{"HEAD", "/v1.47/containers/json", false}, // HEAD blocked for non-ping paths

		// --- Allowed: GET on versioned paths ---
		{"GET", "/v1.47/containers/json", true},
		{"GET", "/v1.24/containers/json", true},
		{"GET", "/v1.47/containers/abc123/json", true},
		{"GET", "/v1.47/containers/abc123/logs", true},
		{"GET", "/v1.47/containers/abc123/stats", true},
		{"GET", "/v1.47/containers/abc123/top", true},
		{"GET", "/v1.47/info", true},
		{"GET", "/v1.47/images/json", true},
		{"GET", "/v1.47/version", true},
		{"GET", "/v1.47/events", true},
		{"GET", "/v1.47/volumes", true},
		{"GET", "/v1.47/volumes/myvolume", true},
		{"GET", "/v1.47/networks", true},
		{"GET", "/v1.47/networks/mynet", true},
		{"GET", "/v1.47/system/df", true},
		{"GET", "/v1.41/volumes/myvol", true},
		{"GET", "/v1.41/networks/mynet", true},

		// --- Allowed: GET on UNVERSIONED paths (RC8/RC9 fix) ---
		{"GET", "/containers/json", true},
		{"GET", "/containers/abc123/json", true},
		{"GET", "/containers/abc123/logs", true},
		{"GET", "/containers/abc123/stats", true},
		{"GET", "/containers/abc123/top", true},
		{"GET", "/info", true},
		{"GET", "/images/json", true},
		{"GET", "/version", true},
		{"GET", "/events", true},
		{"GET", "/volumes", true},
		{"GET", "/volumes/myvolume", true},
		{"GET", "/networks", true},
		{"GET", "/networks/mynet", true},
		{"GET", "/system/df", true},

		// --- Blocked: mutating methods ---
		{"POST", "/containers/create", false},
		{"DELETE", "/containers/abc123", false},
		{"PUT", "/containers/abc123/start", false},
		{"PATCH", "/v1.47/containers/abc123", false},
		{"POST", "/v1.41/containers/create", false},
		{"DELETE", "/v1.41/containers/abc", false},
		{"PUT", "/v1.41/networks/abc", false},
		{"PATCH", "/v1.41/containers/abc/update", false},

		// --- Blocked: paths not on allowlist ---
		{"GET", "/containers/abc123/start", false},
		{"GET", "/containers/abc123/stop", false},
		{"GET", "/exec/abc123/start", false},
		{"GET", "/build", false},
		{"GET", "/v1.47/exec/abc123", false},
		{"GET", "/v1.41/exec/abc/start", false},
		{"GET", "/v1.41/containers/prune", false},

		// --- Path traversal: path.Clean normalises before matching ---
		{"GET", "/v1.47/../containers/json", true},     // resolves to /containers/json — allowed
		{"GET", "/containers/../../etc/passwd", false}, // resolves to /etc/passwd — blocked
		{"GET", "/v1.44//containers/json", true},       // double slash after a legitimate version prefix

		// --- GH #1160 regression: normalizeDockerPath strips the version
		// prefix BEFORE path.Clean runs, so a version prefix only revealed by
		// resolving ".." segments must NOT be treated as a version prefix
		// (divergence row 1) — the un-normalized "/foo/../v1.44/..." is not
		// on the allowlist even after Clean resolves it to
		// "/v1.44/images/x/json", since versionPrefixRe already ran against
		// the raw, traversal-disguised path and found no match there.
		{"GET", "/foo/../v1.44/images/x/json", false},

		// --- GH #1160 regression: versionPrefixRe requires a numeric
		// major.minor segment (^/v\d+\.\d+); a "v"-prefixed segment that
		// isn't numeric must not be accepted as a version prefix (divergence
		// row 2, read path; row 2 HEAD variant) ---
		{"GET", "/vFOO/containers/json", false},
		{"HEAD", "/vBOGUS/_ping", false},

		// --- Allowed: single-segment image/distribution inspect (prefix/suffix match) ---
		{"GET", "/images/alpine/json", true},
		{"GET", "/v1.44/images/alpine/json", true},
		{"GET", "/distribution/alpine/json", true},
		{"GET", "/v1.44/distribution/alpine/json", true},

		// --- Blocked: non-"/json"-suffixed image/distribution endpoints stay
		// blocked; the prefix/suffix match is intentionally narrow.
		{"GET", "/images/create", false},
		{"GET", "/images/ghcr.io/org/repo/history", false},
		{"GET", "/images/ghcr.io/org/repo/get", false},
		{"GET", "/images/ghcr.io/org/repo/changes", false},
		{"GET", "/distribution/create", false},

		// --- Blocked: non-GET on image/distribution inspect paths, including
		// namespaced ones, confirms the unconditional GET-only check still
		// runs before the new prefix/suffix match.
		{"POST", "/images/ghcr.io/org/repo/json", false},
		{"POST", "/distribution/ghcr.io/org/repo/json", false},
		{"DELETE", "/v1.44/images/ghcr.io/org/repo/json", false},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.reqPath, func(t *testing.T) {
			got := f.Allow(tt.method, tt.reqPath, nil)
			assert.Equal(t, tt.allowed, got)
		})
	}
}

// TestFilter_Allow_NamespacedImagePaths proves the fix for the same
// namespaced-image-reference bug already found and fixed once in
// backend/internal/orthrus/muzzle.go (commits 98a68b67 and b71cbd62):
// /images/{name}/json and /distribution/{name}/json must match resource
// identifiers containing multiple "/"-separated segments (the overwhelming
// majority of real-world image references), not just single-segment names
// like "nginx". Uses prefix/suffix matching instead of path.Match, whose
// "*" does not cross "/".
func TestFilter_Allow_NamespacedImagePaths(t *testing.T) {
	f := muzzle.New(false)

	refs := []string{
		"ghcr.io/org/repo",
		"lscr.io/linuxserver/prowlarr",
		"someuser/reponame",
		"registry.example.com/team/project/image",
	}

	for _, prefix := range []string{"/images/", "/distribution/"} {
		for _, ref := range refs {
			p := prefix + ref + "/json"
			t.Run(p, func(t *testing.T) {
				assert.True(t, f.Allow("GET", p, nil))
			})

			vp := "/v1.44" + prefix + ref + "/json"
			t.Run(vp, func(t *testing.T) {
				assert.True(t, f.Allow("GET", vp, nil))
			})
		}
	}
}

// TestFilter_Allow_NamespacedImagePaths_NonGETBlocked confirms the
// unconditional GET-only enforcement in Allow (which runs before any path
// match) still rejects writes against namespaced image/distribution paths
// now that they pass the allowlist's path check.
func TestFilter_Allow_NamespacedImagePaths_NonGETBlocked(t *testing.T) {
	f := muzzle.New(false)

	paths := []string{
		"/images/ghcr.io/org/repo/json",
		"/v1.44/images/ghcr.io/org/repo/json",
		"/distribution/ghcr.io/org/repo/json",
		"/v1.44/distribution/ghcr.io/org/repo/json",
	}

	for _, p := range paths {
		for _, method := range []string{"POST", "PUT", "DELETE", "PATCH"} {
			t.Run(method+" "+p, func(t *testing.T) {
				assert.False(t, f.Allow(method, p, nil))
			})
		}
	}
}

func TestFilter_ServeProxy_Blocked_POST(t *testing.T) {
	f := muzzle.New(false)

	reqStr := "POST /v1.41/containers/create HTTP/1.1\r\nHost: localhost\r\nContent-Length: 0\r\n\r\n"
	var buf bytes.Buffer

	err := f.ServeProxy("/tmp/nonexistent.sock", strings.NewReader(reqStr), &buf)
	require.Error(t, err)
	assert.Contains(t, buf.String(), "403")
}

func TestFilter_ServeProxy_Blocked_DELETE(t *testing.T) {
	f := muzzle.New(false)

	reqStr := "DELETE /v1.41/containers/abc HTTP/1.1\r\nHost: localhost\r\n\r\n"
	var buf bytes.Buffer

	err := f.ServeProxy("/tmp/nonexistent.sock", strings.NewReader(reqStr), &buf)
	require.Error(t, err)
	assert.Contains(t, buf.String(), "403")
}

func TestFilter_ServeProxy_Blocked_PUT(t *testing.T) {
	f := muzzle.New(false)

	reqStr := "PUT /v1.41/networks/abc HTTP/1.1\r\nHost: localhost\r\nContent-Length: 0\r\n\r\n"
	var buf bytes.Buffer

	err := f.ServeProxy("/tmp/nonexistent.sock", strings.NewReader(reqStr), &buf)
	require.Error(t, err)
	assert.Contains(t, buf.String(), "403")
}

func TestFilter_ServeProxy_Blocked_UnversionedPost(t *testing.T) {
	f := muzzle.New(false)

	reqStr := "POST /containers/create HTTP/1.1\r\nHost: localhost\r\nContent-Length: 0\r\n\r\n"
	var buf bytes.Buffer

	err := f.ServeProxy("/tmp/nonexistent.sock", strings.NewReader(reqStr), &buf)
	require.Error(t, err)
	assert.Contains(t, buf.String(), "403")
}

func TestServeProxy_ConnectionCloseSetOnRequest(t *testing.T) {
	f := muzzle.New(false)

	reqSeen := make(chan *http.Request, 1)
	serverErr := make(chan error, 1)

	sockPath, cleanup := startUnixHTTPServer(t, func(conn net.Conn) {
		defer func() { _ = conn.Close() }()

		req, err := http.ReadRequest(bufio.NewReader(conn))
		if err != nil {
			serverErr <- err
			return
		}
		reqSeen <- req

		_, err = io.WriteString(conn, "HTTP/1.1 200 OK\r\nContent-Length: 5\r\nConnection: close\r\n\r\nhello")
		if err != nil {
			serverErr <- err
		}
	})
	defer cleanup()

	var out bytes.Buffer
	req := "GET /containers/json HTTP/1.1\r\nHost: localhost\r\n\r\n"
	err := f.ServeProxy(sockPath, strings.NewReader(req), &out)
	require.NoError(t, err)

	seen := <-reqSeen
	select {
	case err := <-serverErr:
		require.NoError(t, err)
	default:
	}
	assert.True(t, seen.Close)
	assert.Equal(t, "close", strings.ToLower(seen.Header.Get("Connection")))
	assert.Contains(t, out.String(), "200 OK")
	assert.Contains(t, out.String(), "hello")
}

func TestServeProxy_CompletesAfterDockerResponse(t *testing.T) {
	f := muzzle.New(false)
	serverErr := make(chan error, 1)
	body := `{"status":"ok"}`

	sockPath, cleanup := startUnixHTTPServer(t, func(conn net.Conn) {
		defer func() { _ = conn.Close() }()

		_, err := http.ReadRequest(bufio.NewReader(conn))
		if err != nil {
			serverErr <- err
			return
		}

		_, err = io.WriteString(conn, fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", len(body), body))
		if err != nil {
			serverErr <- err
		}
	})
	defer cleanup()

	var out bytes.Buffer
	req := "GET /containers/json HTTP/1.1\r\nHost: localhost\r\n\r\n"

	done := make(chan error, 1)
	go func() {
		done <- f.ServeProxy(sockPath, strings.NewReader(req), &out)
	}()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("ServeProxy did not return after complete response")
	}

	assert.Contains(t, out.String(), body)
	select {
	case err := <-serverErr:
		require.NoError(t, err)
	default:
	}
}

func TestServeProxy_StreamingResponseTerminatesOnWriterClose(t *testing.T) {
	f := muzzle.New(false)

	serverWriteErr := make(chan error, 1)
	serverErr := make(chan error, 1)

	sockPath, cleanup := startUnixHTTPServer(t, func(conn net.Conn) {
		defer func() { _ = conn.Close() }()

		_, err := http.ReadRequest(bufio.NewReader(conn))
		if err != nil {
			serverErr <- err
			return
		}

		_, err = io.WriteString(conn, "HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\nConnection: close\r\n\r\n")
		if err != nil {
			serverErr <- err
			return
		}

		for {
			if _, writeErr := io.WriteString(conn, "6\r\nhello!\r\n"); writeErr != nil {
				serverWriteErr <- writeErr
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	})
	defer cleanup()

	w := newClosableWriter()
	req := "GET /events HTTP/1.1\r\nHost: localhost\r\n\r\n"

	done := make(chan error, 1)
	go func() {
		done <- f.ServeProxy(sockPath, strings.NewReader(req), w)
	}()

	select {
	case <-w.firstWrite:
	case <-time.After(2 * time.Second):
		t.Fatal("stream did not start")
	}

	w.Close()

	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("ServeProxy did not return after writer close")
	}

	select {
	case err := <-serverWriteErr:
		require.Error(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("mock docker stream did not observe closed connection")
	}

	select {
	case err := <-serverErr:
		require.NoError(t, err)
	default:
	}
}

// errReader is an io.Reader that always fails, used to force a mid-body
// read error deterministically instead of relying on a real, flaky I/O
// fault.
type errReader struct {
	err error
}

func (r errReader) Read([]byte) (int, error) {
	return 0, r.err
}

// TestFilter_ServeProxy_BodyReadError_ReturnsError targets muzzle.go's
// io.ReadAll(limited) failure branch in ServeProxy: the request line and
// headers parse successfully (they fit entirely in bufio.Reader's first
// fill, so http.ReadRequest never touches the failing sub-reader), but the
// declared Content-Length body is backed by an io.MultiReader whose second
// segment always errors, so the read is guaranteed to fail only once
// ServeProxy actually starts consuming the body.
func TestFilter_ServeProxy_BodyReadError_ReturnsError(t *testing.T) {
	f := muzzle.New(false)

	header := "POST /containers/create HTTP/1.1\r\nHost: localhost\r\nContent-Length: 5\r\n\r\n"
	boom := errors.New("boom")
	reqReader := io.MultiReader(strings.NewReader(header), errReader{err: boom})

	var buf bytes.Buffer
	err := f.ServeProxy("/tmp/nonexistent.sock", reqReader, &buf)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "read body")
}

// TestFilter_ServeProxy_BodyTooLarge_Returns403AndError targets muzzle.go's
// maxContainerCreateBodyBytes size-limit branch. The size check runs before
// Allow is consulted, so the method/path here don't need to be on the
// allowlist for the case to exercise the intended branch.
func TestFilter_ServeProxy_BodyTooLarge_Returns403AndError(t *testing.T) {
	f := muzzle.New(false)

	const maxContainerCreateBodyBytes = 64 * 1024
	oversized := bytes.Repeat([]byte("a"), maxContainerCreateBodyBytes+1)
	reqStr := fmt.Sprintf("GET /containers/json HTTP/1.1\r\nHost: localhost\r\nContent-Length: %d\r\n\r\n%s", len(oversized), oversized)

	var buf bytes.Buffer
	err := f.ServeProxy("/tmp/nonexistent.sock", strings.NewReader(reqStr), &buf)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "body too large")
	assert.Contains(t, buf.String(), "403")
}

// TestFilter_ServeProxy_DockerWriteError_ReturnsError targets muzzle.go's
// req.Write(conn) failure branch: ServeProxy successfully dials the fake
// Docker socket, but the write of the forwarded request to it fails.
//
// Getting *specifically* the write to fail (as opposed to the dial, or the
// later response read) is an inherently racy thing to force from outside
// with a real kernel socket, and no amount of server-side cleverness makes
// it deterministic, because ServeProxy dials and writes back-to-back with
// no hook this test can synchronize against:
//
//   - Accepting the connection and then abortively closing it (SO_LINGER=0)
//     was tried first (Supervisor Correction 2's suggested starting point)
//     and found flaky: the request's header write is small enough that the
//     kernel routinely buffers it locally and returns success from
//     req.Write's underlying write(2) before the peer's close has
//     propagated, so the error surfaces later instead, on the response
//     read - not on the write itself.
//   - Closing the *listener* without ever accepting - so the connection
//     ServeProxy just dialed is torn down while still sitting unaccepted in
//     the backlog - reliably produces a write failure with the right
//     message *when the race lands in that window*, but measured directly
//     (a bare goroutine closing the listener, racing unsynchronized against
//     ServeProxy's own dial+write, the same shape this test needs), it only
//     lands in that window on ~40-50% of attempts under `-race`; the rest
//     either close before the dial completes (a *dial* error, wrong branch)
//     or after the write already succeeded (a *read* error, wrong branch).
//
// So this test retries with a fresh socket/listener each time, accepting
// only the specific error this line actually produces and discarding any
// other outcome as an inconclusive attempt. Measured per-attempt success
// rate is roughly 40-50%, but attempts within one process are not fully
// independent (occasional short losing streaks were observed) - a
// retryLimit of 25 was empirically observed to exhaust in about 1 in 30
// full `go test` runs under `-race`, so retryLimit=200 is used instead;
// with that limit, 100+ single-binary `-count=N` runs and 40+ separate
// freshly-exec'd process runs (thousands of attempts total) produced zero
// exhaustions. This is the same bounded-retry technique Go's own standard
// library networking tests use for OS-timing-dependent behavior; it is a
// deliberate, documented trade-off, not an oversight.
func TestFilter_ServeProxy_DockerWriteError_ReturnsError(t *testing.T) {
	f := muzzle.New(false)

	const retryLimit = 200
	reqStr := "GET /containers/json HTTP/1.1\r\nHost: localhost\r\n\r\n"

	for attempt := 1; attempt <= retryLimit; attempt++ {
		sockPath := filepath.Join(t.TempDir(), fmt.Sprintf("docker-%d.sock", attempt))
		ln, err := net.Listen("unix", sockPath)
		require.NoError(t, err)

		// Deliberately never call ln.Accept(). Closing the listener while
		// ServeProxy's own net.Dial connection may still be sitting
		// unaccepted in the backlog is what can make the subsequent write
		// fail - see the determinism discussion above for why this only
		// sometimes lands on the write specifically.
		go func() {
			_ = ln.Close()
		}()

		var buf bytes.Buffer
		serveErr := f.ServeProxy(sockPath, strings.NewReader(reqStr), &buf)

		if serveErr != nil && strings.Contains(serveErr.Error(), "forward request to docker") {
			return
		}
	}

	t.Fatalf("did not observe a docker-write error within %d attempts (target branch: muzzle.go ServeProxy's req.Write(conn) failure path)", retryLimit)
}

// --- Write-mode tests (Section 3.3.4/3.4.1 of the Orthrus write-mode spec) ---
//
// These mirror backend/internal/orthrus/muzzle_test.go's write-mode tests
// exactly in intent (though this package's Allow(method, path, body) is
// called directly rather than through the httptest-based ServeHTTP the
// backend uses, since Filter has no http.Handler interface of its own).

func TestFilter_Allow_WriteEndpoints_BlockedWhenWriteDisabled(t *testing.T) {
	f := muzzle.New(false)

	cases := []struct {
		method string
		path   string
	}{
		{"POST", "/containers/create"},
		{"POST", "/images/create"},
		{"POST", "/containers/abc123/start"},
		{"POST", "/containers/abc123/stop"},
		{"POST", "/containers/abc123/restart"},
		{"POST", "/containers/abc123/rename"},
		{"DELETE", "/containers/abc123"},
	}

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			assert.False(t, f.Allow(tc.method, tc.path, nil))
		})
	}
}

func TestFilter_Allow_WriteEndpoints_AllowedWhenWriteEnabled(t *testing.T) {
	f := muzzle.New(true)

	cases := []struct {
		method string
		path   string
	}{
		{"POST", "/images/create"},
		{"POST", "/containers/abc123/start"},
		{"POST", "/containers/abc123/stop"},
		{"POST", "/containers/abc123/restart"},
		{"POST", "/containers/abc123/rename"},
		{"DELETE", "/containers/abc123"},
		{"POST", "/v1.44/containers/abc123/start"},
		{"DELETE", "/v1.44/containers/abc123"},
	}

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			assert.True(t, f.Allow(tc.method, tc.path, nil))
		})
	}
}

// TestFilter_Allow_ContainerRename_QueryParamNameAllowed proves the
// Dockhand image-update rename step end-to-end on the agent side: Docker's
// rename endpoint takes the new container name via a "?name=" query
// parameter, not a request body (unlike /containers/create), and a real
// Docker-generated container ID is a single path segment (no slashes) —
// exactly what allowedWritePatterns's "/containers/*/rename" path.Match
// pattern assumes. Allow receives req.URL.Path (query string already split
// off by the caller, mirroring ServeProxy's real usage), so the query
// string itself is irrelevant to the match — passing it here just documents
// that a realistic rename call site (path plus query) is what this
// allowlist entry exists to unblock.
func TestFilter_Allow_ContainerRename_QueryParamNameAllowed(t *testing.T) {
	f := muzzle.New(true)

	containerID := "3f4d9e2a1b6c8f0d7e5a4b3c2d1e0f9a8b7c6d5e4f3a2b1c0d9e8f7a6b5c4d3e"
	assert.True(t, f.Allow("POST", "/containers/"+containerID+"/rename", nil))
	assert.True(t, f.Allow("POST", "/v1.44/containers/"+containerID+"/rename", nil))
}

// TestFilter_Allow_WriteMode_FakeVersionPrefixBlocked is the write-path
// variant of the GH #1160 regression above (divergence row 3): a
// non-numeric fake version prefix must not be accepted by allowWrite's
// pattern branch either. Before this fix, allowWrite's pattern branch
// matched the raw (non-version-stripped) path directly instead of the
// same normalized path its exact-path branch used, so "/vFOO/..." was
// wrongly treated as reaching "/containers/*/start" via the loose
// path.Match "v*" wildcard.
func TestFilter_Allow_WriteMode_FakeVersionPrefixBlocked(t *testing.T) {
	f := muzzle.New(true)

	cases := []struct {
		method string
		path   string
	}{
		{"POST", "/vFOO/containers/abc/start"},
		{"POST", "/vFOO/containers/create"},
		{"DELETE", "/vFOO/containers/abc"},
	}

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			assert.False(t, f.Allow(tc.method, tc.path, nil))
		})
	}
}

// TestFilter_Allow_WriteMode_NonWriteEndpointsStillBlocked confirms that,
// even with write mode on, endpoints outside the fixed seven-operation list
// (create, images/create, start, stop, restart, rename, delete) — e.g.
// exec, image delete, build, prune, auth, commit, Swarm/service — remain
// permanently blocked. Section 7, Explicit Out-of-Scope.
func TestFilter_Allow_WriteMode_NonWriteEndpointsStillBlocked(t *testing.T) {
	f := muzzle.New(true)

	cases := []struct {
		method string
		path   string
	}{
		{"POST", "/containers/abc123/exec"},
		{"POST", "/exec/xyz/start"},
		{"DELETE", "/images/nginx"},
		{"POST", "/build"},
		{"POST", "/system/prune"},
		{"POST", "/auth"},
		{"POST", "/commit"},
		{"POST", "/networks/create"},
		{"DELETE", "/networks/mynet"},
		{"POST", "/volumes/create"},
		{"DELETE", "/volumes/myvol"},
	}

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			assert.False(t, f.Allow(tc.method, tc.path, nil))
		})
	}
}

func TestFilter_Allow_ContainersCreate_SafeBodyAllowed(t *testing.T) {
	f := muzzle.New(true)

	body := []byte(`{"Image":"nginx:latest","HostConfig":{"PortBindings":{"80/tcp":[{"HostPort":"8080"}]},"RestartPolicy":{"Name":"unless-stopped"},"NetworkMode":"my-bridge"}}`)
	assert.True(t, f.Allow("POST", "/containers/create", body))
	assert.True(t, f.Allow("POST", "/v1.44/containers/create", body))
}

// TestFilter_Allow_ContainersCreate_SafeBodiesAllowed_CapDropAndUsernsMode
// mirrors backend/internal/orthrus/muzzle_test.go's copy of the same name.
func TestFilter_Allow_ContainersCreate_SafeBodiesAllowed_CapDropAndUsernsMode(t *testing.T) {
	f := muzzle.New(true)

	cases := []struct {
		name string
		body string
	}{
		{"CapDrop", `{"Image":"nginx","HostConfig":{"CapDrop":["ALL"]}}`},
		{"UsernsMode empty (inherit daemon default)", `{"Image":"nginx","HostConfig":{"UsernsMode":""}}`},
		{"UsernsMode named remap", `{"Image":"nginx","HostConfig":{"UsernsMode":"default"}}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.True(t, f.Allow("POST", "/containers/create", []byte(tc.body)))
		})
	}
}

// TestFilter_Allow_ContainersCreate_DangerousBodiesRejected mirrors
// backend/internal/orthrus/muzzle_test.go's TestMuzzle_ContainersCreate_DangerousBodiesRejected
// case-for-case, including the local-driver bind-mount-via-volume bypass
// (VolumeOptions.DriverConfig) that both filters must independently reject.
func TestFilter_Allow_ContainersCreate_DangerousBodiesRejected(t *testing.T) {
	f := muzzle.New(true)

	cases := []struct {
		name string
		body string
	}{
		{"Privileged", `{"Image":"nginx","HostConfig":{"Privileged":true}}`},
		{"CapAdd", `{"Image":"nginx","HostConfig":{"CapAdd":["SYS_ADMIN"]}}`},
		{"legacy Binds", `{"Image":"nginx","HostConfig":{"Binds":["/:/host"]}}`},
		{"NetworkMode host", `{"Image":"nginx","HostConfig":{"NetworkMode":"host"}}`},
		{"NetworkMode container:*", `{"Image":"nginx","HostConfig":{"NetworkMode":"container:abc"}}`},
		{"UsernsMode host", `{"Image":"nginx","HostConfig":{"UsernsMode":"host"}}`},
		{"bind-type Mounts", `{"Image":"nginx","HostConfig":{"Mounts":[{"Type":"bind","Source":"/etc","Target":"/x"}]}}`},
		{
			"local-driver bind-mount-via-volume bypass (VolumeOptions.DriverConfig)",
			`{"Image":"nginx","HostConfig":{"Mounts":[{"Type":"volume","Source":"fake","Target":"/etc","VolumeOptions":{"DriverConfig":{"Name":"local","Options":{"type":"none","device":"/etc","o":"bind"}}}}]}}`,
		},
		{"non-empty Devices", `{"Image":"nginx","HostConfig":{"Devices":[{"PathOnHost":"/dev/sda","PathInContainer":"/dev/sda"}]}}`},
		{"non-empty Sysctls", `{"Image":"nginx","HostConfig":{"Sysctls":{"net.ipv4.ip_forward":"1"}}}`},
		{"unrecognized future HostConfig key", `{"Image":"nginx","HostConfig":{"SomeFutureField":true}}`},
		{"malformed JSON", `{"Image": not valid json`},
		// The four cases below each isolate one still-untested fail-closed
		// unmarshal-error branch, distinct from "malformed JSON" above
		// (which only reaches the outer top-level unmarshal) and distinct
		// from the DriverConfig-bypass case above (which has a *valid*
		// VolumeOptions object).
		{"malformed NetworkMode value (non-string JSON)", `{"Image":"nginx","HostConfig":{"NetworkMode":12345}}`},
		{"malformed Mounts value (not a JSON array)", `{"Image":"nginx","HostConfig":{"Mounts":"not-an-array"}}`},
		{"malformed Mounts VolumeOptions (invalid JSON object)", `{"Image":"nginx","HostConfig":{"Mounts":[{"Type":"volume","VolumeOptions":"not-an-object"}]}}`},
		{"malformed HostConfig itself (not a JSON object)", `{"Image":"nginx","HostConfig":"not-an-object"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.False(t, f.Allow("POST", "/containers/create", []byte(tc.body)))
		})
	}
}

// TestFilter_Allow_ContainersCreate_EmptyBodyAllowed locks in
// validateContainerCreateBody's one deliberately *permissive* branch: an
// empty /containers/create body is allowed through rather than rejected as
// malformed. Given its own named test, not folded into the "safe body
// allowed" table, precisely because it documents an intentional
// security-relevant default rather than an incidental pass-through.
func TestFilter_Allow_ContainersCreate_EmptyBodyAllowed(t *testing.T) {
	f := muzzle.New(true)

	assert.True(t, f.Allow("POST", "/containers/create", nil))
	assert.True(t, f.Allow("POST", "/containers/create", []byte{}))
}

// --- Shared anti-drift corpus (Section 3.3.5 of the Orthrus write-mode spec) ---
//
// Loads the exact same testdata/muzzle_corpus.json file that
// backend/internal/orthrus/muzzle_test.go's TestMuzzle_SharedCorpus loads,
// via a relative path across the module boundary — agent/ is a separate Go
// module but plain file I/O in a test isn't constrained by Go import
// resolution, so a single JSON fixture can be the source of truth for both
// independently-maintained allowlist implementations without duplicating
// the data itself (only the enforcement logic is duplicated, by design —
// see the doc comments on hostConfigAllowedKeys etc. in muzzle.go).

type muzzleCorpusCase struct {
	Description       string          `json:"description"`
	Method            string          `json:"method"`
	Path              string          `json:"path"`
	Body              json.RawMessage `json:"body,omitempty"`
	AgentWriteEnabled bool            `json:"agent_write_enabled"`
	WantAllowed       bool            `json:"want_allowed"`
}

func loadMuzzleCorpus(t *testing.T) []muzzleCorpusCase {
	t.Helper()
	data, err := os.ReadFile("../../backend/internal/orthrus/testdata/muzzle_corpus.json")
	require.NoError(t, err)
	var cases []muzzleCorpusCase
	require.NoError(t, json.Unmarshal(data, &cases))
	require.NotEmpty(t, cases)
	return cases
}

func TestFilter_SharedCorpus(t *testing.T) {
	cases := loadMuzzleCorpus(t)
	for _, tc := range cases {
		t.Run(tc.Description, func(t *testing.T) {
			f := muzzle.New(tc.AgentWriteEnabled)

			// The corpus's Path field may include a query string (e.g.
			// "/images/create?fromImage=nginx&tag=latest"), which Allow
			// expects to have already been split off by the caller (mirrors
			// how ServeProxy passes req.URL.Path, not req.URL.RequestURI()).
			p := tc.Path
			if idx := strings.IndexByte(p, '?'); idx >= 0 {
				p = p[:idx]
			}

			got := f.Allow(tc.Method, p, tc.Body)
			assert.Equal(t, tc.WantAllowed, got)
		})
	}
}
