package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newUpgradeTestServer serves upgradeWebSocket and closes each accepted
// connection once the test ends. It reports the upgrade error through upgradeErr.
func newUpgradeTestServer(t *testing.T, upgradeErr chan<- error) *httptest.Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/ws", func(c *gin.Context) {
		conn, err := upgradeWebSocket(c)
		upgradeErr <- err
		if err != nil {
			return
		}
		defer func() { _ = conn.CloseNow() }()
		_, _, _ = conn.Read(c.Request.Context())
	})
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return srv
}

func TestUpgradeWebSocket_RejectsDisallowedOrigin(t *testing.T) {
	upgradeErr := make(chan error, 1)
	srv := newUpgradeTestServer(t, upgradeErr)

	header := http.Header{"Origin": []string{"http://evil.example"}}
	conn, resp, err := websocket.Dial(context.Background(), toWebSocketURL(srv.URL)+"/ws", &websocket.DialOptions{HTTPHeader: header}) //nolint:bodyclose // coder/websocket manages the handshake response body
	if conn != nil {
		_ = conn.CloseNow()
	}
	require.Error(t, err, "a cross-origin upgrade must not succeed")
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)

	select {
	case got := <-upgradeErr:
		assert.True(t, errors.Is(got, errWebSocketOriginDenied), "got %v", got)
	case <-time.After(5 * time.Second):
		t.Fatal("handler did not run")
	}
}

func TestUpgradeWebSocket_AcceptsSameOriginRequest(t *testing.T) {
	upgradeErr := make(chan error, 1)
	srv := newUpgradeTestServer(t, upgradeErr)

	// The Origin host matches the request host, so the upgrade is allowed.
	header := http.Header{"Origin": []string{srv.URL}}
	conn, _, err := websocket.Dial(context.Background(), toWebSocketURL(srv.URL)+"/ws", &websocket.DialOptions{HTTPHeader: header}) //nolint:bodyclose // coder/websocket manages the handshake response body
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.CloseNow() })

	select {
	case got := <-upgradeErr:
		require.NoError(t, got)
	case <-time.After(5 * time.Second):
		t.Fatal("handler did not run")
	}
}

func TestUpgradeWebSocket_NonWebSocketRequestFails(t *testing.T) {
	upgradeErr := make(chan error, 1)
	srv := newUpgradeTestServer(t, upgradeErr)

	// A plain GET without upgrade headers passes the origin check but fails the handshake.
	resp, err := http.Get(srv.URL + "/ws") //nolint:noctx // test-only request against httptest server
	require.NoError(t, err)
	_ = resp.Body.Close()

	select {
	case got := <-upgradeErr:
		require.Error(t, got)
		assert.False(t, errors.Is(got, errWebSocketOriginDenied))
	case <-time.After(5 * time.Second):
		t.Fatal("handler did not run")
	}
}
