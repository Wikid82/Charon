package orthrus

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/hashicorp/yamux"
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

// TestAgentSession_ProxyConn_OpenFails verifies that proxyConn exits cleanly
// when the underlying yamux session is already closed and Open() returns an
// error.
func TestAgentSession_ProxyConn_OpenFails(t *testing.T) {
	serverPipe, clientPipe := net.Pipe()
	t.Cleanup(func() { _ = serverPipe.Close(); _ = clientPipe.Close() })

	cfg := yamux.DefaultConfig()
	cfg.LogOutput = io.Discard

	yamuxSrv, err := yamux.Server(serverPipe, cfg)
	require.NoError(t, err)

	_, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	sess := &AgentSession{
		agentUUID: "open-fail-uuid",
		agentName: "open-fail-agent",
		session:   yamuxSrv,
		cancel:    cancel,
	}

	// Close the yamux session so that Open() returns ErrSessionShutdown.
	require.NoError(t, yamuxSrv.Close())

	tcpServer, tcpClient := net.Pipe()
	t.Cleanup(func() { _ = tcpServer.Close(); _ = tcpClient.Close() })

	done := make(chan struct{})
	go func() {
		defer close(done)
		sess.proxyConn(tcpServer)
	}()

	select {
	case <-done:
		// proxyConn exited correctly after Open() failed.
	case <-time.After(2 * time.Second):
		t.Fatal("proxyConn did not exit after Open() failure")
	}
}

// TestAgentSession_ProxyConn_WriteFails verifies that proxyConn exits cleanly
// when stream.Write() fails to send the stream-type byte.
//
// Strategy: a goroutine reads exactly the 12-byte yamux SYN frame from the
// client side of the pipe (allowing Open() to succeed), then closes the
// connection so that the subsequent DATA write returns an error.
func TestAgentSession_ProxyConn_WriteFails(t *testing.T) {
	serverPipe, clientPipe := net.Pipe()
	t.Cleanup(func() { _ = serverPipe.Close(); _ = clientPipe.Close() })

	cfg := yamux.DefaultConfig()
	cfg.LogOutput = io.Discard
	cfg.EnableKeepAlive = false // prevent keepalive frames before our test

	yamuxSrv, err := yamux.Server(serverPipe, cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = yamuxSrv.Close() })

	_, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	sess := &AgentSession{
		agentUUID: "write-fail-uuid",
		agentName: "write-fail-agent",
		session:   yamuxSrv,
		cancel:    cancel,
	}

	// Consume the 12-byte yamux SYN frame (typeWindowUpdate | flagSYN) so
	// that Open() can complete, then close the pipe so the subsequent DATA
	// frame write fails.
	go func() {
		buf := make([]byte, 12) // yamux headerSize = 12 bytes
		_, _ = io.ReadFull(clientPipe, buf)
		_ = clientPipe.Close()
	}()

	tcpServer, tcpClient := net.Pipe()
	t.Cleanup(func() { _ = tcpServer.Close(); _ = tcpClient.Close() })

	done := make(chan struct{})
	go func() {
		defer close(done)
		sess.proxyConn(tcpServer)
	}()

	select {
	case <-done:
		// proxyConn exited correctly after stream.Write() failed.
	case <-time.After(2 * time.Second):
		t.Fatal("proxyConn did not exit after Write() failure")
	}
}
