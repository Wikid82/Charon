package orthrus

import (
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

	ln, lnErr := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, lnErr)
	t.Cleanup(func() { _ = ln.Close() })

	sess.mu.Lock()
	sess.proxyPort = 8080
	sess.listener = ln
	sess.mu.Unlock()

	addr := sess.GetProxyAddr()
	assert.Equal(t, "127.0.0.1:8080", addr)
}
