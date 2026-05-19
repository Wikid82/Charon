package orthrus

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
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
