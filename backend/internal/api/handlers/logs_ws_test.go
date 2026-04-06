package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	charonlogger "github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func toWebSocketURL(httpURL string) string {
	return "ws" + strings.TrimPrefix(httpURL, "http")
}

func waitFor(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("condition not met within %s", timeout)
}

func TestUpgraderCheckOrigin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		origin         string
		host           string
		xForwardedHost string
		want           bool
	}{
		{"empty origin allows request", "", "example.com", "", true},
		{"invalid URL origin rejects", "://bad-url", "example.com", "", false},
		{"matching host allows", "http://example.com", "example.com", "", true},
		{"non-matching host rejects", "http://evil.com", "example.com", "", false},
		{"X-Forwarded-Host matching allows", "http://proxy.example.com", "backend.internal", "proxy.example.com", true},
		{"X-Forwarded-Host non-matching rejects", "http://evil.com", "backend.internal", "proxy.example.com", false},
		{"origin with port matching", "http://example.com:8080", "example.com:8080", "", true},
		{"origin with port non-matching", "http://example.com:9090", "example.com:8080", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, "/ws", http.NoBody)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			req.Host = tc.host
			if tc.xForwardedHost != "" {
				req.Header.Set("X-Forwarded-Host", tc.xForwardedHost)
			}
			got := upgrader.CheckOrigin(req)
			assert.Equal(t, tc.want, got, "origin=%q host=%q xfh=%q", tc.origin, tc.host, tc.xForwardedHost)
		})
	}
}

func TestLogsWebSocketHandler_DeprecatedWrapperUpgradeFailure(t *testing.T) {
	charonlogger.Init(false, io.Discard)

	r := gin.New()
	r.GET("/logs", LogsWebSocketHandler)

	req := httptest.NewRequest(http.MethodGet, "/logs", http.NoBody)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	assert.NotEqual(t, http.StatusSwitchingProtocols, res.Code)
}

func TestLogsWSHandler_StreamWithFiltersAndTracker(t *testing.T) {
	charonlogger.Init(false, io.Discard)

	tracker := services.NewWebSocketTracker()
	handler := NewLogsWSHandler(tracker)

	r := gin.New()
	r.GET("/logs", handler.HandleWebSocket)

	srv := httptest.NewServer(r)
	defer srv.Close()

	wsURL := toWebSocketURL(srv.URL) + "/logs?level=error&source=api"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)

	waitFor(t, 2*time.Second, func() bool {
		return tracker.GetCount() == 1
	})

	charonlogger.WithFields(map[string]any{"source": "api"}).Info("should-be-filtered-by-level")
	charonlogger.WithFields(map[string]any{"source": "worker"}).Error("should-be-filtered-by-source")
	charonlogger.WithFields(map[string]any{"source": "api"}).Error("should-pass-filters")

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(3*time.Second)))
	_, payload, err := conn.ReadMessage()
	require.NoError(t, err)

	var entry LogEntry
	require.NoError(t, json.Unmarshal(payload, &entry))
	assert.Equal(t, "error", entry.Level)
	assert.Equal(t, "should-pass-filters", entry.Message)
	assert.Equal(t, "api", entry.Source)
	assert.NotEmpty(t, entry.Timestamp)
	require.NotNil(t, entry.Fields)
	assert.Equal(t, "api", entry.Fields["source"])

	require.NoError(t, conn.Close())

	waitFor(t, 2*time.Second, func() bool {
		return tracker.GetCount() == 0
	})
}
