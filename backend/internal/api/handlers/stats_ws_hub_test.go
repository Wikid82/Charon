package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// newHubServer creates an httptest.Server that upgrades connections and routes them
// through the given hub's ServeClient. Use for integration-style hub tests.
func newHubServer(t *testing.T, hub *StatsWSHub) *httptest.Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/ws", func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil) // nosemgrep: go.gorilla.security.audit.websocket-missing-origin-check.websocket-missing-origin-check
		if err != nil {
			return
		}
		client := &statsWSClient{
			conn: conn,
			send: make(chan []byte, statsWSWriteBuffer),
		}
		hub.ServeClient(client)
	})
	return httptest.NewServer(r)
}

// dialHub connects a WebSocket client to the test server.
func dialHub(t *testing.T, srv *httptest.Server) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil) //nolint:bodyclose
	require.NoError(t, err)
	return conn
}

// TestNewStatsWSHub verifies constructor initialises all channels and the client map.
func TestNewStatsWSHub(t *testing.T) {
	t.Parallel()
	hub := NewStatsWSHub()
	require.NotNil(t, hub)
	assert.NotNil(t, hub.clients)
	assert.NotNil(t, hub.broadcast)
	assert.NotNil(t, hub.register)
	assert.NotNil(t, hub.unregister)
}

// TestStatsWSHub_Broadcast_NonBlockingWhenFull verifies that Broadcast never blocks
// even when the internal broadcast channel is at capacity.
func TestStatsWSHub_Broadcast_NonBlockingWhenFull(t *testing.T) {
	t.Parallel()
	hub := NewStatsWSHub()

	// Fill the broadcast channel to capacity (64).
	msg := services.StatsPushMessage{Type: "stats_update"}
	for i := 0; i < 64; i++ {
		hub.broadcast <- msg
	}

	// This call must not block or panic.
	done := make(chan struct{})
	go func() {
		hub.Broadcast(msg)
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(time.Second):
		t.Fatal("Broadcast blocked on a full channel")
	}
}

// TestStatsWSHub_Run_ExitsOnContextCancel verifies Run returns when ctx is cancelled
// with no connected clients.
func TestStatsWSHub_Run_ExitsOnContextCancel(t *testing.T) {
	t.Parallel()
	hub := NewStatsWSHub()
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		hub.Run(ctx)
		close(done)
	}()

	cancel()
	select {
	case <-done:
		// Run exited cleanly.
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not exit after context cancellation")
	}
}

// TestStatsWSHub_BroadcastToConnectedClient verifies that a message broadcast via
// Broadcast is forwarded to a connected WebSocket client.
func TestStatsWSHub_BroadcastToConnectedClient(t *testing.T) {
	t.Parallel()
	hub := NewStatsWSHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	srv := newHubServer(t, hub)
	defer srv.Close()

	conn := dialHub(t, srv)
	t.Cleanup(func() { _ = conn.Close() })

	// Allow the client to register.
	time.Sleep(50 * time.Millisecond)

	summary := services.StatsPushData{
		RequestsLast24h: 42,
		RequestsLast7d:  294,
		RequestsLast30d: 1260,
	}
	hub.Broadcast(services.StatsPushMessage{Type: "stats_update", Data: summary})

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(3*time.Second)))
	_, data, err := conn.ReadMessage()
	require.NoError(t, err)

	var got services.StatsPushMessage
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, "stats_update", got.Type)
	assert.Equal(t, int64(42), got.Data.RequestsLast24h)
}

// TestStatsWSHub_Run_ClosesClientsOnContextCancel verifies that connected clients
// receive a close message when the hub's context is cancelled.
func TestStatsWSHub_Run_ClosesClientsOnContextCancel(t *testing.T) {
	t.Parallel()
	hub := NewStatsWSHub()
	ctx, cancel := context.WithCancel(context.Background())

	go hub.Run(ctx)

	srv := newHubServer(t, hub)
	defer srv.Close()

	conn := dialHub(t, srv)
	t.Cleanup(func() { _ = conn.Close() })

	// Allow the client to register.
	time.Sleep(50 * time.Millisecond)

	// Cancel hub — clients should be closed.
	cancel()

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(3*time.Second)))
	_, _, err := conn.ReadMessage()
	// Expect an error (close frame or EOF) after hub cancellation.
	assert.Error(t, err, "expected connection to be closed after hub cancel")
}

