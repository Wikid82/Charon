package orthrus

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/hashicorp/yamux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// findFreePort returns an ephemeral TCP port that is currently unbound.
// There is a brief race window between Close and the subsequent Listen;
// in practice this is safe for unit tests on Linux with SO_REUSEADDR.
func findFreePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	require.NoError(t, ln.Close())
	return port
}

// sessionWithLoopback creates an AgentSession with the loopback Docker proxy
// already started. clientYamux is the yamux.Client side for the test to use
// as a mock agent. Call cleanup() when done.
func sessionWithLoopback(t *testing.T) (sess *AgentSession, clientYamux *yamux.Session, cleanup func()) {
	t.Helper()
	serverConn, clientConn, wsCleanup := testWSPairBoth(t)

	cfg := yamux.DefaultConfig()
	cfg.LogOutput = io.Discard
	cy, err := yamux.Client(newWSNetConn(clientConn), cfg)
	require.NoError(t, err)

	s, err := NewAgentSession("ext-test-uuid", "ext-test-agent", serverConn)
	require.NoError(t, err)
	require.NoError(t, s.StartDockerProxy())

	return s, cy, func() {
		_ = cy.Close()
		_ = s.Close()
		wsCleanup()
	}
}

// serveMockDockerOnYamux accepts yamux streams from clientYamux and serves
// a minimal HTTP response for each Docker API request.
func serveMockDockerOnYamux(clientYamux *yamux.Session, responses map[string]string) {
	go func() {
		for {
			stream, err := clientYamux.AcceptStream()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close() //nolint:errcheck
				typeBuf := make([]byte, 1)
				if _, err := io.ReadFull(conn, typeBuf); err != nil {
					return
				}
				bufRd := bufio.NewReader(conn)
				req, err := http.ReadRequest(bufRd)
				if err != nil {
					return
				}
				defer req.Body.Close()
				body, ok := responses[req.URL.Path]
				if !ok {
					body = `{}`
				}
				raw := fmt.Sprintf(
					"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s",
					len(body), body,
				)
				_, _ = conn.Write([]byte(raw))
			}(stream)
		}
	}()
}

// U-EXT-01: StartExternalProxy(0) is a no-op — returns nil without binding.
func TestStartExternalProxy_ZeroPortIsNoOp(t *testing.T) {
	serverConn, done := testWSPair(t)
	defer done()

	sess, err := NewAgentSession("ext01-uuid", "ext01-agent", serverConn)
	require.NoError(t, err)
	defer func() { _ = sess.Close() }()

	assert.NoError(t, sess.StartExternalProxy(0))
	status := sess.GetExternalProxyStatus()
	assert.False(t, status.Active)
}

// U-EXT-02: StartExternalProxy before StartDockerProxy returns "loopback proxy not started".
func TestStartExternalProxy_LoopbackNotStarted(t *testing.T) {
	serverConn, done := testWSPair(t)
	defer done()

	sess, err := NewAgentSession("ext02-uuid", "ext02-agent", serverConn)
	require.NoError(t, err)
	defer func() { _ = sess.Close() }()

	port := findFreePort(t)
	err = sess.StartExternalProxy(port)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "loopback proxy not started")
}

