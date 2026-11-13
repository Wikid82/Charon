package orthrus

import (
	"net"
	"testing"
	"time"

	"github.com/hashicorp/yamux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewYamuxServerConfig_Bounds checks the Charon-side yamux bounds. Charon
// never accepts agent-opened streams, so the accept backlog stays small.
func TestNewYamuxServerConfig_Bounds(t *testing.T) {
	cfg := newYamuxServerConfig()
	assert.Equal(t, 8, cfg.AcceptBacklog)
	require.NoError(t, yamux.VerifyConfig(cfg))
}

// TestListenExternal_CapsConcurrentConnections proves the external listener
// admits at most maxExternalProxyConns connections at once: the next Accept
// waits until one of the admitted connections closes.
func TestListenExternal_CapsConcurrentConnections(t *testing.T) {
	ln, err := listenExternal("127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()
	addr := ln.Addr().String()

	var clients []net.Conn
	defer func() {
		for _, c := range clients {
			_ = c.Close()
		}
	}()
	var admitted []net.Conn
	defer func() {
		for _, c := range admitted {
			_ = c.Close()
		}
	}()

	for i := 0; i < maxExternalProxyConns; i++ {
		c, dialErr := net.Dial("tcp", addr)
		require.NoError(t, dialErr)
		clients = append(clients, c)

		sc, acceptErr := ln.Accept()
		require.NoError(t, acceptErr)
		admitted = append(admitted, sc)
	}

	// One more client connects at TCP level; the server must not admit it yet.
	extra, err := net.Dial("tcp", addr)
	require.NoError(t, err)
	clients = append(clients, extra)

	admittedCh := make(chan error, 1)
	go func() {
		sc, acceptErr := ln.Accept()
		if acceptErr == nil {
			_ = sc.Close()
		}
		admittedCh <- acceptErr
	}()

	select {
	case <-admittedCh:
		t.Fatal("listener admitted a connection beyond maxExternalProxyConns")
	case <-time.After(150 * time.Millisecond):
	}

	// Closing one admitted connection frees a slot.
	require.NoError(t, admitted[0].Close())
	select {
	case acceptErr := <-admittedCh:
		assert.NoError(t, acceptErr)
	case <-time.After(5 * time.Second):
		t.Fatal("listener did not admit a connection after a slot was freed")
	}
	assert.Equal(t, 256, maxExternalProxyConns)
}
