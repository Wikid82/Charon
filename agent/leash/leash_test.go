package leash
package leash_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/agent/leash"
)

var testUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// TestLeash_Reconnect verifies that the Leash attempts to reconnect after the server
// closes the connection immediately.
func TestLeash_Reconnect(t *testing.T) {
	connectCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connectCount++
		conn, err := testUpgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		conn.Close()
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

	assert.GreaterOrEqual(t, connectCount, 2, "expected at least 2 connection attempts within the timeout")
}

// TestWsNetConn_ReadWrite verifies that the net.Conn wrapper correctly transfers
// binary data through a WebSocket echo server.
func TestWsNetConn_ReadWrite(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testUpgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()

		_, msg, err := conn.ReadMessage()
		require.NoError(t, err)
		require.NoError(t, conn.WriteMessage(websocket.BinaryMessage, msg))
	}))
	defer srv.Close()

	dialer := websocket.Dialer{}
	wsConn, _, err := dialer.Dial("ws"+srv.URL[4:], nil)
	require.NoError(t, err)
	defer wsConn.Close()

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
