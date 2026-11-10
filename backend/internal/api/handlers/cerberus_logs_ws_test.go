package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

func init() {
}

// TestCerberusLogsHandler_NewHandler verifies handler creation.
func TestCerberusLogsHandler_NewHandler(t *testing.T) {
	t.Parallel()

	watcher := services.NewLogWatcher("/tmp/test.log")
	tracker := services.NewWebSocketTracker()
	handler := NewCerberusLogsHandler(watcher, tracker)

	assert.NotNil(t, handler)
	assert.Equal(t, watcher, handler.watcher)
	assert.Equal(t, tracker, handler.tracker)
}

// TestCerberusLogsHandler_SuccessfulConnection verifies WebSocket upgrade.
func TestCerberusLogsHandler_SuccessfulConnection(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "access.log")

	// Create the log file
	// #nosec G304 -- Test fixture file with controlled path
	_, err := os.Create(logPath)
	require.NoError(t, err)

	watcher := services.NewLogWatcher(logPath)
	err = watcher.Start(context.Background())
	require.NoError(t, err)
	defer watcher.Stop()

	handler := NewCerberusLogsHandler(watcher, nil)

	// Create test server
	router := gin.New()
	router.GET("/ws", handler.LiveLogs)
	server := httptest.NewServer(router)
	defer server.Close()

	// Convert HTTP URL to WebSocket URL
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	// Connect WebSocket
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	defer func() { _ = conn.Close() }()

	assert.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
}

// TestCerberusLogsHandler_ReceiveLogEntries verifies log streaming.
func TestCerberusLogsHandler_ReceiveLogEntries(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "access.log")

	// Create the log file
	// #nosec G304 -- Test fixture uses controlled path from t.TempDir()
	file, err := os.Create(logPath)
	require.NoError(t, err)
	defer func() { _ = file.Close() }()

	watcher := services.NewLogWatcher(logPath)
	err = watcher.Start(context.Background())
	require.NoError(t, err)
	defer watcher.Stop()

	handler := NewCerberusLogsHandler(watcher, nil)

	// Create test server
	router := gin.New()
	router.GET("/ws", handler.LiveLogs)
	server := httptest.NewServer(router)
	defer server.Close()

	// Connect WebSocket
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil) //nolint:bodyclose // WebSocket Dial response body is consumed by the dial
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	// Give the subscription time to register and watcher to seek to end
	time.Sleep(300 * time.Millisecond)

	// Write a log entry
	caddyLog := models.CaddyAccessLog{
		Level:  "info",
		Ts:     float64(time.Now().Unix()),
		Logger: "http.log.access",
		Msg:    "handled request",
		Status: 200,
	}
	caddyLog.Request.RemoteIP = "10.0.0.1"
	caddyLog.Request.Method = "GET"
	caddyLog.Request.URI = "/test"
	caddyLog.Request.Host = "example.com"

	logJSON, err := json.Marshal(caddyLog)
	require.NoError(t, err)
	_, err = file.WriteString(string(logJSON) + "\n")
	require.NoError(t, err)
	_ = file.Sync()

	// Read the entry from WebSocket
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	require.NoError(t, err)

	var entry models.SecurityLogEntry
	err = json.Unmarshal(msg, &entry)
	require.NoError(t, err)

	assert.Equal(t, "10.0.0.1", entry.ClientIP)
	assert.Equal(t, "GET", entry.Method)
	assert.Equal(t, "/test", entry.URI)
	assert.Equal(t, 200, entry.Status)
	assert.Equal(t, "normal", entry.Source)
	assert.False(t, entry.Blocked)
}

