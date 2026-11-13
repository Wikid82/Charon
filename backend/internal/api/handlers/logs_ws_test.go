package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	charonlogger "github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func toWebSocketURL(httpURL string) string {
	return "ws" + strings.TrimPrefix(httpURL, "http")
}

// readWithin reads one message, giving up after timeout. A coder Read that hits its
// deadline closes the connection, so callers must not expect the connection to
// survive a timed-out read.
func readWithin(t *testing.T, conn *websocket.Conn, timeout time.Duration) ([]byte, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_, data, err := conn.Read(ctx)
	return data, err
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("condition not met within %s", 2*time.Second)
}

func TestIsAllowedWebSocketOrigin(t *testing.T) {
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
			got := isAllowedWebSocketOrigin(req)
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
	conn, _, err := websocket.Dial(context.Background(), wsURL, nil) //nolint:bodyclose // coder/websocket manages the handshake response body
	require.NoError(t, err)

	waitFor(t, func() bool {
		return tracker.GetCount() == 1
	})

	charonlogger.WithFields(map[string]any{"source": "api"}).Info("should-be-filtered-by-level")
	charonlogger.WithFields(map[string]any{"source": "worker"}).Error("should-be-filtered-by-source")
	charonlogger.WithFields(map[string]any{"source": "api"}).Error("should-pass-filters")

	payload, err := readWithin(t, conn, 3*time.Second)
	require.NoError(t, err)

	var entry LogEntry
	require.NoError(t, json.Unmarshal(payload, &entry))
	assert.Equal(t, "error", entry.Level)
	assert.Equal(t, "should-pass-filters", entry.Message)
	assert.Equal(t, "api", entry.Source)
	assert.NotEmpty(t, entry.Timestamp)
	require.NotNil(t, entry.Fields)
	assert.Equal(t, "api", entry.Fields["source"])

	require.NoError(t, conn.CloseNow())

	waitFor(t, func() bool {
		return tracker.GetCount() == 0
	})
}

// TestLogsWSHandler_ConnectionListOmitsToken verifies the logs socket connection list does not expose the query token.
func TestLogsWSHandler_ConnectionListOmitsToken(t *testing.T) {
	charonlogger.Init(false, io.Discard)

	tracker := services.NewWebSocketTracker()
	r := gin.New()
	r.GET("/logs", NewLogsWSHandler(tracker).HandleWebSocket)
	r.GET("/websocket/connections", NewWebSocketStatusHandler(tracker).GetConnections)

	srv := httptest.NewServer(r)
	defer srv.Close()

	wsURL := toWebSocketURL(srv.URL) + "/logs?level=error&source=api&token=" + wsTestToken
	conn, _, err := websocket.Dial(context.Background(), wsURL, nil) //nolint:bodyclose // coder/websocket manages the handshake response body
	require.NoError(t, err)
	defer func() { _ = conn.CloseNow() }()

	waitFor(t, func() bool { return tracker.GetCount() == 1 })

	statusResp, err := http.Get(srv.URL + "/websocket/connections") //nolint:noctx // test-only request against httptest server
	require.NoError(t, err)
	defer func() { _ = statusResp.Body.Close() }()
	body, err := io.ReadAll(statusResp.Body)
	require.NoError(t, err)

	assert.NotContains(t, string(body), wsTestToken)
	assert.Contains(t, string(body), "source=api")
}

// streamEndTimeout bounds how long readUntilStreamEnds waits for a stream to end.
const streamEndTimeout = 5 * time.Second

// readUntilStreamEnds reads messages until a read fails and returns that error.
// Frames queued before the close, such as log lines, are read and discarded. It
// fails the test if the stream is still open after timeout.
func readUntilStreamEnds(t *testing.T, conn *websocket.Conn) error {
	t.Helper()
	deadline := time.Now().Add(streamEndTimeout)
	for time.Now().Before(deadline) {
		if _, err := readWithin(t, conn, time.Until(deadline)); err != nil {
			return err
		}
	}
	t.Fatalf("stream still open after %s", streamEndTimeout)
	return nil
}
