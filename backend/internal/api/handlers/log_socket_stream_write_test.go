package handlers

import (
	"context"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	charonlogger "github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/services"
)

// TestLogsWSHandler_FailedEntryWriteEndsStream streams live log entries with a write
// deadline that has already passed. The first entry write fails and the handler must
// end the stream. Keep-alive pings are disabled so only the entry path can fail.
func TestLogsWSHandler_FailedEntryWriteEndsStream(t *testing.T) {
	withLogSocketTimings(t, 60*time.Second, time.Nanosecond, time.Hour)
	charonlogger.Init(false, io.Discard)

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

	// Log until the handler has subscribed and written, then wait for it to close the stream.
	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
	marker := "log-stream-write-" + uuid.NewString()
	go func() {
		for {
			select {
			case <-stop:
				return
			case <-time.After(10 * time.Millisecond):
				charonlogger.Log().Info(marker)
			}
		}
	}()

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	_, _, err = conn.ReadMessage()
	require.Error(t, err, "the server must close the stream after a failed entry write")
}

// TestCerberusLogsHandler_FailedEntryWriteEndsStream appends access log lines while the
// write deadline has already passed. The first entry write fails and the stream ends.
func TestCerberusLogsHandler_FailedEntryWriteEndsStream(t *testing.T) {
	withLogSocketTimings(t, 60*time.Second, time.Nanosecond, time.Hour)

	logPath := filepath.Join(t.TempDir(), "access.log")
	// #nosec G304 -- Test fixture uses controlled path from t.TempDir()
	file, err := os.Create(logPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = file.Close() })

	watcher := services.NewLogWatcher(logPath)
	require.NoError(t, watcher.Start(context.Background()))
	t.Cleanup(watcher.Stop)

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

	// Append lines until the stream ends. Every line is a valid access entry.
	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
	go func() {
		for {
			select {
			case <-stop:
				return
			case <-time.After(20 * time.Millisecond):
				caddyLog := `{"level":"info","ts":1,"logger":"http.log.access","status":200,"request":{"remote_ip":"10.0.0.1","method":"GET","uri":"/","host":"example.com"}}` + "\n"
				_, _ = file.WriteString(caddyLog)
			}
		}
	}()

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	_, _, err = conn.ReadMessage()
	require.Error(t, err, "the server must close the stream after a failed entry write")
}
