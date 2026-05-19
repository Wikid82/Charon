package orthrus

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	gorillaws "github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/Wikid82/charon/backend/internal/models"
)

// S1: Full WS handshake wires proxy; GetProxyAddr returns non-empty address.
func TestOrthrusServer_HandleWebSocket_StartsDockerProxy(t *testing.T) {
	db := setupServerTestDB(t)
	srv, err := NewOrthrusServer(db, setupTestCA(t))
	require.NoError(t, err)
	srv.heartbeatTimeout = 50 * time.Millisecond

	token := "ch_orthrus_proxy_s1" //nolint:gosec // G101: test credential
	hash, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.MinCost)
	require.NoError(t, err)

	agent := &models.OrthrusAgent{
		UUID:        "proxy-s1-uuid",
		Name:        "proxy-s1-agent",
		AuthKeyHash: string(hash),
		Status:      models.OrthrusStatusPending,
	}
	require.NoError(t, db.Create(agent).Error)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/ws", srv.HandleWebSocket)

	ts := httptest.NewServer(router)
	t.Cleanup(srv.Stop) // runs 2nd — must run before DB cleanup
	t.Cleanup(ts.Close) // runs 1st — drains HTTP handlers so wg.Add races are gone

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	header := http.Header{"Authorization": []string{"Bearer " + token}}

	conn, dialResp, err := gorillaws.DefaultDialer.Dial(wsURL, header)
	require.NoError(t, err)
	if dialResp != nil {
		_ = dialResp.Body.Close()
	}
	defer func() { _ = conn.Close() }()

	assert.Eventually(t, func() bool {
		addr, ok := srv.GetProxyAddr("proxy-s1-uuid")
		return ok && addr != ""
	}, 2*time.Second, 20*time.Millisecond, "proxy addr should be set after WS upgrade")
}

// S2: watchHeartbeat closes session; proxy listener stops accepting connections.
func TestOrthrusServer_WatchHeartbeat_ClosesProxyListener(t *testing.T) {
	db := setupServerTestDB(t)
	srv, err := NewOrthrusServer(db, setupTestCA(t))
	require.NoError(t, err)
	t.Cleanup(srv.Stop)
	srv.heartbeatTimeout = 40 * time.Millisecond

	serverConn, done := testWSPair(t)
	defer done()

	sess, err := NewAgentSession("proxy-s2-uuid", "proxy-s2-agent", serverConn)
	require.NoError(t, err)

	require.NoError(t, sess.StartDockerProxy())
	proxyAddr := sess.GetProxyAddr()
	require.NotEmpty(t, proxyAddr)

	// Verify proxy is reachable before watchHeartbeat runs.
	c, err := net.DialTimeout("tcp", proxyAddr, 500*time.Millisecond)
	require.NoError(t, err)
	_ = c.Close()

	agent := &models.OrthrusAgent{
		UUID:   "proxy-s2-uuid",
		Name:   "proxy-s2-agent",
		Status: models.OrthrusStatusOnline,
	}
	require.NoError(t, db.Create(agent).Error)

	// Kill yamux session directly (same package access) without closing listener.
	// watchHeartbeat calls sess.Close() when IsAlive() returns false.
	require.NoError(t, sess.session.Close())

	srv.sessions.Store("proxy-s2-uuid", sess)

	finished := make(chan struct{})
	go func() {
		srv.watchHeartbeat("proxy-s2-uuid", sess)
		close(finished)
	}()

	select {
	case <-finished:
	case <-time.After(600 * time.Millisecond):
		t.Fatal("watchHeartbeat did not exit within deadline")
	}

	// Proxy listener must be closed now.
	_, dialErr := net.DialTimeout("tcp", proxyAddr, 500*time.Millisecond)
	assert.Error(t, dialErr, "proxy listener should be closed after watchHeartbeat")
}
