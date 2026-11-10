package orthrus

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewWSNetConn_WriteSendsOneBinaryMessage checks that each Write on the
// adapter reaches the peer as one binary WebSocket message.
func TestNewWSNetConn_WriteSendsOneBinaryMessage(t *testing.T) {
	serverConn, clientConn, done := testWSPairBoth(t)
	defer done()

	nc := newWSNetConn(context.Background(), serverConn)
	n, err := nc.Write([]byte("hello"))
	require.NoError(t, err)
	assert.Equal(t, 5, n)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	typ, data, err := clientConn.Read(ctx)
	require.NoError(t, err)
	assert.Equal(t, websocket.MessageBinary, typ)
	assert.Equal(t, []byte("hello"), data)
}

func TestNewWSNetConn_LocalAddr_NotNil(t *testing.T) {
	serverConn, done := testWSPair(t)
	defer done()

	nc := newWSNetConn(context.Background(), serverConn)
	addr := nc.LocalAddr()
	assert.NotNil(t, addr)
	assert.Implements(t, (*net.Addr)(nil), addr)
}

func TestNewWSNetConn_RemoteAddr_NotNil(t *testing.T) {
	serverConn, done := testWSPair(t)
	defer done()

	nc := newWSNetConn(context.Background(), serverConn)
	addr := nc.RemoteAddr()
	assert.NotNil(t, addr)
	assert.Implements(t, (*net.Addr)(nil), addr)
}

func TestNewWSNetConn_Deadlines_NoError(t *testing.T) {
	serverConn, done := testWSPair(t)
	defer done()

	nc := newWSNetConn(context.Background(), serverConn)
	// Zero time clears the deadline — should succeed on a live connection.
	assert.NoError(t, nc.SetDeadline(time.Time{}))
	assert.NoError(t, nc.SetReadDeadline(time.Time{}))
	assert.NoError(t, nc.SetWriteDeadline(time.Time{}))
}

func TestAgentSession_GetProxyAddr_WithPort(t *testing.T) {
	serverConn, done := testWSPair(t)
	defer done()

	sess, err := NewAgentSession("port-uuid", "port-agent", false, nil, serverConn)
	require.NoError(t, err)
	defer func() { _ = sess.Close() }()

	sess.mu.Lock()
	sess.proxyPort = 8080
	sess.mu.Unlock()

	addr := sess.GetProxyAddr()
	assert.Equal(t, "127.0.0.1:8080", addr)
}

// TestGetExternalProxyStatus_ErrorFieldPopulated covers session.go:336-338 —
// the errStr assignment when extErr is non-nil.
func TestGetExternalProxyStatus_ErrorFieldPopulated(t *testing.T) {
	serverConn, done := testWSPair(t)
	defer done()

	sess, err := NewAgentSession("ext-err-uuid", "ext-err-agent", false, nil, serverConn)
	require.NoError(t, err)
	defer func() { _ = sess.Close() }()

	bindErr := errors.New("bind failed: address already in use")
	sess.mu.Lock()
	sess.extProxyPort = 9999
	sess.extErr = bindErr
	sess.mu.Unlock()

	status := sess.GetExternalProxyStatus()
	assert.Equal(t, bindErr.Error(), status.Error)
	assert.Equal(t, 9999, status.ConfiguredPort)
	assert.False(t, status.Active)
}

// TestStartExternalProxy_SessionClosed covers session.go:268-271 —
// the IsClosed guard inside StartExternalProxy.
func TestStartExternalProxy_SessionClosed(t *testing.T) {
	sess, _, cleanup := sessionWithLoopback(t)
	defer cleanup()

	require.NoError(t, sess.Close())

	port := findFreePort(t)
	err := sess.StartExternalProxy(port)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot start external proxy on closed session")
}

// TestStartExternalProxy_BindFailure covers session.go:276-282 —
// the error path when net.Listen fails on an already-bound port.
func TestStartExternalProxy_BindFailure(t *testing.T) {
	sess, _, cleanup := sessionWithLoopback(t)
	defer cleanup()

	// Occupy a port to force the bind to fail.
	blocker, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer blocker.Close() //nolint:errcheck
	port := blocker.Addr().(*net.TCPAddr).Port

	err = sess.StartExternalProxy(port)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bind external proxy port")

	status := sess.GetExternalProxyStatus()
	assert.False(t, status.Active)
	assert.NotEmpty(t, status.Error)
	assert.Equal(t, port, status.ConfiguredPort)
}
