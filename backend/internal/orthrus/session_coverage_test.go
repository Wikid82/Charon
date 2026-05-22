package orthrus

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWSNetConn_Write_Success(t *testing.T) {
	serverConn, done := testWSPair(t)
	defer done()

	nc := newWSNetConn(serverConn)
	n, err := nc.Write([]byte("hello"))
	assert.NoError(t, err)
	assert.Equal(t, 5, n)
}

func TestWSNetConn_LocalAddr_NotNil(t *testing.T) {
	serverConn, done := testWSPair(t)
	defer done()

	nc := newWSNetConn(serverConn)
	addr := nc.LocalAddr()
	assert.NotNil(t, addr)
	assert.Implements(t, (*net.Addr)(nil), addr)
}

func TestWSNetConn_RemoteAddr_NotNil(t *testing.T) {
	serverConn, done := testWSPair(t)
	defer done()

	nc := newWSNetConn(serverConn)
	addr := nc.RemoteAddr()
	assert.NotNil(t, addr)
	assert.Implements(t, (*net.Addr)(nil), addr)
}

func TestWSNetConn_SetDeadline_NoError(t *testing.T) {
	serverConn, done := testWSPair(t)
	defer done()

	nc := newWSNetConn(serverConn)
	// Zero time clears the deadline — should succeed on a live connection.
	err := nc.SetDeadline(time.Time{})
	assert.NoError(t, err)
}

func TestWSNetConn_SetReadDeadline_NoError(t *testing.T) {
	serverConn, done := testWSPair(t)
	defer done()

	nc := newWSNetConn(serverConn)
	err := nc.SetReadDeadline(time.Time{})
	assert.NoError(t, err)
}

func TestWSNetConn_SetWriteDeadline_NoError(t *testing.T) {
	serverConn, done := testWSPair(t)
	defer done()

	nc := newWSNetConn(serverConn)
	err := nc.SetWriteDeadline(time.Time{})
	assert.NoError(t, err)
}

func TestAgentSession_GetProxyAddr_WithPort(t *testing.T) {
	serverConn, done := testWSPair(t)
	defer done()

	sess, err := NewAgentSession("port-uuid", "port-agent", serverConn)
	require.NoError(t, err)
	defer func() { _ = sess.Close() }()

	sess.mu.Lock()
	sess.proxyPort = 8080
	sess.mu.Unlock()

	addr := sess.GetProxyAddr()
	assert.Equal(t, "127.0.0.1:8080", addr)
}

// TestWSNetConn_SetDeadline_ClosedConn_ReturnsError covers the error-return path
// inside SetDeadline when the underlying SetReadDeadline call fails on a closed conn.
func TestWSNetConn_SetDeadline_ClosedConn_ReturnsError(t *testing.T) {
	serverConn, done := testWSPair(t)
	defer done()

	nc := newWSNetConn(serverConn)
	// Closing the underlying WebSocket connection invalidates the TCP socket;
	// the subsequent SetReadDeadline call inside SetDeadline will return an error.
	_ = serverConn.Close()

	err := nc.SetDeadline(time.Now().Add(time.Second))
	assert.Error(t, err)
}

// TestGetExternalProxyStatus_ErrorFieldPopulated covers session.go:336-338 —
// the errStr assignment when extErr is non-nil.
func TestGetExternalProxyStatus_ErrorFieldPopulated(t *testing.T) {
	serverConn, done := testWSPair(t)
	defer done()

	sess, err := NewAgentSession("ext-err-uuid", "ext-err-agent", serverConn)
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