// TestCerberusLogsHandler_SourceFilter verifies source filtering.
func TestCerberusLogsHandler_SourceFilter(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "access.log")

	// #nosec G304 -- Test fixture uses controlled path from t.TempDir()
	file, err := os.Create(logPath)
	require.NoError(t, err)
	defer func() { _ = file.Close() }()

	watcher := services.NewLogWatcher(logPath)
	err = watcher.Start(context.Background())
	require.NoError(t, err)
	defer watcher.Stop()

	handler := NewCerberusLogsHandler(watcher, nil)

	router := gin.New()
	router.GET("/ws", handler.LiveLogs)
	server := httptest.NewServer(router)
	defer server.Close()

	// Connect with WAF source filter
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?source=waf"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil) //nolint:bodyclose // WebSocket Dial response body is consumed by the dial
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	time.Sleep(300 * time.Millisecond)

	// Write a normal request (should be filtered out)
	normalLog := models.CaddyAccessLog{
		Level:  "info",
		Ts:     float64(time.Now().Unix()),
		Logger: "http.log.access",
		Msg:    "handled request",
		Status: 200,
	}
	normalLog.Request.RemoteIP = "10.0.0.1"
	normalLog.Request.Method = "GET"
	normalLog.Request.URI = "/normal"
	normalLog.Request.Host = "example.com"

	normalJSON, _ := json.Marshal(normalLog)
	_, _ = file.WriteString(string(normalJSON) + "\n")

	// Write a WAF blocked request (should pass filter)
	wafLog := models.CaddyAccessLog{
		Level:       "info",
		Ts:          float64(time.Now().Unix()),
		Logger:      "http.handlers.waf",
		Msg:         "request blocked",
		Status:      403,
		RespHeaders: map[string][]string{"X-Coraza-Id": {"942100"}},
	}
	wafLog.Request.RemoteIP = "10.0.0.2"
	wafLog.Request.Method = "POST"
	wafLog.Request.URI = "/admin"
	wafLog.Request.Host = "example.com"

	wafJSON, _ := json.Marshal(wafLog)
	_, _ = file.WriteString(string(wafJSON) + "\n")
	_ = file.Sync()

	// Read from WebSocket - should only get WAF entry
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	require.NoError(t, err)

	var entry models.SecurityLogEntry
	err = json.Unmarshal(msg, &entry)
	require.NoError(t, err)

	assert.Equal(t, "waf", entry.Source)
	assert.Equal(t, "10.0.0.2", entry.ClientIP)
	assert.True(t, entry.Blocked)
}

// TestCerberusLogsHandler_BlockedOnlyFilter verifies blocked_only filtering.
func TestCerberusLogsHandler_BlockedOnlyFilter(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "access.log")

	// #nosec G304 -- Test fixture uses controlled path from t.TempDir()
	file, err := os.Create(logPath)
	require.NoError(t, err)
	defer func() { _ = file.Close() }()

	watcher := services.NewLogWatcher(logPath)
	err = watcher.Start(context.Background())
	require.NoError(t, err)
	defer watcher.Stop()

	handler := NewCerberusLogsHandler(watcher, nil)

	router := gin.New()
	router.GET("/ws", handler.LiveLogs)
	server := httptest.NewServer(router)
	defer server.Close()

	// Connect with blocked_only filter
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?blocked_only=true"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil) //nolint:bodyclose // WebSocket Dial response body is consumed by the dial
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	time.Sleep(300 * time.Millisecond)

	// Write a normal 200 request (should be filtered out)
	normalLog := models.CaddyAccessLog{
		Level:  "info",
		Ts:     float64(time.Now().Unix()),
		Logger: "http.log.access",
		Msg:    "handled request",
		Status: 200,
	}
	normalLog.Request.RemoteIP = "10.0.0.1"
	normalLog.Request.Method = "GET"
	normalLog.Request.URI = "/ok"
	normalLog.Request.Host = "example.com"

	normalJSON, _ := json.Marshal(normalLog)
	_, _ = file.WriteString(string(normalJSON) + "\n")

	// Write a rate limited request (should pass filter)
	blockedLog := models.CaddyAccessLog{
		Level:  "info",
		Ts:     float64(time.Now().Unix()),
		Logger: "http.log.access",
		Msg:    "handled request",
		Status: 429,
	}
	blockedLog.Request.RemoteIP = "10.0.0.2"
	blockedLog.Request.Method = "GET"
	blockedLog.Request.URI = "/limited"
	blockedLog.Request.Host = "example.com"

	blockedJSON, _ := json.Marshal(blockedLog)
	_, _ = file.WriteString(string(blockedJSON) + "\n")
	_ = file.Sync()

	// Read from WebSocket - should only get blocked entry
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	require.NoError(t, err)

	var entry models.SecurityLogEntry
	err = json.Unmarshal(msg, &entry)
	require.NoError(t, err)

	assert.True(t, entry.Blocked)
	assert.Equal(t, "ratelimit", entry.Source)
}

