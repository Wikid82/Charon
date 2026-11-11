package handlers

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/services"
)

// newClosedLogSocketConn serves one WebSocket connection, closes it on the server
// side, and hands the closed connection to check. Writes and pings on a closed
// connection fail, which lets tests reach the error branches of the log socket helpers.
func newClosedLogSocketConn(t *testing.T, check func(conn *websocket.Conn)) {
	t.Helper()
	done := make(chan struct{})
	srv := newLogSocketTestServer(t, func(conn *websocket.Conn) {
		defer close(done)
		require.NoError(t, conn.CloseNow())
		check(conn)
	})
	client := dialLogSocketTestServer(t, srv)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("server handler did not run")
	}
	_ = client.CloseNow()
}

func TestConfigureLogSocket_ClosedConnectionDoesNotPanic(t *testing.T) {
	newClosedLogSocketConn(t, func(conn *websocket.Conn) {
		assert.NotPanics(t, func() { configureLogSocket(conn) })
	})
}

// TestWatchLogSocket_ClosedConnectionEndsImmediately covers a connection that is
// already closed when the watch starts. The watch must end without waiting for a
// ping period.
func TestWatchLogSocket_ClosedConnectionEndsImmediately(t *testing.T) {
	withLogSocketTimings(t, 60*time.Second, 10*time.Second, time.Hour)

	newClosedLogSocketConn(t, func(conn *websocket.Conn) {
		select {
		case <-watchLogSocketUntilDone(conn):
		case <-time.After(5 * time.Second):
			t.Fatal("watch did not end on a closed connection")
		}
	})
}

func TestWriteLogSocketHelpers_ClosedConnectionReturnsWriteError(t *testing.T) {
	withLogSocketTimings(t, 60*time.Second, 100*time.Millisecond, 30*time.Second)

	newClosedLogSocketConn(t, func(conn *websocket.Conn) {
		require.Error(t, writeLogSocketJSON(conn, map[string]string{"data": "x"}))
		require.Error(t, writeLogSocketText(conn, []byte("x")))
	})
}

// TestWriteLogSocketJSON_UnmarshalableValueFails covers a value encoding/json
// cannot marshal. The connection is never touched.
func TestWriteLogSocketJSON_UnmarshalableValueFails(t *testing.T) {
	newClosedLogSocketConn(t, func(conn *websocket.Conn) {
		require.Error(t, writeLogSocketJSON(conn, map[string]any{"bad": make(chan int)}))
	})
}

// TestLogsWSHandler_UnansweredPingEndsStream runs the stream with a pong deadline
// that passes before any pong can arrive. The client never reads, so no pong is
// sent. The handler must close the stream instead of looping.
func TestLogsWSHandler_UnansweredPingEndsStream(t *testing.T) {
	withLogSocketTimings(t, 50*time.Millisecond, time.Second, 10*time.Millisecond)

	// The handler reads the package timers, so the cleanup waits for it to return
	// before withLogSocketTimings restores the production values.
	handlerDone := make(chan struct{})
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/logs", func(c *gin.Context) {
		defer close(handlerDone)
		NewLogsWSHandler(nil).HandleWebSocket(c)
	})
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	conn, _, err := websocket.Dial(context.Background(), toWebSocketURL(srv.URL)+"/logs", nil) //nolint:bodyclose // coder/websocket manages the handshake response body
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.CloseNow() })
	t.Cleanup(func() { <-handlerDone })

	// Wait past the pong deadline before reading. Reading answers pings, so the
	// client must stay silent until the server has closed the stream.
	time.Sleep(200 * time.Millisecond)
	err = readUntilStreamEnds(t, conn)
	require.Error(t, err, "the server must close the stream after the unanswered ping")
}

// TestCerberusLogsHandler_UnansweredPingEndsStream mirrors the logs stream test
// for the Cerberus log stream, whose keep-alive failure ends the stream.
func TestCerberusLogsHandler_UnansweredPingEndsStream(t *testing.T) {
	withLogSocketTimings(t, 50*time.Millisecond, time.Second, 10*time.Millisecond)

	logPath := filepath.Join(t.TempDir(), "access.log")
	// #nosec G304 -- Test fixture uses controlled path from t.TempDir()
	require.NoError(t, os.WriteFile(logPath, nil, 0o600))

	watcher := services.NewLogWatcher(logPath)
	require.NoError(t, watcher.Start(context.Background()))
	t.Cleanup(watcher.Stop)

	// As above, the cleanup waits for the handler before the timers are restored.
	handlerDone := make(chan struct{})
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/ws", func(c *gin.Context) {
		defer close(handlerDone)
		NewCerberusLogsHandler(watcher, nil).LiveLogs(c)
	})
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	conn, _, err := websocket.Dial(context.Background(), wsURL, nil) //nolint:bodyclose // WebSocket Dial response body is consumed by the dial
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.CloseNow() })
	t.Cleanup(func() { <-handlerDone })

	// Wait past the pong deadline before reading. Reading answers pings, so the
	// client must stay silent until the server has closed the stream.
	time.Sleep(200 * time.Millisecond)
	err = readUntilStreamEnds(t, conn)
	require.Error(t, err, "the server must close the stream after the unanswered ping")
}
