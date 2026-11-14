package leash_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/hashicorp/yamux"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/agent/leash"
)

// streamTypeDocker mirrors the unexported marker byte leash.go's handleStream
// switches on (streamTypeDocker = 0x01). It can't be imported (leash_test is an
// external test package and the constant is unexported), so the literal value
// is duplicated here deliberately, matching the wire protocol leash.go documents.
const streamTypeDocker = byte(0x01)

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

// acceptWS upgrades a test request to a WebSocket connection, accepting any origin.
func acceptWS(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	return websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
}

// TestLeash_Reconnect verifies that the Leash attempts to reconnect after the server
// closes the connection immediately.
func TestLeash_Reconnect(t *testing.T) {
	var connectCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connectCount.Add(1)
		conn, err := acceptWS(w, r)
		require.NoError(t, err)
		_ = conn.CloseNow()
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

	assert.GreaterOrEqual(t, connectCount.Load(), int32(2), "expected at least 2 connection attempts within the timeout")
}

// TestWsNetConn_ReadWrite verifies that the net.Conn wrapper correctly transfers
// binary data through a WebSocket echo server.
func TestWsNetConn_ReadWrite(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := acceptWS(w, r)
		require.NoError(t, err)
		defer func() { _ = conn.CloseNow() }()

		_, msg, err := conn.Read(context.Background())
		require.NoError(t, err)
		require.NoError(t, conn.Write(context.Background(), websocket.MessageBinary, msg))
	}))
	defer srv.Close()

	wsConn, _, err := websocket.Dial(context.Background(), "ws"+srv.URL[4:], nil) //nolint:bodyclose // coder/websocket closes the handshake body itself; resp.Body is nil on success
	require.NoError(t, err)
	defer func() { _ = wsConn.CloseNow() }()

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

// TestWsNetConn_CloseIsAbrupt verifies that closing the net.Conn tears the
// socket down without a WebSocket close frame, matching the agent's previous
// raw-socket close. The peer must observe an error that carries no close status.
func TestWsNetConn_CloseIsAbrupt(t *testing.T) {
	peerErr := make(chan error, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := acceptWS(w, r)
		require.NoError(t, err)
		defer func() { _ = conn.CloseNow() }()

		_, _, readErr := conn.Read(context.Background())
		peerErr <- readErr
	}))
	defer srv.Close()

	wsConn, _, err := websocket.Dial(context.Background(), "ws"+srv.URL[4:], nil) //nolint:bodyclose // coder/websocket closes the handshake body itself; resp.Body is nil on success
	require.NoError(t, err)

	netConn := leash.NewWSNetConn(wsConn)
	require.NoError(t, netConn.Close())

	select {
	case readErr := <-peerErr:
		require.Error(t, readErr)
		assert.Equal(t, websocket.StatusCode(-1), websocket.CloseStatus(readErr), "peer must not receive a close frame")
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the peer to observe the close")
	}
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
		wsConn, upgradeErr := acceptWS(w, r)
		require.NoError(t, upgradeErr)
		defer func() { _ = wsConn.CloseNow() }()

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

// retiredStreamType is not a defined stream type. The agent must close a
// stream that carries it like any other unknown type, without dialing anything.
const retiredStreamType = byte(0x02)

// TestLeash_Connect_RetiredStreamTypeClosedWithoutDial sends the retired stream
// type followed by a target address and proves the agent closes the stream and
// does not dial the address.
func TestLeash_Connect_RetiredStreamTypeClosedWithoutDial(t *testing.T) {
	targetLn, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = targetLn.Close() }()

	dialed := make(chan net.Conn, 1)
	go func() {
		conn, acceptErr := targetLn.Accept()
		if acceptErr == nil {
			dialed <- conn
		}
	}()

	streamClosed := make(chan error, 1)
	serverDone := make(chan struct{})
	defer close(serverDone)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wsConn, upgradeErr := acceptWS(w, r)
		require.NoError(t, upgradeErr)
		defer func() { _ = wsConn.CloseNow() }()

		session := newYamuxServerSession(t, wsConn)
		defer func() { _ = session.Close() }()

		stream, openErr := session.OpenStream()
		require.NoError(t, openErr)
		defer func() { _ = stream.Close() }()

		addr := targetLn.Addr().String()
		payload := []byte{retiredStreamType, byte((len(addr) >> 8) & 0xff), byte(len(addr) & 0xff)}
		payload = append(payload, addr...)
		_, writeErr := stream.Write(payload)
		require.NoError(t, writeErr)

		// Reading to EOF returns once the agent closes its end of the stream.
		_, readErr := io.ReadAll(stream)
		streamClosed <- readErr

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
	case readErr := <-streamClosed:
		assert.NoError(t, readErr)
	case <-time.After(5 * time.Second):
		t.Fatal("agent did not close the stream")
	}

	select {
	case conn := <-dialed:
		_ = conn.Close()
		t.Fatal("agent dialed the target address of a retired stream type")
	case <-time.After(200 * time.Millisecond):
	}
}
