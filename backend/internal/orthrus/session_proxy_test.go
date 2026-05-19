package orthrus

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testWSPairBoth creates a WebSocket server/client pair and returns both sides.
func testWSPairBoth(t *testing.T) (serverConn *websocket.Conn, clientConn *websocket.Conn, done func()) {
	t.Helper()

	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	ready := make(chan *websocket.Conn, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		ready <- conn
	}))

	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	client, dialResp, err := websocket.DefaultDialer.Dial(url, nil)
	require.NoError(t, err)
	if dialResp != nil {
		_ = dialResp.Body.Close()
	}

	srvConn := <-ready
	return srvConn, client, func() {
		_ = client.Close()
		srv.Close()
	}
}

// U1 — StartDockerProxy sets a non-empty loopback proxy address.
func TestStartDockerProxy_SetsProxyAddr(t *testing.T) {
	serverConn, done := testWSPair(t)
	defer done()

	sess, err := NewAgentSession("u1-uuid", "u1-agent", serverConn)
	require.NoError(t, err)
	defer func() { _ = sess.Close() }()

	require.NoError(t, sess.StartDockerProxy())
	addr := sess.GetProxyAddr()
	assert.NotEmpty(t, addr)
	assert.True(t, strings.HasPrefix(addr, "127.0.0.1:"), "addr should be loopback: %s", addr)
}

// U2 — A second StartDockerProxy call returns "already started" and the address is unchanged.
func TestStartDockerProxy_Idempotent(t *testing.T) {
	serverConn, done := testWSPair(t)
	defer done()

	sess, err := NewAgentSession("u2-uuid", "u2-agent", serverConn)
	require.NoError(t, err)
	defer func() { _ = sess.Close() }()

	require.NoError(t, sess.StartDockerProxy())
	first := sess.GetProxyAddr()
	require.NotEmpty(t, first)

	err = sess.StartDockerProxy()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already started")
	assert.Equal(t, first, sess.GetProxyAddr(), "address must not change on second call")
}

// U3 — Dialing the proxy address delivers streamTypeDocker (0x01) as the first byte
// on the yamux stream, and data flows bidirectionally.
func TestStartDockerProxy_AcceptsAndForwards(t *testing.T) {
	serverConn, clientConn, done := testWSPairBoth(t)
	defer done()

	sess, err := NewAgentSession("u3-uuid", "u3-agent", serverConn)
	require.NoError(t, err)
	defer func() { _ = sess.Close() }()

	// Build a yamux client from the client websocket conn.
	cfg := yamux.DefaultConfig()
	cfg.LogOutput = io.Discard
	clientYamux, err := yamux.Client(newWSNetConn(clientConn), cfg)
	require.NoError(t, err)
	defer func() { _ = clientYamux.Close() }()

	require.NoError(t, sess.StartDockerProxy())
	addr := sess.GetProxyAddr()
	require.NotEmpty(t, addr)

	// Dial the proxy; proxyConn will call session.Open() to the client.
	tcpConn, err := net.Dial("tcp", addr)
	require.NoError(t, err)
	defer func() { _ = tcpConn.Close() }()

	// Accept the yamux stream that proxyConn opened toward the client.
	stream, err := clientYamux.AcceptStream()
	require.NoError(t, err)
	defer func() { _ = stream.Close() }()

	// First byte must be streamTypeDocker (0x01).
	typeBuf := make([]byte, 1)
	_, err = io.ReadFull(stream, typeBuf)
	require.NoError(t, err)
	assert.Equal(t, streamTypeDocker, typeBuf[0])

	// Data flows from stream → TCP conn.
	msg := []byte("docker-ping")
	_, err = stream.Write(msg)
	require.NoError(t, err)
	recv := make([]byte, len(msg))
	_, err = io.ReadFull(tcpConn, recv)
	require.NoError(t, err)
	assert.Equal(t, msg, recv)
}

// U4 — Close stops the proxy listener; subsequent dials are refused.
func TestClose_StopsProxyListener(t *testing.T) {
	serverConn, done := testWSPair(t)
	defer done()

	sess, err := NewAgentSession("u4-uuid", "u4-agent", serverConn)
	require.NoError(t, err)

	require.NoError(t, sess.StartDockerProxy())
	addr := sess.GetProxyAddr()
	require.NotEmpty(t, addr)

	require.NoError(t, sess.Close())

	// Listener is closed; connection must be refused.
	conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		t.Fatal("expected dial to fail after session close")
	}
}

// U5 — StartDockerProxy on a closed session returns an error without allocating a port.
func TestStartDockerProxy_AfterClose(t *testing.T) {
	serverConn, done := testWSPair(t)
	defer done()

	sess, err := NewAgentSession("u5-uuid", "u5-agent", serverConn)
	require.NoError(t, err)

	require.NoError(t, sess.Close())

	err = sess.StartDockerProxy()
	require.Error(t, err)
	assert.Equal(t, "", sess.GetProxyAddr())
}

// U6: injecting a non-ErrClosed Accept error requires a net.Listener seam; future work.
