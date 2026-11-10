package handlers

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/services"
)

// newClosedLogSocketConn serves one WebSocket connection, closes it on the server
// side, and hands the closed connection to check. Deadline setters on a closed
// connection fail, which lets tests reach the error branches of the log socket helpers.
func newClosedLogSocketConn(t *testing.T, check func(conn *websocket.Conn)) {
	t.Helper()
	done := make(chan struct{})
	srv := newLogSocketTestServer(t, func(conn *websocket.Conn) {
		defer close(done)
		require.NoError(t, conn.Close())
		check(conn)
	})
	client := dialLogSocketTestServer(t, srv)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("server handler did not run")
	}
	_ = client.Close()
}

func TestConfigureLogSocket_ClosedConnectionKeepsGoingWithoutDeadlines(t *testing.T) {
	newClosedLogSocketConn(t, func(conn *websocket.Conn) {
		assert.NotPanics(t, func() { configureLogSocket(conn) })

		// The pong handler refreshes the read deadline, which fails on a closed connection.
		pong := conn.PongHandler()
		require.NotNil(t, pong)
		assert.Error(t, pong(""))
	})
}

func TestWriteLogSocketHelpers_ClosedConnectionReturnsWriteError(t *testing.T) {
	withLogSocketTimings(t, 60*time.Second, 100*time.Millisecond, 30*time.Second)

	newClosedLogSocketConn(t, func(conn *websocket.Conn) {
		require.Error(t, writeLogSocketJSON(conn, map[string]string{"data": "x"}))
		require.Error(t, writeLogSocketPing(conn))
	})
}

// TestLogsWSHandler_FailedKeepAlivePingEndsStream runs the stream with a write
// deadline that has already passed, so the keep-alive ping fails while the client
// stays connected. The handler must end the stream instead of looping.
func TestLogsWSHandler_FailedKeepAlivePingEndsStream(t *testing.T) {
	withLogSocketTimings(t, 60*time.Second, time.Nanosecond, 10*time.Millisecond)

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

	conn, resp, err := websocket.DefaultDialer.Dial(toWebSocketURL(srv.URL)+"/logs", nil)
	if resp != nil {
		_ = resp.Body.Close()
	}
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	t.Cleanup(func() { <-handlerDone })

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	_, _, err = conn.ReadMessage()
	require.Error(t, err, "the server must close the stream after the failed ping")
}

// TestCerberusLogsHandler_FailedKeepAlivePingEndsStream mirrors the logs stream
// test for the Cerberus log stream, whose keep-alive failure is logged before return.
func TestCerberusLogsHandler_FailedKeepAlivePingEndsStream(t *testing.T) {
	withLogSocketTimings(t, 60*time.Second, time.Nanosecond, 10*time.Millisecond)

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
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil) //nolint:bodyclose // WebSocket Dial response body is consumed by the dial
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	t.Cleanup(func() { <-handlerDone })
	if resp != nil {
		_ = resp.Body.Close()
	}

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	_, _, err = conn.ReadMessage()
	require.Error(t, err, "the server must close the stream after the failed ping")
}
