package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/logger"
)

// webSocketTestServer wraps a test HTTP server and broadcast hook for WebSocket tests.
type webSocketTestServer struct {
	server *httptest.Server
	url    string
	hook   *logger.BroadcastHook
}

// resetLogger reinitializes the global logger with an in-memory buffer to avoid cross-test leakage.
func resetLogger(t *testing.T) *logger.BroadcastHook {
	t.Helper()
	var buf bytes.Buffer
	logger.Init(true, &buf)
	return logger.GetBroadcastHook()
}

// newWebSocketTestServer builds a gin router exposing the WebSocket handler and starts an httptest server.
func newWebSocketTestServer(t *testing.T) *webSocketTestServer {
	t.Helper()
	gin.SetMode(gin.TestMode)
	hook := resetLogger(t)

	router := gin.New()
	router.GET("/logs/live", LogsWebSocketHandler)

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	return &webSocketTestServer{server: srv, url: wsURL, hook: hook}
}

// dial opens a WebSocket connection to the provided path and asserts upgrade success.
func (s *webSocketTestServer) dial(t *testing.T, path string) *websocket.Conn {
	t.Helper()
	conn, resp, err := websocket.DefaultDialer.Dial(s.url+path, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
	t.Cleanup(func() {
		_ = resp.Body.Close()
	})
	conn.SetReadLimit(1 << 20)
	t.Cleanup(func() {
		_ = conn.Close()
	})
	return conn
}

// sendEntry broadcasts a log entry through the shared hook.
func (s *webSocketTestServer) sendEntry(t *testing.T, lvl logrus.Level, msg string, fields logrus.Fields) {
	t.Helper()
	entry := &logrus.Entry{
		Level:   lvl,
		Message: msg,
		Time:    time.Now().UTC(),
		Data:    fields,
	}
	require.NoError(t, s.hook.Fire(entry))
}

// readLogEntry reads a LogEntry from the WebSocket with a short deadline to avoid flakiness.
func readLogEntry(t *testing.T, conn *websocket.Conn) LogEntry {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	var entry LogEntry
	require.NoError(t, conn.ReadJSON(&entry))
	return entry
}

// waitForListenerCount waits until the broadcast hook reports the desired listener count.
func waitForListenerCount(t *testing.T, hook *logger.BroadcastHook, expected int) {
	t.Helper()
	require.Eventually(t, func() bool {
		return hook.ActiveListeners() == expected
	}, 2*time.Second, 20*time.Millisecond)
}

// subscriberIDs introspects the broadcast hook to return the active subscriber IDs.
func (s *webSocketTestServer) subscriberIDs(t *testing.T) []string {
	t.Helper()
	return s.hook.ListenerIDs()
}