// TestCerberusLogsHandler_IPFilter verifies IP filtering.
func TestCerberusLogsHandler_IPFilter(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "access.log")
	// #nosec G304 -- Test fixture uses controlled path from t.TempDir()
	file, err := os.Create(logPath)
	require.NoError(t, err)
	defer func() { _ = file.Close() }()

	watcher := services.NewLogWatcher(logPath)
	err = watcher.Start(context.Background())
	require.NoError(t, err)
	defer watcher.Stop()

	handler := NewCerberusLogsHandler(watcher, nil)

	router := gin.New()
	router.GET("/ws", handler.LiveLogs)
	server := httptest.NewServer(router)
	defer server.Close()

	// Connect with IP filter
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?ip=192.168"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil) //nolint:bodyclose // WebSocket Dial response body is consumed by the dial
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	time.Sleep(300 * time.Millisecond)

	// Write request from non-matching IP
	log1 := models.CaddyAccessLog{
		Level:  "info",
		Ts:     float64(time.Now().Unix()),
		Logger: "http.log.access",
		Msg:    "handled request",
		Status: 200,
	}
	log1.Request.RemoteIP = "10.0.0.1"
	log1.Request.Method = "GET"
	log1.Request.URI = "/test1"
	log1.Request.Host = "example.com"

	json1, _ := json.Marshal(log1)
	_, _ = file.WriteString(string(json1) + "\n")

	// Write request from matching IP
	log2 := models.CaddyAccessLog{
		Level:  "info",
		Ts:     float64(time.Now().Unix()),
		Logger: "http.log.access",
		Msg:    "handled request",
		Status: 200,
	}
	log2.Request.RemoteIP = "192.168.1.100"
	log2.Request.Method = "POST"
	log2.Request.URI = "/test2"
	log2.Request.Host = "example.com"

	json2, _ := json.Marshal(log2)
	_, _ = file.WriteString(string(json2) + "\n")
	_ = file.Sync()

	// Read from WebSocket - should only get matching IP entry
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	require.NoError(t, err)

	var entry models.SecurityLogEntry
	err = json.Unmarshal(msg, &entry)
	require.NoError(t, err)

	assert.Equal(t, "192.168.1.100", entry.ClientIP)
}

// TestCerberusLogsHandler_ClientDisconnect verifies cleanup on disconnect.
func TestCerberusLogsHandler_ClientDisconnect(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "access.log")

	_, err := os.Create(logPath) //nolint:gosec // G304: Test file in temp directory
	require.NoError(t, err)

	watcher := services.NewLogWatcher(logPath)
	err = watcher.Start(context.Background())
	require.NoError(t, err)
	defer watcher.Stop()

	handler := NewCerberusLogsHandler(watcher, nil)

	router := gin.New()
	router.GET("/ws", handler.LiveLogs)
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil) //nolint:bodyclose // WebSocket Dial response body is consumed by the dial
	require.NoError(t, err)

	// Close the connection
	_ = conn.Close()

	// Give time for cleanup
	time.Sleep(100 * time.Millisecond)

	// Should not panic or leave dangling goroutines
}

// TestCerberusLogsHandler_MultipleClients verifies multiple concurrent clients.
func TestCerberusLogsHandler_MultipleClients(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "access.log")

	file, err := os.Create(logPath) //nolint:gosec // G304: Test file in temp directory
	require.NoError(t, err)
	defer func() { _ = file.Close() }()

	watcher := services.NewLogWatcher(logPath)
	err = watcher.Start(context.Background())
	require.NoError(t, err)
	defer watcher.Stop()

	handler := NewCerberusLogsHandler(watcher, nil)

	router := gin.New()
	router.GET("/ws", handler.LiveLogs)
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	// Connect multiple clients
	conns := make([]*websocket.Conn, 3)
	defer func() {
		// Close all connections after test
		for _, conn := range conns {
			if conn != nil {
				_ = conn.Close()
			}
		}
	}()
	for i := 0; i < 3; i++ {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil) //nolint:bodyclose // WebSocket Dial response body is consumed by the dial
		require.NoError(t, err)
		conns[i] = conn
	}

	time.Sleep(300 * time.Millisecond)

	// Write a log entry
	logEntry := models.CaddyAccessLog{
		Level:  "info",
		Ts:     float64(time.Now().Unix()),
		Logger: "http.log.access",
		Msg:    "handled request",
		Status: 200,
	}
	logEntry.Request.RemoteIP = "10.0.0.1"
	logEntry.Request.Method = "GET"
	logEntry.Request.URI = "/multi"
	logEntry.Request.Host = "example.com"

	logJSON, _ := json.Marshal(logEntry)
	_, _ = file.WriteString(string(logJSON) + "\n")
	_ = file.Sync()

	// All clients should receive the entry
	for i, conn := range conns {
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, msg, err := conn.ReadMessage()
		require.NoError(t, err, "Client %d should receive message", i)

		var entry models.SecurityLogEntry
		err = json.Unmarshal(msg, &entry)
		require.NoError(t, err)
		assert.Equal(t, "/multi", entry.URI)
	}
}

