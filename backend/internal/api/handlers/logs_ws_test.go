package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/logger"
)

func TestLogsWebSocketHandler_SuccessfulConnection(t *testing.T) {
	server := newWebSocketTestServer(t)

	conn := server.dial(t, "/logs/live")

	waitForListenerCount(t, server.hook, 1)
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte("hello")))
}

func TestLogsWebSocketHandler_ReceiveLogEntries(t *testing.T) {
	server := newWebSocketTestServer(t)
	conn := server.dial(t, "/logs/live")

	server.sendEntry(t, logrus.InfoLevel, "hello", logrus.Fields{"source": "api", "user": "alice"})

	received := readLogEntry(t, conn)
	assert.Equal(t, "info", received.Level)
	assert.Equal(t, "hello", received.Message)
	assert.Equal(t, "api", received.Source)
	assert.Equal(t, "alice", received.Fields["user"])
}

func TestLogsWebSocketHandler_LevelFilter(t *testing.T) {
	server := newWebSocketTestServer(t)
	conn := server.dial(t, "/logs/live?level=error")

	server.sendEntry(t, logrus.InfoLevel, "info", logrus.Fields{"source": "api"})
	server.sendEntry(t, logrus.ErrorLevel, "error", logrus.Fields{"source": "api"})

	received := readLogEntry(t, conn)
	assert.Equal(t, "error", received.Level)

	// Ensure no additional messages arrive
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(150*time.Millisecond)))
	_, _, err := conn.ReadMessage()
	assert.Error(t, err)
}

func TestLogsWebSocketHandler_SourceFilter(t *testing.T) {
	server := newWebSocketTestServer(t)
	conn := server.dial(t, "/logs/live?source=api")

	server.sendEntry(t, logrus.InfoLevel, "backend", logrus.Fields{"source": "backend"})
	server.sendEntry(t, logrus.InfoLevel, "api", logrus.Fields{"source": "api"})

	received := readLogEntry(t, conn)
	assert.Equal(t, "api", received.Source)
}

func TestLogsWebSocketHandler_CombinedFilters(t *testing.T) {
	server := newWebSocketTestServer(t)
	conn := server.dial(t, "/logs/live?level=error&source=api")

	server.sendEntry(t, logrus.WarnLevel, "warn api", logrus.Fields{"source": "api"})
	server.sendEntry(t, logrus.ErrorLevel, "error api", logrus.Fields{"source": "api"})
	server.sendEntry(t, logrus.ErrorLevel, "error ui", logrus.Fields{"source": "ui"})

	received := readLogEntry(t, conn)
	assert.Equal(t, "error api", received.Message)
	assert.Equal(t, "api", received.Source)
}

func TestLogsWebSocketHandler_CaseInsensitiveFilters(t *testing.T) {
	server := newWebSocketTestServer(t)
	conn := server.dial(t, "/logs/live?level=ERROR&source=API")

	server.sendEntry(t, logrus.ErrorLevel, "error api", logrus.Fields{"source": "api"})
	received := readLogEntry(t, conn)
	assert.Equal(t, "error api", received.Message)
	assert.Equal(t, "error", received.Level)
}

func TestLogsWebSocketHandler_UpgradeFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/logs/live", LogsWebSocketHandler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/logs/live", http.NoBody)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLogsWebSocketHandler_ClientDisconnect(t *testing.T) {
	server := newWebSocketTestServer(t)
	conn := server.dial(t, "/logs/live")

	waitForListenerCount(t, server.hook, 1)
	require.NoError(t, conn.Close())
	waitForListenerCount(t, server.hook, 0)
}

func TestLogsWebSocketHandler_ChannelClosed(t *testing.T) {
	server := newWebSocketTestServer(t)
	_ = server.dial(t, "/logs/live")

	ids := server.subscriberIDs(t)
	require.Len(t, ids, 1)

	server.hook.Unsubscribe(ids[0])
	waitForListenerCount(t, server.hook, 0)
}

func TestLogsWebSocketHandler_MultipleConnections(t *testing.T) {
	server := newWebSocketTestServer(t)
	const connCount = 5

	conns := make([]*websocket.Conn, 0, connCount)
	for i := 0; i < connCount; i++ {
		conns = append(conns, server.dial(t, "/logs/live"))
	}

	waitForListenerCount(t, server.hook, connCount)

	done := make(chan struct{})
	for _, conn := range conns {
		go func(c *websocket.Conn) {
			defer func() { done <- struct{}{} }()
			for {
				entry := readLogEntry(t, c)
				if entry.Message == "broadcast" {
					assert.Equal(t, "broadcast", entry.Message)
					return
				}
			}
		}(conn)
	}

	server.sendEntry(t, logrus.InfoLevel, "broadcast", logrus.Fields{"source": "api"})

	for i := 0; i < connCount; i++ {
		<-done
	}
}

func TestLogsWebSocketHandler_HighVolumeLogging(t *testing.T) {
	server := newWebSocketTestServer(t)
	conn := server.dial(t, "/logs/live")

	for i := 0; i < 200; i++ {
		server.sendEntry(t, logrus.InfoLevel, fmt.Sprintf("msg-%d", i), logrus.Fields{"source": "api"})
		received := readLogEntry(t, conn)
		assert.Equal(t, fmt.Sprintf("msg-%d", i), received.Message)
	}
}

func TestLogsWebSocketHandler_EmptyLogFields(t *testing.T) {
	server := newWebSocketTestServer(t)
	conn := server.dial(t, "/logs/live")

	server.sendEntry(t, logrus.InfoLevel, "no fields", nil)
	first := readLogEntry(t, conn)
	assert.Equal(t, "", first.Source)

	server.sendEntry(t, logrus.InfoLevel, "empty map", logrus.Fields{})
	second := readLogEntry(t, conn)
	assert.Equal(t, "", second.Source)
}

func TestLogsWebSocketHandler_SubscriberIDUniqueness(t *testing.T) {
	server := newWebSocketTestServer(t)
	_ = server.dial(t, "/logs/live")
	_ = server.dial(t, "/logs/live")

	waitForListenerCount(t, server.hook, 2)
	ids := server.subscriberIDs(t)
	require.Len(t, ids, 2)
	assert.NotEqual(t, ids[0], ids[1])
}

func TestLogsWebSocketHandler_WithRealLogger(t *testing.T) {
	server := newWebSocketTestServer(t)
	conn := server.dial(t, "/logs/live")

	loggerEntry := logger.Log().WithField("source", "api")
	loggerEntry.Info("from logger")

	received := readLogEntry(t, conn)
	assert.Equal(t, "from logger", received.Message)
	assert.Equal(t, "api", received.Source)
}

func TestLogsWebSocketHandler_ConnectionLifecycle(t *testing.T) {
	server := newWebSocketTestServer(t)
	conn := server.dial(t, "/logs/live")

	server.sendEntry(t, logrus.InfoLevel, "first", logrus.Fields{"source": "api"})
	first := readLogEntry(t, conn)
	assert.Equal(t, "first", first.Message)

	require.NoError(t, conn.Close())
	waitForListenerCount(t, server.hook, 0)

	// Ensure no panic when sending after disconnect
	server.sendEntry(t, logrus.InfoLevel, "after-close", logrus.Fields{"source": "api"})
}