// TestStatsWSHub_Run_SlowClientDropsMessage verifies that Broadcast does not deadlock
// when a client's send channel is full.
func TestStatsWSHub_Run_SlowClientDropsMessage(t *testing.T) {
	t.Parallel()
	hub := NewStatsWSHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	srv := newHubServer(t, hub)
	defer srv.Close()

	conn := dialHub(t, srv)
	t.Cleanup(func() { _ = conn.Close() })

	// Allow the client to register.
	time.Sleep(50 * time.Millisecond)

	// Flood the hub with more messages than the client buffer (64). Do NOT read from conn.
	done := make(chan struct{})
	go func() {
		for i := 0; i < 128; i++ {
			hub.Broadcast(services.StatsPushMessage{Type: "stats_update"})
		}
		close(done)
	}()

	select {
	case <-done:
		// No deadlock — slow client messages were dropped gracefully.
	case <-time.After(5 * time.Second):
		t.Fatal("Broadcast deadlocked with a slow client")
	}
}

// TestStatsWSHub_StatsWSHandler_NonWebSocketRequest verifies that StatsWS returns
// gracefully when the WebSocket upgrade fails (plain HTTP request).
func TestStatsWSHub_StatsWSHandler_NonWebSocketRequest(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.RequestLog{}, &models.ProxyHost{}, &models.SSLCertificate{}))
	svc := services.NewStatsService(db)
	h := NewStatsHandler(svc)

	r := gin.New()
	r.GET("/ws", h.StatsWS)
	srv := httptest.NewServer(r)
	defer srv.Close()

	// Plain HTTP GET — no WebSocket upgrade headers — upgrader.Upgrade will fail.
	resp, err := http.Get(srv.URL + "/ws") //nolint:noctx
	require.NoError(t, err)
	_ = resp.Body.Close()
	// The handler returns after the upgrade error — any HTTP response is acceptable.
}

// TestStatsWSHub_StatsWSHandler_NilHub verifies that StatsWS closes the connection
// immediately when no hub is wired (h.hub == nil).
func TestStatsWSHub_StatsWSHandler_NilHub(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.RequestLog{}, &models.ProxyHost{}, &models.SSLCertificate{}))
	svc := services.NewStatsService(db)
	// NewStatsHandler leaves hub as nil.
	h := NewStatsHandler(svc)

	r := gin.New()
	r.GET("/ws", h.StatsWS)
	srv := httptest.NewServer(r)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil) //nolint:bodyclose
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	// Hub is nil — handler closes the conn immediately after upgrade.
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(3*time.Second)))
	_, _, err = conn.ReadMessage()
	assert.Error(t, err, "expected immediate connection close when hub is nil")
}

// TestStatsWSHub_UnregisterRemovesClient verifies unregistering a client removes it
// from the hub and the client's send channel is closed (preventing goroutine leak).
func TestStatsWSHub_UnregisterRemovesClient(t *testing.T) {
	t.Parallel()
	hub := NewStatsWSHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	srv := newHubServer(t, hub)
	defer srv.Close()

	conn := dialHub(t, srv)

	// Allow registration.
	time.Sleep(50 * time.Millisecond)

	hub.mu.RLock()
	clientsBefore := len(hub.clients)
	hub.mu.RUnlock()
	assert.Equal(t, 1, clientsBefore)

	// Close the client-side connection — ServeClient's read loop will detect the close
	// and send to hub.unregister.
	_ = conn.Close()

	require.Eventually(t, func() bool {
		hub.mu.RLock()
		n := len(hub.clients)
		hub.mu.RUnlock()
		return n == 0
	}, 3*time.Second, 25*time.Millisecond, "expected client to be unregistered after disconnect")
}
