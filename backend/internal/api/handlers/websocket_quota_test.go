package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/security"
	"github.com/Wikid82/charon/backend/internal/services"
)

// newQuotaServer serves a stream handler that holds a quota slot for as long as serve runs. The slot is released by a
// deferred release, as the real stream handlers do.
func newQuotaServer(t *testing.T, quota *services.StreamQuota, serve func(c *gin.Context, conn *websocket.Conn)) *httptest.Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/stream", func(c *gin.Context) {
		conn, release, err := upgradeWebSocket(c, nil, security.TrustedProxyMatcher{}, quota)
		if err != nil {
			return
		}
		defer release()
		defer func() { _ = conn.CloseNow() }()
		serve(c, conn)
	})
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return srv
}

// blockingRead holds the stream open until the client goes away.
func blockingRead(c *gin.Context, conn *websocket.Conn) {
	_, _, _ = conn.Read(c.Request.Context())
}

func dialStream(t *testing.T, srv *httptest.Server) (*websocket.Conn, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	conn, resp, err := websocket.Dial(ctx, toWebSocketURL(srv.URL)+"/stream", nil) //nolint:bodyclose // coder/websocket manages the handshake response body
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	return conn, err
}

func TestUpgradeWebSocket_OverQuotaClosesWithTryAgainLater(t *testing.T) {
	quota := services.NewStreamQuota(1, 10)
	srv := newQuotaServer(t, quota, blockingRead)

	first, err := dialStream(t, srv)
	require.NoError(t, err)
	t.Cleanup(func() { _ = first.CloseNow() })

	// The second stream from the same caller is accepted, then closed with 1013 and the stream_limit_reached reason.
	second, err := dialStream(t, srv)
	require.NoError(t, err, "the upgrade is accepted so the browser can read the close code")
	t.Cleanup(func() { _ = second.CloseNow() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _, readErr := second.Read(ctx)
	require.Error(t, readErr)
	assert.Equal(t, websocket.StatusTryAgainLater, websocket.CloseStatus(readErr))
	assert.Contains(t, readErr.Error(), streamLimitReason)
}

func TestUpgradeWebSocket_SlotReleasedWhenStreamEnds(t *testing.T) {
	quota := services.NewStreamQuota(1, 1)
	srv := newQuotaServer(t, quota, blockingRead)

	first, err := dialStream(t, srv)
	require.NoError(t, err)
	require.NoError(t, first.Close(websocket.StatusNormalClosure, ""))

	require.Eventually(t, func() bool {
		release, ok := quota.Acquire("addr:127.0.0.1")
		if ok {
			release()
		}
		return ok
	}, 5*time.Second, 20*time.Millisecond, "the slot must be free again once the stream ended")
}

// TestUpgradeWebSocket_SlotReleasedOnPanic: a handler that panics after taking its slot still gives it back, because
// the release is deferred.
func TestUpgradeWebSocket_SlotReleasedOnPanic(t *testing.T) {
	quota := services.NewStreamQuota(1, 1)
	srv := newQuotaServer(t, quota, func(*gin.Context, *websocket.Conn) {
		panic("stream handler failure")
	})

	_, _ = dialStream(t, srv)

	require.Eventually(t, func() bool {
		release, ok := quota.Acquire("addr:127.0.0.1")
		if ok {
			release()
		}
		return ok
	}, 5*time.Second, 20*time.Millisecond, "a panicking stream must not leak its slot")
}

func TestStreamQuotaKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	authed, _ := gin.CreateTestContext(httptest.NewRecorder())
	authed.Request = httptest.NewRequest(http.MethodGet, "/stream", http.NoBody)
	middleware.SetCaller(authed, 42, "admin")
	assert.Equal(t, "user:42", streamQuotaKey(authed))

	anonymous, _ := gin.CreateTestContext(httptest.NewRecorder())
	anonymous.Request = httptest.NewRequest(http.MethodGet, "/stream", http.NoBody)
	anonymous.Request.RemoteAddr = "192.0.2.9:4000"
	assert.Equal(t, "addr:192.0.2.9", streamQuotaKey(anonymous))
}

func TestWebSocketHandlers_WithStreamQuota(t *testing.T) {
	q := services.NewStreamQuota(1, 1)
	assert.Equal(t, q, NewLogsWSHandler(nil).WithStreamQuota(q).quota)
	assert.Equal(t, q, NewCerberusLogsHandler(nil, nil).WithStreamQuota(q).quota)
	assert.Equal(t, q, NewHecateWSHandler(nil, nil).WithStreamQuota(q).quota)
	assert.Equal(t, q, NewStatsHandler(nil).WithStreamQuota(q).quota)
}
