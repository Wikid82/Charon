package leash_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/agent/leash"
)

// streamTypeDocker and streamTypePortForward mirror the unexported marker
// bytes leash.go's handleStream switches on (streamTypeDocker = 0x01,
// streamTypePortForward = 0x02). They can't be imported (leash_test is an
// external test package and the constants are unexported), so the literal
// values are duplicated here deliberately, matching the wire protocol
// leash.go documents in its own handleStream/handlePortForward comments.
const (
	streamTypeDocker      = byte(0x01)
	streamTypePortForward = byte(0x02)
)

// newYamuxServerSession wraps a server-side (already-upgraded) WebSocket
// connection as a net.Conn via leash.NewWSNetConn (the same helper
// TestWsNetConn_ReadWrite above already exercises) and starts a yamux
// server session on it. This is the test-harness-side counterpart to
// connect()'s own yamux.Client(NewWSNetConn(wsConn), cfg) call, needed so
// the test can open streams that the agent's AcceptStream loop will
// receive.
func newYamuxServerSession(t *testing.T, wsConn *websocket.Conn) *yamux.Session {
	t.Helper()
	netConn := leash.NewWSNetConn(wsConn)
	cfg := yamux.DefaultConfig()
	cfg.LogOutput = io.Discard
	session, err := yamux.Server(netConn, cfg)
	require.NoError(t, err)
	return session
}

var testUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// TestLeash_Reconnect verifies that the Leash attempts to reconnect after the server
// closes the connection immediately.
func TestLeash_Reconnect(t *testing.T) {
	connectCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connectCount++
		conn, err := testUpgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		_ = conn.Close()
	}))
	defer srv.Close()

	wsURL := "ws" + srv.URL[4:] + "/ws"

	log := logrus.New()
	log.SetOutput(io.Discard)

	l := leash.New(leash.Config{
		ServerURL:    wsURL,
		AuthKey:      "ch_orthrus_test",
		AgentID:      "test-agent",
		DockerSock:   "/var/run/docker.sock",
		Log:          log,
		InitialDelay: 50 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_ = l.Run(ctx)

	assert.GreaterOrEqual(t, connectCount, 2, "expected at least 2 connection attempts within the timeout")
}

// TestWsNetConn_ReadWrite verifies that the net.Conn wrapper correctly transfers
// binary data through a WebSocket echo server.
func TestWsNetConn_ReadWrite(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testUpgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer func() { _ = conn.Close() }()

		_, msg, err := conn.ReadMessage()
		require.NoError(t, err)
		require.NoError(t, conn.WriteMessage(websocket.BinaryMessage, msg))
	}))
	defer srv.Close()

	dialer := websocket.Dialer{}
	wsConn, _, err := dialer.Dial("ws"+srv.URL[4:], nil)
	require.NoError(t, err)
	defer func() { _ = wsConn.Close() }()

	netConn := leash.NewWSNetConn(wsConn)

	sent := []byte("hello orthrus")
	n, err := netConn.Write(sent)
	require.NoError(t, err)
	assert.Equal(t, len(sent), n)

	buf := make([]byte, len(sent))
	_, err = io.ReadFull(netConn, buf)
	require.NoError(t, err)
	assert.Equal(t, sent, buf)
}