// TestCerberusLogsHandler_UpgradeFailure verifies non-WebSocket request handling.
func TestCerberusLogsHandler_UpgradeFailure(t *testing.T) {
	t.Parallel()

	watcher := services.NewLogWatcher("/tmp/test.log")
	handler := NewCerberusLogsHandler(watcher, nil)

	router := gin.New()
	router.GET("/ws", handler.LiveLogs)

	// Make a regular HTTP request (not WebSocket)
	req := httptest.NewRequest(http.MethodGet, "/ws", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should fail upgrade (400 Bad Request)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// wsTestToken is a credential-like value used to check that it never reaches connection info or streamed entries.
// It is generated per run so no credential-shaped literal lives in the source.
var wsTestToken = "tok-" + uuid.NewString()

// TestCerberusLogsHandler_ConnectionListOmitsToken verifies the connection list does not expose the query token.
func TestCerberusLogsHandler_ConnectionListOmitsToken(t *testing.T) {
	t.Parallel()

	logPath := filepath.Join(t.TempDir(), "access.log")
	// #nosec G304 -- Test fixture uses controlled path from t.TempDir()
	require.NoError(t, os.WriteFile(logPath, nil, 0o600))

	watcher := services.NewLogWatcher(logPath)
	require.NoError(t, watcher.Start(context.Background()))
	defer watcher.Stop()

	tracker := services.NewWebSocketTracker()
	handler := NewCerberusLogsHandler(watcher, tracker)

	router := gin.New()
	router.GET("/ws", handler.LiveLogs)
	router.GET("/websocket/connections", NewWebSocketStatusHandler(tracker).GetConnections)
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?source=waf&token=" + wsTestToken
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil) //nolint:bodyclose // WebSocket Dial response body is consumed by the dial
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}

	waitFor(t, func() bool { return tracker.GetCount() == 1 })

	statusResp, err := http.Get(server.URL + "/websocket/connections") //nolint:noctx // test-only request against httptest server
	require.NoError(t, err)
	defer func() { _ = statusResp.Body.Close() }()
	body, err := io.ReadAll(statusResp.Body)
	require.NoError(t, err)

	assert.NotContains(t, string(body), wsTestToken)
	assert.Contains(t, string(body), "source=waf")
}

// TestCerberusLogsHandler_StreamedEntryOmitsToken verifies a streamed entry has the query token removed from its URI.
func TestCerberusLogsHandler_StreamedEntryOmitsToken(t *testing.T) {
	t.Parallel()

	logPath := filepath.Join(t.TempDir(), "access.log")
	// #nosec G304 -- Test fixture uses controlled path from t.TempDir()
	file, err := os.Create(logPath)
	require.NoError(t, err)
	defer func() { _ = file.Close() }()

	watcher := services.NewLogWatcher(logPath)
	require.NoError(t, watcher.Start(context.Background()))
	defer watcher.Stop()

	router := gin.New()
	router.GET("/ws", NewCerberusLogsHandler(watcher, nil).LiveLogs)
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil) //nolint:bodyclose // WebSocket Dial response body is consumed by the dial
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}

	time.Sleep(300 * time.Millisecond)

	caddyLog := models.CaddyAccessLog{Level: "info", Ts: float64(time.Now().Unix()), Logger: "http.log.access", Status: 200}
	caddyLog.Request.RemoteIP = "10.0.0.1"
	caddyLog.Request.Method = "GET"
	caddyLog.Request.URI = "/api/items?token=" + wsTestToken + "&page=2"
	caddyLog.Request.Host = "example.com"

	logJSON, err := json.Marshal(caddyLog)
	require.NoError(t, err)
	_, err = file.WriteString(string(logJSON) + "\n")
	require.NoError(t, err)
	require.NoError(t, file.Sync())

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, msg, err := conn.ReadMessage()
	require.NoError(t, err)

	assert.NotContains(t, string(msg), wsTestToken)

	var entry models.SecurityLogEntry
	require.NoError(t, json.Unmarshal(msg, &entry))
	assert.Equal(t, "/api/items?page=2", entry.URI)
}