// U-EXT-03: HTTP GET /version via external proxy reaches mock Docker and returns 200.
func TestStartExternalProxy_FullHTTPChain(t *testing.T) {
	sess, clientYamux, cleanup := sessionWithLoopback(t)
	defer cleanup()

	serveMockDockerOnYamux(clientYamux, map[string]string{
		"/version": `{"Version":"24.0.0","ApiVersion":"1.43"}`,
	})

	port := findFreePort(t)
	require.NoError(t, sess.StartExternalProxy(port))

	assert.Eventually(t, func() bool {
		return sess.GetExternalProxyStatus().Active
	}, 2*time.Second, 10*time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/version", port))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "Version")
}

// U-EXT-04: A second StartExternalProxy call returns "already started".
func TestStartExternalProxy_Idempotent(t *testing.T) {
	sess, _, cleanup := sessionWithLoopback(t)
	defer cleanup()

	port := findFreePort(t)
	require.NoError(t, sess.StartExternalProxy(port))

	err := sess.StartExternalProxy(port)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already started")
}

// U-EXT-05: POST to external proxy returns 403 (Muzzle blocks non-GET methods).
func TestStartExternalProxy_MuzzleBlocksPOST(t *testing.T) {
	sess, _, cleanup := sessionWithLoopback(t)
	defer cleanup()

	port := findFreePort(t)
	require.NoError(t, sess.StartExternalProxy(port))

	assert.Eventually(t, func() bool {
		return sess.GetExternalProxyStatus().Active
	}, 2*time.Second, 10*time.Millisecond)

	resp, err := http.Post(fmt.Sprintf("http://127.0.0.1:%d/containers/json", port), "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// U-EXT-06: GET to a non-allowlisted path returns 403 (Muzzle blocks /exec).
func TestStartExternalProxy_MuzzleBlocksNonAllowlisted(t *testing.T) {
	sess, _, cleanup := sessionWithLoopback(t)
	defer cleanup()

	port := findFreePort(t)
	require.NoError(t, sess.StartExternalProxy(port))

	assert.Eventually(t, func() bool {
		return sess.GetExternalProxyStatus().Active
	}, 2*time.Second, 10*time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/exec/abc123/start", port))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// U-EXT-07: GetExternalProxyStatus when proxy not started returns Active=false.
func TestGetExternalProxyStatus_NotStarted(t *testing.T) {
	sess, _, cleanup := sessionWithLoopback(t)
	defer cleanup()

	status := sess.GetExternalProxyStatus()
	assert.False(t, status.Active)
	assert.Equal(t, 0, status.ActivePort)
	assert.Empty(t, status.BoundAddress)
	assert.Equal(t, 0, status.ConfiguredPort)
}

// U-EXT-08: GetExternalProxyStatus when proxy started returns Active=true with correct port.
func TestGetExternalProxyStatus_Active(t *testing.T) {
	sess, _, cleanup := sessionWithLoopback(t)
	defer cleanup()

	port := findFreePort(t)
	require.NoError(t, sess.StartExternalProxy(port))

	assert.Eventually(t, func() bool {
		return sess.GetExternalProxyStatus().Active
	}, 2*time.Second, 10*time.Millisecond)

	status := sess.GetExternalProxyStatus()
	assert.True(t, status.Active)
	assert.Equal(t, port, status.ConfiguredPort)
	assert.Equal(t, port, status.ActivePort)
	assert.NotEmpty(t, status.BoundAddress)
}

// U-EXT-09: Close() shuts down the external proxy; subsequent TCP dial is refused.
func TestClose_ShutdownsExternalProxy(t *testing.T) {
	sess, _, cleanup := sessionWithLoopback(t)
	defer cleanup()

	port := findFreePort(t)
	require.NoError(t, sess.StartExternalProxy(port))

	assert.Eventually(t, func() bool {
		return sess.GetExternalProxyStatus().Active
	}, 2*time.Second, 10*time.Millisecond)

	require.NoError(t, sess.Close())

	_, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 500*time.Millisecond)
	assert.Error(t, err, "external proxy port should be closed after session Close()")
}

// U-EXT-10: After Close(), a new session can bind the same port successfully.
func TestStartExternalProxy_PortReusableAfterClose(t *testing.T) {
	sess, _, cleanup := sessionWithLoopback(t)
	defer cleanup()

	port := findFreePort(t)
	require.NoError(t, sess.StartExternalProxy(port))

	assert.Eventually(t, func() bool {
		return sess.GetExternalProxyStatus().Active
	}, 2*time.Second, 10*time.Millisecond)

	require.NoError(t, sess.Close())

	// Create a second session and bind the same port.
	sess2, _, cleanup2 := sessionWithLoopback(t)
	defer cleanup2()

	require.NoError(t, sess2.StartExternalProxy(port))
	assert.Eventually(t, func() bool {
		return sess2.GetExternalProxyStatus().Active
	}, 2*time.Second, 10*time.Millisecond)

	status := sess2.GetExternalProxyStatus()
	assert.True(t, status.Active)
	assert.Equal(t, port, status.ActivePort)
}

// Coverage debt: the following lines in session.go and server.go are not
// exercised by unit tests because the failure conditions they guard against
// cannot be induced deterministically without modifying production code.
//
//   session.go – NewAgentSession yamux error path (L127–128):
//     yamux.Server never returns an error when given a valid DefaultConfig.
//     VerifyConfig always passes for the library's own defaults, so the error
//     branch exists only for library contract compliance and is unreachable in
//     practice.
//
//   session.go – StartDockerProxy net.Listen failure (L160–161):
//     net.Listen("tcp", "127.0.0.1:0") only fails under OS-level resource
//     exhaustion (e.g., file-descriptor limit exceeded). This cannot be
//     triggered reliably in unit tests.
//
//   session.go – StartDockerProxy double-check guard (L167–170):
//     The re-check inside the second lock requires two concurrent goroutines
//     to both observe s.listener == nil during the first lock window and then
//     race through net.Listen before either re-acquires the lock. The window
//     is too narrow to hit deterministically without test hooks in the
//     production function.
//
//   session.go – proxyConn write-stream-type-byte failure (L210–212):
//     Triggering this path requires the yamux stream to transition to the
//     "reset" state strictly between the s.session.Open() call (L201) and
//     the stream.Write() call (L209). Because yamux processes RST frames
//     asynchronously in a receive goroutine, this race cannot be enforced
//     without an injection point inside proxyConn itself.
//
//   server.go – HandleWebSocket StartDockerProxy warning (L97–99):
//     Reaching this branch requires a full WebSocket HTTP upgrade, TLS
//     termination, and agent-registration flow. Coverage belongs in the
//     integration test suite, not in orthrus package unit tests.