// TestLeash_Connect_DockerStreamDispatchesThroughFilter proves the full
// write-mode wiring chain actually executes for a real accepted stream:
// connect()'s AcceptStream loop -> handleStream's streamTypeDocker dispatch
// -> handleDockerStream -> the connection-scoped filter.ServeProxy call ->
// net.Dial(dockerSock). None of this had ever been exercised by a test
// before (F.5/F.6 of docs/plans/current_spec.md's coverage follow-up):
// muzzle's own tests call Filter.ServeProxy directly, never through Leash's
// dispatch, so this is the only place that dispatch wiring is proven live.
//
// The test server plays the Charon-server role: it upgrades the WebSocket,
// opens a yamux *server* session on it (the counterpart to connect()'s own
// yamux.Client call), opens one stream, and writes the streamTypeDocker
// marker byte followed by a minimal valid raw HTTP request. The real agent
// side runs via leash.New(...).Run(ctx) in a goroutine, configured with
// DockerSock pointing at a fake unix listener that just accepts and holds
// the connection. Success is that fake listener's Accept() returning within
// a bounded timeout - proof a real stream was dispatched all the way
// through to a real Dial of the configured Docker socket path.
func TestLeash_Connect_DockerStreamDispatchesThroughFilter(t *testing.T) {
	dockerSockPath := filepath.Join(t.TempDir(), "docker.sock")
	dockerLn, err := net.Listen("unix", dockerSockPath)
	require.NoError(t, err)
	defer func() { _ = dockerLn.Close() }()

	dockerAccepted := make(chan net.Conn, 1)
	go func() {
		conn, acceptErr := dockerLn.Accept()
		if acceptErr == nil {
			dockerAccepted <- conn
		}
	}()

	// serverDone gates the test server handler's return (and therefore its
	// deferred session/stream/WebSocket closes) until the test has either
	// observed the docker-side accept or given up, so the underlying
	// transport isn't torn down before the agent has a chance to read the
	// already-written stream data and act on it.
	serverDone := make(chan struct{})
	defer close(serverDone)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wsConn, upgradeErr := testUpgrader.Upgrade(w, r, nil)
		require.NoError(t, upgradeErr)
		defer func() { _ = wsConn.Close() }()

		session := newYamuxServerSession(t, wsConn)
		defer func() { _ = session.Close() }()

		stream, openErr := session.OpenStream()
		require.NoError(t, openErr)
		defer func() { _ = stream.Close() }()

		payload := append([]byte{streamTypeDocker}, []byte("GET /containers/json HTTP/1.1\r\nHost: localhost\r\n\r\n")...)
		_, writeErr := stream.Write(payload)
		require.NoError(t, writeErr)

		<-serverDone
	}))
	defer srv.Close()

	wsURL := "ws" + srv.URL[4:] + "/ws"

	log := logrus.New()
	log.SetOutput(io.Discard)

	l := leash.New(leash.Config{
		ServerURL:  wsURL,
		AuthKey:    "ch_orthrus_test",
		AgentID:    "test-agent",
		DockerSock: dockerSockPath,
		Log:        log,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = l.Run(ctx) }()

	select {
	case conn := <-dockerAccepted:
		_ = conn.Close()
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the docker stream to dial the fake docker socket")
	}
}

// TestLeash_Connect_PortForwardStreamDialsTarget proves handlePortForward's
// successful-dial path (and its deferred conn.Close(), leash.go line 232)
// actually runs for a real accepted stream, the port-forward counterpart to
// the docker-stream test above.
//
// Same harness: the test server opens a stream and writes the
// streamTypePortForward marker byte followed by a 2-byte big-endian address
// length and the address bytes of a fake TCP target listener. Success is
// that target listener's Accept() returning within a bounded timeout,
// proving handleStream's streamTypePortForward case dispatched into
// handlePortForward and it reached net.Dial("tcp", targetAddr) rather than
// one of its early-return error paths (invalid address length, dial
// failure) that are the only parts any pre-existing test exercised.
func TestLeash_Connect_PortForwardStreamDialsTarget(t *testing.T) {
	targetLn, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = targetLn.Close() }()

	targetAccepted := make(chan net.Conn, 1)
	go func() {
		conn, acceptErr := targetLn.Accept()
		if acceptErr == nil {
			targetAccepted <- conn
		}
	}()

	serverDone := make(chan struct{})
	defer close(serverDone)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wsConn, upgradeErr := testUpgrader.Upgrade(w, r, nil)
		require.NoError(t, upgradeErr)
		defer func() { _ = wsConn.Close() }()

		session := newYamuxServerSession(t, wsConn)
		defer func() { _ = session.Close() }()

		stream, openErr := session.OpenStream()
		require.NoError(t, openErr)
		defer func() { _ = stream.Close() }()

		addr := targetLn.Addr().String()
		require.LessOrEqual(t, len(addr), 255, "target address must fit handlePortForward's 1-byte length encoding")
		lenBuf := []byte{byte(len(addr) >> 8), byte(len(addr))}

		payload := append([]byte{streamTypePortForward}, lenBuf...)
		payload = append(payload, []byte(addr)...)
		_, writeErr := stream.Write(payload)
		require.NoError(t, writeErr)

		<-serverDone
	}))
	defer srv.Close()

	wsURL := "ws" + srv.URL[4:] + "/ws"

	log := logrus.New()
	log.SetOutput(io.Discard)

	l := leash.New(leash.Config{
		ServerURL:  wsURL,
		AuthKey:    "ch_orthrus_test",
		AgentID:    "test-agent",
		DockerSock: "/var/run/docker.sock",
		Log:        log,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = l.Run(ctx) }()

	select {
	case conn := <-targetAccepted:
		_ = conn.Close()
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the port-forward stream to dial the fake target listener")
	}
}
