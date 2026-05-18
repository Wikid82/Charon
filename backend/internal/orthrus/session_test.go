package orthrus

import (
	"context"
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

// testWSPair creates a WebSocket server/client pair for testing.
// It returns the server-side *websocket.Conn.
func testWSPair(t *testing.T) (serverConn *websocket.Conn, done func()) {
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
	clientConn, dialResp, err := websocket.DefaultDialer.Dial(url, nil)
	require.NoError(t, err)
	if dialResp != nil {
		_ = dialResp.Body.Close()
	}

	serverConn = <-ready
	return serverConn, func() {
		_ = clientConn.Close()
		srv.Close()
	}
}

func TestNewAgentSession_IsAlive(t *testing.T) {
	serverConn, done := testWSPair(t)
	defer done()

	sess, err := NewAgentSession("uuid-1", "agent-1", serverConn)
	require.NoError(t, err)
	defer func() { _ = sess.Close() }()

	assert.True(t, sess.IsAlive())
}

func TestAgentSession_GetProxyAddr_NoPort(t *testing.T) {
	serverConn, done := testWSPair(t)
	defer done()

	sess, err := NewAgentSession("uuid-1", "agent-1", serverConn)
	require.NoError(t, err)
	defer func() { _ = sess.Close() }()

	assert.Equal(t, "", sess.GetProxyAddr())
}

func TestAgentSession_Close_SetsNotAlive(t *testing.T) {
	serverConn, done := testWSPair(t)
	defer done()

	sess, err := NewAgentSession("uuid-1", "agent-1", serverConn)
	require.NoError(t, err)

	require.NoError(t, sess.Close())
	assert.False(t, sess.IsAlive())
}

func TestAgentSession_StartDockerProxy_SetsProxyAddr(t *testing.T) {
	serverConn, done := testWSPair(t)
	defer done()

	sess, err := NewAgentSession("proxy-uuid", "proxy-agent", serverConn)
	require.NoError(t, err)
	defer func() { _ = sess.Close() }()

	require.NoError(t, sess.StartDockerProxy())

	addr := sess.GetProxyAddr()
	assert.NotEmpty(t, addr)
	assert.True(t, strings.HasPrefix(addr, "127.0.0.1:"), "addr must be on loopback: %s", addr)
}

func TestAgentSession_StartDockerProxy_CalledTwice(t *testing.T) {
	serverConn, done := testWSPair(t)
	defer done()

	sess, err := NewAgentSession("proxy-uuid-2", "proxy-agent-2", serverConn)
	require.NoError(t, err)
	defer func() { _ = sess.Close() }()

	require.NoError(t, sess.StartDockerProxy())
	firstAddr := sess.GetProxyAddr()

	err2 := sess.StartDockerProxy()
	require.Error(t, err2)
	assert.Contains(t, err2.Error(), "already started")
	assert.Equal(t, firstAddr, sess.GetProxyAddr(), "proxy addr must not change on second call")
}

func TestAgentSession_Close_ClosesListener(t *testing.T) {
	serverConn, done := testWSPair(t)
	defer done()

	sess, err := NewAgentSession("close-uuid", "close-agent", serverConn)
	require.NoError(t, err)
	require.NoError(t, sess.StartDockerProxy())
	addr := sess.GetProxyAddr()
	require.NotEmpty(t, addr)

	require.NoError(t, sess.Close())

	conn, dialErr := net.DialTimeout("tcp", addr, 100*time.Millisecond)
	if conn != nil {
		_ = conn.Close()
	}
	assert.Error(t, dialErr, "TCP dial should fail after session close")
}

func TestAgentSession_ProxyConn_WritesStreamTypeByte(t *testing.T) {
	serverPipe, clientPipe := net.Pipe()
	t.Cleanup(func() {
		_ = serverPipe.Close()
		_ = clientPipe.Close()
	})

	cfg := yamux.DefaultConfig()
	cfg.LogOutput = io.Discard
	yamuxServer, err := yamux.Server(serverPipe, cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = yamuxServer.Close() })

	yamuxClient, err := yamux.Client(clientPipe, cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = yamuxClient.Close() })

	_, cancel := context.WithCancel(context.Background())
	sess := &AgentSession{
		agentUUID: "type-uuid",
		agentName: "type-agent",
		session:   yamuxServer,
		cancel:    cancel,
	}
	t.Cleanup(func() { _ = sess.Close() })

	tcpServer, tcpClient := net.Pipe()
	t.Cleanup(func() {
		_ = tcpServer.Close()
		_ = tcpClient.Close()
	})
	go sess.proxyConn(tcpServer)

	stream, err := yamuxClient.Accept()
	require.NoError(t, err)
	defer func() { _ = stream.Close() }()

	buf := make([]byte, 1)
	_, err = io.ReadFull(stream, buf)
	require.NoError(t, err)
	assert.Equal(t, streamTypeDocker, buf[0])
}
