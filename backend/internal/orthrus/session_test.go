package orthrus

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

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

func TestStartExternalProxy_TransportDisablesKeepAlives(t *testing.T) {
	serverConn, done := testWSPair(t)
	defer done()

	sess, err := NewAgentSession("keepalive-uuid", "keepalive-agent", serverConn)
	require.NoError(t, err)
	defer func() { _ = sess.Close() }()

	var connCount atomic.Int32
	mock := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	mock.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			connCount.Add(1)
		}
	}
	mock.Start()
	defer mock.Close()

	mockPort := mock.Listener.Addr().(*net.TCPAddr).Port

	sess.mu.Lock()
	sess.proxyPort = mockPort
	sess.mu.Unlock()

	extPort := findFreePort(t)
	require.NoError(t, sess.StartExternalProxy(extPort))

	assert.Eventually(t, func() bool {
		return sess.GetExternalProxyStatus().Active
	}, 2*time.Second, 10*time.Millisecond)

	client := &http.Client{Timeout: 2 * time.Second}
	for i := 0; i < 2; i++ {
		resp, reqErr := client.Get(fmt.Sprintf("http://127.0.0.1:%d/containers/json", extPort))
		require.NoError(t, reqErr)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		_, _ = io.Copy(io.Discard, resp.Body)
		require.NoError(t, resp.Body.Close())
	}

	assert.Equal(t, int32(2), connCount.Load())
}
